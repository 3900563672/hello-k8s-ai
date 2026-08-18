package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	jaegerprovider "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/jaeger"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

// experimentNameMaxLen 实验名称与租户名称长度上限。
const experimentNameMaxLen = 63

// experimentListMaxLimit 实验列表单页上限。
const experimentListMaxLimit = 200

type createExperimentRequest struct {
	Tenant string `json:"tenant"`
	Name   string `json:"name"`
}

type failExperimentRequest struct {
	Reason string `json:"reason"`
}

// handleCreateExperiment 创建切面（pending）：记录配置快照，等待用户开始实验。
// 切面=一次调度实验的不可变归档单元，配置快照在创建时定格，与实验生命周期绑定。
func (server *Server) handleCreateExperiment(writer http.ResponseWriter, request *http.Request) {
	if !server.requireExperimentStore(writer, request) {
		return
	}
	var body createExperimentRequest
	if err := decodeJSON(writer, request, server.config.HTTP.MaxBodyBytes, &body); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	body.Tenant = strings.TrimSpace(body.Tenant)
	body.Name = strings.TrimSpace(body.Name)
	if err := validateExperimentIdentity(body.Tenant, body.Name); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_EXPERIMENT", err.Error(), false, nil)
		return
	}
	if !server.requireCache(writer, request) {
		return
	}
	snapshot := server.aggregator.CurrentSnapshot(time.Now().UTC())
	configPayload, err := json.Marshal(snapshot.Configuration)
	if err != nil {
		writeProblem(writer, request, http.StatusInternalServerError, "SNAPSHOT_ENCODE_FAILED", err.Error(), false, nil)
		return
	}
	record := store.SegmentRecord{
		SegmentID:      randomIdentifier("segment"),
		Tenant:         body.Tenant,
		Name:           body.Name,
		Status:         string(model.SegmentPending),
		ConfigSnapshot: configPayload,
	}
	if err := server.store.CreateSegment(request.Context(), record); err != nil {
		server.writeExperimentStoreError(writer, request, "创建实验失败", err)
		return
	}
	server.writeExperimentDetail(writer, request, http.StatusCreated, record.SegmentID, nil)
}

// handleStartExperiment 开始实验：pending→running，写入起点全局快照并启动混合采样。
func (server *Server) handleStartExperiment(writer http.ResponseWriter, request *http.Request) {
	if !server.requireExperimentStore(writer, request) {
		return
	}
	segmentID := request.PathValue("id")
	record, err := server.store.GetSegment(request.Context(), segmentID)
	if err != nil {
		server.writeSegmentNotFound(writer, request, segmentID, err)
		return
	}
	if record.Status != string(model.SegmentPending) {
		writeProblem(writer, request, http.StatusConflict, "SEGMENT_NOT_PENDING",
			fmt.Sprintf("实验当前状态为 %s，只有 pending 状态的实验可以开始。", record.Status), false, nil)
		return
	}
	if !server.requireCache(writer, request) {
		return
	}
	startSnapshot, err := json.Marshal(server.aggregator.CurrentSnapshot(time.Now().UTC()))
	if err != nil {
		writeProblem(writer, request, http.StatusInternalServerError, "SNAPSHOT_ENCODE_FAILED", err.Error(), false, nil)
		return
	}
	if err := server.store.UpdateSegmentLifecycle(request.Context(), segmentID, string(model.SegmentRunning), "", startSnapshot, nil); err != nil {
		server.writeExperimentStoreError(writer, request, "开始实验失败", err)
		return
	}
	server.writeExperimentDetail(writer, request, http.StatusOK, segmentID, nil)
}

// handleCompleteExperiment 结束实验：running→completed，写入终点快照、摘要并关联 Trace。
func (server *Server) handleCompleteExperiment(writer http.ResponseWriter, request *http.Request) {
	server.finishExperiment(writer, request, model.SegmentCompleted, "")
}

// handleFailExperiment 标记实验失败：running→failed，写入终点快照、原因并关联 Trace。
func (server *Server) handleFailExperiment(writer http.ResponseWriter, request *http.Request) {
	reason := ""
	var body failExperimentRequest
	if err := decodeJSON(writer, request, server.config.HTTP.MaxBodyBytes, &body); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	reason = strings.TrimSpace(body.Reason)
	if reason == "" {
		reason = "用户标记失败"
	}
	server.finishExperiment(writer, request, model.SegmentFailed, reason)
}

