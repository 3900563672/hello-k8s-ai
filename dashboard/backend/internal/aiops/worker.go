package aiops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops/prompts"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

// 分析单次拉取上限：轮询时最多认领的任务数。
const analysisBatchSize = 8

// Service 是 AIOps 分析服务：入队（API 触发）+ 后台 worker（事件驱动 L1/L2）+ 同步对话（#110 阶段二）。
// 单向依赖：只读 segments/子级摘要，写 aiops_* 表。
type Service struct {
	database store.Store
	llm      LLM
	logger   *slog.Logger
	config   config.AIOpsConfig

	// 同步对话会话限流状态（sessionID → 滑动窗口内调用时间戳）。
	chatMu   sync.Mutex
	chatRate map[string][]time.Time

	// 运行时配置（面板写入，#110 阶段四）保护锁。
	configMu sync.Mutex
}

// NewService 构造分析服务；database 必须可用（调用方已判断）。
func NewService(cfg config.AIOpsConfig, database store.Store, llm LLM, logger *slog.Logger) *Service {
	return &Service{database: database, llm: llm, logger: logger, config: cfg, chatRate: make(map[string][]time.Time)}
}

func randomAnalysisID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err == nil {
		return "aiops-" + hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("aiops-%d", time.Now().UnixNano())
}

// EnqueueAnalysis 为切面创建 pending 分析 + 任务记录（#110 阶段一）；
// 同切面重复入队幂等（analyses 与 jobs 都以 segment_id 唯一）。
func (service *Service) EnqueueAnalysis(ctx context.Context, segmentID string) error {
	analysisID := randomAnalysisID()
	analysis := model.AIOpsAnalysis{
		AnalysisID: analysisID,
		SegmentID:  segmentID,
		Status:     string(model.AIOpsPending),
	}
	if err := service.database.CreateAIOpsAnalysis(ctx, analysis); err != nil {
		return fmt.Errorf("enqueue aiops analysis for segment %s: %w", segmentID, err)
	}
	job := model.AIOpsJob{
		JobID:       analysisID,
		SegmentID:   segmentID,
		Kind:        "analysis",
		Status:      "pending",
		MaxAttempts: 3,
	}
	if err := service.database.CreateAIOpsJob(ctx, job); err != nil {
		return fmt.Errorf("enqueue aiops job for segment %s: %w", segmentID, err)
	}
	return nil
}

// Run 启动后台 worker：先回收崩溃遗留的 running/aggregating，再按 PollInterval 轮询 pending。
// 阻塞直到 ctx 取消。
func (service *Service) Run(ctx context.Context) error {
	requeueContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	requeued, err := service.database.RequeueStaleAIOpsAnalyses(requeueContext, time.Now().UTC().Add(-service.config.StaleRequeueInterval))
	cancel()
	if err != nil {
		service.logger.Warn("AIOps stale analysis requeue failed", "error", err)
	} else if requeued > 0 {
		service.logger.Info("AIOps requeued stale analyses", "count", requeued)
	}
	requeueContext, cancel = context.WithTimeout(ctx, 30*time.Second)
	requeuedJobs, requeueErr := service.database.RequeueStaleAIOpsJobs(requeueContext, time.Now().UTC().Add(-service.config.StaleRequeueInterval))
	cancel()
	if requeueErr != nil {
		service.logger.Warn("AIOps stale job requeue failed", "error", requeueErr)
	} else if requeuedJobs > 0 {
		service.logger.Info("AIOps requeued stale jobs", "count", requeuedJobs)
	}
	ticker := time.NewTicker(service.config.PollInterval)
	defer ticker.Stop()
	windowTicker := time.NewTicker(service.config.WindowInterval)
	defer windowTicker.Stop()
	// 启动时先聚合一轮，避免开启后第一个窗口周期内无 L3/L4 产出。
	service.aggregateWindows(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			service.poll(ctx)
		case <-windowTicker.C:
			service.aggregateWindows(ctx)
		}
	}
}

