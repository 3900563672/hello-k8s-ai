package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/jackc/pgx/v5"
)

// aiopsListMaxLimit AIOps 分析列表单页上限。
const aiopsListMaxLimit = 200

// handleListAIOpsAnalyses 列出 AIOps 分析记录，可按状态过滤（status=pending|running|aggregating|completed|failed）。
func (server *Server) handleListAIOpsAnalyses(writer http.ResponseWriter, request *http.Request) {
	if !server.requireAIOps(writer, request) {
		return
	}
	status := strings.TrimSpace(request.URL.Query().Get("status"))
	if status != "" && !validAIOpsStatus(status) {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_AI_OPS_STATUS",
			"status 必须是 pending、running、aggregating、completed 或 failed 之一。", false, nil)
		return
	}
	limit := queryInteger(request, "limit", 50)
	if limit < 1 || limit > aiopsListMaxLimit {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_LIMIT",
			"limit must be between 1 and 200.", false, nil)
		return
	}
	analyses, err := server.store.ListAIOpsAnalyses(request.Context(), limit, status)
	if err != nil {
		server.writeExperimentStoreError(writer, request, "查询 AIOps 分析列表失败", err)
		return
	}
	writeData(writer, request, http.StatusOK, nonNilAIOpsAnalyses(analyses), false, nil, nil)
}

// handleGetAIOpsAnalysis 返回单条分析；支持 ?segmentId= 按切面查询。
func (server *Server) handleGetAIOpsAnalysis(writer http.ResponseWriter, request *http.Request) {
	if !server.requireAIOps(writer, request) {
		return
	}
	if segmentID := strings.TrimSpace(request.URL.Query().Get("segmentId")); segmentID != "" {
		server.writeAIOpsAnalysis(writer, request, "", segmentID)
		return
	}
	server.writeAIOpsAnalysis(writer, request, request.PathValue("id"), "")
}

// writeAIOpsAnalysis 查询分析主记录 + L1 实体总结；找不到返回 404。
func (server *Server) writeAIOpsAnalysis(writer http.ResponseWriter, request *http.Request, analysisID, segmentID string) {
	var (
		analysis *model.AIOpsAnalysis
		err      error
	)
	if segmentID != "" {
		analysis, err = server.store.GetAIOpsAnalysisBySegment(request.Context(), segmentID)
	} else {
		analysis, err = server.store.GetAIOpsAnalysis(request.Context(), analysisID)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeProblem(writer, request, http.StatusNotFound, "AI_OPS_ANALYSIS_NOT_FOUND",
				"分析记录不存在。", false, nil)
			return
		}
		server.writeExperimentStoreError(writer, request, "查询 AIOps 分析失败", err)
		return
	}
	summaries, err := server.store.ListAIOpsEntitySummaries(request.Context(), analysis.AnalysisID)
	if err != nil {
		server.writeExperimentStoreError(writer, request, "查询 AIOps 实体总结失败", err)
		return
	}
	writeData(writer, request, http.StatusOK, map[string]any{
		"analysis": *analysis,
		"entities": nonNilAIOpsEntitySummaries(summaries),
	}, false, nil, nil)
}

// requireAIOps AIOps 未启用或存储不可用时返回 404。
func (server *Server) requireAIOps(writer http.ResponseWriter, request *http.Request) bool {
	if server.aiops == nil {
		writeProblem(writer, request, http.StatusNotFound, "AI_OPS_DISABLED",
			"AIOps 未启用（需要 AIOPS_ENABLED=true 且配置 API Key）。", false, nil)
		return false
	}
	if !server.store.Available() {
		writeProblem(writer, request, http.StatusServiceUnavailable, "STORE_UNAVAILABLE",
			"持久化存储不可用。", true, nil)
		return false
	}
	return true
}

func validAIOpsStatus(status string) bool {
	switch status {
	case string(model.AIOpsPending), string(model.AIOpsRunning),
		string(model.AIOpsAggregating), string(model.AIOpsCompleted), string(model.AIOpsFailed):
		return true
	default:
		return false
	}
}

func nonNilAIOpsAnalyses(analyses []model.AIOpsAnalysis) []model.AIOpsAnalysis {
	if analyses == nil {
		return []model.AIOpsAnalysis{}
	}
	return analyses
}

func nonNilAIOpsEntitySummaries(summaries []model.AIOpsEntitySummary) []model.AIOpsEntitySummary {
	if summaries == nil {
		return []model.AIOpsEntitySummary{}
	}
	return summaries
}

// handleListAIOpsWindows 列出窗口/日总结（level=L3|L4，按窗口开始时间倒序）。
func (server *Server) handleListAIOpsWindows(writer http.ResponseWriter, request *http.Request) {
	if !server.requireAIOps(writer, request) {
		return
	}
	level := strings.TrimSpace(request.URL.Query().Get("level"))
	if level == "" {
		level = string(model.AIOpsWindowL3)
	}
	if level != string(model.AIOpsWindowL3) && level != string(model.AIOpsWindowL4) {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_AI_OPS_WINDOW_LEVEL",
			"level 必须是 L3 或 L4。", false, nil)
		return
	}
	limit := queryInteger(request, "limit", 50)
	if limit < 1 || limit > aiopsListMaxLimit {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_LIMIT",
			"limit must be between 1 and 200.", false, nil)
		return
	}
	summaries, err := server.store.ListAIOpsWindowSummaries(request.Context(), level, limit)
	if err != nil {
		server.writeExperimentStoreError(writer, request, "查询 AIOps 窗口总结失败", err)
		return
	}
	writeData(writer, request, http.StatusOK, nonNilAIOpsWindowSummaries(summaries), false, nil, nil)
}

// handleListAIOpsAlerts 列出警戒（按触发时间倒序）。
func (server *Server) handleListAIOpsAlerts(writer http.ResponseWriter, request *http.Request) {
	if !server.requireAIOps(writer, request) {
		return
	}
	limit := queryInteger(request, "limit", 50)
	if limit < 1 || limit > aiopsListMaxLimit {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_LIMIT",
			"limit must be between 1 and 200.", false, nil)
		return
	}
	alerts, err := server.store.ListAIOpsAlerts(request.Context(), limit)
	if err != nil {
		server.writeExperimentStoreError(writer, request, "查询 AIOps 警戒失败", err)
		return
	}
	writeData(writer, request, http.StatusOK, nonNilAIOpsAlerts(alerts), false, nil, nil)
}

func nonNilAIOpsWindowSummaries(summaries []model.AIOpsWindowSummary) []model.AIOpsWindowSummary {
	if summaries == nil {
		return []model.AIOpsWindowSummary{}
	}
	return summaries
}

func nonNilAIOpsAlerts(alerts []model.AIOpsAlert) []model.AIOpsAlert {
	if alerts == nil {
		return []model.AIOpsAlert{}
	}
	return alerts
}
