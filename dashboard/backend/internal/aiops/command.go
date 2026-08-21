package aiops

import (
	"context"
	"encoding/json"
	"fmt"
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
	// 模型模板（前端 PRESET_MODEL_TEMPLATES）
	{ID: "preset-model-lite", Name: "轻量在线推理", Kind: TemplateModel, Description: "8 GPU，并发 16，绝对分 75"},
	{ID: "preset-model-standard", Name: "标准在线推理", Kind: TemplateModel, Description: "16 GPU，并发 32，绝对分 100"},
	{ID: "preset-model-batch", Name: "批量离线任务", Kind: TemplateModel, Description: "32 GPU，并发 64，绝对分 60"},
	// 节点模板（前端 PRESET_NODE_TEMPLATES；节点本身是集群既有资源）
	{ID: "preset-node-gpu-pool", Name: "高并发 GPU 池", Kind: TemplateNode, Description: "80 GPU，并发 128"},
	{ID: "preset-node-standard", Name: "标准 GPU 节点", Kind: TemplateNode, Description: "32 GPU，并发 48"},
	{ID: "preset-node-edge", Name: "边缘轻量节点", Kind: TemplateNode, Description: "8 GPU，并发 16"},
	// 租户模板（前端 PRESET_TENANT_TEMPLATES）
	{ID: "preset-tenant-core", Name: "核心在线业务", Kind: TemplateTenant, Description: "P1，基准 20 QPS，TTFT 阈值 800ms"},
	{ID: "preset-tenant-general", Name: "一般在线业务", Kind: TemplateTenant, Description: "P3，基准 10 QPS，TTFT 阈值 500ms"},
	{ID: "preset-tenant-batch", Name: "离线分析批", Kind: TemplateTenant, Description: "P5，基准 0 QPS，TTFT 阈值 2000ms"},
	// 编排策略模板（前端 PRESET_ORCHESTRATOR_TEMPLATES）
	{ID: "preset-orchestrator-core", Name: "核心租户编排策略", Kind: TemplateOrchestrator, Description: "60s 扩容冷却，120s 缩容冷却，不缩零"},
	{ID: "preset-orchestrator-elastic", Name: "弹性扩缩策略", Kind: TemplateOrchestrator, Description: "30s 扩容冷却，90s 缩容冷却，可缩零，快速吸收突发"},
	{ID: "preset-orchestrator-conservative", Name: "保守稳定策略", Kind: TemplateOrchestrator, Description: "120s 扩容冷却，240s 缩容冷却，最多 4 副本"},
	// 流量模板（前端 PRESET_TRAFFIC_TEMPLATES）
	{ID: "preset-traffic-steady", Name: "平稳 10 QPS", Kind: TemplateTraffic, Description: "5 分钟平稳 10 QPS 的基准流量"},
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

// TrafficIntent 是流量自由设计（目标 QPS；曲线数据由前端确认时携带）。
type TrafficIntent struct {
	QPS *int `json:"qps,omitempty"`
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
	completion, err := llm.CompleteJSON(ctx, prompt.System, rawInput, maxTokens)
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
	if intent.Traffic != nil && intent.Traffic.QPS != nil && *intent.Traffic.QPS < 0 {
		return fmt.Errorf("QPS 不能为负数")
	}
	if intent.Rate != nil && *intent.Rate < 0 {
		return fmt.Errorf("倍速不能为负数")
	}
	return nil
}

// ParseCommand 是 Service 层的意图解析入口：LLM 解析 + 目录校验（M2，#94）。
// 执行编排在 api 层，本方法只负责解析与校验，不产生任何写操作。
func (service *Service) ParseCommand(ctx context.Context, rawInput string) (*CommandIntent, error) {
	return ParseCommand(ctx, rawInput, service.llm, service.config.MaxTokensPerCall)
}