// aggregateWindows 执行 M3 时间聚合与警戒：L3 窗口 → L4 日总结 → 分数序列警戒。
// 任一环节失败只记日志，不影响调度主流程与后续轮次。
func (service *Service) aggregateWindows(ctx context.Context) {
	if err := service.runWindowAggregation(ctx); err != nil {
		service.logger.Warn("AIOps L3 window aggregation failed", "error", err)
	}
	if err := service.runDayAggregation(ctx); err != nil {
		service.logger.Warn("AIOps L4 day aggregation failed", "error", err)
	}
	if err := service.evaluateAlerts(ctx); err != nil {
		service.logger.Warn("AIOps alert evaluation failed", "error", err)
	}
}

func (service *Service) poll(ctx context.Context) {
	// 任务队列驱动（#110 阶段一）：FOR UPDATE SKIP LOCKED 认领，一次最多一批。
	for i := 0; i < analysisBatchSize; i++ {
		job, ok, err := service.database.ClaimNextAIOpsJob(ctx)
		if err != nil {
			service.logger.Warn("AIOps poll claim job failed", "error", err)
			return
		}
		if !ok {
			return
		}
		service.processJob(ctx, job)
	}
}

// processJob 执行单个任务：复用 analyses 状态机（claim 幂等，analysis_id=job_id），
// 收尾统一回写任务状态（done/failed + finished_at + last_error）。
func (service *Service) processJob(ctx context.Context, job model.AIOpsJob) {
	analysis := model.AIOpsAnalysis{AnalysisID: job.JobID, SegmentID: job.SegmentID}
	processErr := service.processAnalysis(ctx, analysis)
	finishContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	status, errorText := "done", ""
	if processErr != nil {
		status, errorText = "failed", processErr.Error()
		service.logger.Error("AIOps analysis failed", "jobId", job.JobID, "segmentId", job.SegmentID, "error", processErr)
	}
	if completeErr := service.database.CompleteAIOpsJob(finishContext, job.JobID, status, errorText); completeErr != nil {
		service.logger.Error("AIOps complete job failed", "jobId", job.JobID, "error", completeErr)
	}
}

// processAnalysis 执行单次分析：claim → 加载切面 → L1 批量总结 → L2 混合打分 → 落库。
func (service *Service) processAnalysis(ctx context.Context, analysis model.AIOpsAnalysis) error {
	claimed, err := service.database.ClaimAIOpsAnalysis(ctx, analysis.AnalysisID)
	if err != nil {
		return fmt.Errorf("claim analysis: %w", err)
	}
	if !claimed {
		return nil // 已被其它 worker 认领
	}
	segment, err := service.database.GetSegment(ctx, analysis.SegmentID)
	if err != nil {
		return fmt.Errorf("load segment %s: %w", analysis.SegmentID, err)
	}
	events, err := service.database.ListSegmentEvents(ctx, analysis.SegmentID, 5000)
	if err != nil {
		return fmt.Errorf("load segment events: %w", err)
	}
	metrics, err := service.database.ListSegmentMetrics(ctx, analysis.SegmentID, 5000)
	if err != nil {
		return fmt.Errorf("load segment metrics: %w", err)
	}
	traces, err := service.database.ListSegmentTraces(ctx, analysis.SegmentID)
	if err != nil {
		return fmt.Errorf("load segment traces: %w", err)
	}
	startSnapshot, endSnapshot := parseSnapshots(segment)
	entities := extractEntities(startSnapshot, endSnapshot)

	if err := service.database.UpdateAIOpsAnalysisProgress(ctx, analysis.AnalysisID, string(model.AIOpsRunning), len(entities), 0, ""); err != nil {
		return fmt.Errorf("update analysis progress: %w", err)
	}

	callsLeft := service.config.MaxCallsPerAnalysis
	summaries := make([]model.AIOpsEntitySummary, 0, len(entities))
	done := 0
	for offset := 0; offset < len(entities); offset += service.config.MaxEntitiesPerCall {
		end := offset + service.config.MaxEntitiesPerCall
		if end > len(entities) {
			end = len(entities)
		}
		batch := entities[offset:end]
		batchSummaries, usedCall := service.summarizeEntities(ctx, analysis.AnalysisID, batch, events, callsLeft > 0)
		if usedCall {
			callsLeft--
		}
		summaries = append(summaries, batchSummaries...)
		if err := service.database.UpsertAIOpsEntitySummaries(ctx, analysis.AnalysisID, batchSummaries); err != nil {
			return fmt.Errorf("upsert L1 summaries: %w", err)
		}
		done += len(batch)
		if err := service.database.UpdateAIOpsAnalysisProgress(ctx, analysis.AnalysisID, string(model.AIOpsRunning), len(entities), done, ""); err != nil {
			return fmt.Errorf("update L1 progress: %w", err)
		}
	}
	if err := service.database.UpdateAIOpsAnalysisProgress(ctx, analysis.AnalysisID, string(model.AIOpsAggregating), len(entities), done, ""); err != nil {
		return fmt.Errorf("update analysis to aggregating: %w", err)
	}

	hard := computeHardMetrics(events, metrics, traces)
	hard.QPSTarget = targetQPS(endSnapshot)
	hard.RestartCount = snapshotRestartCount(endSnapshot)
	scores := service.judgeSegment(ctx, hard, summaries, callsLeft > 0)
	scoresPayload, err := json.Marshal(scores)
	if err != nil {
		return fmt.Errorf("marshal scores: %w", err)
	}
	summaryPayload, err := json.Marshal(map[string]any{
		"entityTotal":   len(entities),
		"entityIssues":  countIssues(summaries),
		"verdict":       scores.Verdict,
		"overall":       scores.Overall,
		"hard":          hard,
		"fallbackScore": scores.Reason,
	})
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}
	if err := service.database.CompleteAIOpsAnalysis(ctx, analysis.AnalysisID, scoresPayload, summaryPayload); err != nil {
		return fmt.Errorf("complete analysis: %w", err)
	}
	service.logger.Info("AIOps analysis completed", "analysisId", analysis.AnalysisID,
		"segmentId", analysis.SegmentID, "entities", len(entities), "verdict", scores.Verdict, "overall", scores.Overall)
	return nil
}

