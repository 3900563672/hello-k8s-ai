package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	"github.com/3900563672/hello-k8s-ai/internal/k8sutil"
	"github.com/3900563672/hello-k8s-ai/internal/observability"

	"go.opentelemetry.io/otel/attribute"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	simInstanceTenantIndex            = "simulatorinstance.spec.tenantRef.name"
	simInstanceModelIndex             = "simulatorinstance.spec.modelRef.name"
	simInstanceFinalizer              = "platform.study.com/simulator-instance-controller"
	defaultNamespace                  = "default"
	defaultSimulatorImage             = "simulator:latest"
	defaultSimulatorSA                = "simulator-sa"
	defaultSimulatorMetricsPort int32 = 9090
	deploymentDeleteWait              = 2 * time.Second
)

// SimulatorObservabilityConfig 会注入到动态创建的 Simulator Pod 的环境变量里。
// 字段为空时 tracing 不启用，但 Prometheus 端点仍然在每个 Pod 上可用。
type SimulatorObservabilityConfig struct {
	SDKDisabled      string
	OTLPEndpoint     string
	OTLPInsecure     string
	TracesSampler    string
	TracesSamplerArg string
	Environment      string
	ClusterName      string
	ServiceVersion   string
}

// SimulatorInstanceReconciler 管 Deployment 的创建和更新，以及 SimulatorInstance 的 Phase 和 Conditions。
// Score/Performance 是模拟器进程自己写的，控制器不碰，避免两边互相覆盖。
type SimulatorInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	SimulatorNamespace       string
	SimulatorImage           string
	SimulatorServiceAccount  string
	SimulatorImagePullPolicy corev1.PullPolicy
	SimulatorMetricsPort     int32
	SimulatorObservability   SimulatorObservabilityConfig
}

// +kubebuilder:rbac:groups=platform.study.com,resources=simulatorinstances,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=platform.study.com,resources=simulatorinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.study.com,resources=tenantruntimes,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=platform.study.com,resources=tenantruntimes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.study.com,resources=tenants,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=tenantnodepolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=modelnodepolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *SimulatorInstanceReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (result ctrl.Result, reconcileErr error) {
	ctx, observation := beginReconcile(ctx, "simulator-instance", req)
	defer func() { observation.finish(result, reconcileErr) }()

	logger := log.FromContext(ctx).WithValues("simulatorInstance", req.Name)

	var instance platformv1.SimulatorInstance
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get simulator instance %q: %w", req.Name, err)
	}
	observation.span.SetAttributes(
		attribute.String(traceAttributeTenantName, instance.Spec.TenantRef.Name),
		attribute.String(traceAttributeModelName, instance.Spec.ModelRef.Name),
		attribute.String(traceAttributeSimulatorInstanceName, instance.Name),
		attribute.Int("simulator.desired_replicas", instance.Spec.Replicas),
		attribute.Int64("k8s.resource.generation", instance.Generation),
	)

	if !instance.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&instance, simInstanceFinalizer) {
			return ctrl.Result{}, nil
		}
		deleted, err := r.deleteDeploymentObjects(ctx, &instance)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !deleted {
			return ctrl.Result{RequeueAfter: deploymentDeleteWait}, nil
		}
		if err := r.reconcileTenantRuntime(ctx, instance.Spec.TenantRef.Name); err != nil {
			return ctrl.Result{}, err
		}
		if err := removeFinalizer(ctx, r.Client, &instance, simInstanceFinalizer); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&instance, simInstanceFinalizer) {
		if err := addFinalizer(ctx, r.Client, &instance, simInstanceFinalizer); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: immediateRequeueAfter}, nil
	}

	placementCtx, placementSpan := startOperation(ctx, "simulator-instance", "resolve-eligible-nodes")
	eligibleNodes, err := r.eligibleNodesForInstance(placementCtx, &instance)
	observability.EndSpan(placementSpan, err, attribute.Int("placement.eligible_node_count", len(eligibleNodes)))
	observeOperation("simulator-instance", "resolve-eligible-nodes", err)
	if err != nil {
		_ = r.updateInstanceDeploymentStatus(ctx, instance.Name, nil, 0, err)
		return ctrl.Result{}, err
	}
	deploymentCtx, deploymentSpan := startOperation(
		ctx,
		"simulator-instance",
		"reconcile-deployments",
		attribute.Int("placement.eligible_node_count", len(eligibleNodes)),
	)
	deploymentState, err := r.reconcileDeploymentObjects(deploymentCtx, &instance, eligibleNodes)
	observability.EndSpan(deploymentSpan, err)
	observeOperation("simulator-instance", "reconcile-deployments", err)
	if err != nil {
		_ = r.updateInstanceDeploymentStatus(ctx, instance.Name, nil, len(eligibleNodes), err)
		return ctrl.Result{}, err
	}
	if err := r.updateInstanceDeploymentStatus(ctx, instance.Name, deploymentState, len(eligibleNodes), nil); err != nil {
		return ctrl.Result{}, err
	}
	runtimeCtx, runtimeSpan := startOperation(ctx, "simulator-instance", "update-tenant-runtime")
	err = r.reconcileTenantRuntime(runtimeCtx, instance.Spec.TenantRef.Name)
	observability.EndSpan(runtimeSpan, err)
	observeOperation("simulator-instance", "update-tenant-runtime", err)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.V(1).Info(
		"simulator deployments reconciled",
		"deploymentCount",
		len(deploymentState.Deployments),
		"desiredReplicas",
		deploymentState.DesiredReplicas,
	)
	return ctrl.Result{}, nil
}

