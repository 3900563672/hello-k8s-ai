package aiops

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops/prompts"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// ErrChatUnavailable 对话通道不可用（AIOps 服务未初始化）。
var ErrChatUnavailable = fmt.Errorf("chat unavailable")

// 同步对话（#110 阶段二）参数：
// 消息长度上限、会话限流窗口与每次上限。上下文组装只取「结论型」数据
// （窗口总结 / 警戒 / 已完成分析的分数），不读原始事件，避免上下文污染。
// 上下文预算常量见 budget.go（budgetChatContextRun）。
const chatRateWindow = time.Minute

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

// CheckDailyQuota 全局日配额（#124 降级预案）：基于 aiops_audit_log 统计近 24h 调用次数与 token，
// 超过 AIOPS_DAILY_MAX_CALLS / AIOPS_DAILY_MAX_TOKENS 时拒绝新调用，防止 key 被刷爆。
func (service *Service) CheckDailyQuota(ctx context.Context) error {
	if service.config.DailyMaxCalls <= 0 && service.config.DailyMaxTokens <= 0 {
		return nil
	}
	calls, tokens, err := service.database.SumAIOpsUsageSince(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		return fmt.Errorf("check daily quota: %w", err)
	}
	if service.config.DailyMaxCalls > 0 && calls >= service.config.DailyMaxCalls {
		return fmt.Errorf("今日 AIOps 调用配额已达上限（%d 次/24h）", service.config.DailyMaxCalls)
	}
	if service.config.DailyMaxTokens > 0 && tokens >= service.config.DailyMaxTokens {
		return fmt.Errorf("今日 AIOps token 配额已达上限（%d/24h）", service.config.DailyMaxTokens)
	}
	return nil
}

// ChatContextRefs 是回答生成时注入上下文的结论型引用 ID（#112 阶段 D 持久化用）：
// 窗口总结 / 警戒 / 意图命令，用于事后回溯「这条回答当时引用了什么」。
type ChatContextRefs struct {
	WindowIDs  []string `json:"windowIds,omitempty"`
	AlertIDs   []string `json:"alertIds,omitempty"`
	CommandIDs []string `json:"commandIds,omitempty"`
}

// ChatContext 是对话上下文组装结果：截断后的文本 + 引用 ID（引用与文本截断无关，始终完整收集）。
type ChatContext struct {
	Text string
	Refs ChatContextRefs
}

// ChatBuildContext 组装对话上下文：最近 L3/L4 窗口总结 + 最近警戒 + 最近已完成分析分数 + 最近意图命令。
// 任一读取失败只跳过对应块，不让对话整体失败；全空时返回空对象。
func (service *Service) ChatBuildContext(ctx context.Context) (*ChatContext, error) {
	contextMap := map[string]any{
		"windowSummaries": nil,
		"alerts":          nil,
		"recentAnalyses":  nil,
		"recentCommands":  nil,
	}

	var refs ChatContextRefs
	// L3 窗口总结（最近 3 条，含 scores/summary）。
	windows, err := service.database.ListAIOpsWindowSummaries(ctx, string(model.AIOpsWindowL3), 3)
	if err != nil {
		service.logger.Warn("AIOps chat context: list windows failed", "error", err)
	} else if len(windows) > 0 {
		contextMap["windowSummaries"] = windows
		for _, window := range windows {
			refs.WindowIDs = append(refs.WindowIDs, window.WindowID)
		}
	}
	// 最近警戒（含未确认；后端按 triggered_at 倒序）。
	alerts, err := service.database.ListAIOpsAlerts(ctx, 5)
	if err != nil {
		service.logger.Warn("AIOps chat context: list alerts failed", "error", err)
	} else if len(alerts) > 0 {
		contextMap["alerts"] = alerts
		for _, alert := range alerts {
			refs.AlertIDs = append(refs.AlertIDs, alert.AlertID)
		}
	}
	// 最近意图命令（#112 阶段 C：结论型——raw_input/status/parsed 摘要，供回答引用执行结果）。
	commands, err := service.database.ListAIOpsCommands(ctx, 3)
	if err != nil {
		service.logger.Warn("AIOps chat context: list commands failed", "error", err)
	} else if len(commands) > 0 {
		type commandBrief struct {
			CommandID string          `json:"commandId"`
			RawInput  string          `json:"rawInput"`
			Status    string          `json:"status"`
			Parsed    json.RawMessage `json:"parsed,omitempty"`
		}
		briefs := make([]commandBrief, 0, len(commands))
		for _, command := range commands {
			briefs = append(briefs, commandBrief{
				CommandID: command.CommandID,
				RawInput:  command.RawInput,
				Status:    command.Status,
				Parsed:    command.Parsed,
			})
			refs.CommandIDs = append(refs.CommandIDs, command.CommandID)
		}
		contextMap["recentCommands"] = briefs
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
		return nil, fmt.Errorf("marshal chat context: %w", err)
	}
	text, truncated := truncateChatContext(string(encoded), budgetChatContextRun)
	if truncated {
		service.logger.Warn("AIOps chat context truncated by budget", "budgetRunes", budgetChatContextRun)
	}
	return &ChatContext{Text: text, Refs: refs}, nil
}