// finishExperiment 推进切面到终态：校验 running、写终点快照与摘要、关联 Trace。
func (server *Server) finishExperiment(writer http.ResponseWriter, request *http.Request, status model.SegmentStatus, reason string) {
	if !server.requireExperimentStore(writer, request) {
		return
	}
	segmentID := request.PathValue("id")
	record, err := server.store.GetSegment(request.Context(), segmentID)
	if err != nil {
		server.writeSegmentNotFound(writer, request, segmentID, err)
		return
	}
	if record.Status != string(model.SegmentRunning) {
		writeProblem(writer, request, http.StatusConflict, "SEGMENT_NOT_RUNNING",
			fmt.Sprintf("实验当前状态为 %s，只有 running 状态的实验可以结束。", record.Status), false, nil)
		return
	}
	if !server.requireCache(writer, request) {
		return
	}
	endSnapshot, err := json.Marshal(server.aggregator.CurrentSnapshot(time.Now().UTC()))
	if err != nil {
		writeProblem(writer, request, http.StatusInternalServerError, "SNAPSHOT_ENCODE_FAILED", err.Error(), false, nil)
		return
	}
	summary := server.buildExperimentSummary(request.Context(), record, time.Now().UTC())
	if err := server.store.UpdateSegmentLifecycle(request.Context(), segmentID, string(status), reason, endSnapshot, summary); err != nil {
		server.writeExperimentStoreError(writer, request, "结束实验失败", err)
		return
	}
	warnings := server.linkSegmentTraces(request.Context(), record)
	server.writeExperimentDetail(writer, request, http.StatusOK, segmentID, warnings)
}

// handleListExperiments 列出实验切面，可按状态过滤。
func (server *Server) handleListExperiments(writer http.ResponseWriter, request *http.Request) {
	if !server.requireExperimentStore(writer, request) {
		return
	}
	status := strings.TrimSpace(request.URL.Query().Get("status"))
	if status != "" && !validSegmentStatus(status) {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_SEGMENT_STATUS",
			"status 必须是 pending、running、completed 或 failed 之一。", false, nil)
		return
	}
	limit := queryInteger(request, "limit", 50)
	if limit < 1 || limit > experimentListMaxLimit {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_LIMIT",
			fmt.Sprintf("limit must be between 1 and %d.", experimentListMaxLimit), false, nil)
		return
	}
	records, err := server.store.ListSegments(request.Context(), limit, status)
	if err != nil {
		server.writeExperimentStoreError(writer, request, "查询实验列表失败", err)
		return
	}
	writeData(writer, request, http.StatusOK, nonNilSegmentRecords(records), false, nil, sourceVersions(server.cache.SyncedAt()))
}

// handleExperimentDetail 返回切面详情：segments 一行 + 事件/指标/Trace 三个子集。
func (server *Server) handleExperimentDetail(writer http.ResponseWriter, request *http.Request) {
	if !server.requireExperimentStore(writer, request) {
		return
	}
	segmentID := request.PathValue("id")
	server.writeExperimentDetail(writer, request, http.StatusOK, segmentID, nil)
}

// writeExperimentDetail 查询并返回切面详情；segmentID 不存在时返回 404。
func (server *Server) writeExperimentDetail(writer http.ResponseWriter, request *http.Request, status int, segmentID string, warnings []string) {
	record, err := server.store.GetSegment(request.Context(), segmentID)
	if err != nil {
		server.writeSegmentNotFound(writer, request, segmentID, err)
		return
	}
	detail := model.SegmentDetail{Segment: *record}
	if detail.Events, err = server.store.ListSegmentEvents(request.Context(), segmentID, 5000); err != nil {
		server.writeExperimentStoreError(writer, request, "查询实验事件失败", err)
		return
	}
	if detail.Metrics, err = server.store.ListSegmentMetrics(request.Context(), segmentID, 5000); err != nil {
		server.writeExperimentStoreError(writer, request, "查询实验指标失败", err)
		return
	}
	if detail.Traces, err = server.store.ListSegmentTraces(request.Context(), segmentID); err != nil {
		server.writeExperimentStoreError(writer, request, "查询实验 Trace 失败", err)
		return
	}
	writeData(writer, request, status, detail, false, warnings, sourceVersions(server.cache.SyncedAt()))
}