func (r *SimulatorInstanceReconciler) ensurePlacementDeployment(
	ctx context.Context,
	instance *platformv1.SimulatorInstance,
	name string,
	replicaCount int,
	targetNodes []string,
	placementNode string,
) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.simulatorNamespace(),
		},
	}
	replicas := int32(replicaCount)
	if replicas < 0 {
		return nil, fmt.Errorf("simulator instance %q has invalid placement replicas %d", instance.Name, replicas)
	}

	_, err := controllerutil.CreateOrPatch(ctx, r.Client, deployment, func() error {
		labels := ensureStringMap(&deployment.Labels)
		annotations := ensureStringMap(&deployment.Annotations)
		setWorkloadIdentity(labels, annotations, instance)
		setPlacementIdentity(labels, annotations, placementNode)

		selectorValue := labelValue(instance.Name)
		selectorLabels := map[string]string{instanceLabelKey: selectorValue}
		if deployment.Name != deploymentName(instance.Name) && placementNode != "" {
			selectorLabels[placementLabelKey] = labelValue(placementNode)
		}
		if deployment.CreationTimestamp.IsZero() {
			deployment.Spec.Selector = &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			}
		}
		if deployment.Spec.Selector == nil {
			return fmt.Errorf("deployment %s/%s has an incompatible immutable selector", deployment.Namespace, deployment.Name)
		}
		for key, value := range selectorLabels {
			if deployment.Spec.Selector.MatchLabels[key] != value {
				return fmt.Errorf("deployment %s/%s has an incompatible immutable selector", deployment.Namespace, deployment.Name)
			}
		}

		deployment.Spec.Replicas = new(replicas)
		deployment.Spec.Strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
		deployment.Spec.Template.Labels = copyStringMap(deployment.Spec.Template.Labels)
		deployment.Spec.Template.Annotations = copyStringMap(deployment.Spec.Template.Annotations)
		setWorkloadIdentity(deployment.Spec.Template.Labels, deployment.Spec.Template.Annotations, instance)
		setPlacementIdentity(deployment.Spec.Template.Labels, deployment.Spec.Template.Annotations, placementNode)
		maps.Copy(deployment.Spec.Template.Labels, deployment.Spec.Selector.MatchLabels)

		podSpec := &deployment.Spec.Template.Spec
		podSpec.ServiceAccountName = r.simulatorServiceAccount()
		podSpec.TerminationGracePeriodSeconds = new(int64(15))
		if podSpec.SecurityContext == nil {
			podSpec.SecurityContext = &corev1.PodSecurityContext{}
		}
		podSpec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		}
		setRequiredNodeAffinity(podSpec, targetNodes)
		upsertSimulatorContainer(
			podSpec,
			r.simulatorImage(),
			r.simulatorPullPolicy(),
			instance,
			r.simulatorMetricsPort(),
			r.SimulatorObservability,
		)

		if err := controllerutil.SetControllerReference(instance, deployment, r.Scheme); err != nil {
			return fmt.Errorf("set simulator instance owner: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ensure deployment %s/%s: %w", deployment.Namespace, deployment.Name, err)
	}
	return deployment, nil
}

