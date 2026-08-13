package kubernetes

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	identityInstanceAnnotation = "platform.study.com/instance-name"
	identityTenantAnnotation   = "platform.study.com/tenant-name"
	identityModelAnnotation    = "platform.study.com/model-name"
	identityInstanceLabel      = "platform.study.com/instance"
	identityTenantLabel        = "platform.study.com/tenant"
	identityModelLabel         = "platform.study.com/model"
)

func MapPlatformResource(object *unstructured.Unstructured, now time.Time) model.PlatformResource {
	createdAt := optionalTime(object.GetCreationTimestamp())
	deletion := optionalTime(object.GetDeletionTimestamp())
	spec, _, _ := unstructured.NestedMap(object.Object, "spec")
	status, _, _ := unstructured.NestedMap(object.Object, "status")
	if spec == nil {
		spec = map[string]any{}
	}
	if status == nil {
		status = map[string]any{}
	}
	conditions := conditionsFromAny(status["conditions"])
	apiVersion := object.GetAPIVersion()
	if apiVersion == "" {
		apiVersion = "platform.study.com/v1"
	}
	kind := object.GetKind()
	if kind == "" {
		kind = kindFromObject(object)
	}
	return model.PlatformResource{
		Ref: model.ResourceRef{
			APIVersion: apiVersion,
			Kind:       kind,
			Name:       object.GetName(),
			UID:        string(object.GetUID()),
		},
		Metadata: model.ResourceMetadata{
			Generation:        object.GetGeneration(),
			ResourceVersion:   object.GetResourceVersion(),
			CreatedAt:         createdAt,
			DeletionTimestamp: deletion,
			Labels:            copyStringMap(object.GetLabels()),
			Annotations:       copyStringMap(object.GetAnnotations()),
		},
		Spec:       spec,
		Status:     status,
		Conditions: conditions,
		Derived: map[string]any{
			"freshness": resourceFreshness(status, now),
		},
	}
}

func MapNode(node *corev1.Node, now time.Time) model.ClusterNode {
	ready := false
	conditions := make([]model.Condition, 0, len(node.Status.Conditions))
	for _, condition := range node.Status.Conditions {
		conditions = append(conditions, conditionFromNode(condition))
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			ready = true
		}
	}
	role := "worker"
	if _, exists := node.Labels["node-role.kubernetes.io/control-plane"]; exists {
		role = "control-plane"
	} else if _, exists := node.Labels["node-role.kubernetes.io/master"]; exists {
		role = "control-plane"
	}
	zone := node.Labels[corev1.LabelTopologyZone]
	if zone == "" {
		zone = node.Labels[corev1.LabelFailureDomainBetaZone]
	}
	internalIP := ""
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			internalIP = address.Address
			break
		}
	}
	return model.ClusterNode{
		Ref:         model.ResourceRef{APIVersion: "v1", Kind: "Node", Name: node.Name, UID: string(node.UID)},
		Role:        role,
		Ready:       ready,
		Phase:       map[bool]string{true: "Ready", false: "NotReady"}[ready],
		Schedulable: !node.Spec.Unschedulable,
		Zone:        zone,
		Version:     node.Status.NodeInfo.KubeletVersion,
		InternalIP:  internalIP,
		Conditions:  conditions,
		ObservedAt:  now,
		Capacity:    resourceList(node.Status.Capacity),
		Allocatable: resourceList(node.Status.Allocatable),
	}
}

func MapPod(pod *corev1.Pod) model.Pod {
	conditions := make([]model.Condition, 0, len(pod.Status.Conditions))
	ready := false
	for _, condition := range pod.Status.Conditions {
		conditions = append(conditions, conditionFromPod(condition))
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			ready = true
		}
	}
	containers := make([]model.ContainerState, 0, len(pod.Status.ContainerStatuses))
	for _, status := range pod.Status.ContainerStatuses {
		state, reason := containerState(status.State)
		containers = append(containers, model.ContainerState{
			Name:         status.Name,
			Ready:        status.Ready,
			RestartCount: status.RestartCount,
			State:        state,
			Reason:       reason,
		})
	}
	return model.Pod{
		Ref: model.ResourceRef{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			UID:        string(pod.UID),
		},
		Phase:             string(pod.Status.Phase),
		Ready:             ready,
		NodeName:          pod.Spec.NodeName,
		PodIP:             pod.Status.PodIP,
		StartTime:         optionalTime(pod.Status.StartTime),
		Conditions:        conditions,
		Containers:        containers,
		SimulatorInstance: identity(pod.Annotations, pod.Labels, identityInstanceAnnotation, identityInstanceLabel),
		Tenant:            identity(pod.Annotations, pod.Labels, identityTenantAnnotation, identityTenantLabel),
		Model:             identity(pod.Annotations, pod.Labels, identityModelAnnotation, identityModelLabel),
	}
}

