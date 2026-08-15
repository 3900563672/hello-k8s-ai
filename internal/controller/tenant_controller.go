package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	"github.com/3900563672/hello-k8s-ai/internal/observability"

	"go.opentelemetry.io/otel/attribute"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	tenantPolicyFinalizer = "platform.study.com/tenant-model-policy"
	dependencyRetryAfter  = 15 * time.Second
)

// TenantModelPolicyReconciler 负责将租户-模型策略落地为 SimulatorInstance。
// 需要显式的 Allow，Deny 优先，重复策略只算一个租户/模型对。
type TenantModelPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=platform.study.com,resources=tenantmodelpolicies,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=platform.study.com,resources=tenantmodelpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.study.com,resources=simulatorinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.study.com,resources=tenants,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=models,verbs=get;list;watch

func (r *TenantModelPolicyReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (result ctrl.Result, reconcileErr error) {
	ctx, observation := beginReconcile(ctx, "tenant-model-policy", req)
	defer func() { observation.finish(result, reconcileErr) }()

	logger := log.FromContext(ctx).WithValues("tenantModelPolicy", req.Name)

	var policy platformv1.TenantModelPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get tenant model policy %q: %w", req.Name, err)
	}

	tenantName := policy.Spec.TenantRef.Name
	modelName := policy.Spec.ModelRef.Name
	observation.span.SetAttributes(
		attribute.String(traceAttributeTenantName, tenantName),
		attribute.String(traceAttributeModelName, modelName),
		attribute.String("policy.effect", policy.Spec.Effect),
		attribute.Int64("k8s.resource.generation", policy.Generation),
	)
	// 租户名或模型名为空直接报错，这种策略没意义
	if tenantName == "" || modelName == "" {
		err := fmt.Errorf("policy must reference both a tenant and a model")
		_ = r.setPolicyReadyCondition(ctx, policy.Name, metav1.ConditionFalse, "InvalidReference", err.Error())
		return ctrl.Result{}, err
	}

	if !policy.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&policy, tenantPolicyFinalizer) {
			// 删之前重新计算一次该租户-模型对，有可能实例也要跟着删
			if _, err := r.reconcileTenantModelPair(ctx, tenantName, modelName); err != nil {
				return ctrl.Result{}, err
			}
			if err := removeFinalizer(ctx, r.Client, &policy, tenantPolicyFinalizer); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&policy, tenantPolicyFinalizer) {
		if err := addFinalizer(ctx, r.Client, &policy, tenantPolicyFinalizer); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: immediateRequeueAfter}, nil
	}

	allowed, err := r.reconcileTenantModelPair(ctx, tenantName, modelName)
	if err != nil {
		// 如果引用资源还没就绪（比如 Tenant 还没创建），重试
		if apierrors.IsNotFound(err) {
			message := fmt.Sprintf("waiting for referenced dependency: %v", err)
			_ = r.setPolicyReadyCondition(ctx, policy.Name, metav1.ConditionFalse, "DependencyNotReady", message)
			logger.V(1).Info("referenced dependency is not ready", "error", err)
			return ctrl.Result{RequeueAfter: dependencyRetryAfter}, nil
		}
		_ = r.setPolicyReadyCondition(ctx, policy.Name, metav1.ConditionFalse, "ReconcileFailed", err.Error())
		return ctrl.Result{}, err
	}

	reason := "InstanceReady"
	message := "the effective policy allows the simulator instance"
	if !allowed {
		reason = "Denied"
		message = "the effective policy denies the simulator instance"
	}
	if err := r.setPolicyReadyCondition(ctx, policy.Name, metav1.ConditionTrue, reason, message); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// reconcileTenantModelPair 综合该租户-模型的所有策略，得出最终 Allow/Deny，并确保实例存在或删除。
func (r *TenantModelPolicyReconciler) reconcileTenantModelPair(
	ctx context.Context,
	tenantName string,
	modelName string,
) (allowed bool, operationErr error) {
	ctx, span := startOperation(
		ctx,
		"tenant-model-policy",
		"materialize-pair",
		attribute.String(traceAttributeTenantName, tenantName),
		attribute.String(traceAttributeModelName, modelName),
	)
	defer func() {
		observability.EndSpan(span, operationErr, attribute.Bool("policy.allowed", allowed))
		observeOperation("tenant-model-policy", "materialize-pair", operationErr)
	}()

	var policies platformv1.TenantModelPolicyList
	if err := r.List(ctx, &policies); err != nil {
		return false, fmt.Errorf("list tenant model policies: %w", err)
	}

	// 如果最终不允许，删掉对应的 SimulatorInstance
	if !tenantModelAllowed(policies.Items, tenantName, modelName) {
		return false, r.deleteSimulatorInstanceForPair(ctx, tenantName, modelName)
	}

	// 传一个虚拟的 Allow 策略进去，创建或更新实例
	policy := &platformv1.TenantModelPolicy{
		Spec: platformv1.TenantModelPolicySpec{
			TenantRef: platformv1.ObjectRef{Name: tenantName},
			ModelRef:  platformv1.ObjectRef{Name: modelName},
			Effect:    policyEffectAllow,
		},
	}
	return true, r.ensureSimulatorInstance(ctx, policy)
}