func setPlacementIdentity(labels, annotations map[string]string, nodeName string) {
	if nodeName == "" {
		delete(labels, placementLabelKey)
		delete(annotations, placementNodeAnnotation)
		return
	}
	labels[placementLabelKey] = labelValue(nodeName)
	annotations[placementNodeAnnotation] = nodeName
}

func setWorkloadIdentity(labels, annotations map[string]string, instance *platformv1.SimulatorInstance) {
	setIdentityMetadata(
		labels,
		annotations,
		instance.Name,
		instance.Spec.TenantRef.Name,
		instance.Spec.ModelRef.Name,
	)
}

func copyStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source)+4)
	maps.Copy(result, source)
	return result
}

func setRequiredNodeAffinity(podSpec *corev1.PodSpec, eligibleNodes []string) {
	nodes := append([]string(nil), eligibleNodes...)
	slices.Sort(nodes)

	if len(nodes) == 0 {
		nodes = []string{"no-eligible-worker-node"}
	}

	if podSpec.Affinity == nil {
		podSpec.Affinity = &corev1.Affinity{}
	}

	podSpec.Affinity.NodeAffinity =
		&corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/hostname",
								Operator: corev1.NodeSelectorOpIn,
								Values:   nodes,
							},
						},
					},
				},
			},
		}
}

func upsertSimulatorContainer(
	podSpec *corev1.PodSpec,
	image string,
	pullPolicy corev1.PullPolicy,
	instance *platformv1.SimulatorInstance,
	metricsPort int32,
	telemetry SimulatorObservabilityConfig,
) {
	index := -1
	for i := range podSpec.Containers {
		if podSpec.Containers[i].Name == "simulator" {
			index = i
			break
		}
	}
	if index < 0 {
		podSpec.Containers = append(podSpec.Containers, corev1.Container{Name: "simulator"})
		index = len(podSpec.Containers) - 1
	}
	container := &podSpec.Containers[index]
	container.Image = image
	container.ImagePullPolicy = pullPolicy
	upsertEnv(&container.Env, corev1.EnvVar{Name: "SIMULATOR_INSTANCE_NAME", Value: instance.Name})
	upsertEnv(&container.Env, corev1.EnvVar{Name: "TENANT_NAME", Value: instance.Spec.TenantRef.Name})
	upsertEnv(&container.Env, corev1.EnvVar{Name: "MODEL_NAME", Value: instance.Spec.ModelRef.Name})
	upsertEnv(&container.Env, corev1.EnvVar{
		Name: "POD_NAME",
		ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: "metadata.name",
		}},
	})
	upsertEnv(&container.Env, corev1.EnvVar{
		Name: "POD_NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: "metadata.namespace",
		}},
	})
	upsertEnv(&container.Env, corev1.EnvVar{
		Name: "NODE_NAME",
		ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{
			FieldPath: "spec.nodeName",
		}},
	})
	upsertSimulatorTelemetryEnv(&container.Env, telemetry)
	upsertContainerPort(&container.Ports, corev1.ContainerPort{
		Name:          "metrics",
		ContainerPort: metricsPort,
		Protocol:      corev1.ProtocolTCP,
	})
	container.LivenessProbe = httpProbe("/healthz", "metrics", 2)
	container.ReadinessProbe = httpProbe("/readyz", "metrics", 1)
	container.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: new(false),
		ReadOnlyRootFilesystem:   new(true),
		RunAsNonRoot:             new(true),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

func upsertSimulatorTelemetryEnv(environment *[]corev1.EnvVar, config SimulatorObservabilityConfig) {
	values := []corev1.EnvVar{
		{Name: "OTEL_SDK_DISABLED", Value: config.SDKDisabled},
		{Name: "APP_VERSION", Value: config.ServiceVersion},
		{Name: "DEPLOYMENT_ENVIRONMENT", Value: config.Environment},
		{Name: "K8S_CLUSTER_NAME", Value: config.ClusterName},
		{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: config.OTLPEndpoint},
		{Name: "OTEL_EXPORTER_OTLP_INSECURE", Value: config.OTLPInsecure},
		{Name: "OTEL_SERVICE_NAME", Value: "hello-k8s-ai-simulator"},
		{Name: "OTEL_TRACES_SAMPLER", Value: config.TracesSampler},
		{Name: "OTEL_TRACES_SAMPLER_ARG", Value: config.TracesSamplerArg},
	}
	for _, value := range values {
		// 空值也写进去，这样管理器可以通过配置变化清除之前注入的 OTLP 端点，
		// 下次滚动更新时就能确定性地关闭 tracing。
		upsertEnv(environment, value)
	}
}

