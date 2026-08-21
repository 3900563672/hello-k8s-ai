package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// chatEvent 是 SSE 事件负载（#110 阶段二，AG-UI 轻量子集）：
// lifecycle：一次问答开始/结束（含错误与耗时）；tool：工具步骤开始/结束；text：流式文本增量。
type chatEvent struct {
	Type      string `json:"type"`
	Phase     string `json:"phase,omitempty"`
	Name      string `json:"name,omitempty"`
	Delta     string `json:"delta,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Error     string `json:"error,omitempty"`
	Duration  int64  `json:"durationMs,omitempty"`
}

// handleAIOpsChat 同步对话（SSE 流）：校验 → 限流 → 上下文组装 → 流式 LLM。
// 事件顺序：lifecycle(start) → tool(读取切面总结 start/end) → tool(生成回答 start) → text* → tool(生成回答 end) → lifecycle(end)。
func (server *Server) handleAIOpsChat(writer http.ResponseWriter, request *http.Request) {
	if !server.requireAIOps(writer, request) {
		return
	}
	var payload struct {
		Message   string `json:"message"`
		SessionID string `json:"sessionId"`
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 64<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_CHAT_PAYLOAD",
			"请求体必须是 {message, sessionId} 的 JSON。", false, nil)
		return
	}
	if server.aiops == nil {
		writeProblem(writer, request, http.StatusNotFound, "AI_OPS_DISABLED",
			"AIOps 未启用（需要 AIOPS_ENABLED=true 且配置 API Key）。", false, nil)
		return
	}
	if err := server.aiops.ChatValidateMessage(payload.Message); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_CHAT_MESSAGE", err.Error(), false, nil)
		return
	}
	if !server.aiops.ChatAllowSession(payload.SessionID, time.Now().UTC()) {
		writeProblem(writer, request, http.StatusTooManyRequests, "CHAT_RATE_LIMITED",
			"对话请求过于频繁，请稍后再试。", true, nil)
		return
	}

	if err := server.aiops.CheckDailyQuota(request.Context()); err != nil {
		writeProblem(writer, request, http.StatusTooManyRequests, "DAILY_QUOTA_EXCEEDED",
			err.Error(), true, nil)
		return
	}
	server.streamChat(writer, request, payload.SessionID, payload.Message)
}

// handleListAIOpsChatMessages 返回某会话最近的问答历史（#112 阶段 D 读侧）：按时间正序。
// 查询参数：sessionId 必填；limit 1..200（默认 50）。历史拉取失败时前端可静默降级。
func (server *Server) handleListAIOpsChatMessages(writer http.ResponseWriter, request *http.Request) {
	if !server.requireAIOps(writer, request) {
		return
	}
	sessionID := request.URL.Query().Get("sessionId")
	if sessionID == "" {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_SESSION_ID",
			"sessionId query parameter is required.", false, nil)
		return
	}
	limit := queryInteger(request, "limit", 50)
	if limit < 1 || limit > aiopsListMaxLimit {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_LIMIT",
			"limit must be between 1 and 200.", false, nil)
		return
	}
	messages, err := server.aiops.ChatHistory(request.Context(), sessionID, limit)
	if err != nil {
		writeProblem(writer, request, http.StatusInternalServerError, "CHAT_HISTORY_FAILED",
			"查询对话历史失败。", false, nil)
		return
	}
	if messages == nil {
		messages = []model.AIOpsChatMessage{}
	}
	writeData(writer, request, http.StatusOK, messages, false, nil, nil)
}

// streamChat 发送 SSE 流；flush 失败（客户端断开）即终止。
func (server *Server) streamChat(writer http.ResponseWriter, request *http.Request, sessionID, message string) {
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := writer.(http.Flusher)
	if !ok {
		writeProblem(writer, request, http.StatusInternalServerError, "SSE_UNSUPPORTED",
			"当前服务器不支持流式响应。", false, nil)
		return
	}
	writeSSE := func(event chatEvent) bool {
		encoded, err := json.Marshal(event)
		if err != nil {
			return false
		}
		if _, err := writer.Write(append([]byte("data: "), append(encoded, '\n', '\n')...)); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	var auditUsage aiops.TokenUsage
	usageMu := sync.Mutex{}
	started := time.Now()
	if !writeSSE(chatEvent{Type: "lifecycle", Phase: "start", SessionID: sessionID}) {
		return
	}
	onTool := func(name, phase string) {
		writeSSE(chatEvent{Type: "tool", Name: name, Phase: phase, SessionID: sessionID})
	}
	var answer strings.Builder
	onDelta := func(delta string) {
		if delta == "" {
			return
		}
		answer.WriteString(delta)
		writeSSE(chatEvent{Type: "text", Delta: delta, SessionID: sessionID})
	}
	var refs aiops.ChatContextRefs
	var answerErr error
	if server.aiops != nil {
		refs, answerErr = server.aiops.ChatStream(request.Context(), message, onTool, onDelta, func(usage aiops.TokenUsage) {
			usageMu.Lock()
			auditUsage = usage
			usageMu.Unlock()
		})
	} else {
		answerErr = aiops.ErrChatUnavailable
	}
	duration := time.Since(started)
	auditContext, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	server.aiops.AuditChat(auditContext, sessionID, duration, len([]rune(message)), auditUsage, answerErr)
	cancel()
	// 阶段 D：回答生成成功后持久化问答对（带上下文引用 ID），失败只记日志不影响响应。
	if answerErr == nil && strings.TrimSpace(answer.String()) != "" {
		recordContext, recordCancel := context.WithTimeout(request.Context(), 5*time.Second)
		server.aiops.ChatRecord(recordContext, sessionID, message, answer.String(), refs)
		recordCancel()
	}
	if answerErr != nil {
		writeSSE(chatEvent{Type: "lifecycle", Phase: "end", SessionID: sessionID,
			Error: answerErr.Error(), Duration: duration.Milliseconds()})
		return
	}
	writeSSE(chatEvent{Type: "lifecycle", Phase: "end", SessionID: sessionID,
		Duration: duration.Milliseconds()})
}
