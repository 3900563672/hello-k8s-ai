package aiops

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// ErrChatUnavailable 对话通道不可用（AIOps 服务未初始化）。
var ErrChatUnavailable = fmt.Errorf("chat unavailable")

// 同步对话（#110 阶段二）参数：
// 消息长度上限、会话限流窗口与每次上限。上下文组装只取「结论型」数据
// （窗口总结 / 警戒 / 已完成分析的分数），不读原始事件，避免上下文污染。
const (
	chatRateWindow    = time.Minute
	chatContextTarget = 6000 // 组装上下文的目标字符数（超限按优先级裁剪）
)

// chatSystemPrompt 对话系统提示词：角色 + 可用上下文说明 + 引用要求。
const chatSystemPrompt = `你是 hello-k8s-ai 控制台的 AIOps 助手，面向运维人员。
背景：系统由多个切面（Pod / 节点 / 租户）组成，AIOps 分层分析产出 L2 切面分数、
L3 窗口总结与警戒。你只能基于下方「当前上下文」回答，上下文未覆盖的内容要明确说明不知道，禁止编造。
回答要求：
- 使用中文，简洁、面向运维；
- 涉及分数 / 警戒 / 窗口总结时引用来源（如「L3 窗口总结」「未确认警戒」）；
- 用户问「当前集群什么情况」时，先给总体判断，再列问题点；
- 不要复述整个上下文，只输出结论与依据。`

// ChatValidateMessage 校验单条消息：去空白后非空且不超过长度上限。
func (service *Service) ChatValidateMessage(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return fmt.Errorf("消息不能为空")
	}
	if len([]rune(message)) > service.config.ChatMaxMessageLen {
		return fmt.Errorf("消息过长（上限 %d 字符）", service.config.ChatMaxMessageLen)
	}
	return nil
}

// ChatAllowedModels 返回模型白名单；配置为空时仅允许默认模型。
func (service *Service) ChatAllowedModels() []string {
	if len(service.config.ChatModels) > 0 {
		return service.config.ChatModels
	}
	return []string{service.config.Model}
}

// ChatAllowSession 按会话限流：每个 sessionID 在滑动窗口内最多 ChatRatePerMinute 次。
func (service *Service) ChatAllowSession(sessionID string, now time.Time) bool {
	if sessionID == "" {
		sessionID = "anonymous"
	}
	service.chatMu.Lock()
	defer service.chatMu.Unlock()
	windowStart := now.Add(-chatRateWindow)
	hits := service.chatRate[sessionID]
	kept := hits[:0]
	for _, hit := range hits {
		if hit.After(windowStart) {
			kept = append(kept, hit)
		}
	}
	if len(kept) >= service.config.ChatRatePerMinute {
		service.chatRate[sessionID] = kept
		return false
	}
	service.chatRate[sessionID] = append(kept, now)
	return true
}

// ChatBuildContext 组装对话上下文：最近 L3/L4 窗口总结 + 最近警戒 + 最近已完成分析分数。
// 任一读取失败只跳过对应块，不让对话整体失败；全空时返回空对象。
func (service *Service) ChatBuildContext(ctx context.Context) (string, error) {
	contextMap := map[string]any{
		"windowSummaries": nil,
		"alerts":          nil,
		"recentAnalyses":  nil,
	}
	// L3 窗口总结（最近 3 条，含 scores/summary）。
	windows, err := service.database.ListAIOpsWindowSummaries(ctx, string(model.AIOpsWindowL3), 3)
	if err != nil {
		service.logger.Warn("AIOps chat context: list windows failed", "error", err)
	} else if len(windows) > 0 {
		contextMap["windowSummaries"] = windows
	}
	// 最近警戒（含未确认；后端按 triggered_at 倒序）。
	alerts, err := service.database.ListAIOpsAlerts(ctx, 5)
	if err != nil {
		service.logger.Warn("AIOps chat context: list alerts failed", "error", err)
	} else if len(alerts) > 0 {
		contextMap["alerts"] = alerts
	}
	// 最近已完成分析的分数与结论。
	analyses, err := service.database.ListAIOpsAnalyses(ctx, 5, string(model.AIOpsCompleted))
	if err != nil {
		service.logger.Warn("AIOps chat context: list analyses failed", "error", err)
	} else if len(analyses) > 0 {
		type analysisBrief struct {
			SegmentID string          `json:"segmentId"`
			CreatedAt time.Time       `json:"createdAt"`
			Scores    json.RawMessage `json:"scores,omitempty"`
			Summary   json.RawMessage `json:"summary,omitempty"`
		}
		briefs := make([]analysisBrief, 0, len(analyses))
		for _, analysis := range analyses {
			briefs = append(briefs, analysisBrief{
				SegmentID: analysis.SegmentID,
				CreatedAt: analysis.CreatedAt,
				Scores:    analysis.Scores,
				Summary:   analysis.Summary,
			})
		}
		contextMap["recentAnalyses"] = briefs
	}
	encoded, err := json.Marshal(contextMap)
	if err != nil {
		return "", fmt.Errorf("marshal chat context: %w", err)
	}
	text := string(encoded)
	if len([]rune(text)) > chatContextTarget {
		// 超限裁剪：按「分数 > 结论 > 现象 > 事件」优先级，这里直接丢弃过程型字段
		// （briefs 已只含结论型字段），进一步按字符截断。
		runes := []rune(text)
		text = string(runes[:chatContextTarget]) + "…（上下文已裁剪）"
	}
	return text, nil
}

