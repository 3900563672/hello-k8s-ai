package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleGetAIOpsSettings 返回 LLM 配置掩码状态（#110 阶段四）：只含是否已配置 key / 模型 / 地址，不回显明文。
func (server *Server) handleGetAIOpsSettings(writer http.ResponseWriter, request *http.Request) {
	if !server.requireAIOps(writer, request) {
		return
	}
	writeData(writer, request, http.StatusOK, server.aiops.Settings(), false, nil, nil)
}

// handleUpdateAIOpsSettings 面板写入 LLM 配置：key 仅存服务端内存（重启后由环境变量恢复），
// 响应只返回掩码状态；apiKey 允许为空（只更新模型/地址），但至少提供一个字段。
func (server *Server) handleUpdateAIOpsSettings(writer http.ResponseWriter, request *http.Request) {
	if !server.requireAIOps(writer, request) {
		return
	}
	var payload struct {
		APIKey  string `json:"apiKey"`
		Model   string `json:"model"`
		BaseURL string `json:"baseUrl"`
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 32<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_SETTINGS_PAYLOAD",
			"请求体必须是 {apiKey?, model?, baseUrl?} 的 JSON。", false, nil)
		return
	}
	apiKey := strings.TrimSpace(payload.APIKey)
	model := strings.TrimSpace(payload.Model)
	baseURL := strings.TrimSpace(payload.BaseURL)
	if apiKey == "" && model == "" && baseURL == "" {
		writeProblem(writer, request, http.StatusBadRequest, "EMPTY_SETTINGS",
			"至少提供一个字段（apiKey / model / baseUrl）。", false, nil)
		return
	}
	if len(apiKey) > 0 && len(apiKey) < 8 {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_API_KEY",
			"API Key 长度异常，请检查。", false, nil)
		return
	}
	server.aiops.ConfigureLLM(baseURL, apiKey, model)
	server.logger.Info("AIOps LLM settings updated via panel", "model", model, "baseURLChanged", baseURL != "")
	writeData(writer, request, http.StatusOK, server.aiops.Settings(), false, nil, nil)
}