func MapDeployment(deployment *appsv1.Deployment) model.Deployment {
	desired := int32(0)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	conditions := make([]model.Condition, 0, len(deployment.Status.Conditions))
	for _, condition := range deployment.Status.Conditions {
		conditions = append(conditions, model.Condition{
			Type:               string(condition.Type),
			Status:             string(condition.Status),
			Reason:             condition.Reason,
			Message:            condition.Message,
			LastTransitionTime: optionalTime(&condition.LastTransitionTime),
		})
	}
	return model.Deployment{
		Ref: model.ResourceRef{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  deployment.Namespace,
			Name:       deployment.Name,
			UID:        string(deployment.UID),
		},
		DesiredReplicas:     desired,
		UpdatedReplicas:     deployment.Status.UpdatedReplicas,
		ReadyReplicas:       deployment.Status.ReadyReplicas,
		AvailableReplicas:   deployment.Status.AvailableReplicas,
		UnavailableReplicas: deployment.Status.UnavailableReplicas,
		Conditions:          conditions,
		SimulatorInstance:   identity(deployment.Annotations, deployment.Labels, identityInstanceAnnotation, identityInstanceLabel),
		Tenant:              identity(deployment.Annotations, deployment.Labels, identityTenantAnnotation, identityTenantLabel),
		Model:               identity(deployment.Annotations, deployment.Labels, identityModelAnnotation, identityModelLabel),
	}
}

func MapService(service *corev1.Service) model.Service {
	ports := make([]map[string]any, 0, len(service.Spec.Ports))
	for _, port := range service.Spec.Ports {
		ports = append(ports, map[string]any{
			"name":       port.Name,
			"port":       port.Port,
			"targetPort": port.TargetPort.String(),
			"protocol":   string(port.Protocol),
		})
	}
	return model.Service{
		Ref: model.ResourceRef{
			APIVersion: "v1",
			Kind:       "Service",
			Namespace:  service.Namespace,
			Name:       service.Name,
			UID:        string(service.UID),
		},
		Type:      string(service.Spec.Type),
		ClusterIP: service.Spec.ClusterIP,
		Ports:     ports,
		Selector:  copyStringMap(service.Spec.Selector),
	}
}

func MapLease(lease *coordinationv1.Lease, now time.Time) model.Lease {
	holder := ""
	if lease.Spec.HolderIdentity != nil {
		holder = *lease.Spec.HolderIdentity
	}
	duration := int32(0)
	if lease.Spec.LeaseDurationSeconds != nil {
		duration = *lease.Spec.LeaseDurationSeconds
	}
	var renewTime *time.Time
	if lease.Spec.RenewTime != nil {
		value := lease.Spec.RenewTime.Time.UTC()
		renewTime = &value
	}
	age := int64(0)
	fresh := false
	if renewTime != nil {
		age = max(0, now.Sub(*renewTime).Milliseconds())
		fresh = duration > 0 && age <= int64(duration)*1000
	}
	return model.Lease{
		Ref: model.ResourceRef{
			APIVersion: "coordination.k8s.io/v1",
			Kind:       "Lease",
			Namespace:  lease.Namespace,
			Name:       lease.Name,
			UID:        string(lease.UID),
		},
		HolderIdentity: holder,
		RenewTime:      renewTime,
		LeaseDuration:  duration,
		Fresh:          fresh,
		AgeMs:          age,
	}
}

func MapEvent(event *corev1.Event) model.Event {
	eventTime := event.EventTime.Time
	if eventTime.IsZero() && event.Series != nil {
		eventTime = event.Series.LastObservedTime.Time
	}
	if eventTime.IsZero() {
		eventTime = event.LastTimestamp.Time
	}
	if eventTime.IsZero() {
		eventTime = event.FirstTimestamp.Time
	}
	if eventTime.IsZero() {
		eventTime = event.CreationTimestamp.Time
	}
	count := event.Count
	if event.Series != nil && event.Series.Count > count {
		count = event.Series.Count
	}
	reportingController := event.ReportingController
	if reportingController == "" {
		reportingController = event.Source.Component
	}
	return model.Event{
		ID:        string(event.UID),
		EventTime: eventTime.UTC(),
		Type:      event.Type,
		Reason:    event.Reason,
		Message:   event.Message,
		Count:     count,
		FirstSeen: optionalTime(&event.FirstTimestamp),
		LastSeen:  optionalTime(&event.LastTimestamp),
		Regarding: model.ResourceRef{
			APIVersion: event.InvolvedObject.APIVersion,
			Kind:       event.InvolvedObject.Kind,
			Namespace:  event.InvolvedObject.Namespace,
			Name:       event.InvolvedObject.Name,
			UID:        string(event.InvolvedObject.UID),
		},
		ReportingController: reportingController,
		Source:              event.Source.Host,
	}
}

