package aiops

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

// fakeStore 只实现 worker 需要的 store 方法，其余走 Disabled 兜底。
type fakeStore struct {
	store.Disabled
	mu           sync.Mutex
	analyses     map[string]model.AIOpsAnalysis
	summaries    map[string][]model.AIOpsEntitySummary
	segment      *store.SegmentRecord
	segmentErr   error
	events       []model.SegmentEvent
	metrics      []model.MetricBucket
	traces       []model.TraceSummary
	pendingOrder []string
	requeued     int
	jobs         map[string]model.AIOpsJob
	jobOrder     []string
}

func newFakeStore(segment *store.SegmentRecord) *fakeStore {
	return &fakeStore{
		analyses:  make(map[string]model.AIOpsAnalysis),
		summaries: make(map[string][]model.AIOpsEntitySummary),
		jobs:      make(map[string]model.AIOpsJob),
		segment:   segment,
	}
}

func (store *fakeStore) Available() bool { return true }

func (store *fakeStore) CreateAIOpsAnalysis(_ context.Context, analysis model.AIOpsAnalysis) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.analyses[analysis.SegmentID]; exists {
		return nil
	}
	analysis.CreatedAt = time.Now().UTC()
	analysis.UpdatedAt = analysis.CreatedAt
	store.analyses[analysis.SegmentID] = analysis
	store.pendingOrder = append(store.pendingOrder, analysis.AnalysisID)
	return nil
}

func (store *fakeStore) ClaimAIOpsAnalysis(_ context.Context, analysisID string) (bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for segmentID, analysis := range store.analyses {
		if analysis.AnalysisID == analysisID {
			if analysis.Status != string(model.AIOpsPending) {
				return false, nil
			}
			analysis.Status = string(model.AIOpsRunning)
			analysis.UpdatedAt = time.Now().UTC()
			store.analyses[segmentID] = analysis
			return true, nil
		}
	}
	return false, nil
}

func (store *fakeStore) RequeueStaleAIOpsAnalyses(_ context.Context, _ time.Time) (int, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.requeued++
	return 0, nil
}

func (store *fakeStore) UpdateAIOpsAnalysisProgress(_ context.Context, analysisID, status string, l1Total, l1Done int, errorText string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for segmentID, analysis := range store.analyses {
		if analysis.AnalysisID == analysisID {
			analysis.Status = status
			analysis.L1Total = l1Total
			analysis.L1Done = l1Done
			analysis.Error = errorText
			analysis.UpdatedAt = time.Now().UTC()
			store.analyses[segmentID] = analysis
			return nil
		}
	}
	return errors.New("analysis not found")
}

func (store *fakeStore) CompleteAIOpsAnalysis(_ context.Context, analysisID string, scores, summary json.RawMessage) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for segmentID, analysis := range store.analyses {
		if analysis.AnalysisID == analysisID {
			analysis.Status = string(model.AIOpsCompleted)
			analysis.Scores = scores
			analysis.Summary = summary
			analysis.UpdatedAt = time.Now().UTC()
			store.analyses[segmentID] = analysis
			return nil
		}
	}
	return errors.New("analysis not found")
}

func (store *fakeStore) FailAIOpsAnalysis(_ context.Context, analysisID, errorText string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for segmentID, analysis := range store.analyses {
		if analysis.AnalysisID == analysisID {
			analysis.Status = string(model.AIOpsFailed)
			analysis.Error = errorText
			analysis.UpdatedAt = time.Now().UTC()
			store.analyses[segmentID] = analysis
			return nil
		}
	}
	return errors.New("analysis not found")
}

func (store *fakeStore) ListAIOpsWindowSummaries(_ context.Context, _ string, _ int) ([]model.AIOpsWindowSummary, error) {
	return nil, nil
}

func (store *fakeStore) ListAIOpsAlerts(_ context.Context, _ int) ([]model.AIOpsAlert, error) {
	return nil, nil
}

