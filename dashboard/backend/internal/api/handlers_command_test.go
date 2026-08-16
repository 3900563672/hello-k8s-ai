package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/kubernetes"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

type failingCompleteStore struct {
	store.Disabled
	mu             sync.Mutex
	released       bool
	completeCalled bool
}

func (database *failingCompleteStore) Available() bool { return true }

func (database *failingCompleteStore) ReserveIdempotency(
	_ context.Context,
	key string,
	requestHash string,
	expiresAt time.Time,
) (*store.IdempotencyRecord, bool, error) {
	return &store.IdempotencyRecord{Key: key, RequestHash: requestHash, State: "pending", CreatedAt: time.Now().UTC(), ExpiresAt: expiresAt}, true, nil
}

func (database *failingCompleteStore) CompleteIdempotency(context.Context, string, string, int, json.RawMessage) error {
	database.mu.Lock()
	defer database.mu.Unlock()
	database.completeCalled = true
	return io.ErrUnexpectedEOF
}

func (database *failingCompleteStore) ReleaseIdempotency(context.Context, string, string) error {
	database.mu.Lock()
	defer database.mu.Unlock()
	database.released = true
	return nil
}

func TestIdempotencyCompletionFailureReleasesReservation(t *testing.T) {
	database := &failingCompleteStore{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var calls int
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		writeData(writer, request, http.StatusAccepted, map[string]any{"applied": true}, false, nil, nil)
	})
	handler := idempotencyMiddleware(database, 1024, logger, next)

	send := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/configuration:apply", io.NopCloser(
			strings.NewReader(`{"resources":[{"kind":"Tenant","name":"tenant-a"}]}`),
		))
		request.Header.Set("Idempotency-Key", "command-1")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}

	first := send()
	if first.Code != http.StatusAccepted {
		t.Fatalf("first response status = %d, want %d", first.Code, http.StatusAccepted)
	}
	database.mu.Lock()
	released := database.released
	database.mu.Unlock()
	if !released {
		t.Fatal("completion failure did not release the idempotency reservation")
	}

	second := send()
	if second.Code != http.StatusAccepted {
		t.Fatalf("retry response status = %d, want %d (key must not be stuck pending)", second.Code, http.StatusAccepted)
	}
	if calls != 2 {
		t.Fatalf("handler ran %d times, want 2 after reservation release", calls)
	}
	if second.Header().Get("X-Idempotent-Replay") == "true" {
		t.Fatal("retry was replayed from cache, want fresh execution after release")
	}
}

type stubApplier struct {
	failAfter int
	applied   int
}

func (stub *stubApplier) Apply(_ context.Context, intent kubernetes.ApplyIntent, dryRun bool) (*unstructured.Unstructured, string, error) {
	stub.applied++
	if stub.failAfter > 0 && stub.applied > stub.failAfter {
		return nil, "apply", errors.New("injected failure")
	}
	object := &unstructured.Unstructured{}
	object.SetAPIVersion("platform.study.com/v1")
	object.SetKind(intent.Kind)
	object.SetName(intent.Name)
	object.SetUID(types.UID("uid-" + intent.Name))
	object.SetResourceVersion("rv-" + intent.Name)
	if dryRun {
		return object, "dry-run", nil
	}
	return object, "apply", nil
}

func TestApplyConfigurationBatchReturnsPartialResultsOnMidBatchFailure(t *testing.T) {
	server := &Server{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), store: store.Disabled{}}
	applier := &stubApplier{failAfter: 1}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/configuration:apply", nil)
	request.Header.Set("Idempotency-Key", "batch-1")

	results, failed := server.applyConfigurationBatch(applier, request, "op-1", []kubernetes.ApplyIntent{
		{Kind: "Tenant", Name: "tenant-a"},
		{Kind: "Tenant", Name: "tenant-b"},
	}, false)

	if len(results) != 1 {
		t.Fatalf("successful results = %d, want 1", len(results))
	}
	if results[0].Ref.Name != "tenant-a" || results[0].Convergence != "pending" {
		t.Fatalf("first result = %+v, want tenant-a pending", results[0])
	}
	if failed == nil {
		t.Fatal("expected failed result for tenant-b, got nil")
	}
	if failed.Ref.Name != "tenant-b" || failed.Convergence != "failed" || failed.Error == "" {
		t.Fatalf("failed result = %+v, want tenant-b failed with error", failed)
	}
}

func TestApplyConfigurationBatchSucceedsForAllResources(t *testing.T) {
	server := &Server{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), store: store.Disabled{}}
	applier := &stubApplier{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/configuration:apply", nil)
	request.Header.Set("Idempotency-Key", "batch-2")

	results, failed := server.applyConfigurationBatch(applier, request, "op-2", []kubernetes.ApplyIntent{
		{Kind: "Model", Name: "model-a"},
		{Kind: "Model", Name: "model-b"},
	}, true)

	if failed != nil {
		t.Fatalf("unexpected failure: %+v", failed)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	for _, result := range results {
		if result.Convergence != "dry-run" {
			t.Fatalf("dry-run result convergence = %q, want dry-run", result.Convergence)
		}
	}
}
