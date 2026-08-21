package aiops

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops/prompts"
)

// 本文件实现 M2 意图执行（#94）：一句话 → LLM 结构化解析 → 目录校验 → 用户确认 → 执行编排。
// 解析与校验是纯函数，执行编排在 api 层复用既有写通道（gateway/store/aggregator），
// 本包不持有 Kubernetes 依赖，保持单向依赖。

// TemplateKind 是模板目录的类别：model/node/tenant/orchestrator/traffic。
type TemplateKind string

const (
	TemplateModel        TemplateKind = "model"
	TemplateNode         TemplateKind = "node"
	TemplateTenant       TemplateKind = "tenant"
	TemplateOrchestrator TemplateKind = "orchestrator"
	TemplateTraffic      TemplateKind = "traffic"
)

// TemplateEntry 是模板目录的一行（只读元数据，与前端 presetTemplates.ts 的 id/name 对应；
// 完整模板数据仍在前端内存，执行确认时由前端携带数据，后端只校验 id 合法）。
type TemplateEntry struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Kind        TemplateKind `json:"kind"`
	Description string       `json:"description,omitempty"`
}

// TemplateCatalog 是 AI 可选模板的完整目录；LLM 只能从该目录选 id。
var TemplateCatalog = []TemplateEntry{
	// 模型模板（与集群 Model CR 名一一对应，由 hack/aiops-templates-seed.sh 预置）
	{ID: "preset-model-001", Name: "轻量在线推理", Kind: TemplateModel, Description: "8 GPU，并发 16，绝对分 75，冷启 800ms"},
	{ID: "preset-model-002", Name: "标准在线推理", Kind: TemplateModel, Description: "16 GPU，并发 32，绝对分 100，冷启 1500ms"},
	{ID: "preset-model-003", Name: "批量离线任务", Kind: TemplateModel, Description: "32 GPU，并发 64，绝对分 60，冷启 5000ms"},
	{ID: "preset-model-004", Name: "高并发推荐", Kind: TemplateModel, Description: "64 GPU，并发 96，绝对分 95，冷启 1200ms"},
	{ID: "preset-model-005", Name: "图像生成服务", Kind: TemplateModel, Description: "40 GPU，并发 24，绝对分 85，冷启 3000ms"},
	{ID: "preset-model-006", Name: "语音实时转写", Kind: TemplateModel, Description: "24 GPU，并发 40，绝对分 90，冷启 1000ms"},
	{ID: "preset-model-007", Name: "向量检索服务", Kind: TemplateModel, Description: "12 GPU，并发 48，绝对分 88，冷启 900ms"},
	{ID: "preset-model-008", Name: "多模态理解", Kind: TemplateModel, Description: "48 GPU，并发 32，绝对分 92，冷启 2500ms"},
	{ID: "preset-model-009", Name: "代码补全", Kind: TemplateModel, Description: "20 GPU，并发 64，绝对分 87，冷启 1100ms"},
	{ID: "preset-model-010", Name: "长文本摘要", Kind: TemplateModel, Description: "16 GPU，并发 20，绝对分 78，冷启 4000ms"},
	// 节点模板（与集群 WorkerNode CR 名一一对应，由 hack/aiops-templates-seed.sh 预置）
	{ID: "preset-node-001", Name: "高并发 GPU 池", Kind: TemplateNode, Description: "80 GPU，并发 128"},
	{ID: "preset-node-002", Name: "标准 GPU 节点", Kind: TemplateNode, Description: "32 GPU，并发 48"},
	{ID: "preset-node-003", Name: "边缘轻量节点", Kind: TemplateNode, Description: "8 GPU，并发 16"},
	{ID: "preset-node-004", Name: "推理加速节点", Kind: TemplateNode, Description: "48 GPU，并发 96"},
	{ID: "preset-node-005", Name: "大显存节点", Kind: TemplateNode, Description: "64 GPU，并发 64"},
	{ID: "preset-node-006", Name: "训练节点", Kind: TemplateNode, Description: "72 GPU，并发 32"},
	{ID: "preset-node-007", Name: "弹性扩缩节点", Kind: TemplateNode, Description: "40 GPU，并发 80"},
	{ID: "preset-node-008", Name: "高可用节点", Kind: TemplateNode, Description: "56 GPU，并发 72"},
	{ID: "preset-node-009", Name: "通用计算节点", Kind: TemplateNode, Description: "24 GPU，并发 56"},
	{ID: "preset-node-010", Name: "混合负载节点", Kind: TemplateNode, Description: "16 GPU，并发 40"},
	// 租户模板（与集群 Tenant CR 名一一对应，由 hack/aiops-templates-seed.sh 预置；qps 预置 0 = 空环境）
	{ID: "preset-tenant-001", Name: "核心在线业务", Kind: TemplateTenant, Description: "P1，基准 0 QPS，TTFT 阈值 800ms"},
	{ID: "preset-tenant-002", Name: "一般在线业务", Kind: TemplateTenant, Description: "P3，基准 0 QPS，TTFT 阈值 500ms"},
	{ID: "preset-tenant-003", Name: "离线分析批", Kind: TemplateTenant, Description: "P5，基准 0 QPS，TTFT 阈值 2000ms"},
	{ID: "preset-tenant-004", Name: "实时风控", Kind: TemplateTenant, Description: "P1，基准 0 QPS，TTFT 阈值 300ms"},
	{ID: "preset-tenant-005", Name: "搜索服务", Kind: TemplateTenant, Description: "P2，基准 0 QPS，TTFT 阈值 400ms"},
	{ID: "preset-tenant-006", Name: "视频渲染批", Kind: TemplateTenant, Description: "P5，基准 0 QPS，TTFT 阈值 3000ms"},
	{ID: "preset-tenant-007", Name: "交互式助手", Kind: TemplateTenant, Description: "P2，基准 0 QPS，TTFT 阈值 600ms"},
	{ID: "preset-tenant-008", Name: "数据管道", Kind: TemplateTenant, Description: "P4，基准 0 QPS，TTFT 阈值 1500ms"},
	{ID: "preset-tenant-009", Name: "模型微调任务", Kind: TemplateTenant, Description: "P4，基准 0 QPS，TTFT 阈值 2500ms"},
	{ID: "preset-tenant-010", Name: "边缘推理", Kind: TemplateTenant, Description: "P3，基准 0 QPS，TTFT 阈值 700ms"},
	// 编排策略模板（前端 PRESET_ORCHESTRATOR_TEMPLATES）
	{ID: "preset-orchestrator-core", Name: "核心租户编排策略", Kind: TemplateOrchestrator, Description: "60s 扩容冷却，120s 缩容冷却，不缩零"},
	{ID: "preset-orchestrator-elastic", Name: "弹性扩缩策略", Kind: TemplateOrchestrator, Description: "30s 扩容冷却，90s 缩容冷却，可缩零，快速吸收突发"},
	{ID: "preset-orchestrator-conservative", Name: "保守稳定策略", Kind: TemplateOrchestrator, Description: "120s 扩容冷却，240s 缩容冷却，最多 4 副本"},
	// 流量模板（前端 PRESET_TRAFFIC_TEMPLATES）
	{ID: "preset-traffic-steady", Name: "平稳 10 QPS", Kind: TemplateTraffic, Description: "5 分钟平稳 10 QPS 的基准流量"},
	{ID: "preset-traffic-tidal-2h", Name: "2 小时潮汐", Kind: TemplateTraffic, Description: "2 小时潮汐：峰值 50 QPS，30 分钟周期正弦涨落"},
	{ID: "preset-traffic-spike", Name: "脉冲峰值", Kind: TemplateTraffic, Description: "前 2 分钟无流量，随后 1 分钟冲到 50 QPS 并维持，再回落"},
	{ID: "preset-traffic-ramp", Name: "渐进斜坡", Kind: TemplateTraffic, Description: "5 分钟从 0 线性爬坡到 25 QPS"},
}