func (store *fakeStore) CreateAIOpsJob(_ context.Context, job model.AIOpsJob) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, existing := range store.jobs {
		if existing.SegmentID == job.SegmentID {
			return nil
		}
	}
	job.CreatedAt = time.Now().UTC()
	job.UpdatedAt = job.CreatedAt
	store.jobs[job.JobID] = job
	store.jobOrder = append(store.jobOrder, job.JobID)
	return nil
}

func (store *fakeStore) ClaimNextAIOpsJob(_ context.Context) (model.AIOpsJob, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, jobID := range store.jobOrder {
		job, exists := store.jobs[jobID]
		if exists && job.Status == "pending" {
			job.Status = "running"
			job.Attempts++
			now := time.Now().UTC()
			job.StartedAt = &now
			job.UpdatedAt = now
			store.jobs[jobID] = job
			return job, true, nil
		}
	}
	return model.AIOpsJob{}, false, nil
}

func (store *fakeStore) CompleteAIOpsJob(_ context.Context, jobID, status, errorText string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	job, exists := store.jobs[jobID]
	if !exists {
		return errors.New("job not found")
	}
	job.Status = status
	job.LastError = errorText
	now := time.Now().UTC()
	job.FinishedAt = &now
	job.UpdatedAt = now
	store.jobs[jobID] = job
	return nil
}

func (store *fakeStore) ListAIOpsJobs(_ context.Context, _ int, status string) ([]model.AIOpsJob, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var result []model.AIOpsJob
	for _, jobID := range store.jobOrder {
		job := store.jobs[jobID]
		if status == "" || job.Status == status {
			result = append(result, job)
		}
	}
	return result, nil
}

func (store *fakeStore) RequeueStaleAIOpsJobs(_ context.Context, _ time.Time) (int, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	count := 0
	for jobID, job := range store.jobs {
		if job.Status == "running" {
			job.Status = "pending"
			store.jobs[jobID] = job
			count++
		}
	}
	return count, nil
}

func (store *fakeStore) ListAIOpsAnalyses(_ context.Context, limit int, status string) ([]model.AIOpsAnalysis, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var result []model.AIOpsAnalysis
	for _, analysisID := range store.pendingOrder {
		for _, analysis := range store.analyses {
			if analysis.AnalysisID == analysisID && (status == "" || analysis.Status == status) {
				result = append(result, analysis)
				if len(result) >= limit {
					return result, nil
				}
			}
		}
	}
	return result, nil
}

func (store *fakeStore) GetAIOpsAnalysis(_ context.Context, analysisID string) (*model.AIOpsAnalysis, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, analysis := range store.analyses {
		if analysis.AnalysisID == analysisID {
			copy := analysis
			return &copy, nil
		}
	}
	return nil, errors.New("not found")
}

func (store *fakeStore) GetAIOpsAnalysisBySegment(_ context.Context, segmentID string) (*model.AIOpsAnalysis, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	analysis, exists := store.analyses[segmentID]
	if !exists {
		return nil, errors.New("not found")
	}
	copy := analysis
	return &copy, nil
}

func (store *fakeStore) UpsertAIOpsEntitySummaries(_ context.Context, analysisID string, summaries []model.AIOpsEntitySummary) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.summaries[analysisID] = append(store.summaries[analysisID], summaries...)
	return nil
}

func (store *fakeStore) ListAIOpsEntitySummaries(_ context.Context, analysisID string) ([]model.AIOpsEntitySummary, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := append([]model.AIOpsEntitySummary(nil), store.summaries[analysisID]...)
	// 与 postgres 实现一致：问题实体在前，再按创建顺序。
	sort.SliceStable(result, func(i, j int) bool { return result[i].IssueFlag && !result[j].IssueFlag })
	return result, nil
}

func (store *fakeStore) GetSegment(_ context.Context, _ string) (*store.SegmentRecord, error) {
	if store.segmentErr != nil {
		return nil, store.segmentErr
	}
	return store.segment, nil
}

