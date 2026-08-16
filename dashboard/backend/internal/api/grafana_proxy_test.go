package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

func TestGrafanaProxyPreservesSubPathAndForwards(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte("<html>grafana panel</html>"))
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	proxy := newGrafanaProxy(logger, config.ProviderConfig{URL: backend.URL, Enabled: true})

	request := httptest.NewRequest(http.MethodGet, "http://dashboard.example/grafana/d/hello-k8s-ai-overview?kiosk", nil)
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if receivedPath != "/grafana/d/hello-k8s-ai-overview" {
		t.Fatalf("backend path = %q, want /grafana/d/hello-k8s-ai-overview", receivedPath)
	}
	if !strings.Contains(recorder.Body.String(), "grafana panel") {
		t.Fatalf("response body = %q, want forwarded body", recorder.Body.String())
	}
}

func TestGrafanaProxyRootPath(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
		writer.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	proxy := newGrafanaProxy(logger, config.ProviderConfig{URL: backend.URL, Enabled: true})

	request := httptest.NewRequest(http.MethodGet, "http://dashboard.example/grafana/", nil)
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, request)

	if receivedPath != "/grafana/" {
		t.Fatalf("backend path = %q, want /grafana/", receivedPath)
	}
}

func TestGrafanaProxyUnavailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	proxy := newGrafanaProxy(logger, config.ProviderConfig{URL: "http://127.0.0.1:1", Enabled: true})

	request := httptest.NewRequest(http.MethodGet, "http://dashboard.example/grafana/", nil)
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", recorder.Code)
	}
}

func TestGrafanaProxyInvalidConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	proxy := newGrafanaProxy(logger, config.ProviderConfig{URL: "://bad", Enabled: true})

	request := httptest.NewRequest(http.MethodGet, "http://dashboard.example/grafana/", nil)
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
}