// TemplateByID 按 id 查目录，找不到返回 nil。
func TemplateByID(id string) *TemplateEntry {
	for i := range TemplateCatalog {
		if TemplateCatalog[i].ID == id {
			return &TemplateCatalog[i]
		}
	}
	return nil
}

// CommandIntent 是 LLM 解析出的结构化意图（aiops_commands.parsed 的内容）。
// 场景锚点只作切面语义标记，实际执行立即开始（#94）。
type CommandIntent struct {
	SceneTimeAnchor   string             `json:"sceneTimeAnchor,omitempty"` // 场景时间锚点（用户语义，如 "美国时间 09:00"）
	DurationMinutes   int                `json:"durationMinutes,omitempty"` // 持续时长（分钟）
	SceneType         string             `json:"sceneType,omitempty"`       // 场景类型（如 "突发流量高峰"）
	TargetTenant      string             `json:"targetTenant,omitempty"`    // 目标租户（租户名，写流量/建实验用）
	TemplateSelection TemplateSelections `json:"templateSelection"`         // 模板选择（只能选目录内 id）
	Traffic           *TrafficIntent     `json:"traffic,omitempty"`         // 流量自由设计（与模板二选一）
	Rate              *int               `json:"rate,omitempty"`            // 可选倍速（SimulationClock rate）
}