// ChatHistory 返回某会话最近的问答历史（#112 阶段 D 读侧）：按时间正序，最多 limit 条。
// 存储不可用时返回错误，由调用方决定是否降级（前端历史拉取失败可静默，本地会话仍可用）。
func (service *Service) ChatHistory(ctx context.Context, sessionID string, limit int) ([]model.AIOpsChatMessage, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionId 不能为空")
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	messages, err := service.database.ListAIOpsChatMessages(ctx, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list chat history: %w", err)
	}
	return messages, nil
}

// ChatSystemPrompt 返回对话系统提示词（#112：模板渲染，带版本/哈希）。
func (service *Service) ChatSystemPrompt() string {
	prompt, err := prompts.ChatAssistant.Render(nil)
	if err != nil {
		return ""
	}
	return prompt.System
}

// SettingsState 是 /aiops/settings 的掩码状态（#110 阶段四）：key 只显示是否配置，不回显明文。
type SettingsState struct {
	Configured    bool   `json:"configured"`
	Model         string `json:"model"`
	BaseURL       string `json:"baseUrl"`
	KeyConfigured bool   `json:"keyConfigured"`
	Enabled       bool   `json:"enabled"`
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

// QuotaStatus 返回日配额用量与上限（#134）：0 上限 = 未启用配额；面板展示剩余额度。
func (service *Service) QuotaStatus(ctx context.Context) (model.AIOpsQuotaStatus, error) {
	if service.config.DailyMaxCalls <= 0 && service.config.DailyMaxTokens <= 0 {
		return model.AIOpsQuotaStatus{Enabled: false}, nil
	}
	calls, tokens, err := service.database.SumAIOpsUsageSince(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		return model.AIOpsQuotaStatus{}, fmt.Errorf("query aiops quota: %w", err)
	}
	return model.AIOpsQuotaStatus{
		Enabled:    true,
		CallsUsed:  calls,
		CallsMax:   service.config.DailyMaxCalls,
		TokensUsed: tokens,
		TokensMax:  service.config.DailyMaxTokens,
	}, nil
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
	state.Enabled = service.enabled
	return state
}

// SetEnabled 运行时开关（面板写入，仅服务端内存；重启后恢复为部署级启用态）。
func (service *Service) SetEnabled(enabled bool) {
	service.configMu.Lock()
	defer service.configMu.Unlock()
	service.enabled = enabled
}

// Enabled 返回运行时开关状态。
func (service *Service) Enabled() bool {
	service.configMu.Lock()
	defer service.configMu.Unlock()
	return service.enabled
}

// ChatRecord 持久化一次问答对（#112 阶段 D）：user 消息 + assistant 回答 + 上下文引用。
// 引用 ID 只落在 assistant 消息上；失败只记日志（与审计同策略），不影响对话主流程。
func (service *Service) ChatRecord(ctx context.Context, sessionID, message, answer string, refs ChatContextRefs) {
	if sessionID == "" {
		sessionID = "anonymous"
	}
	windowIDs, err := json.Marshal(refs.WindowIDs)
	if err != nil {
		service.logger.Warn("AIOps chat record: marshal window ids failed", "error", err)
		return
	}
	alertIDs, err := json.Marshal(refs.AlertIDs)
	if err != nil {
		service.logger.Warn("AIOps chat record: marshal alert ids failed", "error", err)
		return
	}
	commandIDs, err := json.Marshal(refs.CommandIDs)
	if err != nil {
		service.logger.Warn("AIOps chat record: marshal command ids failed", "error", err)
		return
	}
	emptyIDs := json.RawMessage("[]")
	now := time.Now().UTC()
	userMessage := model.AIOpsChatMessage{
		MessageID: randomAnalysisID(), SessionID: sessionID, Role: "user",
		Content: message, WindowIDs: emptyIDs, AlertIDs: emptyIDs, CommandIDs: emptyIDs, CreatedAt: now,
	}
	if err := service.database.CreateAIOpsChatMessage(ctx, userMessage); err != nil {
		service.logger.Warn("AIOps chat record: save user message failed", "error", err)
		return
	}
	assistantMessage := model.AIOpsChatMessage{
		MessageID: randomAnalysisID(), SessionID: sessionID, Role: "assistant",
		Content: answer, WindowIDs: windowIDs, AlertIDs: alertIDs, CommandIDs: commandIDs, CreatedAt: now,
	}
	if err := service.database.CreateAIOpsChatMessage(ctx, assistantMessage); err != nil {
		service.logger.Warn("AIOps chat record: save assistant message failed", "error", err)
	}
}

// AuditChat 记录一次同步对话调用（#110 阶段四）：模型 / 耗时 / 消息长度 / token 用量 / 结果。
// 审计失败只记日志，不影响对话主流程。
func (service *Service) AuditChat(ctx context.Context, sessionID string, duration time.Duration, messageLen int, usage TokenUsage, err error) {
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
		AuditID:          randomAnalysisID(),
		SessionID:        sessionID,
		Kind:             "chat",
		Model:            modelName,
		DurationMS:       duration.Milliseconds(),
		MessageLen:       messageLen,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		Status:           status,
		Error:            errorText,
	}
	if auditErr := service.database.CreateAIOpsAuditLog(ctx, audit); auditErr != nil {
		service.logger.Warn("AIOps audit log failed", "error", auditErr)
	}
}