func conditionsFromAny(value any) []model.Condition {
	items, ok := value.([]any)
	if !ok {
		return []model.Condition{}
	}
	conditions := make([]model.Condition, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		condition := model.Condition{
			Type:               text(object["type"]),
			Status:             text(object["status"]),
			Reason:             text(object["reason"]),
			Message:            text(object["message"]),
			ObservedGeneration: integer64(object["observedGeneration"]),
		}
		if parsed, ok := parseTime(object["lastTransitionTime"]); ok {
			condition.LastTransitionTime = &parsed
		}
		conditions = append(conditions, condition)
	}
	return conditions
}

func resourceFreshness(status map[string]any, now time.Time) string {
	observedAt, ok := parseTime(status["observedAt"])
	if !ok {
		return "notReported"
	}
	age := now.Sub(observedAt)
	switch {
	case age < 0:
		return "clockSkew"
	case age <= 30*time.Second:
		return "fresh"
	default:
		return "stale"
	}
}

func conditionFromNode(condition corev1.NodeCondition) model.Condition {
	return model.Condition{
		Type:               string(condition.Type),
		Status:             string(condition.Status),
		Reason:             condition.Reason,
		Message:            condition.Message,
		LastTransitionTime: optionalTime(&condition.LastTransitionTime),
	}
}

func conditionFromPod(condition corev1.PodCondition) model.Condition {
	return model.Condition{
		Type:               string(condition.Type),
		Status:             string(condition.Status),
		Reason:             condition.Reason,
		Message:            condition.Message,
		LastTransitionTime: optionalTime(&condition.LastTransitionTime),
	}
}

func optionalTime(value any) *time.Time {
	switch typed := value.(type) {
	case metav1.Time:
		if typed.IsZero() {
			return nil
		}
		result := typed.Time.UTC()
		return &result
	case *metav1.Time:
		if typed == nil || typed.IsZero() {
			return nil
		}
		result := typed.Time.UTC()
		return &result
	default:
		return nil
	}
}

func parseTime(value any) (time.Time, bool) {
	raw, ok := value.(string)
	if !ok || raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func text(value any) string {
	if result, ok := value.(string); ok {
		return result
	}
	return ""
}

func integer64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int32:
		return int64(typed)
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func kindFromObject(object *unstructured.Unstructured) string {
	if kind, found, _ := unstructured.NestedString(object.Object, "kind"); found {
		return kind
	}
	return "Unknown"
}

func identity(annotations, labels map[string]string, annotationKey, labelKey string) string {
	if value := annotations[annotationKey]; value != "" {
		return value
	}
	return labels[labelKey]
}

func containerState(state corev1.ContainerState) (string, string) {
	switch {
	case state.Running != nil:
		return "running", ""
	case state.Waiting != nil:
		return "waiting", state.Waiting.Reason
	case state.Terminated != nil:
		return "terminated", state.Terminated.Reason
	default:
		return "unknown", ""
	}
}

func resourceList(values corev1.ResourceList) map[string]string {
	result := make(map[string]string, len(values))
	for name, quantity := range values {
		result[string(name)] = quantity.String()
	}
	return result
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func SortWorkloads(workloads *model.Workloads) {
	sort.Slice(workloads.Nodes, func(i, j int) bool { return workloads.Nodes[i].Ref.Name < workloads.Nodes[j].Ref.Name })
	sort.Slice(workloads.Pods, func(i, j int) bool {
		return namespacedName(workloads.Pods[i].Ref) < namespacedName(workloads.Pods[j].Ref)
	})
	sort.Slice(workloads.Deployments, func(i, j int) bool {
		return namespacedName(workloads.Deployments[i].Ref) < namespacedName(workloads.Deployments[j].Ref)
	})
	sort.Slice(workloads.Services, func(i, j int) bool {
		return namespacedName(workloads.Services[i].Ref) < namespacedName(workloads.Services[j].Ref)
	})
	sort.Slice(workloads.Leases, func(i, j int) bool {
		return namespacedName(workloads.Leases[i].Ref) < namespacedName(workloads.Leases[j].Ref)
	})
	sort.Slice(workloads.Events, func(i, j int) bool { return workloads.Events[i].EventTime.After(workloads.Events[j].EventTime) })
}

func namespacedName(ref model.ResourceRef) string {
	return strings.TrimPrefix(fmt.Sprintf("%s/%s", ref.Namespace, ref.Name), "/")
}
