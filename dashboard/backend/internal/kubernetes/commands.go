package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

type ApplyIntent struct {
	Kind            string            `json:"kind"`
	Name            string            `json:"name"`
	Spec            map[string]any    `json:"spec"`
	ResourceVersion string            `json:"resourceVersion,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
}

type Gateway struct {
	cache *Cache
}

func NewGateway(cache *Cache) *Gateway {
	return &Gateway{cache: cache}
}

func (gateway *Gateway) Apply(ctx context.Context, intent ApplyIntent, dryRun bool) (*unstructured.Unstructured, string, error) {
	descriptor, err := validateIntent(intent)
	if err != nil {
		return nil, "", err
	}
	resourceClient := gateway.cache.clients.Dynamic.Resource(descriptor.GVR)
	existing, exists, err := gateway.cache.GetPlatform(descriptor.Kind, intent.Name)
	if err != nil {
		return nil, "", err
	}
	options := metav1.UpdateOptions{}
	createOptions := metav1.CreateOptions{}
	if dryRun {
		options.DryRun = []string{metav1.DryRunAll}
		createOptions.DryRun = []string{metav1.DryRunAll}
	}

	if !exists {
		if intent.ResourceVersion != "" {
			return nil, "", apierrors.NewConflict(descriptor.GVR.GroupResource(), intent.Name, errors.New("resource does not exist at expected resourceVersion"))
		}
		object := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": descriptor.GVR.GroupVersion().String(),
			"kind":       descriptor.Kind,
			"metadata": map[string]any{
				"name": intent.Name,
			},
			"spec": deepCopyMap(intent.Spec),
		}}
		labels := maps.Clone(intent.Labels)
		if labels == nil {
			labels = map[string]string{}
		}
		labels["app.kubernetes.io/managed-by"] = "hello-k8s-ai-dashboard"
		object.SetLabels(labels)
		created, err := resourceClient.Create(ctx, object, createOptions)
		if err != nil {
			return nil, "", err
		}
		return created, "create", nil
	}

	if intent.ResourceVersion != "" && existing.GetResourceVersion() != intent.ResourceVersion {
		return nil, "", apierrors.NewConflict(
			descriptor.GVR.GroupResource(),
			intent.Name,
			fmt.Errorf("expected resourceVersion %s, current cache has %s", intent.ResourceVersion, existing.GetResourceVersion()),
		)
	}
	existing.Object["spec"] = deepCopyMap(intent.Spec)
	if len(intent.Labels) > 0 {
		labels := maps.Clone(existing.GetLabels())
		if labels == nil {
			labels = map[string]string{}
		}
		for key, value := range intent.Labels {
			labels[key] = value
		}
		existing.SetLabels(labels)
	}
	updated, err := resourceClient.Update(ctx, existing, options)
	if err != nil {
		return nil, "", err
	}
	return updated, "update", nil
}

func (gateway *Gateway) Delete(ctx context.Context, kind, name, resourceVersion string, dryRun bool) error {
	descriptor, exists := DescriptorForKind(kind)
	if !exists || !descriptor.UserWritable {
		return fmt.Errorf("kind %q is not writable through the Dashboard", kind)
	}
	if problems := validation.IsDNS1123Subdomain(name); len(problems) > 0 {
		return fmt.Errorf("invalid resource name: %s", strings.Join(problems, "; "))
	}
	options := metav1.DeleteOptions{PropagationPolicy: ptr(metav1.DeletePropagationBackground)}
	if resourceVersion != "" {
		options.Preconditions = &metav1.Preconditions{ResourceVersion: &resourceVersion}
	}
	if dryRun {
		options.DryRun = []string{metav1.DryRunAll}
	}
	return gateway.cache.clients.Dynamic.Resource(descriptor.GVR).Delete(ctx, name, options)
}

func (gateway *Gateway) SetTenantQPS(ctx context.Context, name string, qps int, resourceVersion string, dryRun bool) (*unstructured.Unstructured, error) {
	if qps < 0 {
		return nil, errors.New("qps must not be negative")
	}
	descriptor, _ := DescriptorForKind("Tenant")
	existing, found, err := gateway.cache.GetPlatform("Tenant", name)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, apierrors.NewNotFound(descriptor.GVR.GroupResource(), name)
	}
	if resourceVersion != "" && existing.GetResourceVersion() != resourceVersion {
		return nil, apierrors.NewConflict(
			descriptor.GVR.GroupResource(), name,
			fmt.Errorf("expected resourceVersion %s, current cache has %s", resourceVersion, existing.GetResourceVersion()),
		)
	}
	if err := unstructured.SetNestedField(existing.Object, int64(qps), "spec", "qps"); err != nil {
		return nil, fmt.Errorf("set Tenant spec.qps: %w", err)
	}
	options := metav1.UpdateOptions{}
	if dryRun {
		options.DryRun = []string{metav1.DryRunAll}
	}
	return gateway.cache.clients.Dynamic.Resource(descriptor.GVR).Update(ctx, existing, options)
}

// SetSimulationRate 更新集群唯一的 SimulationClock/default。
func (gateway *Gateway) SetSimulationRate(
	ctx context.Context,
	rate int,
	resourceVersion string,
	dryRun bool,
) (*unstructured.Unstructured, string, error) {
	if rate < 1 || rate > 20 {
		return nil, "", errors.New("rate must be between 1 and 20")
	}
	descriptor, _ := DescriptorForKind("SimulationClock")
	resourceClient := gateway.cache.clients.Dynamic.Resource(descriptor.GVR)
	options := metav1.UpdateOptions{}
	createOptions := metav1.CreateOptions{}
	if dryRun {
		options.DryRun = []string{metav1.DryRunAll}
		createOptions.DryRun = []string{metav1.DryRunAll}
	}

	existing, err := resourceClient.Get(ctx, "default", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if resourceVersion != "" {
			return nil, "", apierrors.NewConflict(
				descriptor.GVR.GroupResource(),
				"default",
				errors.New("resource does not exist at expected resourceVersion"),
			)
		}
		object := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": descriptor.GVR.GroupVersion().String(),
			"kind":       descriptor.Kind,
			"metadata": map[string]any{
				"name": "default",
				"labels": map[string]any{
					"app.kubernetes.io/managed-by": "hello-k8s-ai-dashboard",
				},
			},
			"spec": map[string]any{"rate": int64(rate)},
		}}
		created, createErr := resourceClient.Create(ctx, object, createOptions)
		if createErr == nil {
			return created, "create", nil
		}
		// Controller 也会补建默认 Clock；两者并发时读取胜出的对象后继续更新。
		if !apierrors.IsAlreadyExists(createErr) {
			return nil, "", createErr
		}
		existing, err = resourceClient.Get(ctx, "default", metav1.GetOptions{})
		if err != nil {
			return nil, "", err
		}
	}
	if err != nil {
		return nil, "", err
	}
	if resourceVersion != "" && existing.GetResourceVersion() != resourceVersion {
		return nil, "", apierrors.NewConflict(
			descriptor.GVR.GroupResource(),
			"default",
			fmt.Errorf("expected resourceVersion %s, current object has %s", resourceVersion, existing.GetResourceVersion()),
		)
	}
	if err := unstructured.SetNestedField(existing.Object, int64(rate), "spec", "rate"); err != nil {
		return nil, "", fmt.Errorf("set SimulationClock spec.rate: %w", err)
	}
	updated, err := resourceClient.Update(ctx, existing, options)
	if err != nil {
		return nil, "", err
	}
	return updated, "update", nil
}

func validateIntent(intent ApplyIntent) (ResourceDescriptor, error) {
	descriptor, exists := DescriptorForKind(intent.Kind)
	if !exists || !descriptor.UserWritable {
		return ResourceDescriptor{}, fmt.Errorf("kind %q is not writable through the Dashboard", intent.Kind)
	}
	if problems := validation.IsDNS1123Subdomain(intent.Name); len(problems) > 0 {
		return ResourceDescriptor{}, fmt.Errorf("invalid resource name: %s", strings.Join(problems, "; "))
	}
	if intent.Spec == nil {
		return ResourceDescriptor{}, errors.New("spec is required")
	}
	if descriptor.Kind == "Model" {
		if _, exists := intent.Spec["absoluteScore"]; !exists {
			return ResourceDescriptor{}, errors.New("Model 缺少必填字段 spec.absoluteScore")
		}
	}
	allowed := writableSpecFields[descriptor.Kind]
	for field := range intent.Spec {
		if _, ok := allowed[field]; !ok {
			return ResourceDescriptor{}, fmt.Errorf("field spec.%s is not writable for %s", field, descriptor.Kind)
		}
	}
	return descriptor, nil
}

var writableSpecFields = map[string]map[string]struct{}{
	"Model":      fields("displayName", "gpuUnits", "maxConcurrency", "absoluteScore", "coldStartMs", "performance"),
	"WorkerNode": fields("displayName", "gpu", "maxConcurrency"),
	"Tenant": fields(
		"displayName", "priority", "qps", "ttftThresholdMs", "queueThreshold",
		"ttftScaleDownThresholdMs", "queueScaleDownThreshold",
	),
	"TenantModelPolicy": fields("tenantRef", "modelRef", "effect"),
	"TenantNodePolicy":  fields("tenantRef", "nodeRef", "effect"),
	"ModelNodePolicy":   fields("modelRef", "nodeRef", "effect"),
	"Orchestrator": fields(
		"tenantRef", "scaleUpCooldownSeconds", "scaleDownCooldownSeconds",
		"allowScaleToZero", "minReplicas", "maxReplicas",
	),
}

func fields(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func deepCopyMap(value map[string]any) map[string]any {
	return runtimeDeepCopyJSON(value).(map[string]any)
}

func runtimeDeepCopyJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = runtimeDeepCopyJSON(item)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = runtimeDeepCopyJSON(item)
		}
		return result
	default:
		return typed
	}
}

func ptr[T any](value T) *T {
	return &value
}
