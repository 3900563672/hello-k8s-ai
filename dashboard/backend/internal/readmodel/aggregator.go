package readmodel

import (
	"fmt"
	"sort"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/kubernetes"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Aggregator struct {
	cache *kubernetes.Cache
}

func NewAggregator(cache *kubernetes.Cache) *Aggregator {
	return &Aggregator{cache: cache}
}

func (aggregator *Aggregator) Configuration(now time.Time) model.Configuration {
	resources := make(map[string][]model.PlatformResource, len(kubernetes.PlatformResources))
	for _, descriptor := range kubernetes.PlatformResources {
		objects := aggregator.cache.ListPlatform(descriptor.Kind)
		mapped := make([]model.PlatformResource, 0, len(objects))
		for _, object := range objects {
			resource := kubernetes.MapPlatformResource(object, now)
			resource.Ref.Kind = descriptor.Kind
			resource.Ref.APIVersion = descriptor.GVR.GroupVersion().String()
			mapped = append(mapped, resource)
		}
		resources[descriptor.Kind] = mapped
	}

	configuration := model.Configuration{
		AsOf:         now,
		Availability: "available",
		Models:       resources["Model"],
		WorkerNodes:  resources["WorkerNode"],
		Tenants:      resources["Tenant"],
		Policies: model.PolicySet{
			TenantModel: resources["TenantModelPolicy"],
			TenantNode:  resources["TenantNodePolicy"],
			ModelNode:   resources["ModelNodePolicy"],
		},
		Orchestrators:      resources["Orchestrator"],
		SimulatorInstances: resources["SimulatorInstance"],
		TenantPerformance:  resources["TenantPerformance"],
		TenantRuntimes:     resources["TenantRuntime"],
	}
	aggregator.deriveConfiguration(&configuration)
	return configuration
}

func (aggregator *Aggregator) deriveConfiguration(configuration *model.Configuration) {
	instancesByModel := make(map[string][]model.PlatformResource)
	instancesByTenant := make(map[string][]model.PlatformResource)
	for _, instance := range configuration.SimulatorInstances {
		modelName := nestedRefName(instance.Spec, "modelRef")
		tenantName := nestedRefName(instance.Spec, "tenantRef")
		instancesByModel[modelName] = append(instancesByModel[modelName], instance)
		instancesByTenant[tenantName] = append(instancesByTenant[tenantName], instance)
		available := numberInt(instance.Status["availableReplicas"])
		desired := numberInt(instance.Spec["replicas"])
		instance.Derived["desiredAvailableGap"] = max(0, desired-available)
	}

	for index := range configuration.Models {
		resource := &configuration.Models[index]
		instances := instancesByModel[resource.Ref.Name]
		available := 0
		desired := 0
		allocatedQPS := 0
		for _, instance := range instances {
			available += numberInt(instance.Status["availableReplicas"])
			desired += numberInt(instance.Spec["replicas"])
			allocatedQPS += nestedInt(instance.Spec, "traffic", "qps")
		}
		resource.Derived["instanceCount"] = len(instances)
		resource.Derived["desiredReplicas"] = desired
		resource.Derived["availableReplicas"] = available
		resource.Derived["allocatedQPS"] = allocatedQPS
	}

	performanceByTenant := make(map[string]model.PlatformResource)
	for _, performance := range configuration.TenantPerformance {
		performanceByTenant[nestedRefName(performance.Spec, "tenantRef")] = performance
	}
	runtimeByTenant := make(map[string]model.PlatformResource)
	for _, runtime := range configuration.TenantRuntimes {
		runtimeByTenant[nestedRefName(runtime.Spec, "tenantRef")] = runtime
	}
	for index := range configuration.Tenants {
		resource := &configuration.Tenants[index]
		instances := instancesByTenant[resource.Ref.Name]
		allocatedQPS := 0
		readyReplicas := 0
		for _, instance := range instances {
			allocatedQPS += nestedInt(instance.Spec, "traffic", "qps")
			readyReplicas += numberInt(instance.Status["availableReplicas"])
		}
		resource.Derived["instanceCount"] = len(instances)
		resource.Derived["simulatorAvailableReplicas"] = readyReplicas
		resource.Derived["allocatedQPS"] = allocatedQPS
		resource.Derived["allocationBalanced"] = allocatedQPS == numberInt(resource.Spec["qps"])
		if performance, exists := performanceByTenant[resource.Ref.Name]; exists {
			resource.Derived["performancePhase"] = text(performance.Status["phase"])
			resource.Derived["performanceFreshness"] = performance.Derived["freshness"]
		}
		if runtime, exists := runtimeByTenant[resource.Ref.Name]; exists {
			// SimulatorInstance Controller 将可用副本总数写入
			// TenantRuntime.status.instanceCount。
			resource.Derived["readyReplicaCount"] = numberInt(runtime.Status["instanceCount"])
			resource.Derived["runtimePhase"] = text(runtime.Status["phase"])
		} else {
			resource.Derived["readyReplicaCount"] = readyReplicas
		}
	}

	coreNodes := make(map[string]model.ClusterNode)
	now := configuration.AsOf
	for _, node := range aggregator.cache.ListNodes() {
		mapped := kubernetes.MapNode(node, now)
		coreNodes[mapped.Ref.Name] = mapped
	}
	for index := range configuration.WorkerNodes {
		resource := &configuration.WorkerNodes[index]
		capacityGPU := numberInt(resource.Spec["gpu"])
		capacityConcurrency := numberInt(resource.Spec["maxConcurrency"])
		usedGPU := numberInt(resource.Status["usedGPU"])
		usedConcurrency := numberInt(resource.Status["usedConcurrency"])
		resource.Derived["freeGPU"] = max(0, capacityGPU-usedGPU)
		resource.Derived["freeConcurrency"] = max(0, capacityConcurrency-usedConcurrency)
		resource.Derived["gpuUtilization"] = ratio(usedGPU, capacityGPU)
		resource.Derived["concurrencyUtilization"] = ratio(usedConcurrency, capacityConcurrency)
		if node, exists := coreNodes[resource.Ref.Name]; exists {
			resource.Derived["coreNodeLinked"] = true
			resource.Derived["coreNodeReady"] = node.Ready
			resource.Derived["coreNodeSchedulable"] = node.Schedulable
		} else {
			resource.Derived["coreNodeLinked"] = false
		}
	}
}