// TemplateSelections 是各类型模板的选择（id 列表；节点为集群既有节点名）。
type TemplateSelections struct {
	ModelIDs        []string `json:"modelIds,omitempty"`
	NodeNames       []string `json:"nodeNames,omitempty"`
	TenantIDs       []string `json:"tenantIds,omitempty"`
	OrchestratorIDs []string `json:"orchestratorIds,omitempty"`
	TrafficIDs      []string `json:"trafficIds,omitempty"`
}

// 流量形状与上限（防 AI 把流量设得离谱打爆环境）。
const (
	MaxTrafficQPS      = 200 // 单命令流量峰值上限（QPS）
	MaxSimulationRate  = 100 // 倍速上限
	DefaultTidalPeriod = 30  // 潮汐默认周期（分钟）
	DefaultPeakQPS     = 20  // 用户未给数字时 AI 的默认峰值（QPS）
)

// AIOpsLimits 是意图执行的硬限制与能力（单一事实源）：解析校验与前端提示共用，
// 通过 GET /api/v1/aiops/limits 暴露，保证用户在任何入口都能看到"为什么流量是这样"（#134）。
type AIOpsLimits struct {
	MaxTrafficQPS             int      `json:"maxTrafficQPS"`             // 单命令峰值 QPS 上限（超限自动钳制并在 applied 中说明）
	MaxSimulationRate         int      `json:"maxSimulationRate"`         // 倍速上限（超限自动钳制并在 applied 中说明）
	TrafficShapes             []string `json:"trafficShapes"`             // 支持的流量波形（AI 按意图描绘，无需手画）
	DefaultTidalPeriodMinutes int      `json:"defaultTidalPeriodMinutes"` // 潮汐默认周期（分钟）
	DefaultPeakQPS            int      `json:"defaultPeakQPS"`            // 用户未给数字时 AI 的默认峰值
	DefaultRate               int      `json:"defaultRate"`               // 未指定倍速时的默认值
	TrafficRequiresTenant     bool     `json:"trafficRequiresTenant"`     // 写流量必须指定目标租户
	UnlimitedDuration         bool     `json:"unlimitedDuration"`         // 模拟时长不设上限（用户/AI 说了算）
	SupportsStop              bool     `json:"supportsStop"`              // 执行中可随时停止（QPS 归零）
}

// Limits 返回当前意图执行的硬限制与能力（与 ValidateCommandIntent/NormalizeTrafficIntent 共用同一组常量）。
func Limits() AIOpsLimits {
	return AIOpsLimits{
		MaxTrafficQPS:             MaxTrafficQPS,
		MaxSimulationRate:         MaxSimulationRate,
		TrafficShapes:             []string{string(TrafficShapeSteady), string(TrafficShapeTidal), string(TrafficShapeSpike), string(TrafficShapeRamp)},
		DefaultTidalPeriodMinutes: DefaultTidalPeriod,
		DefaultPeakQPS:            DefaultPeakQPS,
		DefaultRate:               1,
		TrafficRequiresTenant:     true,
		UnlimitedDuration:         true,
		SupportsStop:              true,
	}
}

// TrafficShape 是流量波形：steady 固定 / tidal 潮汐 / spike 脉冲 / ramp 斜坡。
type TrafficShape string

const (
	TrafficShapeSteady TrafficShape = "steady"
	TrafficShapeTidal  TrafficShape = "tidal"
	TrafficShapeSpike  TrafficShape = "spike"
	TrafficShapeRamp   TrafficShape = "ramp"
)