func (store *fakeStore) ListSegmentEvents(context.Context, string, int) ([]model.SegmentEvent, error) {
	return store.events, nil
}

func (store *fakeStore) ListSegmentMetrics(context.Context, string, int) ([]model.MetricBucket, error) {
	return store.metrics, nil
}

func (store *fakeStore) ListSegmentTraces(context.Context, string) ([]model.TraceSummary, error) {
	return store.traces, nil
}

// fakeLLM 按调用次数返回脚本化响应。
type fakeLLM struct {
	mu        sync.Mutex
	responses []string
	errs      []error
	calls     int
}

func newFakeLLM(responses []string, errs []error) *fakeLLM {
	return &fakeLLM{responses: responses, errs: errs}
}

func (llm *fakeLLM) StreamComplete(_ context.Context, system, user string, _ int, onDelta func(string), _ func(TokenUsage)) error {
	llm.mu.Lock()
	defer llm.mu.Unlock()
	if len(llm.responses) == 0 {
		return nil
	}
	response := llm.responses[0]
	llm.responses = llm.responses[1:]
	if len(llm.errs) > 0 && llm.errs[0] != nil {
		err := llm.errs[0]
		llm.errs = llm.errs[1:]
		return err
	}
	_ = system
	_ = user
	// 按 2 字符切分模拟流式增量。
	for i := 0; i < len(response); i += 2 {
		end := i + 2
		if end > len(response) {
			end = len(response)
		}
		onDelta(response[i:end])
	}
	return nil
}

func (llm *fakeLLM) CompleteJSON(context.Context, string, string, int) (string, error) {
	llm.mu.Lock()
	defer llm.mu.Unlock()
	index := llm.calls
	llm.calls++
	if index < len(llm.errs) && llm.errs[index] != nil {
		return "", llm.errs[index]
	}
	if index < len(llm.responses) {
		return llm.responses[index], nil
	}
	return "{}", nil
}

func (llm *fakeLLM) callCount() int {
	llm.mu.Lock()
	defer llm.mu.Unlock()
	return llm.calls
}

func testService(database *fakeStore, llm LLM) *Service {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg := config.AIOpsConfig{
		Enabled:             true,
		Model:               "fake",
		MaxTokensPerCall:    500,
		MaxCallsPerAnalysis: 8,
		MaxEntitiesPerCall:  20,
		PollInterval:        time.Hour,
	}
	return NewService(cfg, database, llm, logger)
}

