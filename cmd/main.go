/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	// 注册 Kubernetes 客户端认证插件，使命令入口可使用 Azure、GCP、OIDC 等认证方式。
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	"github.com/3900563672/hello-k8s-ai/internal/controller"
	"github.com/3900563672/hello-k8s-ai/internal/observability"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(platformv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func main() {
	if err := run(); err != nil {
		setupLog.Error(err, "manager exited")
		os.Exit(1)
	}
}

// nolint:gocyclo
func run() error {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	var simulatorNamespace, simulatorImage, simulatorServiceAccount, simulatorPullPolicy string
	var simulatorMetricsPort int
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "Metrics 端点监听地址。"+
		"HTTPS 使用 :8443，HTTP 使用 :8080，设为 0 时关闭 Metrics 服务。")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "健康探针监听地址。")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"为 Controller Manager 启用选主，确保同一时间只有一个实例处于活动状态。")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"通过 HTTPS 提供 Metrics 端点；使用 --metrics-secure=false 可改为 HTTP。")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "Webhook 证书所在目录。")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "Webhook 证书文件名。")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "Webhook 私钥文件名。")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "",
		"Metrics Server 证书所在目录。")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "Metrics Server 证书文件名。")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "Metrics Server 私钥文件名。")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"为 Metrics 和 Webhook Server 启用 HTTP/2。")
	flag.StringVar(
		&simulatorNamespace,
		"simulator-namespace",
		firstNonEmpty(os.Getenv("SIMULATOR_NAMESPACE"), os.Getenv("POD_NAMESPACE"), "default"),
		"Simulator Deployment 所在的 Namespace。",
	)
	flag.StringVar(
		&simulatorImage,
		"simulator-image",
		firstNonEmpty(os.Getenv("SIMULATOR_IMAGE"), "simulator:latest"),
		"Simulator Deployment 使用的容器镜像。",
	)
	flag.StringVar(
		&simulatorServiceAccount,
		"simulator-service-account",
		firstNonEmpty(
			os.Getenv("SIMULATOR_SERVICE_ACCOUNT"),
			derivedSimulatorServiceAccount(os.Getenv("POD_SERVICE_ACCOUNT")),
		),
		"Simulator Pod 使用的 ServiceAccount。",
	)
	flag.StringVar(
		&simulatorPullPolicy,
		"simulator-image-pull-policy",
		firstNonEmpty(os.Getenv("SIMULATOR_IMAGE_PULL_POLICY"), string(corev1.PullIfNotPresent)),
		"Simulator Deployment 使用的镜像拉取策略。",
	)
	flag.IntVar(
		&simulatorMetricsPort,
		"simulator-metrics-port",
		9090,
		"Simulator Metrics 与健康检查端点使用的容器端口。",
	)
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	pullPolicy := corev1.PullPolicy(simulatorPullPolicy)
	if pullPolicy != corev1.PullAlways && pullPolicy != corev1.PullIfNotPresent && pullPolicy != corev1.PullNever {
		return fmt.Errorf("invalid simulator image pull policy %q", simulatorPullPolicy)
	}
	if simulatorMetricsPort < 1 || simulatorMetricsPort > 65535 {
		return fmt.Errorf("simulator metrics port must be between 1 and 65535: %d", simulatorMetricsPort)
	}

	ctx := ctrl.SetupSignalHandler()
	shutdownTracing, err := observability.SetupTracing(
		ctx,
		"hello-k8s-ai-controller",
		firstNonEmpty(os.Getenv("APP_VERSION"), "dev"),
	)
	if err != nil {
		// 遥测初始化失败不能阻止控制面启动。
		setupLog.Error(err, "failed to initialize tracing; continuing without export")
		shutdownTracing = func(context.Context) error { return nil }
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdownTracing(shutdownCtx); err != nil {
			setupLog.Error(err, "failed to shut down tracing")
		}
	}()

	// 默认关闭 HTTP/2，规避 Stream Cancellation 和 Rapid Reset 漏洞：
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling HTTP/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Webhook 初始 TLS 配置。
	webhookTLSOpts := tlsOpts
	webhookServerOptions := webhook.Options{
		TLSOpts: webhookTLSOpts,
	}

	if len(webhookCertPath) > 0 {
		setupLog.Info("initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}

	webhookServer := webhook.NewServer(webhookServerOptions)

	// Metrics 端点由 config/default/kustomization.yaml 启用，此处配置服务参数：
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// 使用 FilterProvider 保护 Metrics 端点，只允许经过认证和授权的用户及 ServiceAccount 访问。
		// RBAC 配置位于 config/rbac/kustomization.yaml：
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// 未提供证书时，controller-runtime 会为 Metrics Server 生成自签名证书。
	// 该方式适合开发和测试，不建议用于生产环境。
	//
	// 启用 cert-manager 时，解除下列配置标记的注释：
	// - config/default/kustomization.yaml 中的 [METRICS-WITH-CERTS]；
	// - config/prometheus/kustomization.yaml 中的 [PROMETHEUS-WITH-CERTS]。
	if len(metricsCertPath) > 0 {
		setupLog.Info("initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("load Kubernetes REST config: %w", err)
	}
	observability.InstrumentKubernetesConfig(restConfig)

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "5e2d7bf4.platform.study.com",
		// LeaderElectionReleaseOnCancel 可在 Manager 停止时主动释放选主锁，缩短切换时间。
		// 只有在 Manager 停止后进程立即退出时才可启用；若退出前还需执行清理，启用它并不安全。
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		return fmt.Errorf("create manager: %w", err)
	}

	if err := (&controller.OrchestratorReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("set up orchestrator controller: %w", err)
	}
	if err := (&controller.SimulatorInstanceReconciler{
		Client:                   mgr.GetClient(),
		Scheme:                   mgr.GetScheme(),
		SimulatorNamespace:       simulatorNamespace,
		SimulatorImage:           simulatorImage,
		SimulatorServiceAccount:  simulatorServiceAccount,
		SimulatorImagePullPolicy: pullPolicy,
		SimulatorMetricsPort:     int32(simulatorMetricsPort),
		SimulatorObservability: controller.SimulatorObservabilityConfig{
			SDKDisabled: firstNonEmpty(
				os.Getenv("SIMULATOR_OTEL_SDK_DISABLED"),
				"false",
			),
			OTLPEndpoint: firstNonEmpty(
				os.Getenv("SIMULATOR_OTEL_EXPORTER_OTLP_ENDPOINT"),
				os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
			),
			OTLPInsecure: firstNonEmpty(
				os.Getenv("SIMULATOR_OTEL_EXPORTER_OTLP_INSECURE"),
				os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"),
				"true",
			),
			TracesSampler: firstNonEmpty(
				os.Getenv("SIMULATOR_OTEL_TRACES_SAMPLER"),
				os.Getenv("OTEL_TRACES_SAMPLER"),
				"parentbased_traceidratio",
			),
			TracesSamplerArg: firstNonEmpty(
				os.Getenv("SIMULATOR_OTEL_TRACES_SAMPLER_ARG"),
				os.Getenv("OTEL_TRACES_SAMPLER_ARG"),
				"0.2",
			),
			Environment: firstNonEmpty(
				os.Getenv("SIMULATOR_DEPLOYMENT_ENVIRONMENT"),
				os.Getenv("DEPLOYMENT_ENVIRONMENT"),
			),
			ClusterName: firstNonEmpty(
				os.Getenv("SIMULATOR_K8S_CLUSTER_NAME"),
				os.Getenv("K8S_CLUSTER_NAME"),
			),
			ServiceVersion: firstNonEmpty(
				os.Getenv("SIMULATOR_APP_VERSION"),
				os.Getenv("APP_VERSION"),
				"dev",
			),
		},
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("set up simulatorinstance controller: %w", err)
	}
	if err := (&controller.TenantModelPolicyReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("set up tenant-model-policy controller: %w", err)
	}
	if err := (&controller.PerformanceCollectorReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("set up performancecollector controller: %w", err)
	}
	if err := (&controller.TrafficReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("set up traffic controller: %w", err)
	}
	if err := (&controller.WorkerNodeUsageReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("set up workernodeusage controller: %w", err)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("set up ready check: %w", err)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("run manager: %w", err)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func derivedSimulatorServiceAccount(managerServiceAccount string) string {
	const (
		managerSuffix          = "controller-manager"
		defaultSimulatorSAName = "simulator-sa"
	)
	if prefix, found := strings.CutSuffix(managerServiceAccount, managerSuffix); found {
		return prefix + defaultSimulatorSAName
	}
	return defaultSimulatorSAName
}