// ensureSimulatorInstance 创建或更新 SimulatorInstance，只维护元数据和 OwnerReference。
// 副本数和 QPS 留给 Orchestrator 和 Traffic 控制器去管。
func (r *TenantModelPolicyReconciler) ensureSimulatorInstance(ctx context.Context, policy *platformv1.TenantModelPolicy) error {
	tenantName := policy.Spec.TenantRef.Name
	modelName := policy.Spec.ModelRef.Name

	var tenant platformv1.Tenant
	if err := r.Get(ctx, client.ObjectKey{Name: tenantName}, &tenant); err != nil {
		return fmt.Errorf("get referenced tenant %q: %w", tenantName, err)
	}
	// 租户本身在删，那实例也应该删
	if !tenant.DeletionTimestamp.IsZero() {
		return r.deleteSimulatorInstanceForPair(ctx, tenantName, modelName)
	}

	var model platformv1.Model
	if err := r.Get(ctx, client.ObjectKey{Name: modelName}, &model); err != nil {
		return fmt.Errorf("get referenced model %q: %w", modelName, err)
	}
	if !model.DeletionTimestamp.IsZero() {
		return r.deleteSimulatorInstanceForPair(ctx, tenantName, modelName)
	}

	instanceName := generateInstanceName(tenantName, modelName)
	var instance platformv1.SimulatorInstance
	err := r.Get(ctx, client.ObjectKey{Name: instanceName}, &instance)
	if apierrors.IsNotFound(err) {
		// 新建实例骨架
		instance = platformv1.SimulatorInstance{
			ObjectMeta: metav1.ObjectMeta{Name: instanceName},
			Spec: platformv1.SimulatorInstanceSpec{
				TenantRef: platformv1.ObjectRef{Name: tenantName},
				ModelRef:  platformv1.ObjectRef{Name: modelName},
				Replicas:  0,
				Traffic:   platformv1.TrafficSpec{QPS: 0},
				TimeScale: platformv1.DefaultSimulationRate,
			},
		}
		setInstanceIdentityMetadata(&instance, tenantName, modelName)
		if err := controllerutil.SetControllerReference(&tenant, &instance, r.Scheme); err != nil {
			return fmt.Errorf("set tenant owner on simulator instance %q: %w", instanceName, err)
		}
		if err := r.Create(ctx, &instance); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create simulator instance %q: %w", instanceName, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get simulator instance %q: %w", instanceName, err)
	}

	// 实例已存在，检查一下引用是否正确，避免同名冲突
	if instance.Spec.TenantRef.Name != tenantName || instance.Spec.ModelRef.Name != modelName {
		return fmt.Errorf(
			"simulator instance name collision: %q belongs to tenant %q model %q",
			instanceName,
			instance.Spec.TenantRef.Name,
			instance.Spec.ModelRef.Name,
		)
	}

	// 更新标签、注解，清理掉旧控制器遗留的 OwnerReference，然后把当前租户设为 owner
	before := instance.DeepCopy()
	setInstanceIdentityMetadata(&instance, tenantName, modelName)
	removeLegacyPolicyControllerOwner(&instance)
	if err := controllerutil.SetControllerReference(&tenant, &instance, r.Scheme); err != nil {
		return fmt.Errorf("set tenant owner on simulator instance %q: %w", instanceName, err)
	}
	if err := r.Patch(ctx, &instance, client.MergeFrom(before)); err != nil {
		return fmt.Errorf("patch simulator instance %q metadata: %w", instanceName, err)
	}
	return nil
}

func setInstanceIdentityMetadata(instance *platformv1.SimulatorInstance, tenantName, modelName string) {
	labels := ensureStringMap(&instance.Labels)
	annotations := ensureStringMap(&instance.Annotations)
	setIdentityMetadata(labels, annotations, instance.Name, tenantName, modelName)
}

// 移除旧版 Controller 遗留的 TenantModelPolicy OwnerReference。
func removeLegacyPolicyControllerOwner(instance *platformv1.SimulatorInstance) {
	owners := instance.OwnerReferences[:0]
	for _, owner := range instance.OwnerReferences {
		if owner.Controller != nil && *owner.Controller && owner.Kind == "TenantModelPolicy" {
			continue
		}
		owners = append(owners, owner)
	}
	instance.OwnerReferences = owners
}