func validTrafficShape(shape string) bool {
	switch TrafficShape(shape) {
	case "", TrafficShapeSteady, TrafficShapeTidal, TrafficShapeSpike, TrafficShapeRamp:
		return true
	}
	return false
}

// TrafficIntent 是流量自由设计：steady 用 QPS；tidal/spike/ramp 用 PeakQPS（+PeriodMinutes）。
// 波形由执行端调度器按模拟时间推进（倍速下墙钟 = 模拟时长/倍速）。
type TrafficIntent struct {
	QPS           *int   `json:"qps,omitempty"`           // 固定流量（平稳场景）
	Shape         string `json:"shape,omitempty"`         // steady|tidal|spike|ramp，默认 steady
	PeakQPS       *int   `json:"peakQps,omitempty"`       // 曲线峰值（潮汐/脉冲/斜坡用）
	PeriodMinutes *int   `json:"periodMinutes,omitempty"` // 潮汐周期（分钟），默认 30
}

// TrafficShapeQPS 计算给定模拟时刻的波形 QPS（供执行端调度器与测试使用）。
// elapsed 为模拟时间（分钟）；periodMinutes 为潮汐周期（分钟，<=0 用默认 30）。
// 值域保证 [max(1, peak/5), peak]。
func TrafficShapeQPS(elapsedMinutes float64, shape TrafficShape, peak, periodMinutes int) int {
	if peak < 1 {
		return 0
	}
	base := max(1, peak/5)
	switch shape {
	case TrafficShapeTidal:
		period := periodMinutes
		if period <= 0 {
			period = DefaultTidalPeriod
		}
		phase := 2 * math.Pi * elapsedMinutes / float64(period)
		value := float64(base) + float64(peak-base)*(0.5+0.5*math.Sin(phase))
		return int(math.Round(value))
	case TrafficShapeRamp:
		// 斜坡：线性 0 → peak，到达峰值后维持
		rampMinutes := periodMinutes
		if rampMinutes <= 0 {
			rampMinutes = DefaultTidalPeriod
		}
		if elapsedMinutes >= float64(rampMinutes) {
			return peak
		}
		return int(math.Round(float64(peak) * elapsedMinutes / float64(rampMinutes)))
	case TrafficShapeSpike:
		// 脉冲：前 80% 时间低水位，后 20% 时间峰值
		duration := periodMinutes
		if duration <= 0 {
			duration = DefaultTidalPeriod
		}
		if elapsedMinutes >= float64(duration)*0.8 {
			return peak
		}
		return base
	default: // steady
		return peak
	}
}

// commandSystemPrompt 用模板目录渲染命令意图解析提示词（#112 提示词工程化）。
func commandSystemPrompt() (prompts.Prompt, error) {
	var builder strings.Builder
	for _, entry := range TemplateCatalog {
		builder.WriteString(fmt.Sprintf("- %s（%s）：%s\n", entry.ID, entry.Name, entry.Description))
	}
	return prompts.CommandIntent.Render(map[string]any{"Catalog": builder.String()})
}

// ParseCommand 解析一句话为结构化意图；LLM 失败返回错误（调用方落库失败态），
// 解析结果必须通过目录校验，否则视为解析失败（不部分执行）。
func ParseCommand(ctx context.Context, rawInput string, llm LLM, maxTokens int) (*CommandIntent, error) {
	prompt, err := commandSystemPrompt()
	if err != nil {
		return nil, fmt.Errorf("render command prompt: %w", err)
	}
	completion, err := llm.CompleteJSON(ctx, prompt.System, rawInput, maxTokens, prompt.Temperature)
	if err != nil {
		return nil, fmt.Errorf("parse command intent: %w", err)
	}
	var intent CommandIntent
	if err := json.Unmarshal([]byte(completion.Content), &intent); err != nil {
		return nil, fmt.Errorf("parse command intent JSON: %w", err)
	}
	if err := ValidateCommandIntent(&intent); err != nil {
		return nil, err
	}
	return &intent, nil
}