// ChatSystemPrompt 返回对话系统提示词。
func (service *Service) ChatSystemPrompt() string {
	return chatSystemPrompt
}

// SettingsState 是 /aiops/settings 的掩码状态（#110 阶段四）：key 只显示是否配置，不回显明文。
type SettingsState struct {
	Configured    bool   `json:"configured"`
	Model         string `json:"model"`
	BaseURL       string `json:"baseUrl"`
	KeyConfigured bool   `json:"keyConfigured"`
}

// ConfigureLLM 运行时更新 LLM 配置（面板写入，仅服务端内存；重启后由环境变量恢复）。
// 空字段保持不变；apiKey 为空时也允许只改模型/地址。
func (service *Service) ConfigureLLM(baseURL, apiKey, model string) {
	if updater, ok := service.llm.(interface {
		UpdateConfig(baseURL, apiKey, model string)
	}); ok {
		updater.UpdateConfig(baseURL, apiKey, model)
	}
	service.configMu.Lock()
	defer service.configMu.Unlock()
	if strings.TrimSpace(baseURL) != "" {
		service.config.OpenAIBaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	}
	if strings.TrimSpace(apiKey) != "" {
		service.config.OpenAIAPIKey = strings.TrimSpace(apiKey)
	}
	if strings.TrimSpace(model) != "" {
		service.config.Model = strings.TrimSpace(model)
	}
}

// Settings 返回当前配置掩码状态。
func (service *Service) Settings() SettingsState {
	service.configMu.Lock()
	defer service.configMu.Unlock()
	state := SettingsState{
		Model:         service.config.Model,
		BaseURL:       service.config.OpenAIBaseURL,
		KeyConfigured: strings.TrimSpace(service.config.OpenAIAPIKey) != "",
	}
	state.Configured = state.KeyConfigured
	return state
}

// AuditChat 记录一次同步对话调用（#110 阶段四）：模型 / 耗时 / 消息长度 / 结果。
// 审计失败只记日志，不影响对话主流程。
func (service *Service) AuditChat(ctx context.Context, sessionID string, duration time.Duration, messageLen int, err error) {
	status := "ok"
	errorText := ""
	if err != nil {
		status = "failed"
		errorText = err.Error()
	}
	service.configMu.Lock()
	modelName := service.config.Model
	service.configMu.Unlock()
	audit := model.AIOpsAuditLog{
		AuditID:    randomAnalysisID(),
		SessionID:  sessionID,
		Kind:       "chat",
		Model:      modelName,
		DurationMS: duration.Milliseconds(),
		MessageLen: messageLen,
		Status:     status,
		Error:      errorText,
	}
	if auditErr := service.database.CreateAIOpsAuditLog(ctx, audit); auditErr != nil {
		service.logger.Warn("AIOps audit log failed", "error", auditErr)
	}
}

// ChatStream 流式生成回答：先校验消息，再组装上下文（工具步骤回调），最后流式调 LLM。
// onTool 用于上报工具步骤（start/end + 名称）；onDelta 接收文本增量。
func (service *Service) ChatStream(ctx context.Context, message string, onTool func(name, phase string), onDelta func(string)) error {
	if err := service.ChatValidateMessage(message); err != nil {
		return err
	}
	onTool("读取切面总结", "start")
	contextText, err := service.ChatBuildContext(ctx)
	if err != nil {
		return fmt.Errorf("build chat context: %w", err)
	}
	onTool("读取切面总结", "end")

	userPrompt := fmt.Sprintf("用户问题：%s\n\n当前上下文：\n%s", message, contextText)
	onTool("生成回答", "start")
	err = service.llm.StreamComplete(ctx, chatSystemPrompt, userPrompt, service.config.MaxTokensPerCall, onDelta)
	onTool("生成回答", "end")
	if err != nil {
		return fmt.Errorf("stream chat answer: %w", err)
	}
	return nil
}
