package prometheus

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

const matrixBody = `{"status":"success","data":{"resultType":"matrix","result":[
	{"metric":{"tenant":"tenant-a"},"values":[[1723464000.25,"12.5"],[1723464005,"13"]]}
]}}`

func newPromClient(t *testing.T, server *httptest.Server, enabled bool) *Client {
	t.Helper()
	client, err := New(config.ProviderConfig{
		URL: server.URL, Enabled: enabled, Timeout: 3 * time.Second,
		MaxWindow: 2 * time.Hour, CacheTTL: time.Minute,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

func TestPromNewAndCatalog(t *testing.T) {
	if _, err := New(config.ProviderConfig{URL: "://bad", Enabled: true}); err == nil {
		t.Fatal("bad URL should fail")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	client := newPromClient(t, server, false)
	if client.Enabled() {
		t.Fatal("disabled client should report Enabled()=false")
	}
	catalog := client.Catalog()
	if len(catalog) < 5 {
		t.Fatalf("catalog too small: %d", len(catalog))
	}
	first := catalog[0]
	if first["metricId"] == "" || first["unit"] == "" {
		t.Fatalf("catalog entries malformed: %+v", first)
	}
}

func TestPromQueryRangeSuccessAndCache(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/v1/query_range" {
			http.NotFound(w, r)
			return
		}
		query := r.URL.Query()
		if query.Get("query") == "" || query.Get("start") == "" || query.Get("end") == "" || query.Get("step") == "" {
			t.Errorf("missing query params: %v", query)
		}
		if !strings.Contains(query.Get("query"), "tenant") {
			t.Errorf("filters not applied: %v", query.Get("query"))
		}
		fmt.Fprint(w, matrixBody)
	}))
	defer server.Close()

	client := newPromClient(t, server, true)
	start := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	result, err := client.QueryRange(context.Background(), Query{
		MetricID: "simulator.qps", Start: start, End: start.Add(5 * time.Minute),
		Step: 15 * time.Second, Tenant: "tenant-a",
	})
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if result.MetricID != "simulator.qps" || result.Unit != "requests/s" || len(result.Series) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.Series[0].Points) != 2 || result.Series[0].Points[0].Value != 12.5 {
		t.Fatalf("unexpected points: %+v", result.Series[0].Points)
	}
	// 命中缓存不再请求。
	if _, err := client.QueryRange(context.Background(), Query{
		MetricID: "simulator.qps", Start: start, End: start.Add(5 * time.Minute),
		Step: 15 * time.Second, Tenant: "tenant-a",
	}); err != nil {
		t.Fatalf("cached QueryRange: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 upstream call (cache hit), got %d", calls)
	}
}

func TestPromQueryRangeErrorStates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":"error","errorType":"bad_data","error":"boom"}`)
	}))
	defer server.Close()

	client := newPromClient(t, server, true)
	start := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	if _, err := client.QueryRange(context.Background(), Query{
		MetricID: "simulator.qps", Start: start, End: start.Add(5 * time.Minute), Step: 15 * time.Second,
	}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error status should surface: %v", err)
	}
	if _, err := client.QueryRange(context.Background(), Query{MetricID: "nope"}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("unknown metric error = %v, want ErrInvalidQuery", err)
	}
	if _, err := client.QueryRange(context.Background(), Query{MetricID: "simulator.qps", Start: start, End: start.Add(time.Minute), Step: time.Second}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("step too small error = %v, want ErrInvalidQuery", err)
	}
	if _, err := newPromClient(t, server, false).QueryRange(context.Background(), Query{MetricID: "simulator.qps"}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled error = %v, want ErrDisabled", err)
	}
}

func TestPromHealthAndValidation(t *testing.T) {
	var healthCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/query" {
			healthCalls++
			fmt.Fprint(w, `{"status":"success","data":{"resultType":"scalar","result":[1723464000,"1"]}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newPromClient(t, server, true)
	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if healthCalls != 1 {
		t.Fatalf("health should call /api/v1/query once, got %d", healthCalls)
	}
	if err := newPromClient(t, server, false).Health(context.Background()); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled health error = %v, want ErrDisabled", err)
	}

	at := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)
	bad := []Query{
		{Start: at},          // 缺 End
		{Start: at, End: at}, // start >= end
		{Start: at, End: at.Add(3 * time.Hour), Step: 15 * time.Second},                        // 超 maxWindow
		{Start: at, End: at.Add(time.Minute), Step: 2 * time.Second},                           // step 过小
		{Start: at, End: at.Add(time.Minute), Step: 15 * time.Second, Tenant: "bad\x00tenant"}, // 非法字符
	}
	for i, query := range bad {
		if err := client.validate(query); err == nil {
			t.Fatalf("case %d should fail validation", i)
		}
	}
	normalized := normalizeQuery(Query{})
	if normalized.Step <= 0 || normalized.Start.IsZero() || normalized.End.IsZero() {
		t.Fatalf("normalizeQuery unexpected: %+v", normalized)
	}
	if got := formatPromTime(time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)); got != "1787184000.000" {
		t.Fatalf("formatPromTime = %q", got)
	}
	filters := cloneFilters(map[string]string{"a": "1", "b": "2"})
	if len(filters) != 2 || filters["a"] != "1" {
		t.Fatalf("cloneFilters unexpected: %v", filters)
	}
}