// ValidateCommandIntent 校验权限边界：所有模板 id 必须在目录内（防越权/编造 id）。
// 节点存在性与租户存在性由执行 gate 在 api 层基于集群 cache 校验。
func ValidateCommandIntent(intent *CommandIntent) error {
	if intent == nil {
		return fmt.Errorf("意图为空")
	}
	ids := make([]string, 0, 16)
	ids = append(ids, intent.TemplateSelection.ModelIDs...)
	ids = append(ids, intent.TemplateSelection.TenantIDs...)
	ids = append(ids, intent.TemplateSelection.OrchestratorIDs...)
	ids = append(ids, intent.TemplateSelection.TrafficIDs...)
	for _, id := range ids {
		if id == "" {
			continue
		}
		if TemplateByID(id) == nil {
			return fmt.Errorf("模板 id %q 不在目录中（只允许选择既有模板，不允许修改或创建模板）", id)
		}
	}
	for _, id := range intent.TemplateSelection.NodeNames {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("节点名不能为空")
		}
	}
	if intent.Traffic != nil {
		if intent.Traffic.QPS != nil && *intent.Traffic.QPS < 1 {
			return fmt.Errorf("固定流量 QPS 必须为正整数（当前 %d）", *intent.Traffic.QPS)
		}
		if intent.Traffic.PeakQPS != nil && *intent.Traffic.PeakQPS < 1 {
			return fmt.Errorf("峰值 QPS 必须为正整数（当前 %d）", *intent.Traffic.PeakQPS)
		}
		if !validTrafficShape(intent.Traffic.Shape) {
			return fmt.Errorf("流量形状 %q 不支持（支持 steady/tidal/spike/ramp）", intent.Traffic.Shape)
		}
		if intent.Traffic.PeriodMinutes != nil && *intent.Traffic.PeriodMinutes < 1 {
			return fmt.Errorf("潮汐周期必须为正整数分钟")
		}
		if intent.TargetTenant == "" {
			return fmt.Errorf("写流量必须指定目标租户 targetTenant")
		}
	}
	if intent.Rate != nil && *intent.Rate < 1 {
		return fmt.Errorf("倍速必须为正整数（当前 %d）", *intent.Rate)
	}
	// 超上限与缺省值不在此拒绝（#134）：由 NormalizeTrafficIntent 钳制/补默认并返回 applied 说明，
	// 让用户看到"要求 500 → 生效 200（超上限）"而不是解析失败。
	return nil
}

// TrafficPoint 是 AI 描绘波形的采样点：X 为模拟秒，Y 为 QPS。
type TrafficPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// AppliedValue 记录一个执行字段的请求值→生效值及原因（#134：限制必须可见）。
type AppliedValue struct {
	Field     string `json:"field"`
	Requested *int   `json:"requested,omitempty"`
	Effective int    `json:"effective"`
	Reason    string `json:"reason"` // ok | clamped-to-max | defaulted
}

// TrafficApplied 是命令的生效参数与 AI 描绘的波形（由 parsed 动态重算，不落库）。
type TrafficApplied struct {
	Values           []AppliedValue `json:"values"`
	Curve            []TrafficPoint `json:"curve"`
	WallClockSeconds int            `json:"wallClockSeconds"`
}