// summarizeEntities 对一批实体做 L1 总结；LLM 可用且预算内时调用模型，否则规则兜底。
func (service *Service) summarizeEntities(ctx context.Context, analysisID string, batch []entityFact,
	events []model.SegmentEvent, llmAvailable bool) ([]model.AIOpsEntitySummary, bool) {
	usedCall := false
	if llmAvailable {
		userPrompt, err := l1UserPrompt(batch)
		if err == nil {
			usedCall = true
			parsed, _, ok, reason := callStructured(ctx, service, prompts.L1Entity, userPrompt,
				parseEntityResults, validateEntityResults)
			if ok {
				service.logger.Debug("AIOps L1 batch summarized by LLM", "analysisId", analysisID, "entities", len(parsed))
				return service.normalizeEntityResults(analysisID, batch, parsed), true
			}
			service.logger.Warn("AIOps L1 falling back to rules", "analysisId", analysisID, "reason", reason)
		}
	}
	return service.ruleSummaries(analysisID, batch, events), usedCall
}

// judgeSegment L2 打分：LLM 可用且预算内时调用，否则规则兜底。
func (service *Service) judgeSegment(ctx context.Context, hard hardMetrics, summaries []model.AIOpsEntitySummary, llmAvailable bool) model.AIOpsScores {
	if llmAvailable {
		userPrompt, truncated, err := l2UserPrompt(hard, summaries, budgetL2Summaries, service.logger)
		if err == nil {
			if truncated {
				service.logger.Warn("AIOps L2 input truncated by budget", "budgetRunes", budgetL2Summaries)
			}
			scores, _, ok, reason := callStructured(ctx, service, prompts.L2Scores, userPrompt,
				func(content string) (model.AIOpsScores, error) {
					var scores model.AIOpsScores
					err := json.Unmarshal([]byte(content), &scores)
					return scores, err
				}, validateScores)
			if ok {
				service.logger.Debug("AIOps L2 judged by LLM")
				return normalizeScores(scores)
			}
			service.logger.Warn("AIOps L2 falling back to rules", "reason", reason)
		}
	}
	fallback := fallbackScores(hard)
	return fallback
}

