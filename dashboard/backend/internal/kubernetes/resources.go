package kubernetes

import "k8s.io/apimachinery/pkg/runtime/schema"

type ResourceDescriptor struct {
	Kind         string
	Plural       string
	GVR          schema.GroupVersionResource
	UserWritable bool
}

var PlatformResources = []ResourceDescriptor{
	{Kind: "Model", Plural: "models", GVR: platformGVR("models"), UserWritable: true},
	{Kind: "WorkerNode", Plural: "workernodes", GVR: platformGVR("workernodes"), UserWritable: true},
	{Kind: "Tenant", Plural: "tenants", GVR: platformGVR("tenants"), UserWritable: true},
	{Kind: "TenantModelPolicy", Plural: "tenantmodelpolicies", GVR: platformGVR("tenantmodelpolicies"), UserWritable: true},
	{Kind: "TenantNodePolicy", Plural: "tenantnodepolicies", GVR: platformGVR("tenantnodepolicies"), UserWritable: true},
	{Kind: "ModelNodePolicy", Plural: "modelnodepolicies", GVR: platformGVR("modelnodepolicies"), UserWritable: true},
	{Kind: "Orchestrator", Plural: "orchestrators", GVR: platformGVR("orchestrators"), UserWritable: true},
	{Kind: "SimulatorInstance", Plural: "simulatorinstances", GVR: platformGVR("simulatorinstances")},
	{Kind: "TenantPerformance", Plural: "tenantperformances", GVR: platformGVR("tenantperformances")},
	{Kind: "TenantRuntime", Plural: "tenantruntimes", GVR: platformGVR("tenantruntimes")},
}

func platformGVR(resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "platform.study.com", Version: "v1", Resource: resource}
}

func DescriptorForKind(kind string) (ResourceDescriptor, bool) {
	for _, descriptor := range PlatformResources {
		if descriptor.Kind == kind {
			return descriptor, true
		}
	}
	return ResourceDescriptor{}, false
}
