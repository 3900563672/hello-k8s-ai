package jaeger

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

func mustParseQuery(t *testing.T, rawQuery string) url.Values {
	t.Helper()
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatalf("parse query %q: %v", rawQuery, err)
	}
	return query
}

const traceBody = `{"data":[{"traceID":"trace-1","spans":[
{"traceID":"trace-1","spanID":"root","operationName":"controller.reconcile","startTime":1000000,"duration":5000,"processID":"p1","tags":[{"key":"platform.tenant.name","type":"string","value":"tenant-a"}]},
{"traceID":"trace-1","spanID":"child","operationName":"simulator.tick","startTime":1000010,"duration":2000,"processID":"p2","references":[{"refType":"CHILD_OF","spanID":"root"}],"tags":[{"key":"error","type":"bool","value":true}]}
],"processes":{"p1":{"serviceName":"hello-k8s-ai-controller"},"p2":{"serviceName":"hello-k8s-ai-simulator"}}}]}`

func newTestClient(t *testing.T, server *httptest.Server, enabled bool) *Client {
	t.Helper()
	client, err := New(config.ProviderConfig{
		URL: server.URL, Enabled: enabled, Timeout: 3 * time.Second, MaxWindow: 2 * time.Hour,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client
}

func TestNewRejectsBadURL(t *testing.T) {
	if _, err := New(config.ProviderConfig{URL: "://bad", Enabled: true}); err == nil {
		t.Fatal("bad URL should fail")
	}
	if _, err := New(config.ProviderConfig{URL: "ftp://x", Enabled: true}); err == nil {
		t.Fatal("non-http URL should fail")
	}
}

func TestTraceFetchesDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/traces/trace-1" {
			fmt.Fprint(w, traceBody)
			return
		}
		if r.URL.Path == "/api/traces/missing" {
			fmt.Fprint(w, `{"data":[]}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(t, server, true)
	detail, err := client.Trace(context.Background(), "trace-1")
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}
	if detail.TraceID != "trace-1" || len(detail.Spans) != 2 {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if detail.Spans[0].Service != "hello-k8s-ai-controller" || detail.Spans[1].Status != "error" {
		t.Fatalf("unexpected spans: %+v", detail.Spans)
	}
	if _, err := client.Trace(context.Background(), "missing"); !errors.Is(err, ErrTraceNotFound) {
		t.Fatalf("missing trace error = %v, want ErrTraceNotFound", err)
	}
	for _, bad := range []string{"", "a/b", "a?b", "a#b", strings.Repeat("x", 65)} {
		if _, err := client.Trace(context.Background(), bad); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("trace %q error = %v, want ErrInvalidQuery", bad, err)
		}
	}
	disabled := newTestClient(t, server, false)
	if _, err := disabled.Trace(context.Background(), "trace-1"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled trace error = %v, want ErrDisabled", err)
	}
	if disabled.Enabled() {
		t.Fatal("disabled client should report Enabled()=false")
	}
}

func TestHealthAndServices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/services" {
			fmt.Fprint(w, `{"data":["zzz","hello-k8s-ai-controller","aaa"]}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(t, server, true)
	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if err := newTestClient(t, server, false).Health(context.Background()); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled health error = %v, want ErrDisabled", err)
	}
}

func TestSearchWithFilters(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		if r.URL.Path == "/api/traces" {
			fmt.Fprint(w, traceBody)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := newTestClient(t, server, true)
	start := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	traces, err := client.Search(context.Background(), SearchRequest{
		Start: start, End: end, Service: "hello-k8s-ai-simulator", Operation: "tick",
		Tenant: "tenant-a", Model: "model-a", Instance: "instance-a",
		MinDuration: 1500 * time.Microsecond, MaxDuration: 5 * time.Millisecond, Limit: 10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(traces) != 1 || traces[0].TraceID != "trace-1" || traces[0].Entities["tenant"] != "tenant-a" {
		t.Fatalf("unexpected traces: %+v", traces)
	}
	query := mustParseQuery(t, gotQuery)
	if query.Get("service") != "hello-k8s-ai-simulator" || query.Get("operation") != "tick" ||
		query.Get("limit") != "10" || query.Get("minDuration") != "1500us" || query.Get("maxDuration") != "5000us" {
		t.Fatalf("unexpected query: %v", query)
	}
	if !strings.Contains(query.Get("tags"), "tenant-a") || !strings.Contains(query.Get("tags"), "model-a") ||
		!strings.Contains(query.Get("tags"), "instance-a") {
		t.Fatalf("tags not encoded: %v", query.Get("tags"))
	}

	if _, err := newTestClient(t, server, false).Search(context.Background(), SearchRequest{}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled search error = %v, want ErrDisabled", err)
	}
}

func TestSearchWithoutServiceUsesCatalog(t *testing.T) {
	var traceCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/services":
			fmt.Fprint(w, `{"data":["hello-k8s-ai-controller","hello-k8s-ai-simulator","hello-k8s-ai-simulator-2","hello-k8s-ai-simulator-3","hello-k8s-ai-simulator-4","other-service"]}`)
		case "/api/traces":
			traceCalls++
			fmt.Fprint(w, traceBody)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server, true)
	start := time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)
	traces, err := client.Search(context.Background(), SearchRequest{Start: start, End: start.Add(15 * time.Minute), Limit: 20})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(traces) != 1 {
		t.Fatalf("unexpected traces: %+v", traces)
	}
	// hello-k8s-ai 前缀 5 个被裁剪到 4 个，other-service 被过滤。
	if traceCalls != 4 {
		t.Fatalf("expected 4 service calls (capped), got %d", traceCalls)
	}
}

func TestValidateMoreCases(t *testing.T) {
	client := &Client{enabled: true, maxWindow: time.Hour}
	at := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC)

	cases := []SearchRequest{
		{Start: at, End: at.Add(2 * time.Hour), Limit: 20},                                                       // 超 maxWindow
		{Start: at, End: at.Add(time.Minute), Limit: 0},                                                          // limit 过小
		{Start: at, End: at.Add(time.Minute), Limit: 101},                                                        // limit 过大
		{Start: at, End: at.Add(time.Minute), Limit: 10, MinDuration: 5 * time.Second, MaxDuration: time.Second}, // min > max
		{Start: at, End: at.Add(time.Minute), Limit: 10, Service: "bad\x00name"},                                 // 非法字符
	}
	for i, request := range cases {
		if err := client.validate(request); err == nil {
			t.Fatalf("case %d should fail validation", i)
		}
	}
	// 合法请求应通过校验（Limit 归一化在 Search 内做，这里直接给合法值）。
	if err := client.validate(SearchRequest{Start: at, End: at.Add(time.Minute), Limit: 10}); err != nil {
		t.Fatalf("valid request should pass: %v", err)
	}
	// normalizeSearch 默认窗口/limit。
	normalized := normalizeSearch(SearchRequest{})
	if normalized.Limit != 20 || normalized.Start.After(normalized.End) || normalized.Start.IsZero() {
		t.Fatalf("normalizeSearch unexpected: %+v", normalized)
	}
	if got := formatDuration(1500 * time.Microsecond); got != "1500us" {
		t.Fatalf("formatDuration = %q", got)
	}
}
