package controller

import (
	"testing"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"

	corev1 "k8s.io/api/core/v1"
)

// 测试 upsertSimulatorContainer 是否按预期注入了探针、端口、环境变量和遥测配置。
func TestUpsertSimulatorContainerAddsObservabilityContract(t *testing.T) {
	instance := &platformv1.SimulatorInstance{}
	instance.Name = "instance-a"
	instance.Spec.TenantRef.Name = "tenant-a"
	instance.Spec.ModelRef.Name = "model-a"
	podSpec := &corev1.PodSpec{}
	telemetry := SimulatorObservabilityConfig{
		SDKDisabled:      "false",
		OTLPEndpoint:     "http://collector:4317",
		OTLPInsecure:     "true",
		TracesSampler:    "parentbased_traceidratio",
		TracesSamplerArg: "0.5",
		Environment:      "kind",
		ClusterName:      "docker-desktop",
		ServiceVersion:   "test",
	}

	// 调用被测函数
	upsertSimulatorContainer(podSpec, "simulator:test", corev1.PullNever, instance, 9090, telemetry)
	if len(podSpec.Containers) != 1 {
		t.Fatalf("container count = %d, want 1", len(podSpec.Containers))
	}
	container := podSpec.Containers[0]

	// 必须配置了健康检查
	if container.LivenessProbe == nil || container.ReadinessProbe == nil {
		t.Fatal("simulator probes were not configured")
	}

	// 验证 metrics 端口
	if len(container.Ports) != 1 || container.Ports[0].Name != "metrics" || container.Ports[0].ContainerPort != 9090 {
		t.Fatalf("metrics port = %#v, want named port 9090", container.Ports)
	}

	// 验证写入的环境变量
	wantEnvironment := map[string]string{
		"SIMULATOR_INSTANCE_NAME":     "instance-a",
		"TENANT_NAME":                 "tenant-a",
		"MODEL_NAME":                  "model-a",
		"OTEL_EXPORTER_OTLP_ENDPOINT": "http://collector:4317",
		"OTEL_EXPORTER_OTLP_INSECURE": "true",
		"OTEL_SDK_DISABLED":           "false",
		"OTEL_TRACES_SAMPLER":         "parentbased_traceidratio",
		"OTEL_TRACES_SAMPLER_ARG":     "0.5",
		"DEPLOYMENT_ENVIRONMENT":      "kind",
		"K8S_CLUSTER_NAME":            "docker-desktop",
		"APP_VERSION":                 "test",
	}
	for name, want := range wantEnvironment {
		got, found := environmentValue(container.Env, name)
		if !found || got != want {
			t.Errorf("environment %s = %q, %t; want %q, true", name, got, found, want)
		}
	}

	// NODE_NAME 应该是 downward API 引用，不是直接写的值
	if node := environmentVariable(container.Env, "NODE_NAME"); node == nil ||
		node.ValueFrom == nil || node.ValueFrom.FieldRef == nil || node.ValueFrom.FieldRef.FieldPath != "spec.nodeName" {
		t.Fatalf("NODE_NAME downward API environment is not configured: %#v", node)
	}

	// 再次调用 upsertSimulatorTelemetryEnv 传空配置，验证端点被清空（允许关闭 tracing）
	upsertSimulatorTelemetryEnv(&container.Env, SimulatorObservabilityConfig{})
	if got, found := environmentValue(container.Env, "OTEL_EXPORTER_OTLP_ENDPOINT"); !found || got != "" {
		t.Fatalf("cleared OTLP endpoint = %q, %t; want empty, true", got, found)
	}
}

// environmentValue 从环境变量列表中按名字查找并返回值，found 标识是否存在。
func environmentValue(environment []corev1.EnvVar, name string) (string, bool) {
	for _, variable := range environment {
		if variable.Name == name {
			return variable.Value, true
		}
	}
	return "", false
}

// environmentVariable 返回指定名称的 EnvVar 指针，未找到返回 nil。
func environmentVariable(environment []corev1.EnvVar, name string) *corev1.EnvVar {
	for i := range environment {
		if environment[i].Name == name {
			return &environment[i]
		}
	}
	return nil
}