func (aggregator *Aggregator) Workloads(now time.Time) model.Workloads {
	workloads := model.Workloads{
		Nodes:       make([]model.ClusterNode, 0),
		Pods:        make([]model.Pod, 0),
		Deployments: make([]model.Deployment, 0),
		Services:    make([]model.Service, 0),
		Leases:      make([]model.Lease, 0),
		Events:      make([]model.Event, 0),
	}
	for _, node := range aggregator.cache.ListNodes() {
		workloads.Nodes = append(workloads.Nodes, kubernetes.MapNode(node, now))
	}
	for _, pod := range aggregator.cache.ListPods() {
		workloads.Pods = append(workloads.Pods, kubernetes.MapPod(pod))
	}
	for _, deployment := range aggregator.cache.ListDeployments() {
		workloads.Deployments = append(workloads.Deployments, kubernetes.MapDeployment(deployment))
	}
	for _, service := range aggregator.cache.ListServices() {
		workloads.Services = append(workloads.Services, kubernetes.MapService(service))
	}
	for _, lease := range aggregator.cache.ListLeases() {
		workloads.Leases = append(workloads.Leases, kubernetes.MapLease(lease, now))
	}
	for _, event := range aggregator.cache.ListEvents() {
		workloads.Events = append(workloads.Events, kubernetes.MapEvent(event))
	}
	kubernetes.SortWorkloads(&workloads)
	if len(workloads.Events) > 500 {
		workloads.Events = workloads.Events[:500]
	}
	return workloads
}