func upsertContainerPort(ports *[]corev1.ContainerPort, desired corev1.ContainerPort) {
	for i := range *ports {
		if (*ports)[i].Name == desired.Name {
			(*ports)[i] = desired
			return
		}
	}
	*ports = append(*ports, desired)
}

func httpProbe(path, portName string, initialDelaySeconds int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
			Path: path,
			Port: intstr.FromString(portName),
		}},
		InitialDelaySeconds: initialDelaySeconds,
		PeriodSeconds:       10,
		TimeoutSeconds:      2,
		FailureThreshold:    3,
	}
}

func upsertEnv(environment *[]corev1.EnvVar, desired corev1.EnvVar) {
	for i := range *environment {
		if (*environment)[i].Name == desired.Name {
			(*environment)[i] = desired
			return
		}
	}
	*environment = append(*environment, desired)
}

func (r *SimulatorInstanceReconciler) eligibleNodesForInstance(
	ctx context.Context,
	instance *platformv1.SimulatorInstance,
) ([]string, error) {
	var tenantPolicies platformv1.TenantNodePolicyList
	if err := r.List(ctx, &tenantPolicies); err != nil {
		return nil, fmt.Errorf("list tenant node policies: %w", err)
	}
	var modelPolicies platformv1.ModelNodePolicyList
	if err := r.List(ctx, &modelPolicies); err != nil {
		return nil, fmt.Errorf("list model node policies: %w", err)
	}
	return eligibleNodeNames(
		instance.Spec.TenantRef.Name,
		instance.Spec.ModelRef.Name,
		tenantPolicies.Items,
		modelPolicies.Items,
	), nil
}

func (r *SimulatorInstanceReconciler) updateInstanceDeploymentStatus(
	ctx context.Context,
	instanceName string,
	deploymentState *simulatorDeploymentState,
	eligibleNodeCount int,
	reconcileErr error,
) error {
	return k8sutil.PatchStatusWithRetry(ctx, r.Client, instanceName, false,
		func() *platformv1.SimulatorInstance { return &platformv1.SimulatorInstance{} },
		func(instance *platformv1.SimulatorInstance) error {
			condition := metav1.Condition{Type: conditionTypeReady, ObservedGeneration: instance.Generation}
			instance.Status.AvailableReplicas = 0
			if deploymentState != nil {
				instance.Status.AvailableReplicas = deploymentState.AvailableReplicas
			}

			switch {
			case reconcileErr != nil:
				instance.Status.Phase = phaseFailed
				condition.Status = metav1.ConditionFalse
				condition.Reason = "DeploymentReconcileFailed"
				condition.Message = reconcileErr.Error()
			case instance.Spec.Replicas == 0:
				instance.Status.Phase = phaseRunning
				condition.Status = metav1.ConditionTrue
				condition.Reason = "ScaledToZero"
				condition.Message = "the simulator instance is intentionally scaled to zero"
			case eligibleNodeCount == 0:
				instance.Status.Phase = phasePending
				condition.Status = metav1.ConditionFalse
				condition.Reason = "NoEligibleNodes"
				condition.Message = "no node is allowed by the current tenant/model node policies"
			case deploymentState != nil && deploymentState.Failed:
				instance.Status.Phase = phaseFailed
				condition.Status = metav1.ConditionFalse
				condition.Reason = "DeploymentFailed"
				condition.Message = "the simulator Deployment reports ReplicaFailure"
			case deploymentState != nil && deploymentState.AvailableReplicas >= instance.Spec.Replicas:
				instance.Status.Phase = phaseRunning
				condition.Status = metav1.ConditionTrue
				condition.Reason = "DeploymentAvailable"
				condition.Message = "all desired simulator replicas are available"
			default:
				instance.Status.Phase = phasePending
				condition.Status = metav1.ConditionFalse
				condition.Reason = "DeploymentProgressing"
				condition.Message = "waiting for simulator replicas to become available"
			}
			meta.SetStatusCondition(&instance.Status.Conditions, condition)
			return nil
		})
}

