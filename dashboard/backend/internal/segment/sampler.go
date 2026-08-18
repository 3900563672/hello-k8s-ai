// Package segment 实现切面（Segment）混合采样器（issue #51）。
// 基线模式按固定间隔采样，检测到关键事件后进入高保真窗口加密采样，
// 事件平静后回到基线。采样器只处理 running 状态的切面，生命周期由 API 层驱动。
package segment

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	prometheusprovider "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/prometheus"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

const (
	severityInfo    = "info"
	severityWarning = "warning"
	instanceEntity  = "SimulatorInstance/"
)

// Config 混合采样参数；由 config.Persistence 提供默认值，环境变量可覆盖。
type Config struct {
	BaselineInterval   time.Duration // 平静期基线采样间隔
	BurstInterval      time.Duration // 高保真窗口采样间隔（也是调度器最小节拍）
	QuiescenceWindow   time.Duration // 关键事件后保持高保真的时长
	BurstReplicaDelta  int           // 副本数突变阈值，达到即触发 burst 事件
	ErrorRateThreshold float64       // 错误率均值阈值，超过记录 error 事件
	TTFTThresholdMS    float64       // TTFT p95 阈值（ms），超过记录 alert 事件
}

// MetricSource 抽象 Prometheus 区间查询，便于单元测试注入假数据源。
type MetricSource interface {
	QueryRange(context.Context, prometheusprovider.Query) (model.MetricResult, error)
}

// SnapshotSource 抽象全局状态快照，便于单元测试注入假快照。
type SnapshotSource interface {
	CurrentSnapshot(time.Time) model.CurrentSnapshot
}

// Sampler 是切面的后台采样器：自动发现 running 切面、混合采样、终态冲刷分桶。
// 所有状态只由 Run 的单个 goroutine 读写，不需要额外加锁。
type Sampler struct {
	database       store.Store
	metricSource   MetricSource
	snapshotSource SnapshotSource
	logger         *slog.Logger
	config         Config

	active map[string]*activeSegment
}

// activeSegment 是采样器对单个 running 切面的内存状态。
type activeSegment struct {
	record        model.SegmentRecord
	lastEventPoll time.Time // 资源事件游标：只拉取新事件
	lastSample    time.Time // 上次实际采样时间
	lastCritical  time.Time // 最近一次关键事件时间，决定是否处于高保真窗口
	lastReplicas  map[string]int
	lastDecisions map[string]string // 决策去重指纹
	buckets       map[string]*bucketAccumulator
}

// bucketAccumulator 累积某个指标在一个 1 分钟桶内的采样点。
type bucketAccumulator struct {
	metricName string
	start      time.Time
	end        time.Time
	values     []float64
	seen       map[int64]struct{} // 按秒去重，避免重叠查询重复计入
}

func (acc *bucketAccumulator) add(value float64, at time.Time) {
	key := at.UTC().Unix()
	if _, exists := acc.seen[key]; exists {
		return
	}
	acc.seen[key] = struct{}{}
	acc.values = append(acc.values, value)
}

func (acc *bucketAccumulator) complete(now time.Time) bool {
	return !now.Before(acc.end)
}

