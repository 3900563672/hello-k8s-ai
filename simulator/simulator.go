package main

import (
	"context"
	"fmt"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	"github.com/3900563672/hello-k8s-ai/internal/observability"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	traceAttributeSimulatorInstanceName = "platform.simulator_instance.name"
	traceAttributeSimulatorReporterID   = "platform.simulator.reporter.id"
)

// Simulator 把本地模拟引擎的计算结果写回 SimulatorInstance 的状态。
type Simulator struct {
	client   client.Client
	name     string
	interval time.Duration
	engine   *SimEngine
	// simulationElapsed 只由成功读取配置后的 tick 推进，不使用墙钟差值。
	// 这样 Controller 卡顿恢复后不会一次性制造突发请求。
	simulationElapsed time.Duration
	now               func() time.Time // 可注入的时间源，测试用
	reporterID        string
	metrics           *simulatorMetrics
}

// Run 主循环，定期执行 reconcile 直到 context 结束或实例被删除。
func (s *Simulator) Run(ctx context.Context) {
	interval := s.simulationInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	klog.Infof("simulator for instance %q started", s.name)
	for {
		select {
		case <-ctx.Done():
			klog.Infof("simulator for instance %q stopped", s.name)
			return
		case <-ticker.C:
			if err := s.reconcile(ctx); err != nil {
				if apierrors.IsNotFound(err) {
					klog.Infof("SimulatorInstance %q 已不存在，退出", s.name)
					return
				}
				klog.Errorf("reconcile SimulatorInstance %q: %v", s.name, err)
			}
		}
	}
}

// reconcile 拉取实例规格，读取模型参数，跑模拟引擎，再把分数和性能写回。
func (s *Simulator) reconcile(ctx context.Context) (operationErr error) {
	started := time.Now()
	assignedQPS := 0
	availableReplicas := 0
	effectiveScore := 0
	poolScore := 0
	queueLength := 0
	avgTTFT := 0
	factor := 0.0
	timeScale := platformv1.DefaultSimulationRate
	simulationStep := time.Duration(0)
	simulationElapsed := s.simulationElapsed
	hasTTFT := false

	// 创建 trace span，记录每次模拟调用的输入输出
	ctx, span := observability.Tracer("simulator").Start(
		ctx,
		"simulator.tick",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String(traceAttributeSimulatorInstanceName, s.name),
			attribute.String(traceAttributeSimulatorReporterID, s.reporterID),
		),
	)
	defer func() {
		outcome := "success"
		if operationErr != nil {
			outcome = "error"
		}
		if s.metrics != nil {
			s.metrics.ticks.WithLabelValues(outcome).Inc()
			s.metrics.tickDuration.Observe(time.Since(started).Seconds())
		}
		observability.EndSpan(
			span,
			operationErr,
			attribute.String("simulator.tick.outcome", outcome),
			attribute.Int("traffic.assigned_qps", assignedQPS),
			attribute.Int("simulator.available_replicas", availableReplicas),
			attribute.Int("simulator.effective_score", effectiveScore),
			attribute.Int("simulator.pool_score", poolScore),
			attribute.Int("simulator.time_scale", timeScale),
			attribute.Float64("simulator.step_seconds", simulationStep.Seconds()),
			attribute.Float64("simulator.elapsed_seconds", simulationElapsed.Seconds()),
			attribute.Float64("simulator.cold_start_factor", factor),
			attribute.Int("performance.queue_depth", queueLength),
			attribute.Int("performance.ttft_ms", avgTTFT),
			attribute.Bool("performance.ttft_available", hasTTFT),
		)
	}()

	var instance platformv1.SimulatorInstance
	if err := s.client.Get(ctx, types.NamespacedName{Name: s.name}, &instance); err != nil {
		return err
	}
	// 实例正在删除，不需要再上报
	if !instance.DeletionTimestamp.IsZero() {
		return apierrors.NewNotFound(platformv1.GroupVersion.WithResource("simulatorinstances").GroupResource(), s.name)
	}

	var model platformv1.Model
	if err := s.client.Get(ctx, types.NamespacedName{Name: instance.Spec.ModelRef.Name}, &model); err != nil {
		return fmt.Errorf("get model %q: %w", instance.Spec.ModelRef.Name, err)
	}
	if model.Spec.MaxConcurrency < 1 {
		return fmt.Errorf("model %q has invalid maxConcurrency %d", model.Name, model.Spec.MaxConcurrency)
	}

	// 取 EffectiveScore，并按本轮倍速推进模拟时间。
	if instance.Status.EffectiveScore != nil {
		effectiveScore = max(0, *instance.Status.EffectiveScore)
	}
	timeScale = normalizedTimeScale(instance.Spec.TimeScale)
	simulationStep = s.advanceSimulationTime(timeScale)
	simulationElapsed = s.simulationElapsed
	factor = coldStartFactorForElapsed(simulationElapsed, int64(model.Spec.ColdStartMs))
	perReplicaScore := scaledScore(effectiveScore, factor)
	availableReplicas = max(0, instance.Status.AvailableReplicas)
	poolScore = saturatingMultiply(perReplicaScore, availableReplicas)
	assignedQPS = max(0, instance.Spec.Traffic.QPS)

	// 并发模型变了就重建引擎
	if s.engine == nil || s.engine.maxConcurrency != model.Spec.MaxConcurrency {
		s.engine = NewSimEngine(model.Spec.MaxConcurrency)
		if s.metrics != nil {
			s.metrics.engineReinitializations.Inc()
		}
	}
	performanceSpec := model.Spec.Performance
	// 把总 QPS 均摊到每个副本
	perReplicaQPS := 0.0
	if availableReplicas > 0 {
		perReplicaQPS = float64(instance.Spec.Traffic.QPS) / float64(availableReplicas)
	}

	// 驱动模拟引擎前进一步
	avgTTFT, queueLength, hasTTFT = s.engine.StepRate(
		simulationStep,
		perReplicaQPS,
		perReplicaScore,
		effectiveScore,
		performanceSpec.PrefillBaseMs,
		performanceSpec.PrefillPerTokenUs,
		performanceSpec.DecodePerTokenMs,
		factor,
	)

	// 只在有 QPS 且有可用副本时才输出性能数据，否则置空
	var performance *platformv1.InstancePerformance
	if instance.Spec.Traffic.QPS > 0 && availableReplicas > 0 {
		performance = &platformv1.InstancePerformance{
			Queue: &platformv1.InstancePerformanceMetric{Value: queueLength, Unit: "requests"},
		}
		if hasTTFT {
			performance.TTFT = &platformv1.InstancePerformanceMetric{Value: avgTTFT, Unit: "ms"}
		}
	}

	// 更新 Prometheus 指标
	if s.metrics != nil {
		s.metrics.assignedQPS.Set(float64(assignedQPS))
		s.metrics.availableReplicas.Set(float64(availableReplicas))
		s.metrics.effectiveScore.Set(float64(effectiveScore))
		s.metrics.poolScore.Set(float64(poolScore))
		s.metrics.coldStartFactor.Set(factor)
		s.metrics.timeScale.Set(float64(timeScale))
		s.metrics.simulationStepSeconds.Set(simulationStep.Seconds())
		s.metrics.simulationElapsedSeconds.Set(simulationElapsed.Seconds())
		s.metrics.queueDepth.Set(float64(queueLength))
		if hasTTFT {
			s.metrics.ttftSeconds.Set(float64(avgTTFT) / 1000)
		} else {
			s.metrics.ttftSeconds.Set(0)
		}
	}

	err := s.updateOwnedStatus(ctx, poolScore, performance, metav1.NewTime(s.currentTime()))
	if s.metrics != nil {
		outcome := "success"
		if err != nil {
			outcome = "error"
		}
		s.metrics.statusUpdates.WithLabelValues(outcome).Inc()
	}
	return err
}