func (r *TenantModelPolicyReconciler) deleteSimulatorInstanceForPair(ctx context.Context, tenantName, modelName string) error {
	instanceName := generateInstanceName(tenantName, modelName)
	var instance platformv1.SimulatorInstance
	if err := r.Get(ctx, client.ObjectKey{Name: instanceName}, &instance); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get simulator instance %q before delete: %w", instanceName, err)
	}
	// 确认实例引用没被篡改，防止删错
	if instance.Spec.TenantRef.Name != tenantName || instance.Spec.ModelRef.Name != modelName {
		return fmt.Errorf("refusing to delete colliding simulator instance %q", instanceName)
	}
	if err := r.Delete(ctx, &instance); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete simulator instance %q: %w", instanceName, err)
	}
	return nil
}

// generateInstanceName 生成 "<tenant>-<model>" 格式的实例名，太长就用哈希截断。
func generateInstanceName(tenantName, modelName string) string {
	const maxNameLength = 253
	joined := tenantName + "-" + modelName
	if len(joined) <= maxNameLength {
		return joined
	}
	// 用分隔符 \x00 确保不同组合的哈希不一样，防止截断后碰撞
	sum := sha256.Sum256([]byte(tenantName + "\x00" + modelName))
	suffix := hex.EncodeToString(sum[:8])
	prefix := strings.TrimRight(joined[:maxNameLength-len(suffix)-1], ".-")
	return prefix + "-" + suffix
}

func (r *TenantModelPolicyReconciler) setPolicyReadyCondition(
	ctx context.Context,
	name string,
	status metav1.ConditionStatus,
	reason string,
	message string,
) error {
	return retryOnConflict(func() error {
		var policy platformv1.TenantModelPolicy
		if err := r.Get(ctx, client.ObjectKey{Name: name}, &policy); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		before := policy.DeepCopy()
		meta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
			Type:               conditionTypeReady,
			Status:             status,
			ObservedGeneration: policy.Generation,
			Reason:             reason,
			Message:            message,
		})
		// 状态没变就不发 API 了
		if conditionsEqual(before.Status.Conditions, policy.Status.Conditions) {
			return nil
		}
		return r.Status().Patch(ctx, &policy, client.MergeFrom(before))
	})
}

func conditionsEqual(left, right []metav1.Condition) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Type != right[i].Type ||
			left[i].Status != right[i].Status ||
			left[i].ObservedGeneration != right[i].ObservedGeneration ||
			left[i].Reason != right[i].Reason ||
			left[i].Message != right[i].Message {
			return false
		}
	}
	return true
}

func (r *TenantModelPolicyReconciler) mapInstanceToPolicies(ctx context.Context, obj client.Object) []reconcile.Request {
	instance, ok := obj.(*platformv1.SimulatorInstance)
	if !ok {
		return nil
	}
	return r.policyRequests(ctx, instance.Spec.TenantRef.Name, instance.Spec.ModelRef.Name)
}

func (r *TenantModelPolicyReconciler) mapTenantToPolicies(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.policyRequests(ctx, obj.GetName(), "")
}

func (r *TenantModelPolicyReconciler) mapModelToPolicies(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.policyRequests(ctx, "", obj.GetName())
}

func (r *TenantModelPolicyReconciler) policyRequests(ctx context.Context, tenantName, modelName string) []reconcile.Request {
	var policies platformv1.TenantModelPolicyList
	if err := r.List(ctx, &policies); err != nil {
		log.FromContext(ctx).Error(err, "list TenantModelPolicies while mapping event")
		return nil
	}
	requests := make([]reconcile.Request, 0)
	for i := range policies.Items {
		policy := &policies.Items[i]
		if tenantName != "" && policy.Spec.TenantRef.Name != tenantName {
			continue
		}
		if modelName != "" && policy.Spec.ModelRef.Name != modelName {
			continue
		}
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Name: policy.Name}})
	}
	return requests
}

func (r *TenantModelPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	instanceLifecycleChanged := lifecyclePredicate(func(e event.UpdateEvent) bool {
		oldInstance, oldOK := e.ObjectOld.(*platformv1.SimulatorInstance)
		newInstance, newOK := e.ObjectNew.(*platformv1.SimulatorInstance)
		return oldOK && newOK &&
			(oldInstance.Spec.TenantRef.Name != newInstance.Spec.TenantRef.Name ||
				oldInstance.Spec.ModelRef.Name != newInstance.Spec.ModelRef.Name ||
				!oldInstance.DeletionTimestamp.Equal(newInstance.DeletionTimestamp))
	})

	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		Named("tenantmodelpolicy").
		For(&platformv1.TenantModelPolicy{}).
		Watches(
			&platformv1.SimulatorInstance{},
			handler.EnqueueRequestsFromMapFunc(r.mapInstanceToPolicies),
			builder.WithPredicates(instanceLifecycleChanged),
		)
	watchGenerationChanges(controllerBuilder, &platformv1.Tenant{}, r.mapTenantToPolicies)
	watchGenerationChanges(controllerBuilder, &platformv1.Model{}, r.mapModelToPolicies)
	return controllerBuilder.Complete(r)
}
