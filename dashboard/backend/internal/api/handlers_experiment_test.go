package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	dashboardkubernetes "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/kubernetes"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/readmodel"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

// experimentStoreStub 模拟切面持久化：内存 map，生命周期语义与 SQL 实现一致。
type experimentStoreStub struct {
	store.Disabled
	mu      sync.Mutex
	records map[string]model.SegmentRecord
	events  map[string][]model.SegmentEvent
	metrics map[string][]store.MetricBucket
	traces  map[string][]model.TraceSummary
	linked  map[string][]string
}

func newExperimentStoreStub() *experimentStoreStub {
	return &experimentStoreStub{
		records: map[string]model.SegmentRecord{},
		events:  map[string][]model.SegmentEvent{},
		metrics: map[string][]store.MetricBucket{},
		traces:  map[string][]model.TraceSummary{},
		linked:  map[string][]string{},
	}
}

func (stub *experimentStoreStub) Available() bool { return true }

func (stub *experimentStoreStub) ReserveIdempotency(
	_ context.Context,
	key string,
	requestHash string,
	expiresAt time.Time,
) (*store.IdempotencyRecord, bool, error) {
	return &store.IdempotencyRecord{Key: key, RequestHash: requestHash, State: "pending", CreatedAt: time.Now().UTC(), ExpiresAt: expiresAt}, true, nil
}

func (stub *experimentStoreStub) CompleteIdempotency(context.Context, string, string, int, json.RawMessage) error {
	return nil
}

func (stub *experimentStoreStub) ReleaseIdempotency(context.Context, string, string) error {
	return nil
}

func (stub *experimentStoreStub) CreateSegment(_ context.Context, record store.SegmentRecord) error {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.records[record.SegmentID] = model.SegmentRecord(record)
	return nil
}

func (stub *experimentStoreStub) UpdateSegmentLifecycle(_ context.Context, segmentID, status, reason string, snapshot, summary json.RawMessage) error {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	record, exists := stub.records[segmentID]
	if !exists {
		return fmt.Errorf("update segment lifecycle: %w", pgx.ErrNoRows)
	}
	record.Status = status
	record.Reason = reason
	if status == string(model.SegmentRunning) {
		record.StartSnapshot = snapshot
		now := time.Now().UTC()
		record.StartedAt = &now
	} else {
		record.EndSnapshot = snapshot
		record.Summary = summary
		now := time.Now().UTC()
		record.EndedAt = &now
	}
	stub.records[segmentID] = record
	return nil
}

func (stub *experimentStoreStub) ListSegments(_ context.Context, _ int, status string) ([]store.SegmentRecord, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	var records []store.SegmentRecord
	for _, record := range stub.records {
		if status == "" || record.Status == status {
			records = append(records, store.SegmentRecord(record))
		}
	}
	return records, nil
}

func (stub *experimentStoreStub) GetSegment(_ context.Context, segmentID string) (*store.SegmentRecord, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	record, exists := stub.records[segmentID]
	if !exists {
		return nil, fmt.Errorf("get segment %s: %w", segmentID, pgx.ErrNoRows)
	}
	// 与 SQL 语义保持一致：完整生命周期后 StartedAt 存在；测试用 stub 不写 StartedAt，
	// 使完成路径跳过 Jaeger 关联（Jaeger 是外部 HTTP 依赖，不在本测试范围）。
	copy := store.SegmentRecord(record)
	copy.StartedAt = nil
	return &copy, nil
}

func (stub *experimentStoreStub) RecordSegmentEvent(_ context.Context, event store.SegmentEvent) error {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.events[event.SegmentID] = append(stub.events[event.SegmentID], model.SegmentEvent(event))
	return nil
}

func (stub *experimentStoreStub) AppendSegmentMetrics(_ context.Context, segmentID string, buckets []store.MetricBucket) error {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.metrics[segmentID] = append(stub.metrics[segmentID], buckets...)
	return nil
}

func (stub *experimentStoreStub) LinkSegmentTraces(_ context.Context, segmentID string, traceIDs []string) error {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.linked[segmentID] = append(stub.linked[segmentID], traceIDs...)
	return nil
}

func (stub *experimentStoreStub) ListSegmentEvents(_ context.Context, segmentID string, _ int) ([]store.SegmentEvent, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	var events []store.SegmentEvent
	for _, event := range stub.events[segmentID] {
		events = append(events, store.SegmentEvent(event))
	}
	return events, nil
}

func (stub *experimentStoreStub) ListSegmentMetrics(_ context.Context, segmentID string, _ int) ([]store.MetricBucket, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	var buckets []store.MetricBucket
	for _, bucket := range stub.metrics[segmentID] {
		buckets = append(buckets, bucket)
	}
	return buckets, nil
}

func (stub *experimentStoreStub) ListSegmentTraces(_ context.Context, segmentID string) ([]model.TraceSummary, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	return append([]model.TraceSummary(nil), stub.traces[segmentID]...), nil
}