func (aggregator *Aggregator) Traffic(now time.Time) model.Traffic {
	tenants := aggregator.cache.ListPlatform("Tenant")
	instances := aggregator.cache.ListPlatform("SimulatorInstance")
	performance := aggregator.cache.ListPlatform("TenantPerformance")
	runtimes := aggregator.cache.ListPlatform("TenantRuntime")

	podsByInstance := make(map[string][]model.Pod)
	for _, pod := range aggregator.cache.ListPods() {
		mapped := kubernetes.MapPod(pod)
		if mapped.SimulatorInstance != "" {
			podsByInstance[mapped.SimulatorInstance] = append(podsByInstance[mapped.SimulatorInstance], mapped)
		}
	}
	performanceByTenant := indexByRef(performance, "tenantRef")
	runtimeByTenant := indexByRef(runtimes, "tenantRef")
	instancesByTenant := make(map[string][]*unstructured.Unstructured)
	for _, instance := range instances {
		instancesByTenant[nestedObjectRefName(instance, "tenantRef")] = append(instancesByTenant[nestedObjectRefName(instance, "tenantRef")], instance)
	}

	result := model.Traffic{AsOf: now, Tenants: make([]model.TenantTraffic, 0, len(tenants))}
	for _, tenant := range tenants {
		tenantName := tenant.GetName()
		tenantTraffic := model.TenantTraffic{
			Tenant: model.ResourceRef{
				APIVersion: "platform.study.com/v1",
				Kind:       "Tenant",
				Name:       tenantName,
				UID:        string(tenant.GetUID()),
			},
			DisplayName:  nestedString(tenant.Object, "spec", "displayName"),
			Priority:     nestedString(tenant.Object, "spec", "priority"),
			RequestedQPS: nestedNumberInt(tenant.Object, "spec", "qps"),
			Performance:  model.Performance{Freshness: "notReported"},
			Instances:    make([]model.TrafficInstance, 0),
		}

		if performanceObject := performanceByTenant[tenantName]; performanceObject != nil {
			tenantTraffic.Performance = mapPerformance(performanceObject, now)
		}
		if runtimeObject := runtimeByTenant[tenantName]; runtimeObject != nil {
			tenantTraffic.ReadyReplicaCount = nestedNumberInt(runtimeObject.Object, "status", "instanceCount")
			tenantTraffic.RuntimePhase = nestedString(runtimeObject.Object, "status", "phase")
		}

		availableFromInstances := 0
		for _, instance := range instancesByTenant[tenantName] {
			mapped := mapTrafficInstance(instance, podsByInstance[instance.GetName()], now)
			tenantTraffic.Instances = append(tenantTraffic.Instances, mapped)
			tenantTraffic.AllocatedQPS += mapped.AssignedQPS
			availableFromInstances += mapped.AvailableReplicas
		}
		if runtimeByTenant[tenantName] == nil {
			tenantTraffic.ReadyReplicaCount = availableFromInstances
		}
		tenantTraffic.AllocationBalanced = tenantTraffic.AllocatedQPS == tenantTraffic.RequestedQPS
		sort.Slice(tenantTraffic.Instances, func(i, j int) bool {
			return tenantTraffic.Instances[i].Name < tenantTraffic.Instances[j].Name
		})
		result.Tenants = append(result.Tenants, tenantTraffic)
	}
	sort.Slice(result.Tenants, func(i, j int) bool { return result.Tenants[i].Tenant.Name < result.Tenants[j].Tenant.Name })
	return result
}

func (aggregator *Aggregator) CurrentSnapshot(now time.Time) model.CurrentSnapshot {
	return model.CurrentSnapshot{
		CapturedAt:    now,
		Configuration: aggregator.Configuration(now),
		Traffic:       aggregator.Traffic(now),
		Workloads:     aggregator.Workloads(now),
	}
}

func (aggregator *Aggregator) Counts(configuration model.Configuration, traffic model.Traffic, workloads model.Workloads) map[string]int {
	readyReplicas := 0
	for _, tenant := range traffic.Tenants {
		readyReplicas += tenant.ReadyReplicaCount
	}
	readyNodes := 0
	for _, node := range workloads.Nodes {
		if node.Ready {
			readyNodes++
		}
	}
	return map[string]int{
		"tenants":            len(configuration.Tenants),
		"models":             len(configuration.Models),
		"workerNodes":        len(configuration.WorkerNodes),
		"simulatorInstances": len(configuration.SimulatorInstances),
		"readyReplicas":      readyReplicas,
		"nodes":              len(workloads.Nodes),
		"readyNodes":         readyNodes,
		"pods":               len(workloads.Pods),
		"deployments":        len(workloads.Deployments),
	}
}

func mapTrafficInstance(instance *unstructured.Unstructured, pods []model.Pod, now time.Time) model.TrafficInstance {
	observedAt, hasObservedAt := nestedTime(instance.Object, "status", "observedAt")
	freshness := "notReported"
	if hasObservedAt {
		age := now.Sub(observedAt)
		if age < 0 {
			freshness = "clockSkew"
		} else if age <= 30*time.Second {
			freshness = "fresh"
		} else {
			freshness = "stale"
		}
	}
	return model.TrafficInstance{
		Name:              instance.GetName(),
		Model:             nestedString(instance.Object, "spec", "modelRef", "name"),
		DesiredReplicas:   nestedNumberInt(instance.Object, "spec", "replicas"),
		AvailableReplicas: nestedNumberInt(instance.Object, "status", "availableReplicas"),
		AssignedQPS:       nestedNumberInt(instance.Object, "spec", "traffic", "qps"),
		Score:             nestedOptionalInt64(instance.Object, "status", "score"),
		EffectiveScore:    nestedOptionalInt64(instance.Object, "status", "effectiveScore"),
		Phase:             nestedString(instance.Object, "status", "phase"),
		ObservedAt:        optionalParsedTime(observedAt, hasObservedAt),
		Freshness:         freshness,
		Pods:              pods,
	}
}