// NormalizeTrafficIntent 把意图数值收敛到可执行范围（#134）：
// 超上限钳制（500 QPS → 200）、缺省补默认（未给峰值 → 20；未给倍速 → 1；非稳态未给时长 → 一个周期），
// 就地修改 intent（执行端直接用生效值），返回 applied 记录（请求值→生效值+原因）与 AI 描绘的波形曲线。
// 结构非法（负数/坏形状/缺租户）由 ValidateCommandIntent 前置拒绝。
func NormalizeTrafficIntent(intent *CommandIntent) (*TrafficApplied, error) {
	if intent == nil || intent.Traffic == nil {
		return nil, nil
	}
	traffic := intent.Traffic
	shape := TrafficShape(strings.TrimSpace(traffic.Shape))
	if shape == "" {
		shape = TrafficShapeSteady
		traffic.Shape = string(shape)
	}
	applied := &TrafficApplied{Values: []AppliedValue{}}
	add := func(field string, requested *int, effective int, reason string) {
		applied.Values = append(applied.Values, AppliedValue{Field: field, Requested: requested, Effective: effective, Reason: reason})
	}

	// 峰值：稳态用 qps，非稳态用 peakQps；缺省默认 20；超上限钳制到 MaxTrafficQPS。
	peak := 0
	if shape == TrafficShapeSteady {
		if traffic.QPS != nil {
			peak = *traffic.QPS
		} else {
			peak = DefaultPeakQPS
			traffic.QPS = &peak
			add("peakQps", nil, peak, "defaulted")
		}
		if peak > MaxTrafficQPS {
			requested := peak
			peak = MaxTrafficQPS
			traffic.QPS = &peak
			add("peakQps", &requested, peak, "clamped-to-max")
		}
	} else {
		if traffic.PeakQPS != nil {
			peak = *traffic.PeakQPS
		} else {
			peak = DefaultPeakQPS
			traffic.PeakQPS = &peak
			add("peakQps", nil, peak, "defaulted")
		}
		if peak > MaxTrafficQPS {
			requested := peak
			peak = MaxTrafficQPS
			traffic.PeakQPS = &peak
			add("peakQps", &requested, peak, "clamped-to-max")
		}
	}

	// 周期：非稳态缺省默认 DefaultTidalPeriod。
	period := DefaultTidalPeriod
	if shape != TrafficShapeSteady {
		if traffic.PeriodMinutes != nil && *traffic.PeriodMinutes >= 1 {
			period = *traffic.PeriodMinutes
		} else {
			traffic.PeriodMinutes = &period
			add("periodMinutes", nil, period, "defaulted")
		}
	}

	// 时长：不设上限（用户/AI 说了算，前端可见进度、随时可停）；非稳态缺省跑一个周期。
	if intent.DurationMinutes <= 0 && shape != TrafficShapeSteady {
		duration := period
		intent.DurationMinutes = duration
		add("durationMinutes", nil, duration, "defaulted")
	}

	// 倍速：缺省 1；超上限钳制到 MaxSimulationRate。
	rate := 1
	if intent.Rate == nil {
		intent.Rate = &rate
		add("rate", nil, rate, "defaulted")
	} else {
		rate = *intent.Rate
		if rate > MaxSimulationRate {
			requested := rate
			rate = MaxSimulationRate
			intent.Rate = &rate
			add("rate", &requested, rate, "clamped-to-max")
		}
	}

	// 墙钟：模拟时长/倍速（用户看到"还要等多久"）。
	wallSeconds := 0
	if intent.DurationMinutes > 0 {
		wallSeconds = max(1, intent.DurationMinutes*60/rate)
	}
	applied.WallClockSeconds = wallSeconds

	// AI 描绘波形：curve 即生效曲线（前端预览与运行插值共用）。
	curveDuration := intent.DurationMinutes
	if curveDuration <= 0 {
		curveDuration = DefaultTidalPeriod
	}
	applied.Curve = GenerateTrafficCurve(shape, peak, period, curveDuration)
	return applied, nil
}

// GenerateTrafficCurve 按形状参数生成波形采样点（自适应粒度，≤约 121 点）：
// 30 分钟内 1 分钟粒度、6 小时内 5 分钟、24 小时内 15 分钟、更长 30 分钟。
func GenerateTrafficCurve(shape TrafficShape, peak, periodMinutes, durationMinutes int) []TrafficPoint {
	if durationMinutes <= 0 {
		durationMinutes = DefaultTidalPeriod
	}
	stepSeconds := 60
	switch {
	case durationMinutes > 24*60:
		stepSeconds = 30 * 60
	case durationMinutes > 6*60:
		stepSeconds = 15 * 60
	case durationMinutes > 60:
		stepSeconds = 5 * 60
	}
	durationSeconds := durationMinutes * 60
	points := make([]TrafficPoint, 0, durationSeconds/stepSeconds+1)
	for seconds := 0; seconds <= durationSeconds; seconds += stepSeconds {
		points = append(points, TrafficPoint{X: seconds, Y: TrafficShapeQPS(float64(seconds)/60, shape, peak, periodMinutes)})
	}
	if last := points[len(points)-1]; last.X != durationSeconds {
		points = append(points, TrafficPoint{X: durationSeconds, Y: TrafficShapeQPS(float64(durationMinutes), shape, peak, periodMinutes)})
	}
	return points
}

// ParseCommand 是 Service 层的意图解析入口：LLM 解析 + 目录校验（M2，#94）。
// 执行编排在 api 层，本方法只负责解析与校验，不产生任何写操作。
func (service *Service) ParseCommand(ctx context.Context, rawInput string) (*CommandIntent, error) {
	return ParseCommand(ctx, rawInput, service.llm, service.config.MaxTokensPerCall)
}
