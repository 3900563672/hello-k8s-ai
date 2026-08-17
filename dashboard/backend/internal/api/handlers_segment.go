package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	jaegerprovider "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/jaeger"
	prometheusprovider "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/prometheus"
)

// maxSegmentWindow 段查询允许的最大时间跨度，避免超长区间压垮 Prometheus/Jaeger 查询。
const maxSegmentWindow = 24 * time.Hour

// segmentMetricIDs 段切面查询的指标集合，与 overview 保持一致。
var segmentMetricIDs = []string{
	"simulator.ttft", "simulator.queue", "simulator.qps",
	"simulator.errorRate", "simulator.tickLatency",
}

// handleSegment 返回时间段切面：起点/终点全局状态 + [start, end] 区间指标与 Trace。
// 段查询回答"一次调度/实验从什么状态开始、到什么状态结束、中间发生了什么"。
func (server *Server) handleSegment(writer http.ResponseWriter, request *http.Request) {
	start, end, err := parseSegmentWindow(request)
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_SEGMENT_WINDOW", err.Error(), false, nil)
		return
	}
	segment := model.SegmentOverview{
		Availability: "unavailable",
		Start:        start,
		End:          end,
		Metrics:      map[string]model.MetricResult{},
		Traces:       []model.TraceSummary{},
		Freshness:    map[string]model.ProviderState{},
	}
	startSnapshot, startWarning := server.segmentSnapshot(request, start, "起点")
	endSnapshot, endWarning := server.segmentSnapshot(request, end, "终点")
	if startSnapshot == nil || endSnapshot == nil {
		warnings := []string{}
		if startWarning != "" {
			warnings = append(warnings, startWarning)
		}
		if endWarning != "" {
			warnings = append(warnings, endWarning)
		}
		writeData(writer, request, http.StatusOK, segment, true, warnings, sourceVersions(server.cache.SyncedAt()))
		return
	}
	segment.Availability = "available"
	segment.StartSnapshot = startSnapshot
	segment.EndSnapshot = endSnapshot

	warnings := server.segmentCoverageWarnings(start, end, server.currentClockState().ServerTime)
	var mutex sync.Mutex
	var group sync.WaitGroup
	for _, metricID := range segmentMetricIDs {
		metricID := metricID
		group.Add(1)
		go func() {
			defer group.Done()
			result, queryErr := server.prometheus.QueryRange(request.Context(), prometheusprovider.Query{
				MetricID: metricID,
				Start:    start,
				End:      end,
				Tenant:   request.URL.Query().Get("tenant"),
				Model:    request.URL.Query().Get("model"),
				Instance: request.URL.Query().Get("instance"),
				Node:     request.URL.Query().Get("node"),
			})
			mutex.Lock()
			defer mutex.Unlock()
			if queryErr != nil {
				warnings = append(warnings, metricID+": "+queryErr.Error())
				return
			}
			segment.Metrics[metricID] = result
		}()
	}
	group.Add(1)
	go func() {
		defer group.Done()
		traces, queryErr := server.jaeger.Search(request.Context(), jaegerprovider.SearchRequest{
			Start: start, End: end, Limit: 50,
			Tenant: request.URL.Query().Get("tenant"), Model: request.URL.Query().Get("model"), Instance: request.URL.Query().Get("instance"),
		})
		mutex.Lock()
		defer mutex.Unlock()
		if queryErr != nil {
			warnings = append(warnings, "traces: "+queryErr.Error())
			return
		}
		segment.Traces = traces
	}()
	group.Wait()
	if len(segment.Traces) > 0 {
		server.indexTraces(request.Context(), segment.Traces)
	}
	providers, providerWarnings := server.providerStates(request.Context())
	segment.Freshness = providers
	warnings = append(warnings, providerWarnings...)
	writeData(writer, request, http.StatusOK, segment, len(warnings) > 0, warnings, sourceVersions(server.cache.SyncedAt()))
}

// parseSegmentWindow 解析并校验段查询的 start/end 参数。
func parseSegmentWindow(request *http.Request) (time.Time, time.Time, error) {
	startRaw := strings.TrimSpace(request.URL.Query().Get("start"))
	endRaw := strings.TrimSpace(request.URL.Query().Get("end"))
	if startRaw == "" || endRaw == "" {
		return time.Time{}, time.Time{}, errors.New("start and end query parameters are required")
	}
	start, err := time.Parse(time.RFC3339Nano, startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start timestamp: %w", err)
	}
	end, err := time.Parse(time.RFC3339Nano, endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end timestamp: %w", err)
	}
	start = start.UTC()
	end = end.UTC()
	if !start.Before(end) {
		return time.Time{}, time.Time{}, errors.New("start must be before end")
	}
	if end.Sub(start) > maxSegmentWindow {
		return time.Time{}, time.Time{}, fmt.Errorf("segment window %s exceeds maximum %s", end.Sub(start), maxSegmentWindow)
	}
	return start, end, nil
}

// segmentSnapshot 读取 at 之前最近的持久化快照并转换为段快照。
// 返回的第二个值为空字符串表示成功；否则为可直接展示的中文告警。
func (server *Server) segmentSnapshot(request *http.Request, at time.Time, label string) (*model.SegmentSnapshot, string) {
	if !server.store.Available() {
		return nil, "持久化存储不可用，无法读取" + label + "快照。"
	}
	stored, err := server.store.SnapshotAt(request.Context(), at)
	if err != nil {
		return nil, label + "快照查询失败：" + err.Error()
	}
	if stored == nil {
		return nil, label + "之前没有持久化快照，无法构成段切面。"
	}
	var snapshot model.CurrentSnapshot
	if err := json.Unmarshal(stored.Payload, &snapshot); err != nil {
		return nil, label + "快照解码失败：" + err.Error()
	}
	return &model.SegmentSnapshot{
		SnapshotID:    stored.ID,
		CapturedAt:    snapshot.CapturedAt,
		Configuration: snapshot.Configuration,
		Traffic:       snapshot.Traffic,
		Workloads:     snapshot.Workloads,
	}, ""
}

// segmentCoverageWarnings 针对段起点/终点分别计算 Prometheus/Jaeger 覆盖告警，
// 让"段内没有数据"与"数据超出保留窗口已丢失"可区分。
func (server *Server) segmentCoverageWarnings(start, end, now time.Time) []string {
	var warnings []string
	prometheusRetention := server.config.Prometheus.Retention
	if prometheusRetention > 0 && start.Before(now.Add(-prometheusRetention)) {
		warnings = append(warnings, fmt.Sprintf(
			"Prometheus 保留窗口为 %s，段起点早于保留窗口，区间指标可能已不完整。",
			prometheusRetention,
		))
	}
	if server.config.Jaeger.Retention > 0 {
		if end.Before(now.Add(-server.config.Jaeger.Retention)) {
			warnings = append(warnings, fmt.Sprintf(
				"Jaeger 保留窗口为 %s，段终点早于保留窗口，Trace 可能已不完整。",
				server.config.Jaeger.Retention,
			))
		}
	} else if end.Before(now.Add(-historyQueryWindow)) {
		warnings = append(warnings, "Jaeger 为内存存储（无持久化），历史 Trace 仅随进程存活保留，段内 Trace 可能已丢失。")
	}
	return warnings
}
