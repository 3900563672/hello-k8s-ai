package model

import (
	"encoding/json"
	"time"
)

// 切面（Segment）生命周期状态：pending（配置快照已记录）→ running（混合采样进行中）
// → completed/failed（终点快照+聚合沉淀）→ 封存（永久只读，不可再变更）。
type SegmentStatus string

const (
	SegmentPending   SegmentStatus = "pending"
	SegmentRunning   SegmentStatus = "running"
	SegmentCompleted SegmentStatus = "completed"
	SegmentFailed    SegmentStatus = "failed"
)

// 切面内事件类型（六类，与 issue #51 语义一致）；Pod 个体事件不进切面。
const (
	SegmentEventDecision    = "decision"     // 扩缩容/放置决策
	SegmentEventAlert       = "alert"        // 告警触发
	SegmentEventError       = "error"        // 错误
	SegmentEventGap         = "gap"          // 时间线缺口/事件丢弃
	SegmentEventBurst       = "burst"        // 副本数快速变化
	SegmentEventPhaseChange = "phase_change" // 运行阶段变化
)

// SegmentRecord 是 segments 表的一行：一次调度实验的完整归档单元。
type SegmentRecord struct {
	SegmentID      string          `json:"segmentId"`
	Tenant         string          `json:"tenant"`
	Name           string          `json:"name"`
	Status         string          `json:"status"`
	Reason         string          `json:"reason,omitempty"`
	ConfigSnapshot json.RawMessage `json:"configSnapshot,omitempty"`
	StartSnapshot  json.RawMessage `json:"startSnapshot,omitempty"`
	EndSnapshot    json.RawMessage `json:"endSnapshot,omitempty"`
	Summary        json.RawMessage `json:"summary,omitempty"`
	StartedAt      *time.Time      `json:"startedAt,omitempty"`
	EndedAt        *time.Time      `json:"endedAt,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// SegmentEvent 是切面内的一条事件（决策/告警/错误/gap/突变/阶段变化）。
type SegmentEvent struct {
	EventID    string          `json:"eventId"`
	SegmentID  string          `json:"segmentId"`
	EventType  string          `json:"eventType"`
	OccurredAt time.Time       `json:"occurredAt"`
	Entity     string          `json:"entity,omitempty"`
	Severity   string          `json:"severity,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

// MetricBucket 是切面内某个指标在 1 分钟桶内的聚合。
type MetricBucket struct {
	MetricName  string    `json:"metricName"`
	BucketStart time.Time `json:"bucketStart"`
	BucketEnd   time.Time `json:"bucketEnd"`
	Min         float64   `json:"min"`
	Max         float64   `json:"max"`
	Avg         float64   `json:"avg"`
	P95         float64   `json:"p95"`
}

// SegmentDetail 是切面详情 API 的返回体：segments 一行 + 三个子表 + 关联 Trace。
type SegmentDetail struct {
	Segment SegmentRecord  `json:"segment"`
	Events  []SegmentEvent `json:"events"`
	Metrics []MetricBucket `json:"metrics"`
	Traces  []TraceSummary `json:"traces"`
}
