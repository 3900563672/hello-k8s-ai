package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	"github.com/3900563672/hello-k8s-ai/internal/observability"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	instanceName   string
	updateInterval time.Duration
	podName        string
	podNamespace   string
	metricsAddress string
)

func init() {
	flag.StringVar(&instanceName, "instance", os.Getenv("SIMULATOR_INSTANCE_NAME"), "SimulatorInstance 名")
	flag.DurationVar(&updateInterval, "interval", 5*time.Second, "状态模拟间隔")
	flag.StringVar(&podName, "pod-name", os.Getenv("POD_NAME"), "当前 Pod 名")
	flag.StringVar(&podNamespace, "pod-namespace", os.Getenv("POD_NAMESPACE"), "当前 Pod 命名空间")
	flag.StringVar(
		&metricsAddress,
		"metrics-bind-address",
		":9090",
		"metrics 和健康检查监听地址，0 表示禁用",
	)
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()

	if instanceName == "" {
		klog.Fatal("必须通过 --instance 或 SIMULATOR_INSTANCE_NAME 指定实例名")
	}
	if updateInterval <= 0 {
		klog.Fatalf("--interval 必须大于 0，当前值 %s", updateInterval)
	}
	if podName == "" || podNamespace == "" {
		klog.Fatal("必须通过 --pod-name/POD_NAME 和 --pod-namespace/POD_NAMESPACE 指定 Pod 标识")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 初始化 OpenTelemetry tracing，失败则用空实现
	shutdownTracing, err := observability.SetupTracing(
		ctx,
		"hello-k8s-ai-simulator",
		os.Getenv("APP_VERSION"),
	)
	if err != nil {
		klog.Errorf("初始化 OpenTelemetry tracing 失败: %v", err)
		shutdownTracing = func(context.Context) error { return nil }
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(shutdownCtx); err != nil {
			klog.Errorf("刷新 OpenTelemetry traces: %v", err)
		}
	}()

	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("加载 in-cluster Kubernetes 配置失败: %v", err)
	}
	config.UserAgent = "hello-k8s-ai-simulator"
	observability.InstrumentKubernetesConfig(config) // 给 API 调用加 trace

	scheme := runtime.NewScheme()
	if err := platformv1.AddToScheme(scheme); err != nil {
		klog.Fatalf("注册平台 API scheme 失败: %v", err)
	}
	kubernetesClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		klog.Fatalf("创建 Kubernetes client 失败: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("创建 Kubernetes clientset 失败: %v", err)
	}

	// Prometheus 指标注册
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	simulatorMetrics := newSimulatorMetrics(registry)

	// 启动可观测性 HTTP 服务
	observabilityServer := newObservabilityServer(metricsAddress, registry)
	go func() {
		if err := serveObservability(observabilityServer); err != nil {
			klog.Errorf("启动 simulator 可观测性端点失败: %v", err)
		}
	}()
	defer func() {
		if err := shutdownObservability(observabilityServer); err != nil {
			klog.Errorf("关闭 simulator 可观测性端点失败: %v", err)
		}
	}()

	// Leader election，保证同一实例只有一个 reporter 在写状态
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      reporterLeaseName(instanceName),
			Namespace: podNamespace,
		},
		Client: clientset.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: podName,
		},
	}
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		ReleaseOnCancel: true,
		Name:            "simulator-status-reporter",
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(leaderCtx context.Context) {
				simulatorMetrics.setLeader(true)
				recordLeadershipEvent(leaderCtx, "acquired", instanceName, podName)
				simulator := &Simulator{
					client:     kubernetesClient,
					name:       instanceName,
					interval:   updateInterval,
					reporterID: podName,
					metrics:    simulatorMetrics,
				}
				simulator.Run(leaderCtx)
			},
			OnStoppedLeading: func() {
				simulatorMetrics.setLeader(false)
				recordLeadershipEvent(ctx, "lost", instanceName, podName)
				klog.Infof("SimulatorInstance %q 的 reporter 租约已失去", instanceName)
			},
			OnNewLeader: func(identity string) {
				simulatorMetrics.leadershipChanges.WithLabelValues("observed").Inc()
				if identity != podName {
					klog.Infof("Pod %q 正在上报 SimulatorInstance %q 的状态", identity, instanceName)
				}
			},
		},
	})
}

// recordLeadershipEvent 记录 leader 变更事件到 trace。
func recordLeadershipEvent(ctx context.Context, event, instance, reporter string) {
	_, span := observability.Tracer("simulator").Start(
		ctx,
		"simulator.leadership."+event,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String(traceAttributeSimulatorInstanceName, instance),
			attribute.String(traceAttributeSimulatorReporterID, reporter),
			attribute.String("leadership.event", event),
		),
	)
	observability.EndSpan(span, nil)
}

// reporterLeaseName 生成 Lease 名称，超长时用哈希截断。
func reporterLeaseName(name string) string {
	const (
		prefix    = "simulator-reporter-"
		maxLength = 253
	)
	candidate := prefix + name
	if len(candidate) <= maxLength {
		return candidate
	}
	sum := sha256.Sum256([]byte(name))
	suffix := hex.EncodeToString(sum[:8])
	prefixPart := strings.TrimRight(candidate[:maxLength-len(suffix)-1], ".-")
	return prefixPart + "-" + suffix
}
