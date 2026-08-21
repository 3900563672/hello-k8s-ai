package model

import (
	"encoding/json"
	"time"
)

// AIOps 分析状态机（总纲 #92/#93）：pending → running(L1) → aggregating(L2) → completed/failed。
type AIOpsAnalysisStatus string

const (
	AIOpsPending     AIOpsAnalysisStatus = "pending"
	AIOpsRunning     AIOpsAnalysisStatus = "running"
	AIOpsAggregating AIOpsAnalysisStatus = "aggregating"
	AIOpsCompleted   AIOpsAnalysisStatus = "completed"
	AIOpsFailed      AIOpsAnalysisStatus = "failed"
)

// AIOpsClassification 是 L1 实体总结的分类（前端以颜色区分：优质/可疑/问题）。
type AIOpsClassification string

const (
	AIOpsHealthy AIOpsClassification = "healthy"
	AIOpsSuspect AIOpsClassification = "suspect"
	AIOpsProblem AIOpsClassification = "problem"
)

// AIOpsAnalysis 是 aiops_analyses 表的一行：一次切面的 L1/L2 分析主记录。
type AIOpsAnalysis struct {
	AnalysisID string          `json:"analysisId"`
	SegmentID  string          `json:"segmentId"`
	Status     string          `json:"status"`
	L1Total    int             `json:"l1Total"`
	L1Done     int             `json:"l1Done"`
	Scores     json.RawMessage `json:"scores,omitempty"`
	Summary    json.RawMessage `json:"summary,omitempty"`
	Error      string          `json:"error,omitempty"`
	Attempts   int             `json:"attempts"`
	Kind       string          `json:"kind"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

// AIOpsEntitySummary 是 aiops_entity_summaries 表的一行：L1 单实体总结。
type AIOpsEntitySummary struct {
	SummaryID      string    `json:"summaryId"`
	AnalysisID     string    `json:"analysisId"`
	EntityKind     string    `json:"entityKind"`
	EntityName     string    `json:"entityName"`
	Classification string    `json:"classification"`
	Phenomenon     string    `json:"phenomenon"`
	IssueFlag      bool      `json:"issueFlag"`
	Conclusion     string    `json:"conclusion"`
	CreatedAt      time.Time `json:"createdAt"`
}

// AIOpsScores 是 L2 打分的分维度结构（goal 目标达成 / stability 稳定性 / efficiency 效率 / anomaly 异常）。
type AIOpsScores struct {
	Goal       int    `json:"goal"`
	Stability  int    `json:"stability"`
	Efficiency int    `json:"efficiency"`
	Anomaly    int    `json:"anomaly"`
	Overall    int    `json:"overall"`
	Verdict    string `json:"verdict"`
	Reason     string `json:"reason"`
}

// AIOpsCommandStatus 是 aiops_commands 表的状态机（#94 M2）：
// parsed → confirmed → gate → executing → verified → done；拒绝/失败为 rejected/failed；
// 波形调度执行中可用户停止 → stopped（#134）。
type AIOpsCommandStatus string

const (
	AIOpsCommandParsed    AIOpsCommandStatus = "parsed"
	AIOpsCommandConfirmed AIOpsCommandStatus = "confirmed"
	AIOpsCommandGate      AIOpsCommandStatus = "gate"
	AIOpsCommandExecuting AIOpsCommandStatus = "executing"
	AIOpsCommandVerified  AIOpsCommandStatus = "verified"
	AIOpsCommandDone      AIOpsCommandStatus = "done"
	AIOpsCommandRejected  AIOpsCommandStatus = "rejected"
	AIOpsCommandFailed    AIOpsCommandStatus = "failed"
	AIOpsCommandStopped   AIOpsCommandStatus = "stopped"
)

// AIOpsCommand 是 aiops_commands 表的一行：一句话意图的解析、确认与执行记录。
type AIOpsCommand struct {
	CommandID string          `json:"commandId"`
	RawInput  string          `json:"rawInput"`
	Parsed    json.RawMessage `json:"parsed"`
	Status    string          `json:"status"`
	Steps     json.RawMessage `json:"steps"`
	Error     string          `json:"error,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`

	// Applied 是动态计算的生效参数与 AI 描绘波形（#134，不落库，响应时附加）。
	Applied json.RawMessage `json:"applied,omitempty"`
}

