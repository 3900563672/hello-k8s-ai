package controller

import (
	"context"
	"time"

	"github.com/3900563672/hello-k8s-ai/internal/observability"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	controllermetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// 各控制器 reconcile 结果计数，按控制器和结果分类
	controllerReconcileOutcomes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: componentController,
		Name:      "reconcile_outcomes_total",
		Help:      "Number of reconciliations by controller and bounded outcome.",
	}, []string{componentController, metricLabelOutcome})

	// 业务操作计数，比如创建 Deployment、更新状态等
	controllerBusinessOperations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: componentController,
		Name:      "business_operations_total",
		Help:      "Number of domain operations performed by controllers.",
	}, []string{componentController, "operation", metricLabelOutcome})

	// 扩缩容决策计数，按动作和原因
	orchestratorDecisions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: componentOrchestrator,
		Name:      "decisions_total",
		Help:      "Number of scaling decisions by action and reason.",
	}, []string{"action", "reason"})

	// 实际执行的扩缩容操作计数，按方向和结果
	orchestratorScalingOperations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: componentOrchestrator,
		Name:      "scaling_operations_total",
		Help:      "Number of persisted scaling operations by direction and outcome.",
	}, []string{"direction", metricLabelOutcome})

	// 一次决策的耗时（从收集数据到算出结果）
	orchestratorDecisionDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: componentOrchestrator,
		Name:      "decision_duration_seconds",
		Help:      "Time spent gathering inputs and calculating one scaling decision.",
		Buckets:   prometheus.DefBuckets,
	})

	// 一次快照中看到的待处理扩缩计划数量
	orchestratorPendingPlans = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: componentOrchestrator,
		Name:      "pending_scale_plans",
		Help:      "Distribution of recoverable pending scaling plans seen per snapshot.",
		Buckets:   []float64{0, 1, 2, 4, 8, 16},
	})

	// 流量分配执行次数，按结果和权重模式
	trafficAllocationRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: componentTraffic,
		Name:      "allocation_runs_total",
		Help:      "Number of traffic allocation runs by outcome and weighting mode.",
	}, []string{metricLabelOutcome, "mode"})

	// 一次流量分配的耗时
	trafficAllocationDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: componentTraffic,
		Name:      "allocation_duration_seconds",
		Help:      "Time spent collecting scores and persisting a traffic allocation.",
		Buckets:   prometheus.DefBuckets,
	})

	// 租户请求的 QPS 分布
	trafficRequestedQPS = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: componentTraffic,
		Name:      "requested_qps",
		Help:      "Distribution of tenant QPS requested in allocation runs.",
		Buckets:   []float64{0, 1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000},
	})

	// 成功分配到各实例的 QPS 分布
	trafficAllocatedQPS = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: componentTraffic,
		Name:      "allocated_qps",
		Help:      "Distribution of QPS successfully allocated in allocation runs.",
		Buckets:   []float64{0, 1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000},
	})

	// 性能采样计数，标记样本是否有效
	performanceSamples = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Subsystem: componentPerformance,
		Name:      "samples_total",
		Help:      "Number of simulator performance samples classified by result.",
	}, []string{"result"})

	// 一次租户性能聚合的耗时
	performanceAggregationDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: componentPerformance,
		Name:      "aggregation_duration_seconds",
		Help:      "Time spent aggregating a tenant performance snapshot.",
		Buckets:   prometheus.DefBuckets,
	})

	// 一次聚合中纳入的有效样本数分布
	performanceFreshSampleCount = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Subsystem: componentPerformance,
		Name:      "fresh_sample_count",
		Help:      "Distribution of fresh simulator samples in tenant aggregations.",
		Buckets:   []float64{0, 1, 2, 4, 8, 16, 32, 64},
	})

	// 各节点已用 GPU 量（从调度上去的 pod 推算）
	workerNodeGPUUnitsUsed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystemWorkerNode,
		Name:      "gpu_units_used",
		Help:      "GPU units currently derived from scheduled simulator pods.",
	}, []string{metricLabelNode})

	// 各节点已用并发数
	workerNodeConcurrencyUsed = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystemWorkerNode,
		Name:      "concurrency_used",
		Help:      "Concurrency currently derived from scheduled simulator pods.",
	}, []string{metricLabelNode})

	// 各节点物理内存水位（同名真实 Node 已分配 requests 占 allocatable 百分比）
	workerNodeMemoryUsagePercent = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystemWorkerNode,
		Name:      "memory_usage_percent",
		Help:      "Physical memory pressure percent derived from node allocatable and pod requests.",
	}, []string{metricLabelNode})

	// 各节点物理 CPU 水位
	workerNodeCPUUsagePercent = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: metricsSubsystemWorkerNode,
		Name:      "cpu_usage_percent",
		Help:      "Physical CPU pressure percent derived from node allocatable and pod requests.",
	}, []string{metricLabelNode})

	// Orchestrator 资源受限降级状态（1=任一可调度节点物理水位超阈值，扩容被挂起）
	orchestratorResourceLimited = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Subsystem: componentOrchestrator,
		Name:      "resource_limited",
		Help:      "Whether the orchestrator is in resource-limited degradation (1) or not (0).",
	}, []string{"tenant"})
)