// buildExperimentSummary 汇总切面内的群体数据：时长、各事件类型计数与分桶数。
func (server *Server) buildExperimentSummary(ctx context.Context, record *model.SegmentRecord, end time.Time) json.RawMessage {
	events, err := server.store.ListSegmentEvents(ctx, record.SegmentID, 5000)
	if err != nil {
		server.logger.Warn("Could not count segment events for summary", "segmentId", record.SegmentID, "error", err)
	}
	metrics, err := server.store.ListSegmentMetrics(ctx, record.SegmentID, 5000)
	if err != nil {
		server.logger.Warn("Could not count segment metrics for summary", "segmentId", record.SegmentID, "error", err)
	}
	eventCounts := map[string]int{}
	for _, event := range events {
		eventCounts[event.EventType]++
	}
	durationSeconds := 0
	if record.StartedAt != nil {
		durationSeconds = int(end.Sub(*record.StartedAt).Seconds())
	}
	summary, _ := json.Marshal(map[string]any{
		"durationSeconds": durationSeconds,
		"eventCounts":     eventCounts,
		"metricBuckets":   len(metrics),
	})
	return summary
}

// linkSegmentTraces 检索实验窗口内的 Trace，写入索引并关联到切面。
// Trace 检索失败不阻塞实验结束，只返回告警供前端展示。
func (server *Server) linkSegmentTraces(ctx context.Context, record *model.SegmentRecord) []string {
	if record.StartedAt == nil {
		return nil
	}
	traces, err := server.jaeger.Search(ctx, jaegerprovider.SearchRequest{
		Start: *record.StartedAt,
		End:   time.Now().UTC(),
		Limit: 200,
	})
	if err != nil {
		return []string{"Trace 关联失败：" + err.Error()}
	}
	if len(traces) == 0 {
		return nil
	}
	server.indexTraces(ctx, traces)
	traceIDs := make([]string, 0, len(traces))
	for _, trace := range traces {
		traceIDs = append(traceIDs, trace.TraceID)
	}
	if err := server.store.LinkSegmentTraces(ctx, record.SegmentID, traceIDs); err != nil {
		return []string{"Trace 关联失败：" + err.Error()}
	}
	return nil
}

// requireExperimentStore 校验持久化存储可用；实验切面完全依赖数据库。
func (server *Server) requireExperimentStore(writer http.ResponseWriter, request *http.Request) bool {
	if server.store.Available() {
		return true
	}
	writeProblem(writer, request, http.StatusServiceUnavailable, "STORE_UNAVAILABLE",
		"持久化存储不可用，无法管理实验切面。", true, nil)
	return false
}

// writeExperimentStoreError 统一持久化失败响应：写路径 503，读路径降级。
func (server *Server) writeExperimentStoreError(writer http.ResponseWriter, request *http.Request, action string, err error) {
	writeProblem(writer, request, http.StatusServiceUnavailable, "SEGMENT_STORE_FAILED",
		action+"："+err.Error(), true, nil)
}

// writeSegmentNotFound 统一切面不存在响应。
func (server *Server) writeSegmentNotFound(writer http.ResponseWriter, request *http.Request, segmentID string, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeProblem(writer, request, http.StatusNotFound, "SEGMENT_NOT_FOUND",
			fmt.Sprintf("实验 %s 不存在。", segmentID), false, nil)
		return
	}
	server.writeExperimentStoreError(writer, request, "查询实验失败", err)
}

// validSegmentStatus 校验切面状态过滤值。
func validSegmentStatus(status string) bool {
	switch status {
	case string(model.SegmentPending), string(model.SegmentRunning),
		string(model.SegmentCompleted), string(model.SegmentFailed):
		return true
	}
	return false
}

// validateExperimentIdentity 校验实验的租户与名称：必填、限长、无控制字符。
func validateExperimentIdentity(tenant, name string) error {
	if tenant == "" {
		return errors.New("tenant 必填")
	}
	if name == "" {
		return errors.New("name 必填")
	}
	if len(tenant) > experimentNameMaxLen || len(name) > experimentNameMaxLen {
		return fmt.Errorf("tenant 与 name 长度不能超过 %d 个字符", experimentNameMaxLen)
	}
	for _, r := range tenant + name {
		if r < 0x20 || r == 0x7f {
			return errors.New("tenant 与 name 不能包含控制字符")
		}
	}
	return nil
}

// nonNilSegmentRecords 避免空列表被编码为 null。
func nonNilSegmentRecords(records []store.SegmentRecord) []store.SegmentRecord {
	if records == nil {
		return []store.SegmentRecord{}
	}
	return records
}