// ChatStream 流式生成回答：先校验消息，再组装上下文（工具步骤回调），最后流式调 LLM。
// onTool 用于上报工具步骤（start/end + 名称）；onDelta 接收文本增量；onUsage 接收 token 用量（审计）。
func (service *Service) ChatStream(ctx context.Context, message string, onTool func(name, phase string), onDelta func(string), onUsage func(TokenUsage)) (ChatContextRefs, error) {
	if err := service.ChatValidateMessage(message); err != nil {
		return ChatContextRefs{}, err
	}
	onTool("读取切面总结", "start")
	chatContext, err := service.ChatBuildContext(ctx)
	if err != nil {
		return ChatContextRefs{}, fmt.Errorf("build chat context: %w", err)
	}
	onTool("读取切面总结", "end")

	userPrompt := fmt.Sprintf("用户问题：%s\n\n当前上下文：\n%s", message, chatContext.Text)
	prompt, renderErr := prompts.ChatAssistant.Render(nil)
	if renderErr != nil {
		return ChatContextRefs{}, fmt.Errorf("render chat prompt: %w", renderErr)
	}
	onTool("生成回答", "start")
	err = service.llm.StreamComplete(ctx, prompt.System, userPrompt, service.config.MaxTokensPerCall, prompt.Temperature, onDelta, onUsage)
	onTool("生成回答", "end")
	if err != nil {
		return ChatContextRefs{}, fmt.Errorf("stream chat answer: %w", err)
	}
	return chatContext.Refs, nil
}