// AIOpsWindowLevel 是时间聚合层级（#95 M3）：L3 窗口总结 / L4 日总结。
type AIOpsWindowLevel string

const (
	AIOpsWindowL3 AIOpsWindowLevel = "L3"
	AIOpsWindowL4 AIOpsWindowLevel = "L4"
)

// AIOpsWindowSummary 是 aiops_window_summaries 表的一行：窗口/日级聚合认知。
type AIOpsWindowSummary struct {
	WindowID    string          `json:"windowId"`
	Level       string          `json:"level"`
	WindowStart time.Time       `json:"windowStart"`
	WindowEnd   time.Time       `json:"windowEnd"`
	Scores      json.RawMessage `json:"scores,omitempty"`
	Summary     json.RawMessage `json:"summary,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// AIOpsQuotaStatus 是日配额用量状态（#134：页面可见，被拒前先知道还剩多少）。
type AIOpsQuotaStatus struct {
	Enabled    bool  `json:"enabled"`
	CallsUsed  int   `json:"callsUsed"`
	CallsMax   int   `json:"callsMax"`
	TokensUsed int64 `json:"tokensUsed"`
	TokensMax  int64 `json:"tokensMax"`
}

// AIOpsAuditLog 是 aiops_audit_log 表的一行：同步对话/分析调用审计（#110 阶段四）。
type AIOpsAuditLog struct {
	AuditID          string    `json:"auditId"`
	SessionID        string    `json:"sessionId"`
	Kind             string    `json:"kind"`
	Model            string    `json:"model"`
	DurationMS       int64     `json:"durationMs"`
	MessageLen       int       `json:"messageLen"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	Status           string    `json:"status"`
	Error            string    `json:"error,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

// AIOpsJob 是 aiops_jobs 表的一行：任务级状态（#110 阶段一，异步可见性）。
// DB 即队列：worker 用 SKIP LOCKED 认领 pending，状态/重试/失败原因可直接 SQL 查询。
type AIOpsJob struct {
	JobID       string     `json:"jobId"`
	SegmentID   string     `json:"segmentId"`
	Kind        string     `json:"kind"`
	Status      string     `json:"status"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"maxAttempts"`
	LastError   string     `json:"lastError,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// AIOpsAlert 是 aiops_alerts 表的一行：分数序列规则触发的警戒（不进 Prometheus）。
type AIOpsAlert struct {
	AlertID        string          `json:"alertId"`
	Rule           string          `json:"rule"`
	Severity       string          `json:"severity"`
	TriggeredAt    time.Time       `json:"triggeredAt"`
	AnalysisID     *string         `json:"analysisId,omitempty"`
	Interpretation json.RawMessage `json:"interpretation,omitempty"`
	AckedAt        *time.Time      `json:"ackedAt,omitempty"`
}

// AIOpsChatMessage 是 aiops_chat_messages 表的一行：同步对话的问答对（#112 阶段 D）。
// window_ids / alert_ids / command_ids 记录回答生成时注入的结论型上下文引用
// （窗口总结 / 警戒 / 意图命令的 ID 数组），用于事后回溯「这条回答当时引用了什么」。
type AIOpsChatMessage struct {
	MessageID  string          `json:"messageId"`
	SessionID  string          `json:"sessionId"`
	Role       string          `json:"role"`
	Content    string          `json:"content"`
	WindowIDs  json.RawMessage `json:"windowIds,omitempty"`
	AlertIDs   json.RawMessage `json:"alertIds,omitempty"`
	CommandIDs json.RawMessage `json:"commandIds,omitempty"`
	CreatedAt  time.Time       `json:"createdAt"`
}