func deploymentFailed(deployment *appsv1.Deployment) bool {
	if deployment == nil {
		return false
	}
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// reconcileTenantRuntime 统计所有实例的可用副本数（不只是期望副本数），给 TenantRuntime 设置 Phase。
func (r *SimulatorInstanceReconciler) reconcileTenantRuntime(ctx context.Context, tenantName string) error {
	var tenant platformv1.Tenant
	if err := r.Get(ctx, client.ObjectKey{Name: tenantName}, &tenant); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	var instances platformv1.SimulatorInstanceList
	if err := r.List(ctx, &instances, client.MatchingFields{simInstanceTenantIndex: tenantName}); err != nil {
		return fmt.Errorf("list simulator instances for tenant runtime %q: %w", tenantName, err)
	}

	readyReplicas := int32(0)
	desiredReplicas := int32(0)
	failed := false
	for i := range instances.Items {
		instance := &instances.Items[i]
		if !instance.DeletionTimestamp.IsZero() {
			continue
		}
		desiredReplicas += int32(instance.Spec.Replicas)
		plan, persisted, planErr := decodeNodePlacementPlan(instance.Annotations[nodePlacementsAnnotation])
		if planErr != nil || (persisted && nodePlacementReplicaCount(plan) != instance.Spec.Replicas) {
			failed = true
			continue
		}
		desiredNames, planErr := deploymentNamesForPlacementPlan(instance.Name, plan, persisted)
		if planErr != nil {
			failed = true
			continue
		}
		deployments, err := r.listInstanceDeployments(ctx, instance.Name)
		if err != nil {
			return err
		}
		for j := range deployments {
			deployment := &deployments[j]
			if _, desired := desiredNames[deployment.Name]; !desired {
				continue
			}
			readyReplicas += deployment.Status.AvailableReplicas
			failed = failed || deploymentFailed(deployment)
		}
	}

	runtimeObject := &platformv1.TenantRuntime{ObjectMeta: metav1.ObjectMeta{Name: tenantName}}
	if _, err := controllerutil.CreateOrPatch(ctx, r.Client, runtimeObject, func() error {
		runtimeObject.Spec.TenantRef = platformv1.TenantReference{Name: tenantName}
		return controllerutil.SetControllerReference(&tenant, runtimeObject, r.Scheme)
	}); err != nil {
		return fmt.Errorf("ensure tenant runtime %q: %w", tenantName, err)
	}

	return r.updateTenantRuntimeStatus(ctx, tenantName, int(readyReplicas), int(desiredReplicas), failed)
}

func (r *SimulatorInstanceReconciler) updateTenantRuntimeStatus(
	ctx context.Context,
	tenantName string,
	ready int,
	desired int,
	failed bool,
) error {
	return k8sutil.PatchStatusWithRetry(ctx, r.Client, tenantName, false,
		func() *platformv1.TenantRuntime { return &platformv1.TenantRuntime{} },
		func(runtimeObject *platformv1.TenantRuntime) error {
			runtimeObject.Status.InstanceCount = nonNegative(ready)
			condition := metav1.Condition{Type: conditionTypeReady, ObservedGeneration: runtimeObject.Generation}
			switch {
			case failed:
				runtimeObject.Status.Phase = phaseFailed
				condition.Status = metav1.ConditionFalse
				condition.Reason = "DeploymentFailed"
				condition.Message = "at least one simulator Deployment reports a failure"
			case desired == 0:
				runtimeObject.Status.Phase = phaseRunning
				condition.Status = metav1.ConditionTrue
				condition.Reason = "ScaledToZero"
				condition.Message = "the tenant is ready with zero desired simulator replicas"
			case ready >= desired:
				runtimeObject.Status.Phase = phaseRunning
				condition.Status = metav1.ConditionTrue
				condition.Reason = "AllReplicasAvailable"
				condition.Message = "all desired simulator replicas are available"
			default:
				runtimeObject.Status.Phase = phasePending
				condition.Status = metav1.ConditionFalse
				condition.Reason = "ReplicasProgressing"
				condition.Message = fmt.Sprintf("%d of %d desired simulator replicas are available", ready, desired)
			}
			meta.SetStatusCondition(&runtimeObject.Status.Conditions, condition)
			return nil
		})
}

func deploymentName(instanceName string) string {
	const (
		prefix        = "simulator-"
		maxNameLength = 253
	)
	name := prefix + instanceName
	if len(name) <= maxNameLength {
		return name
	}
	// 实例名太长时用哈希截断，保证不超过 K8s 的 253 字符限制
	sum := sha256.Sum256([]byte(instanceName))
	suffix := hex.EncodeToString(sum[:8])
	trimmed := strings.TrimRight(instanceName[:maxNameLength-len(prefix)-len(suffix)-1], ".-")
	return prefix + trimmed + "-" + suffix
}

func (r *SimulatorInstanceReconciler) simulatorNamespace() string {
	return valueOrDefault(r.SimulatorNamespace, defaultNamespace)
}

func (r *SimulatorInstanceReconciler) simulatorImage() string {
	return valueOrDefault(r.SimulatorImage, defaultSimulatorImage)
}

func (r *SimulatorInstanceReconciler) simulatorServiceAccount() string {
	return valueOrDefault(r.SimulatorServiceAccount, defaultSimulatorSA)
}

func (r *SimulatorInstanceReconciler) simulatorPullPolicy() corev1.PullPolicy {
	return valueOrDefault(r.SimulatorImagePullPolicy, corev1.PullIfNotPresent)
}

func (r *SimulatorInstanceReconciler) simulatorMetricsPort() int32 {
	if r.SimulatorMetricsPort > 0 {
		return r.SimulatorMetricsPort
	}
	return defaultSimulatorMetricsPort
}

func (r *SimulatorInstanceReconciler) mapTenantNodePolicyToInstances(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapNodePolicyToInstances(ctx, obj)
}

func (r *SimulatorInstanceReconciler) mapModelNodePolicyToInstances(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapNodePolicyToInstances(ctx, obj)
}

func (r *SimulatorInstanceReconciler) mapNodePolicyToInstances(ctx context.Context, obj client.Object) []reconcile.Request {
	switch policy := obj.(type) {
	case *platformv1.TenantNodePolicy:
		return r.instanceRequests(ctx, simInstanceTenantIndex, policy.Spec.TenantRef.Name)
	case *platformv1.ModelNodePolicy:
		return r.instanceRequests(ctx, simInstanceModelIndex, policy.Spec.ModelRef.Name)
	default:
		return nil
	}
}

func (r *SimulatorInstanceReconciler) instanceRequests(ctx context.Context, field, value string) []reconcile.Request {
	if value == "" {
		return nil
	}
	var instances platformv1.SimulatorInstanceList
	if err := r.List(ctx, &instances, client.MatchingFields{field: value}); err != nil {
		log.FromContext(ctx).Error(err, "list SimulatorInstances while mapping policy event", "field", field, "value", value)
		return nil
	}
	requests := make([]reconcile.Request, 0, len(instances.Items))
	for i := range instances.Items {
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Name: instances.Items[i].Name}})
	}
	return requests
}