func (acc *bucketAccumulator) snapshot() store.MetricBucket {
	values := append([]float64(nil), acc.values...)
	sort.Float64s(values)
	if len(values) == 0 {
		return store.MetricBucket{
			MetricName:  acc.metricName,
			BucketStart: acc.start,
			BucketEnd:   acc.end,
		}
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return store.MetricBucket{
		MetricName:  acc.metricName,
		BucketStart: acc.start,
		BucketEnd:   acc.end,
		Min:         values[0],
		Max:         values[len(values)-1],
		Avg:         sum / float64(len(values)),
		P95:         percentile(values, 0.95),
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	index := int(math.Ceil(p*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

// New 创建采样器。config 必须由调用方提供非零值（config.Load 已保证）。
func New(config Config, database store.Store, metricSource MetricSource, snapshotSource SnapshotSource, logger *slog.Logger) *Sampler {
	return &Sampler{
		database:       database,
		metricSource:   metricSource,
		snapshotSource: snapshotSource,
		logger:         logger,
		config:         config,
		active:         make(map[string]*activeSegment),
	}
}

// Run 以最小节拍（高保真间隔）运行采样循环，直到 ctx 取消。
func (sampler *Sampler) Run(ctx context.Context) {
	ticker := time.NewTicker(sampler.config.BurstInterval)
	defer ticker.Stop()
	sampler.logger.Info("Segment sampler started",
		"baselineInterval", sampler.config.BaselineInterval,
		"burstInterval", sampler.config.BurstInterval,
		"quiescenceWindow", sampler.config.QuiescenceWindow,
		"burstReplicaDelta", sampler.config.BurstReplicaDelta)
	for {
		select {
		case <-ctx.Done():
			sampler.logger.Info("Segment sampler stopped")
			return
		case <-ticker.C:
			sampler.tick(ctx)
		}
	}
}

// tick 同步 running 切面集合，并对到期切面执行一次采样。
func (sampler *Sampler) tick(ctx context.Context) {
	now := time.Now().UTC()
	records, err := sampler.database.ListSegments(ctx, 100, string(model.SegmentRunning))
	if err != nil {
		sampler.logger.Warn("Segment sampler could not list running segments", "error", err)
		return
	}
	sampler.syncActive(ctx, records, now)
	for _, state := range sampler.active {
		if !state.due(now, sampler.config) {
			continue
		}
		sampler.sample(ctx, state, now)
	}
}

// syncActive 注册新出现的 running 切面；对已离开 running 的切面冲刷内存分桶。
// 后端重启后会自动恢复对残留 running 切面的采样（自愈，无需人工干预）。
func (sampler *Sampler) syncActive(ctx context.Context, records []model.SegmentRecord, now time.Time) {
	running := make(map[string]model.SegmentRecord, len(records))
	for _, record := range records {
		running[record.SegmentID] = record
	}
	for id, state := range sampler.active {
		if _, stillRunning := running[id]; stillRunning {
			state.record = running[id]
			continue
		}
		sampler.flushSegment(ctx, state, now)
		delete(sampler.active, id)
		sampler.logger.Info("Segment sampler stopped tracking", "segmentId", id)
	}
	for id, record := range running {
		if _, exists := sampler.active[id]; exists {
			continue
		}
		sampler.active[id] = &activeSegment{
			record:        record,
			lastEventPoll: now.Add(-sampler.config.BaselineInterval),
			lastSample:    now.Add(-sampler.config.BaselineInterval),
			lastReplicas:  make(map[string]int),
			lastDecisions: make(map[string]string),
			buckets:       make(map[string]*bucketAccumulator),
		}
		sampler.logger.Info("Segment sampler started tracking", "segmentId", id, "tenant", record.Tenant, "name", record.Name)
	}
}

// due 判断切面是否需要在本次节拍采样：高保真窗口内每个节拍都采样，
// 否则按基线间隔采样。
func (state *activeSegment) due(now time.Time, config Config) bool {
	if now.Sub(state.lastCritical) < config.QuiescenceWindow {
		return true
	}
	return now.Sub(state.lastSample) >= config.BaselineInterval
}

// sample 执行一次完整采样：资源事件分类、副本数演化、指标聚合与分桶冲刷。
func (sampler *Sampler) sample(ctx context.Context, state *activeSegment, now time.Time) {
	sampler.collectResourceEvents(ctx, state, now)
	sampler.collectReplicaChanges(ctx, state, now)
	sampler.collectMetrics(ctx, state, now)
	sampler.flushCompletedBuckets(ctx, state, now)
	state.lastSample = now
}

// collectResourceEvents 拉取自上次轮询以来的资源事件并分类为切面事件。
func (sampler *Sampler) collectResourceEvents(ctx context.Context, state *activeSegment, now time.Time) {
	changes, err := sampler.database.ListResourceEvents(ctx, state.lastEventPoll, 500)
	if err != nil {
		sampler.logger.Warn("Segment sampler could not list resource events", "segmentId", state.record.SegmentID, "error", err)
		return
	}
	state.lastEventPoll = now
	for _, change := range changes {
		event, critical := classifyResourceEvent(change, state)
		if event == nil {
			continue
		}
		event.SegmentID = state.record.SegmentID
		if err := sampler.database.RecordSegmentEvent(ctx, *event); err != nil {
			sampler.logger.Warn("Segment sampler could not record segment event", "segmentId", state.record.SegmentID, "eventType", event.EventType, "error", err)
			continue
		}
		if critical {
			state.lastCritical = now
		}
	}
}

// classifyResourceEvent 把一条资源事件映射为切面事件：
//   - TimelineGap（recorder 写入的丢弃/写失败记录）→ gap；
//   - Orchestrator.status.lastScaling 变化 → decision（扩缩容决策序列，按指纹去重）；
//   - SimulatorInstance spec（replicas/qps）变化 → decision（驱动调度的输入变化）。
//
// Pod 个体事件不在监听范围内，切面只记录群体演化，防止事件风暴。
func classifyResourceEvent(change model.ResourceChange, state *activeSegment) (*model.SegmentEvent, bool) {
	entity := change.Ref.Kind + "/" + change.Ref.Name
	switch change.Ref.Kind {
	case "TimelineGap":
		return &model.SegmentEvent{
			EventType:  model.SegmentEventGap,
			OccurredAt: change.OccurredAt,
			Entity:     entity,
			Severity:   severityWarning,
			Payload:    change.Payload,
		}, true
	case "Orchestrator":
		payload, fingerprint := scalingDecision(change)
		if payload == nil {
			return nil, false
		}
		if state.lastDecisions[entity] == fingerprint {
			return nil, false
		}
		state.lastDecisions[entity] = fingerprint
		return &model.SegmentEvent{
			EventType:  model.SegmentEventDecision,
			OccurredAt: change.OccurredAt,
			Entity:     entity,
			Severity:   severityInfo,
			Payload:    payload,
		}, true
	case "SimulatorInstance":
		payload, fingerprint := instanceSpecDecision(change)
		if payload == nil {
			return nil, false
		}
		key := "spec/" + entity
		if state.lastDecisions[key] == fingerprint {
			return nil, false
		}
		state.lastDecisions[key] = fingerprint
		return &model.SegmentEvent{
			EventType:  model.SegmentEventDecision,
			OccurredAt: change.OccurredAt,
			Entity:     entity,
			Severity:   severityInfo,
			Payload:    payload,
		}, true
	}
	return nil, false
}

// scalingDecision 提取 Orchestrator 的最近一次扩缩记录；没有真实扩缩动作时返回空。
func scalingDecision(change model.ResourceChange) (json.RawMessage, string) {
	var object struct {
		Status struct {
			LastScaling *struct {
				Action       string `json:"action"`
				InstanceName string `json:"instanceName"`
				OldReplicas  int    `json:"oldReplicas"`
				NewReplicas  int    `json:"newReplicas"`
			} `json:"lastScaling"`
		} `json:"status"`
	}
	if err := json.Unmarshal(change.Payload, &object); err != nil || object.Status.LastScaling == nil {
		return nil, ""
	}
	scaling := object.Status.LastScaling
	payload, _ := json.Marshal(map[string]any{
		"action":       scaling.Action,
		"instanceName": scaling.InstanceName,
		"oldReplicas":  scaling.OldReplicas,
		"newReplicas":  scaling.NewReplicas,
	})
	fingerprint := fmt.Sprintf("%s|%s|%d|%d", scaling.Action, scaling.InstanceName, scaling.OldReplicas, scaling.NewReplicas)
	return payload, fingerprint
}

// instanceSpecDecision 提取 SimulatorInstance 的 spec 变化；纯 status 更新不算决策。
func instanceSpecDecision(change model.ResourceChange) (json.RawMessage, string) {
	var object struct {
		Spec struct {
			Replicas *int `json:"replicas"`
			Traffic  struct {
				QPS *int `json:"qps"`
			} `json:"traffic"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(change.Payload, &object); err != nil {
		return nil, ""
	}
	replicas := object.Spec.Replicas
	qps := object.Spec.Traffic.QPS
	if replicas == nil && qps == nil {
		return nil, ""
	}
	replicaValue, qpsValue := 0, 0
	if replicas != nil {
		replicaValue = *replicas
	}
	if qps != nil {
		qpsValue = *qps
	}
	payload, _ := json.Marshal(map[string]any{
		"replicas": replicaValue,
		"qps":      qpsValue,
	})
	fingerprint := fmt.Sprintf("replicas=%d|qps=%d", replicaValue, qpsValue)
	return payload, fingerprint
}

// collectReplicaChanges 对比当前快照与上次采样：副本数变化记 phase_change，
// 变化量达到阈值记 burst 并进入高保真窗口。
func (sampler *Sampler) collectReplicaChanges(ctx context.Context, state *activeSegment, now time.Time) {
	replicas := make(map[string]int)
	snapshot := sampler.snapshotSource.CurrentSnapshot(now)
	for _, tenant := range snapshot.Traffic.Tenants {
		for _, instance := range tenant.Instances {
			replicas[instance.Name] = instance.DesiredReplicas
		}
	}
	for name, current := range replicas {
		previous, exists := state.lastReplicas[name]
		state.lastReplicas[name] = current
		if !exists || previous == current {
			continue
		}
		payload, _ := json.Marshal(map[string]any{"from": previous, "to": current})
		entity := instanceEntity + name
		sampler.recordEvent(ctx, state, model.SegmentEvent{
			EventType:  model.SegmentEventPhaseChange,
			OccurredAt: now,
			Entity:     entity,
			Severity:   severityInfo,
			Payload:    payload,
		})
		delta := current - previous
		if delta < 0 {
			delta = -delta
		}
		if delta >= sampler.config.BurstReplicaDelta {
			sampler.recordEvent(ctx, state, model.SegmentEvent{
				EventType:  model.SegmentEventBurst,
				OccurredAt: now,
				Entity:     entity,
				Severity:   severityWarning,
				Payload:    payload,
			})
			state.lastCritical = now
		}
	}
	for name := range state.lastReplicas {
		if _, exists := replicas[name]; !exists {
			delete(state.lastReplicas, name)
		}
	}
}

// collectMetrics 拉取最近窗口的指标并累积到 1 分钟桶；桶按分钟对齐，
// 同秒数据去重，避免高保真窗口的重叠查询把同一采样点重复计入。
func (sampler *Sampler) collectMetrics(ctx context.Context, state *activeSegment, now time.Time) {
	windowStart := now.Add(-2 * sampler.config.BaselineInterval)
	for _, metricID := range samplerMetricIDs {
		result, err := sampler.metricSource.QueryRange(ctx, prometheusprovider.Query{
			MetricID: metricID,
			Start:    windowStart,
			End:      now,
		})
		if err != nil {
			sampler.logger.Debug("Segment sampler metric query failed", "segmentId", state.record.SegmentID, "metricId", metricID, "error", err)
			continue
		}
		for _, series := range result.Series {
			for _, point := range series.Points {
				value := point.Value
				if math.IsNaN(value) || math.IsInf(value, 0) {
					continue
				}
				bucketStart := point.Time.UTC().Truncate(time.Minute)
				key := metricID + "|" + bucketStart.Format(time.RFC3339)
				acc := state.buckets[key]
				if acc == nil {
					acc = &bucketAccumulator{
						metricName: metricID,
						start:      bucketStart,
						end:        bucketStart.Add(time.Minute),
						seen:       make(map[int64]struct{}),
					}
					state.buckets[key] = acc
				}
				acc.add(value, point.Time)
			}
		}
	}
}

// flushCompletedBuckets 把已完整的分钟桶写入持久化存储，并做阈值检测：
// 错误率均值超阈值记 error，TTFT p95 超阈值记 alert，两者都会触发高保真窗口。
func (sampler *Sampler) flushCompletedBuckets(ctx context.Context, state *activeSegment, now time.Time) {
	var buckets []store.MetricBucket
	for key, acc := range state.buckets {
		if !acc.complete(now) {
			continue
		}
		buckets = append(buckets, acc.snapshot())
		delete(state.buckets, key)
	}
	if len(buckets) == 0 {
		return
	}
	if err := sampler.database.AppendSegmentMetrics(ctx, state.record.SegmentID, buckets); err != nil {
		sampler.logger.Warn("Segment sampler could not append segment metrics", "segmentId", state.record.SegmentID, "error", err)
		return
	}
	for _, bucket := range buckets {
		switch {
		case (bucket.MetricName == "simulator.errorRate" || bucket.MetricName == "controller.errorRate") && bucket.Avg > sampler.config.ErrorRateThreshold:
			payload, _ := json.Marshal(map[string]any{
				"metric": bucket.MetricName, "bucketStart": bucket.BucketStart, "avg": bucket.Avg, "max": bucket.Max,
			})
			sampler.recordEvent(ctx, state, model.SegmentEvent{
				EventType:  model.SegmentEventError,
				OccurredAt: now,
				Entity:     bucket.MetricName,
				Severity:   severityWarning,
				Payload:    payload,
			})
			state.lastCritical = now
		case bucket.MetricName == "simulator.ttft" && bucket.P95 > sampler.config.TTFTThresholdMS:
			payload, _ := json.Marshal(map[string]any{
				"metric": bucket.MetricName, "bucketStart": bucket.BucketStart, "p95": bucket.P95, "max": bucket.Max,
			})
			sampler.recordEvent(ctx, state, model.SegmentEvent{
				EventType:  model.SegmentEventAlert,
				OccurredAt: now,
				Entity:     bucket.MetricName,
				Severity:   severityWarning,
				Payload:    payload,
			})
			state.lastCritical = now
		}
	}
}

func (sampler *Sampler) recordEvent(ctx context.Context, state *activeSegment, event model.SegmentEvent) {
	event.SegmentID = state.record.SegmentID
	if err := sampler.database.RecordSegmentEvent(ctx, event); err != nil {
		sampler.logger.Warn("Segment sampler could not record segment event", "segmentId", state.record.SegmentID, "eventType", event.EventType, "error", err)
	}
}

// flushSegment 把切面剩余的内存分桶落库（终态冲刷，防止最后一分钟丢失）。
func (sampler *Sampler) flushSegment(ctx context.Context, state *activeSegment, now time.Time) {
	var buckets []store.MetricBucket
	for key, acc := range state.buckets {
		buckets = append(buckets, acc.snapshot())
		delete(state.buckets, key)
	}
	if len(buckets) == 0 {
		return
	}
	flushContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sampler.database.AppendSegmentMetrics(flushContext, state.record.SegmentID, buckets); err != nil {
		sampler.logger.Warn("Segment sampler could not flush segment metrics", "segmentId", state.record.SegmentID, "error", err)
	}
}

// Active 返回当前正在采样的切面数量，用于健康与调试日志。
func (sampler *Sampler) Active() int {
	return len(sampler.active)
}

// samplerMetricIDs 切面指标集合：模拟器核心指标 + 控制器错误比例。
var samplerMetricIDs = []string{
	"simulator.ttft", "simulator.queue", "simulator.qps",
	"simulator.errorRate", "simulator.tickLatency", "controller.errorRate",
}