// updateOwnedStatus 只更新模拟器负责的字段 (Score, Performance, ObservedAt, ReporterID)，
// 不会覆盖 Phase / Conditions / AvailableReplicas。
func (s *Simulator) updateOwnedStatus(
	ctx context.Context,
	score int,
	performance *platformv1.InstancePerformance,
	observedAt metav1.Time,
) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest platformv1.SimulatorInstance
		if err := s.client.Get(ctx, types.NamespacedName{Name: s.name}, &latest); err != nil {
			return err
		}
		before := latest.DeepCopy()
		latest.Status.Score = new(score)
		latest.Status.Performance = performance
		latest.Status.ObservedAt = observedAt.DeepCopy()
		latest.Status.ReporterID = s.reporterID
		return s.client.Status().Patch(ctx, &latest, client.MergeFrom(before))
	})
}

// scaledScore 计算衰减后的单副本分数，避免溢出。
func scaledScore(score int, factor float64) int {
	if score <= 0 || factor <= 0 {
		return 0
	}
	maxInt := int(^uint(0) >> 1)
	value := float64(score) * factor
	if value >= float64(maxInt) {
		return maxInt
	}
	scaled := int(value)
	if scaled == 0 {
		return 1
	}
	return scaled
}

// saturatingMultiply 饱和乘法，溢出时返回 MaxInt。
func saturatingMultiply(left, right int) int {
	if left <= 0 || right <= 0 {
		return 0
	}
	maxInt := int(^uint(0) >> 1)
	if left > maxInt/right {
		return maxInt
	}
	return left * right
}

func (s *Simulator) currentTime() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *Simulator) simulationInterval() time.Duration {
	if s.interval <= 0 {
		return 5 * time.Second
	}
	return s.interval
}

func normalizedTimeScale(rate int) int {
	if rate < platformv1.DefaultSimulationRate {
		return platformv1.DefaultSimulationRate
	}
	if rate > platformv1.MaxSimulationRate {
		return platformv1.MaxSimulationRate
	}
	return rate
}

// advanceSimulationTime 按固定 tick 推进模拟时间。
// 使用固定步长而不是实际墙钟间隔，可避免进程暂停后产生补偿性流量尖峰。
func (s *Simulator) advanceSimulationTime(rate int) time.Duration {
	step := scaleDuration(s.simulationInterval(), float64(normalizedTimeScale(rate)))
	s.simulationElapsed = addDurationSaturated(s.simulationElapsed, step)
	return step
}