func (r *SimulatorInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := registerSimulatorInstanceIndexes(
		context.Background(),
		mgr,
		"simulator instance",
		simInstanceTenantIndex,
		simInstanceModelIndex,
	); err != nil {
		return err
	}

	instanceChanged := lifecyclePredicate(func(e event.UpdateEvent) bool {
		oldInstance, oldOK := e.ObjectOld.(*platformv1.SimulatorInstance)
		newInstance, newOK := e.ObjectNew.(*platformv1.SimulatorInstance)
		return oldOK && newOK &&
			(oldInstance.Spec.Replicas != newInstance.Spec.Replicas ||
				oldInstance.Spec.TenantRef.Name != newInstance.Spec.TenantRef.Name ||
				oldInstance.Spec.ModelRef.Name != newInstance.Spec.ModelRef.Name ||
				oldInstance.Annotations[nodePlacementsAnnotation] != newInstance.Annotations[nodePlacementsAnnotation] ||
				!oldInstance.DeletionTimestamp.Equal(newInstance.DeletionTimestamp))
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("simulatorinstance").
		For(&platformv1.SimulatorInstance{}, builder.WithPredicates(instanceChanged)).
		Owns(&appsv1.Deployment{}).
		Watches(
			&platformv1.TenantNodePolicy{},
			handler.EnqueueRequestsFromMapFunc(r.mapTenantNodePolicyToInstances),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&platformv1.ModelNodePolicy{},
			handler.EnqueueRequestsFromMapFunc(r.mapModelNodePolicyToInstances),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}