// newExperimentTestServer 构造带真实 informer cache 的测试 Server：
// 空集群也能同步，快照为空 JSON，覆盖 requireCache 与聚合器调用链。
func newExperimentTestServer(t *testing.T, database store.Store) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	listKinds := make(map[schema.GroupVersionResource]string, len(dashboardkubernetes.PlatformResources))
	for _, descriptor := range dashboardkubernetes.PlatformResources {
		listKinds[descriptor.GVR] = descriptor.Kind + "List"
	}
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds)
	typedClient := kubernetesfake.NewSimpleClientset()
	clients := &dashboardkubernetes.Clients{
		Kubernetes: typedClient,
		Dynamic:    dynamicClient,
		Context:    "test",
		Version:    "v1.36.0",
	}
	cacheState, err := dashboardkubernetes.NewCache(clients, config.KubernetesConfig{
		ResyncPeriod: time.Hour, CacheSyncTimeout: 5 * time.Second,
	}, logger, nil)
	if err != nil {
		t.Fatalf("create informer cache: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = cacheState.Run(ctx) }()
	waitContext, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	if err := cacheState.WaitUntilSynced(waitContext); err != nil {
		t.Fatalf("wait for informer cache: %v", err)
	}
	return &Server{
		config:     config.Config{HTTP: config.HTTPConfig{MaxBodyBytes: 1 << 20}},
		logger:     logger,
		cache:      cacheState,
		aggregator: readmodel.NewAggregator(cacheState),
		store:      database,
	}
}

type experimentEnvelope struct {
	Data json.RawMessage `json:"data"`
	Meta struct {
		Warnings []string `json:"warnings"`
	} `json:"meta"`
}

func decodeExperimentResponse(t *testing.T, recorder *httptest.ResponseRecorder) experimentEnvelope {
	t.Helper()
	var envelope experimentEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v\nbody=%s", err, recorder.Body.String())
	}
	return envelope
}

func performExperimentRequest(t *testing.T, server *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	if method == http.MethodPost {
		// 写请求经幂等中间件，每个测试请求使用独立 Key。
		request.Header.Set("Idempotency-Key", "test-"+randomIdentifier("exp"))
	}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	return recorder
}

func TestCreateExperimentValidatesRequest(t *testing.T) {
	server := newExperimentTestServer(t, newExperimentStoreStub())
	cases := []struct {
		name string
		body string
		want int
	}{
		{name: "invalid json", body: `{`, want: http.StatusBadRequest},
		{name: "missing tenant", body: `{"name":"实验一"}`, want: http.StatusBadRequest},
		{name: "missing name", body: `{"tenant":"tenant-a"}`, want: http.StatusBadRequest},
		{name: "control character", body: `{"tenant":"tenant-a","name":"bad\nname"}`, want: http.StatusBadRequest},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			recorder := performExperimentRequest(t, server, http.MethodPost, "/api/v1/experiments", test.body)
			if recorder.Code != test.want {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, test.want, recorder.Body.String())
			}
		})
	}
}

func TestCreateExperimentRequiresStore(t *testing.T) {
	server := newExperimentTestServer(t, store.Disabled{})
	recorder := performExperimentRequest(t, server, http.MethodPost, "/api/v1/experiments", `{"tenant":"tenant-a","name":"实验一"}`)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
}

