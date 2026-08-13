package observability

import (
	"context"
	"testing"
)

// 没有设置任何 OTLP 端点时，tracing 应该以 no-op 方式运行，不报错也不创建实际导出器。
func TestSetupTracingIsNoOpWithoutEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	t.Setenv("OTEL_SDK_DISABLED", "false")

	shutdown, err := SetupTracing(context.Background(), "test-service", "test")
	if err != nil {
		t.Fatalf("setup disabled tracing: %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown disabled tracing: %v", err)
	}
}

// 没有 span 的 context 应返回空 TraceID，不能 panic。
func TestTraceIDWithoutSpanIsEmpty(t *testing.T) {
	if got := TraceID(context.Background()); got != "" {
		t.Fatalf("TraceID() = %q, want empty", got)
	}
}

// 覆盖采样器环境变量的解析，包括默认、固定策略、比例以及异常输入。
func TestSamplerFromEnvironment(t *testing.T) {
	tests := []struct {
		name      string
		sampler   string
		argument  string
		wantError bool
	}{
		{name: "default"},
		{name: "always on", sampler: "always_on"},
		{name: "always off", sampler: "always_off"},
		{name: "ratio", sampler: "traceidratio", argument: "0.25"},
		{name: "parent ratio", sampler: "parentbased_traceidratio", argument: "1"},
		{name: "bad ratio", sampler: "traceidratio", argument: "1.5", wantError: true},
		{name: "unsupported", sampler: "jaeger_remote", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("OTEL_TRACES_SAMPLER", test.sampler)
			t.Setenv("OTEL_TRACES_SAMPLER_ARG", test.argument)
			sampler, err := samplerFromEnvironment()
			if test.wantError {
				if err == nil {
					t.Fatalf("samplerFromEnvironment() = %v, want error", sampler)
				}
				return
			}
			if err != nil || sampler == nil {
				t.Fatalf("samplerFromEnvironment() = %v, %v; want non-nil sampler", sampler, err)
			}
		})
	}
}