func init() {
	// 所有指标一次性注册到 controller-runtime 的 registry 里
	controllermetrics.Registry.MustRegister(
		controllerReconcileOutcomes,
		controllerBusinessOperations,
		orchestratorDecisions,
		orchestratorScalingOperations,
		orchestratorDecisionDuration,
		orchestratorPendingPlans,
		trafficAllocationRuns,
		trafficAllocationDuration,
		trafficRequestedQPS,
		trafficAllocatedQPS,
		performanceSamples,
		performanceAggregationDuration,
		performanceFreshSampleCount,
		workerNodeGPUUnitsUsed,
		workerNodeConcurrencyUsed,
		workerNodeMemoryUsagePercent,
		workerNodeCPUUsagePercent,
		orchestratorResourceLimited,
	)
}

type reconcileObservation struct {
	controller string
	span       trace.Span
}

// beginReconcile 开始一次 reconcile，创建 span 并把 traceID 打进日志。
func beginReconcile(ctx context.Context, controllerName string, request ctrl.Request) (context.Context, *reconcileObservation) {
	// 拼几个常用的 attribute
	attrs := []attribute.KeyValue{
		attribute.String("controller.name", controllerName),
		attribute.String("k8s.resource.name", request.Name),
	}
	if request.Namespace != "" {
		attrs = append(attrs, attribute.String("k8s.namespace.name", request.Namespace))
	}

	ctx, span := observability.Tracer("controller").Start(
		ctx,
		"controller."+controllerName+".reconcile",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...),
	)

	// 如果 tracer 生成了 traceID，就挂到日志上，方便排查
	if traceID := observability.TraceID(ctx); traceID != "" {
		ctx = log.IntoContext(ctx, log.FromContext(ctx).WithValues("traceID", traceID))
	}
	return ctx, &reconcileObservation{controller: controllerName, span: span}
}

// finish 结束这次 reconcile，记指标、写 span 状态。
func (observation *reconcileObservation) finish(result ctrl.Result, err error) {
	outcome := operationOutcomeSuccess
	if err != nil {
		outcome = operationOutcomeError
	} else if result.RequeueAfter > 0 {
		outcome = "requeue" // 有 requeue 也算一种状态
	}

	// 记一笔 reconcile 结果
	controllerReconcileOutcomes.WithLabelValues(observation.controller, outcome).Inc()

	attrs := []attribute.KeyValue{attribute.String("reconcile.outcome", outcome)}
	if result.RequeueAfter > 0 {
		attrs = append(attrs, attribute.Int64("reconcile.requeue_after_ms", result.RequeueAfter.Milliseconds()))
	}
	observability.EndSpan(observation.span, err, attrs...)
}

// startOperation 开始一个子操作（比如 create deployment），创建子 span。
func startOperation(ctx context.Context, component, operation string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return observability.Tracer(component).Start(
		ctx,
		component+"."+operation,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...),
	)
}

// observeOperation 记录一次业务操作的结果（成功/失败）。
func observeOperation(controllerName, operation string, err error) {
	outcome := operationOutcomeSuccess
	if err != nil {
		outcome = operationOutcomeError
	}
	controllerBusinessOperations.WithLabelValues(controllerName, operation, outcome).Inc()
}

func durationSeconds(started time.Time) float64 {
	return time.Since(started).Seconds()
}