func TestExperimentLifecycleCreateStartCompleteFail(t *testing.T) {
	database := newExperimentStoreStub()
	server := newExperimentTestServer(t, database)

	created := performExperimentRequest(t, server, http.MethodPost, "/api/v1/experiments", `{"tenant":"tenant-a","name":"实验一"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body=%s", created.Code, created.Body.String())
	}
	createEnvelope := decodeExperimentResponse(t, created)
	var createdDetail model.SegmentDetail
	if err := json.Unmarshal(createEnvelope.Data, &createdDetail); err != nil {
		t.Fatalf("decode created detail: %v", err)
	}
	if createdDetail.Segment.Status != string(model.SegmentPending) {
		t.Fatalf("created status = %s, want pending", createdDetail.Segment.Status)
	}
	if len(createdDetail.Segment.ConfigSnapshot) == 0 {
		t.Fatal("created segment must carry a configuration snapshot")
	}
	segmentID := createdDetail.Segment.SegmentID

	// 重复开始同一实验：pending→running 只允许一次
	started := performExperimentRequest(t, server, http.MethodPost, "/api/v1/experiments/"+segmentID+"/start", "")
	if started.Code != http.StatusOK {
		t.Fatalf("start status = %d, want 200; body=%s", started.Code, started.Body.String())
	}
	startEnvelope := decodeExperimentResponse(t, started)
	var startedDetail model.SegmentDetail
	if err := json.Unmarshal(startEnvelope.Data, &startedDetail); err != nil {
		t.Fatalf("decode started detail: %v", err)
	}
	if startedDetail.Segment.Status != string(model.SegmentRunning) {
		t.Fatalf("started status = %s, want running", startedDetail.Segment.Status)
	}
	if len(startedDetail.Segment.StartSnapshot) == 0 {
		t.Fatal("started segment must carry a start snapshot")
	}
	again := performExperimentRequest(t, server, http.MethodPost, "/api/v1/experiments/"+segmentID+"/start", "")
	if again.Code != http.StatusConflict {
		t.Fatalf("double start status = %d, want 409", again.Code)
	}

	// 完成：写入终点快照与摘要
	completed := performExperimentRequest(t, server, http.MethodPost, "/api/v1/experiments/"+segmentID+"/complete", "")
	if completed.Code != http.StatusOK {
		t.Fatalf("complete status = %d, want 200; body=%s", completed.Code, completed.Body.String())
	}
	completeEnvelope := decodeExperimentResponse(t, completed)
	var completedDetail model.SegmentDetail
	if err := json.Unmarshal(completeEnvelope.Data, &completedDetail); err != nil {
		t.Fatalf("decode completed detail: %v", err)
	}
	if completedDetail.Segment.Status != string(model.SegmentCompleted) {
		t.Fatalf("completed status = %s, want completed", completedDetail.Segment.Status)
	}
	if len(completedDetail.Segment.EndSnapshot) == 0 {
		t.Fatal("completed segment must carry an end snapshot")
	}
	var summary map[string]any
	if err := json.Unmarshal(completedDetail.Segment.Summary, &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if _, exists := summary["durationSeconds"]; !exists {
		t.Fatalf("summary missing durationSeconds: %#v", summary)
	}
	completeAgain := performExperimentRequest(t, server, http.MethodPost, "/api/v1/experiments/"+segmentID+"/complete", "")
	if completeAgain.Code != http.StatusConflict {
		t.Fatalf("double complete status = %d, want 409", completeAgain.Code)
	}

	// 失败路径：新建一个实验并标记失败
	failedCreated := performExperimentRequest(t, server, http.MethodPost, "/api/v1/experiments", `{"tenant":"tenant-a","name":"实验二"}`)
	failedEnvelope := decodeExperimentResponse(t, failedCreated)
	var failedDetail model.SegmentDetail
	if err := json.Unmarshal(failedEnvelope.Data, &failedDetail); err != nil {
		t.Fatalf("decode failed-created detail: %v", err)
	}
	failedID := failedDetail.Segment.SegmentID
	performExperimentRequest(t, server, http.MethodPost, "/api/v1/experiments/"+failedID+"/start", "")
	failed := performExperimentRequest(t, server, http.MethodPost, "/api/v1/experiments/"+failedID+"/fail", `{"reason":"指标异常"}`)
	if failed.Code != http.StatusOK {
		t.Fatalf("fail status = %d, want 200; body=%s", failed.Code, failed.Body.String())
	}
	failEnvelope := decodeExperimentResponse(t, failed)
	var failedDone model.SegmentDetail
	if err := json.Unmarshal(failEnvelope.Data, &failedDone); err != nil {
		t.Fatalf("decode failed detail: %v", err)
	}
	if failedDone.Segment.Status != string(model.SegmentFailed) || failedDone.Segment.Reason != "指标异常" {
		t.Fatalf("failed segment = %#v", failedDone.Segment)
	}
}

func TestExperimentListAndDetail(t *testing.T) {
	database := newExperimentStoreStub()
	server := newExperimentTestServer(t, database)
	created := performExperimentRequest(t, server, http.MethodPost, "/api/v1/experiments", `{"tenant":"tenant-a","name":"实验一"}`)
	createEnvelope := decodeExperimentResponse(t, created)
	var createdDetail model.SegmentDetail
	if err := json.Unmarshal(createEnvelope.Data, &createdDetail); err != nil {
		t.Fatalf("decode created detail: %v", err)
	}
	segmentID := createdDetail.Segment.SegmentID
	if err := database.RecordSegmentEvent(context.Background(), model.SegmentEvent{
		SegmentID: segmentID, EventType: model.SegmentEventDecision,
	}); err != nil {
		t.Fatalf("seed event: %v", err)
	}

	list := performExperimentRequest(t, server, http.MethodGet, "/api/v1/experiments?status=pending", "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", list.Code)
	}
	listEnvelope := decodeExperimentResponse(t, list)
	var records []store.SegmentRecord
	if err := json.Unmarshal(listEnvelope.Data, &records); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(records) != 1 || records[0].SegmentID != segmentID {
		t.Fatalf("list records = %#v", records)
	}

	invalid := performExperimentRequest(t, server, http.MethodGet, "/api/v1/experiments?status=bogus", "")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status filter = %d, want 400", invalid.Code)
	}

	detail := performExperimentRequest(t, server, http.MethodGet, "/api/v1/experiments/"+segmentID, "")
	if detail.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", detail.Code)
	}
	detailEnvelope := decodeExperimentResponse(t, detail)
	var got model.SegmentDetail
	if err := json.Unmarshal(detailEnvelope.Data, &got); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if len(got.Events) != 1 || got.Events[0].EventType != model.SegmentEventDecision {
		t.Fatalf("detail events = %#v", got.Events)
	}

	missing := performExperimentRequest(t, server, http.MethodGet, "/api/v1/experiments/nope", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing detail status = %d, want 404", missing.Code)
	}
}