// ruleSummaries 规则兜底 L1：按错误事件/重启/未就绪分类。
func (service *Service) ruleSummaries(analysisID string, batch []entityFact, events []model.SegmentEvent) []model.AIOpsEntitySummary {
	eventByEntity := make(map[string][]string)
	for _, event := range events {
		for _, entity := range batch {
			if event.Entity != "" && (strings.HasSuffix(event.Entity, entity.Name) || event.Entity == entity.Name) {
				eventByEntity[entity.Name] = append(eventByEntity[entity.Name], fmt.Sprintf("%s(%s)", event.EventType, event.Severity))
			}
		}
	}
	summaries := make([]model.AIOpsEntitySummary, 0, len(batch))
	for _, entity := range batch {
		result := classifyEntity(entity, eventByEntity[entity.Name])
		summaries = append(summaries, model.AIOpsEntitySummary{
			SummaryID:      randomAnalysisID(),
			AnalysisID:     analysisID,
			EntityKind:     result.EntityKind,
			EntityName:     result.EntityName,
			Classification: result.Classification,
			Phenomenon:     result.Phenomenon,
			IssueFlag:      result.IssueFlag,
			Conclusion:     result.Conclusion,
		})
	}
	return summaries
}

// normalizeEntityResults 收敛 LLM 返回的 L1 结果；LLM 遗漏的实体用规则兜底补齐，
// 保证 L1 全量覆盖（不做健康过滤，所有实体都要有总结）。
func (service *Service) normalizeEntityResults(analysisID string, batch []entityFact, results []l1EntityResult) []model.AIOpsEntitySummary {
	byName := make(map[string]l1EntityResult, len(results))
	for _, result := range results {
		byName[result.EntityName] = result
	}
	summaries := make([]model.AIOpsEntitySummary, 0, len(batch))
	for _, entity := range batch {
		result, exists := byName[entity.Name]
		if !exists {
			fallback := service.ruleSummaries(analysisID, []entityFact{entity}, nil)
			summaries = append(summaries, fallback...)
			continue
		}
		classification := result.Classification
		switch classification {
		case string(model.AIOpsHealthy), string(model.AIOpsSuspect), string(model.AIOpsProblem):
		default:
			classification = string(model.AIOpsHealthy)
		}
		summaries = append(summaries, model.AIOpsEntitySummary{
			SummaryID:      randomAnalysisID(),
			AnalysisID:     analysisID,
			EntityKind:     result.EntityKind,
			EntityName:     result.EntityName,
			Classification: classification,
			Phenomenon:     strings.TrimSpace(result.Phenomenon),
			IssueFlag:      result.IssueFlag || classification != string(model.AIOpsHealthy),
			Conclusion:     strings.TrimSpace(result.Conclusion),
		})
	}
	return summaries
}

// targetQPS 从终点快照的租户流量目标求和（L2 目标达成率分母）。
func targetQPS(snapshot *model.CurrentSnapshot) float64 {
	if snapshot == nil {
		return 0
	}
	var total int
	for _, traffic := range snapshot.Traffic.Tenants {
		total += traffic.RequestedQPS
	}
	return float64(total)
}

// snapshotRestartCount 统计终点快照里所有 Pod 的容器重启总数。
func snapshotRestartCount(snapshot *model.CurrentSnapshot) int {
	if snapshot == nil {
		return 0
	}
	total := 0
	for _, pod := range snapshot.Workloads.Pods {
		total += podRestarts(pod)
	}
	return total
}

// parseEntityResults 解析 L1 模型输出：容忍前后缀文本，截取首个 JSON 数组。
func parseEntityResults(content string) ([]l1EntityResult, error) {
	trimmed := strings.TrimSpace(content)
	if start := strings.Index(trimmed, "["); start >= 0 {
		if end := strings.LastIndex(trimmed, "]"); end > start {
			trimmed = trimmed[start : end+1]
		}
	}
	var results []l1EntityResult
	if err := json.Unmarshal([]byte(trimmed), &results); err != nil {
		return nil, fmt.Errorf("parse L1 JSON: %w", err)
	}
	return results, nil
}

func parseSnapshots(segment *store.SegmentRecord) (*model.CurrentSnapshot, *model.CurrentSnapshot) {
	parse := func(payload json.RawMessage) *model.CurrentSnapshot {
		if len(payload) == 0 {
			return nil
		}
		var snapshot model.CurrentSnapshot
		if err := json.Unmarshal(payload, &snapshot); err != nil {
			return nil
		}
		return &snapshot
	}
	return parse(segment.StartSnapshot), parse(segment.EndSnapshot)
}

func countIssues(summaries []model.AIOpsEntitySummary) int {
	count := 0
	for _, summary := range summaries {
		if summary.IssueFlag {
			count++
		}
	}
	return count
}