func segmentWithSnapshots() *store.SegmentRecord {
	startPayload, _ := json.Marshal(model.CurrentSnapshot{
		Workloads: model.Workloads{Pods: []model.Pod{
			{Ref: model.ResourceRef{Name: "pod-a"}, Phase: "Running", Ready: true},
		}},
	})
	endPayload, _ := json.Marshal(model.CurrentSnapshot{
		Configuration: model.Configuration{Tenants: []model.PlatformResource{
			{Ref: model.ResourceRef{Name: "tenant-1"}},
		}},
		Traffic: model.Traffic{Tenants: []model.TenantTraffic{
			{Tenant: model.ResourceRef{Name: "tenant-1"}, RequestedQPS: 100},
		}},
		Workloads: model.Workloads{
			Pods: []model.Pod{
				{Ref: model.ResourceRef{Name: "pod-a"}, Phase: "Failed", Ready: false,
					Containers: []model.ContainerState{{Name: "c", RestartCount: 2}}},
			},
			Nodes: []model.ClusterNode{
				{Ref: model.ResourceRef{Name: "node-1"}, Ready: true, Role: "worker", Schedulable: true},
			},
		},
	})
	return &store.SegmentRecord{
		SegmentID:     "segment-1",
		Tenant:        "tenant-1",
		Name:          "test",
		Status:        string(model.SegmentCompleted),
		StartSnapshot: startPayload,
		EndSnapshot:   endPayload,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
}

// TestProcessAnalysisWithLLM 验证 LLM 可用时 L1/L2 全链路完成。
func TestProcessAnalysisWithLLM(t *testing.T) {
	database := newFakeStore(segmentWithSnapshots())
	database.events = []model.SegmentEvent{{EventID: "e1", SegmentID: "segment-1", EventType: model.SegmentEventError}}
	llm := newFakeLLM([]string{
		`[{"entityKind":"Pod","entityName":"pod-a","phenomenon":"Failed","issueFlag":true,"classification":"problem","conclusion":"restart"}]`,
		`{"goal":90,"stability":70,"efficiency":80,"anomaly":50,"overall":78,"verdict":"attention","reason":"ok"}`,
	}, nil)
	service := testService(database, llm)

	if err := service.EnqueueAnalysis(context.Background(), "segment-1"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	analysis, err := database.GetAIOpsAnalysisBySegment(context.Background(), "segment-1")
	if err != nil || analysis == nil {
		t.Fatalf("get analysis: %v", err)
	}
	if err := service.processAnalysis(context.Background(), *analysis); err != nil {
		t.Fatalf("process: %v", err)
	}
	completed, _ := database.GetAIOpsAnalysisBySegment(context.Background(), "segment-1")
	if completed.Status != string(model.AIOpsCompleted) {
		t.Fatalf("status = %s, want completed", completed.Status)
	}
	var scores model.AIOpsScores
	if err := json.Unmarshal(completed.Scores, &scores); err != nil {
		t.Fatalf("scores: %v", err)
	}
	if scores.Overall != 78 || scores.Verdict != "attention" {
		t.Fatalf("unexpected scores: %+v", scores)
	}
	summaries, _ := database.ListAIOpsEntitySummaries(context.Background(), completed.AnalysisID)
	if len(summaries) != 3 { // pod-a + node-1 + tenant-1
		t.Fatalf("entity summaries = %d, want 3", len(summaries))
	}
	if summaries[0].EntityName != "pod-a" || summaries[0].Classification != string(model.AIOpsProblem) {
		t.Fatalf("unexpected first summary: %+v", summaries[0])
	}
}

// TestProcessAnalysisFallback 验证 LLM 全失败时规则兜底仍能完成。
func TestProcessAnalysisFallback(t *testing.T) {
	database := newFakeStore(segmentWithSnapshots())
	llm := newFakeLLM(nil, []error{errors.New("down"), errors.New("down")})
	service := testService(database, llm)

	if err := service.EnqueueAnalysis(context.Background(), "segment-1"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	analysis, _ := database.GetAIOpsAnalysisBySegment(context.Background(), "segment-1")
	if err := service.processAnalysis(context.Background(), *analysis); err != nil {
		t.Fatalf("process: %v", err)
	}
	completed, _ := database.GetAIOpsAnalysisBySegment(context.Background(), "segment-1")
	if completed.Status != string(model.AIOpsCompleted) {
		t.Fatalf("status = %s, want completed", completed.Status)
	}
	var scores model.AIOpsScores
	if err := json.Unmarshal(completed.Scores, &scores); err != nil {
		t.Fatalf("scores: %v", err)
	}
	if scores.Overall < 0 || scores.Overall > 100 {
		t.Fatalf("fallback overall out of range: %d", scores.Overall)
	}
	if scores.Verdict != "problem" {
		t.Fatalf("pod failed with restarts, want verdict problem, got %s", scores.Verdict)
	}
	summaries, _ := database.ListAIOpsEntitySummaries(context.Background(), completed.AnalysisID)
	found := false
	for _, summary := range summaries {
		if summary.EntityName == "pod-a" && summary.Classification == string(model.AIOpsProblem) {
			found = true
		}
	}
	if !found {
		t.Fatalf("pod-a should be classified problem by fallback: %+v", summaries)
	}
}

// TestEnqueueIdempotent 验证同切面重复入队不产生第二条记录。
func TestEnqueueIdempotent(t *testing.T) {
	database := newFakeStore(segmentWithSnapshots())
	service := testService(database, newFakeLLM(nil, nil))
	ctx := context.Background()
	if err := service.EnqueueAnalysis(ctx, "segment-1"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := service.EnqueueAnalysis(ctx, "segment-1"); err != nil {
		t.Fatalf("enqueue again: %v", err)
	}
	analyses, _ := database.ListAIOpsAnalyses(ctx, 10, "")
	if len(analyses) != 1 {
		t.Fatalf("analyses = %d, want 1", len(analyses))
	}
}

// TestParseEntityResults 验证容忍前后缀文本的解析。
func TestParseEntityResults(t *testing.T) {
	content := "```json\n[{\"entityKind\":\"Pod\",\"entityName\":\"p1\",\"classification\":\"healthy\"}]\n```"
	results, err := parseEntityResults(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(results) != 1 || results[0].EntityName != "p1" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

// TestExtractEntities 验证实体提取合并与分类信息。
func TestExtractEntities(t *testing.T) {
	segment := segmentWithSnapshots()
	start, end := parseSnapshots(segment)
	entities := extractEntities(start, end)
	if len(entities) != 3 {
		t.Fatalf("entities = %d, want 3 (pod-a, node-1, tenant-1)", len(entities))
	}
	var pod *entityFact
	for index := range entities {
		if entities[index].Name == "pod-a" {
			pod = &entities[index]
		}
	}
	if pod == nil || pod.Restarts != 2 || pod.Changes != "phase Running→Failed" {
		t.Fatalf("unexpected pod fact: %+v", pod)
	}
}

// TestPollProcessesJob 验证任务队列驱动：Enqueue 建 pending job → poll 认领 →
// 复用 analyses 状态机完成 → job 收尾 done；失败路径回写 failed + last_error。
func TestPollProcessesJob(t *testing.T) {
	database := newFakeStore(segmentWithSnapshots())
	llm := newFakeLLM(nil, []error{errors.New("down"), errors.New("down")})
	service := testService(database, llm)

	if err := service.EnqueueAnalysis(context.Background(), "segment-1"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	jobs, err := database.ListAIOpsJobs(context.Background(), 10, "")
	if err != nil || len(jobs) != 1 || jobs[0].Status != "pending" {
		t.Fatalf("jobs = %+v, err = %v; want 1 pending", jobs, err)
	}

	service.poll(context.Background())

	jobs, _ = database.ListAIOpsJobs(context.Background(), 10, "")
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(jobs))
	}
	if jobs[0].Status != "done" {
		t.Fatalf("job status = %s, want done (fallback completes)", jobs[0].Status)
	}
	if jobs[0].Attempts != 1 {
		t.Fatalf("job attempts = %d, want 1", jobs[0].Attempts)
	}
	analysis, err := database.GetAIOpsAnalysisBySegment(context.Background(), "segment-1")
	if err != nil || analysis == nil {
		t.Fatalf("get analysis: %v", err)
	}
	if analysis.Status != string(model.AIOpsCompleted) {
		t.Fatalf("analysis status = %s, want completed", analysis.Status)
	}
}

// TestJobFailedRecordsError 验证处理失败时任务回写 failed + last_error，attempts 递增。
func TestJobFailedRecordsError(t *testing.T) {
	database := newFakeStore(segmentWithSnapshots())
	llm := newFakeLLM(nil, []error{errors.New("down"), errors.New("down")})
	service := testService(database, llm)
	// 让 GetSegment 失败，processAnalysis 在 claim 后直接报错。
	database.segmentErr = errors.New("segment gone")

	if err := service.EnqueueAnalysis(context.Background(), "segment-1"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	service.poll(context.Background())

	jobs, _ := database.ListAIOpsJobs(context.Background(), 10, "")
	if len(jobs) != 1 || jobs[0].Status != "failed" {
		t.Fatalf("job = %+v, want failed", jobs)
	}
	if jobs[0].LastError == "" {
		t.Fatalf("last_error empty, want reason")
	}
	if jobs[0].Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", jobs[0].Attempts)
	}
}
