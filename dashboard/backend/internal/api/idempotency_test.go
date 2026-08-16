package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

type memoryIdempotencyStore struct {
	store.Disabled
	mu     sync.Mutex
	record *store.IdempotencyRecord
}

func (database *memoryIdempotencyStore) Available() bool { return true }

func (database *memoryIdempotencyStore) ReserveIdempotency(
	_ context.Context,
	key string,
	requestHash string,
	expiresAt time.Time,
) (*store.IdempotencyRecord, bool, error) {
	database.mu.Lock()
	defer database.mu.Unlock()
	if database.record == nil {
		database.record = &store.IdempotencyRecord{
			Key: key, RequestHash: requestHash, State: "pending",
			CreatedAt: time.Now().UTC(), ExpiresAt: expiresAt,
		}
		copy := *database.record
		return &copy, true, nil
	}
	copy := *database.record
	copy.Response = append(json.RawMessage(nil), database.record.Response...)
	return &copy, false, nil
}

func (database *memoryIdempotencyStore) CompleteIdempotency(
	_ context.Context,
	key string,
	requestHash string,
	status int,
	response json.RawMessage,
) error {
	database.mu.Lock()
	defer database.mu.Unlock()
	database.record.Key = key
	database.record.RequestHash = requestHash
	database.record.State = "completed"
	database.record.StatusCode = status
	database.record.Response = append(json.RawMessage(nil), response...)
	return nil
}

func (database *memoryIdempotencyStore) ReleaseIdempotency(context.Context, string, string) error {
	database.mu.Lock()
	database.record = nil
	database.mu.Unlock()
	return nil
}

func TestIdempotencyMiddlewareReplaysCompletedCommand(t *testing.T) {
	database := &memoryIdempotencyStore{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var calls atomic.Int32
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("handler could not decode restored request body: %v", err)
		}
		writeData(writer, request, http.StatusAccepted, body, false, nil, nil)
	})
	handler := idempotencyMiddleware(database, 1024, logger, next)

	first := httptest.NewRequest(http.MethodPost, "/api/v1/configuration:apply", io.NopCloser(
		strings.NewReader(`{"resources":[{"kind":"Tenant","name":"tenant-a"}]}`),
	))
	first.Header.Set("Idempotency-Key", "command-1")
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, first)

	second := httptest.NewRequest(http.MethodPost, "/api/v1/configuration:apply", io.NopCloser(
		strings.NewReader(`{"resources":[{"kind":"Tenant","name":"tenant-a"}]}`),
	))
	second.Header.Set("Idempotency-Key", "command-1")
	secondResponse := httptest.NewRecorder()
	handler.ServeHTTP(secondResponse, second)

	if calls.Load() != 1 {
		t.Fatalf("command handler ran %d times, want 1", calls.Load())
	}
	if secondResponse.Code != http.StatusAccepted || secondResponse.Header().Get("X-Idempotent-Replay") != "true" {
		t.Fatalf("completed command was not replayed: status=%d headers=%v", secondResponse.Code, secondResponse.Header())
	}
	if firstResponse.Body.String() != secondResponse.Body.String() {
		t.Fatalf("replayed response differs from original\nfirst: %s\nsecond: %s", firstResponse.Body, secondResponse.Body)
	}
}

func TestIdempotencyMiddlewareRejectsKeyReuseForDifferentRequest(t *testing.T) {
	database := &memoryIdempotencyStore{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := idempotencyMiddleware(database, 1024, logger, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writeData(writer, request, http.StatusAccepted, map[string]bool{"accepted": true}, false, nil, nil)
	}))

	first := httptest.NewRequest(http.MethodPatch, "/api/v1/tenants/tenant-a/traffic", strings.NewReader(`{"qps":10}`))
	first.Header.Set("Idempotency-Key", "command-2")
	handler.ServeHTTP(httptest.NewRecorder(), first)

	second := httptest.NewRequest(http.MethodPatch, "/api/v1/tenants/tenant-a/traffic", strings.NewReader(`{"qps":20}`))
	second.Header.Set("Idempotency-Key", "command-2")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, second)
	if response.Code != http.StatusConflict {
		t.Fatalf("key reuse status = %d, want %d", response.Code, http.StatusConflict)
	}
	var result problemEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || result.Error.Code != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("unexpected key reuse response: %s", response.Body.String())
	}
}

func TestIdempotencyMiddlewareRequiresKeyForCommands(t *testing.T) {
	database := &memoryIdempotencyStore{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := idempotencyMiddleware(database, 1024, logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("command without an idempotency key reached the handler")
	}))
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/configuration/Tenant/tenant-a", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("missing key status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
func TestIdempotencySkipsGrafanaProxyQueries(t *testing.T) {
	handler := idempotencyMiddleware(nil, 1<<20, nil, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodPost, "http://dashboard.example/grafana/api/ds/query", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("POST /grafana/api/ds/query = %d, want %d (no Idempotency-Key required)", response.Code, http.StatusOK)
	}
}
