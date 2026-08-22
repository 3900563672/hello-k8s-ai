package httputil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClientSetsTimeoutAndTransport(t *testing.T) {
	client := NewClient(3 * time.Second)
	if client.Timeout != 3*time.Second {
		t.Fatalf("Timeout = %v, want 3s", client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.MaxIdleConns != 20 || transport.MaxIdleConnsPerHost != 10 {
		t.Fatalf("MaxIdleConns = %d/%d, want 20/10", transport.MaxIdleConns, transport.MaxIdleConnsPerHost)
	}
	if transport.ResponseHeaderTimeout != 3*time.Second {
		t.Fatalf("ResponseHeaderTimeout = %v, want 3s", transport.ResponseHeaderTimeout)
	}
}

func TestParseBaseURL(t *testing.T) {
	parsed, err := ParseBaseURL("https://prom.example.com", "prometheus")
	if err != nil {
		t.Fatalf("ParseBaseURL valid = %v", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "prom.example.com" {
		t.Fatalf("ParseBaseURL = %s://%s", parsed.Scheme, parsed.Host)
	}

	if _, err := ParseBaseURL("://bad", "prometheus"); err == nil {
		t.Fatal("ParseBaseURL malformed should error")
	}
	if _, err := ParseBaseURL("ftp://prom.example.com", "prometheus"); err == nil ||
		!strings.Contains(err.Error(), "must use http or https") {
		t.Fatalf("ParseBaseURL ftp should error, got %v", err)
	}
}

func TestResolve(t *testing.T) {
	base, err := ParseBaseURL("https://prom.example.com/api/v1/?token=abc", "prometheus")
	if err != nil {
		t.Fatal(err)
	}
	resolved := Resolve(base, "/query")
	if resolved.String() != "https://prom.example.com/api/v1/query" {
		t.Fatalf("Resolve = %s", resolved.String())
	}
	if base.String() != "https://prom.example.com/api/v1/?token=abc" {
		t.Fatal("Resolve must not mutate base URL")
	}
	// base 上的 RawQuery 被清空，避免查询串泄漏到新请求
	resolved2 := Resolve(base, "/range")
	if strings.Contains(resolved2.String(), "token=abc") {
		t.Fatalf("Resolve should drop RawQuery, got %s", resolved2.String())
	}
}

func TestGetJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/value" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		if request.Header.Get("Accept") != "application/json" {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]int{"value": 42})
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	endpoint, err := ParseBaseURL(server.URL, "test")
	if err != nil {
		t.Fatal(err)
	}

	var target struct {
		Value int `json:"value"`
	}
	if err := GetJSON(context.Background(), client, Resolve(endpoint, "/api/value"), &target, "test", 4096); err != nil {
		t.Fatalf("GetJSON = %v", err)
	}
	if target.Value != 42 {
		t.Fatalf("decoded value = %d, want 42", target.Value)
	}
}

func TestGetJSONErrorPaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/error":
			writer.WriteHeader(http.StatusInternalServerError)
			_, _ = writer.Write([]byte("boom"))
		case "/api/badjson":
			_, _ = writer.Write([]byte("<html>not json</html>"))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(2 * time.Second)
	endpoint, _ := ParseBaseURL(server.URL, "test")

	err := GetJSON(context.Background(), client, Resolve(endpoint, "/api/error"), &map[string]any{}, "test", 4096)
	if err == nil || !strings.Contains(err.Error(), "returned HTTP 500") {
		t.Fatalf("error status should surface, got %v", err)
	}

	err = GetJSON(context.Background(), client, Resolve(endpoint, "/api/badjson"), &map[string]any{}, "test", 4096)
	if err == nil || !strings.Contains(err.Error(), "decode test response") {
		t.Fatalf("bad json should surface decode error, got %v", err)
	}

	err = GetJSON(context.Background(), client, Resolve(endpoint, "/api/notfound"), &map[string]any{}, "test", 4096)
	if err == nil {
		t.Fatal("404 should error")
	}

	// 响应体超过 maxBodyBytes 应解码失败（LimitReader 截断 JSON）
	big := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"value":123456789}`))
	}))
	defer big.Close()
	bigEndpoint, _ := ParseBaseURL(big.URL, "test")
	err = GetJSON(context.Background(), client, Resolve(bigEndpoint, "/"), &map[string]any{}, "test", 8)
	if err == nil {
		t.Fatal("truncated body should fail decode")
	}
}