func mapPerformance(object *unstructured.Unstructured, now time.Time) model.Performance {
	result := model.Performance{
		SampleCount: nestedNumberInt(object.Object, "status", "sampleCount"),
		Phase:       nestedString(object.Object, "status", "phase"),
		Freshness:   "notReported",
	}
	if value, found := nestedNumber(object.Object, "status", "performance", "avgTTFT", "value"); found {
		result.AvgTTFT = &model.NumberValue{Value: value, Unit: nestedString(object.Object, "status", "performance", "avgTTFT", "unit")}
	}
	if value, found := nestedNumber(object.Object, "status", "performance", "avgQueue", "value"); found {
		result.AvgQueue = &model.NumberValue{Value: value, Unit: nestedString(object.Object, "status", "performance", "avgQueue", "unit")}
	}
	if observedAt, found := nestedTime(object.Object, "status", "observedAt"); found {
		result.ObservedAt = &observedAt
		age := now.Sub(observedAt)
		switch {
		case age < 0:
			result.Freshness = "clockSkew"
		case age <= 30*time.Second:
			result.Freshness = "fresh"
		default:
			result.Freshness = "stale"
		}
	}
	return result
}

func indexByRef(objects []*unstructured.Unstructured, refField string) map[string]*unstructured.Unstructured {
	result := make(map[string]*unstructured.Unstructured, len(objects))
	for _, object := range objects {
		if name := nestedObjectRefName(object, refField); name != "" {
			result[name] = object
		}
	}
	return result
}

func nestedObjectRefName(object *unstructured.Unstructured, field string) string {
	return nestedString(object.Object, "spec", field, "name")
}

func nestedRefName(object map[string]any, field string) string {
	reference, ok := object[field].(map[string]any)
	if !ok {
		return ""
	}
	return text(reference["name"])
}

func nestedInt(object map[string]any, path ...string) int {
	current := any(object)
	for _, segment := range path {
		mapping, ok := current.(map[string]any)
		if !ok {
			return 0
		}
		current = mapping[segment]
	}
	return numberInt(current)
}

func nestedString(object map[string]any, fields ...string) string {
	value, found, _ := unstructured.NestedString(object, fields...)
	if !found {
		return ""
	}
	return value
}

func nestedNumberInt(object map[string]any, fields ...string) int {
	value, found := nestedNumber(object, fields...)
	if !found {
		return 0
	}
	return int(value)
}

func nestedNumber(object map[string]any, fields ...string) (float64, bool) {
	value, found, _ := unstructured.NestedFieldNoCopy(object, fields...)
	if !found {
		return 0, false
	}
	switch typed := value.(type) {
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	default:
		return 0, false
	}
}

func nestedOptionalInt64(object map[string]any, fields ...string) *int64 {
	value, found := nestedNumber(object, fields...)
	if !found {
		return nil
	}
	result := int64(value)
	return &result
}

func nestedTime(object map[string]any, fields ...string) (time.Time, bool) {
	raw := nestedString(object, fields...)
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func optionalParsedTime(value time.Time, present bool) *time.Time {
	if !present {
		return nil
	}
	return &value
}

func numberInt(value any) int {
	switch typed := value.(type) {
	case int64:
		return int(typed)
	case int32:
		return int(typed)
	case int:
		return typed
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	default:
		return 0
	}
}

func text(value any) string {
	if result, ok := value.(string); ok {
		return result
	}
	return ""
}

func ratio(used, capacity int) float64 {
	if capacity <= 0 {
		return 0
	}
	return float64(used) / float64(capacity)
}

func ValidateCurrent(cache *kubernetes.Cache) error {
	if cache == nil {
		return fmt.Errorf("Kubernetes cache is nil")
	}
	if !cache.Synced() {
		return fmt.Errorf("Kubernetes informer cache is not synchronized")
	}
	return nil
}
