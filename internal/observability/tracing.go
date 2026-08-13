package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/client-go/rest"
)

const instrumentationName = "github.com/3900563672/hello-k8s-ai"

// ShutdownFunc 用于刷新和关闭 tracing 管线，即使 tracing 未启用也可以安全调用。
type ShutdownFunc func(context.Context) error

// SetupTracing 安装 OTLP/gRPC tracing 管线。
// 如果 OTLP 端点为空或 OTEL_SDK_DISABLED=true，则使用 no-op provider，
// 这样本地开发和遥测故障不会影响业务。
func SetupTracing(ctx context.Context, serviceName, serviceVersion string) (ShutdownFunc, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if tracingDisabled() || traceEndpoint() == "" {
		return func(context.Context) error { return nil }, nil
	}
	if strings.TrimSpace(serviceName) == "" {
		return nil, errors.New("service name is required")
	}
	// 从环境变量读取采样策略
	sampler, err := samplerFromEnvironment()
	if err != nil {
		return nil, err
	}

	// 构造 Resource，包含 service 信息和 K8s 环境变量
	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithProcess(),
		resource.WithAttributes(resourceAttributes(serviceName, serviceVersion)...),
	)
	if err != nil {
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(provider)
	return provider.Shutdown, nil
}

// Tracer 返回一个带组件名的 tracer，组件名应该是 controller 名等有界值。
func Tracer(component string) trace.Tracer {
	return otel.Tracer(instrumentationName + "/" + component)
}

// EndSpan 记录错误和可选属性，然后结束 span。
func EndSpan(span trace.Span, err error, attrs ...attribute.KeyValue) {
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "operation failed")
	}
	span.End()
}

// InstrumentKubernetesConfig 为普通的 Kubernetes API 调用加上客户端 span。
// 注意：长连接 watch 请求被排除，否则会挤满 trace 而对 reconcile 无意义。
func InstrumentKubernetesConfig(config *rest.Config) {
	if config == nil {
		return
	}
	previous := config.WrapTransport
	config.WrapTransport = func(transport http.RoundTripper) http.RoundTripper {
		if previous != nil {
			transport = previous(transport)
		}
		return otelhttp.NewTransport(
			transport,
			otelhttp.WithFilter(func(request *http.Request) bool {
				return request.URL.Query().Get("watch") != "true"
			}),
		)
	}
}

// TraceID 返回当前上下文中的 trace ID，用于日志关联。空字符串表示没有有效 span。
func TraceID(ctx context.Context) string {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return ""
	}
	return spanContext.TraceID().String()
}

func tracingDisabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_SDK_DISABLED")), "true")
}

func traceEndpoint() string {
	return firstNonEmpty(
		strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")),
		strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
	)
}

// samplerFromEnvironment 根据 OTEL_TRACES_SAMPLER 和 OTEL_TRACES_SAMPLER_ARG 创建采样器。
func samplerFromEnvironment() (sdktrace.Sampler, error) {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER")))
	if name == "" {
		name = "parentbased_always_on"
	}
	switch name {
	case "always_on":
		return sdktrace.AlwaysSample(), nil
	case "always_off":
		return sdktrace.NeverSample(), nil
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample()), nil
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample()), nil
	case "traceidratio", "parentbased_traceidratio":
		ratio, err := traceIDRatioFromEnvironment()
		if err != nil {
			return nil, err
		}
		ratioSampler := sdktrace.TraceIDRatioBased(ratio)
		if name == "parentbased_traceidratio" {
			return sdktrace.ParentBased(ratioSampler), nil
		}
		return ratioSampler, nil
	default:
		return nil, fmt.Errorf("unsupported trace sampler %q from OTEL_TRACES_SAMPLER", name)
	}
}

func traceIDRatioFromEnvironment() (float64, error) {
	raw := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG"))
	if raw == "" {
		return 1, nil
	}
	ratio, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("parse trace sampler argument %q: %w", raw, err)
	}
	if ratio < 0 || ratio > 1 {
		return 0, fmt.Errorf("trace sampler argument from OTEL_TRACES_SAMPLER_ARG must be between 0 and 1: %q", raw)
	}
	return ratio, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// resourceAttributes 构造 Resource 的属性集合，包含 K8s 相关环境变量。
func resourceAttributes(serviceName, serviceVersion string) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(valueOrDefault(serviceVersion, "dev")),
	}
	// 从环境变量注入 K8s 属性，只添加非空值
	appendEnvironmentAttribute := func(environmentName string, key attribute.Key) {
		if value := strings.TrimSpace(os.Getenv(environmentName)); value != "" {
			attrs = append(attrs, key.String(value))
		}
	}
	appendEnvironmentAttribute("POD_NAME", semconv.K8SPodNameKey)
	appendEnvironmentAttribute("POD_NAMESPACE", semconv.K8SNamespaceNameKey)
	appendEnvironmentAttribute("NODE_NAME", semconv.K8SNodeNameKey)
	appendEnvironmentAttribute("K8S_CLUSTER_NAME", semconv.K8SClusterNameKey)
	appendEnvironmentAttribute("DEPLOYMENT_ENVIRONMENT", semconv.DeploymentEnvironmentNameKey)
	appendEnvironmentAttribute("POD_NAME", semconv.ServiceInstanceIDKey)
	appendEnvironmentAttribute("SIMULATOR_INSTANCE_NAME", "platform.simulator_instance.name")
	appendEnvironmentAttribute("TENANT_NAME", "platform.tenant.name")
	appendEnvironmentAttribute("MODEL_NAME", "platform.model.name")
	return attrs
}
