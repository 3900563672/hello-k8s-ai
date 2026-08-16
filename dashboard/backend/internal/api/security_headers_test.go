package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeadersAllowGrafanaFraming(t *testing.T) {
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	handler := securityHeadersMiddleware(next)

	request := httptest.NewRequest(http.MethodGet, "http://dashboard.example/grafana/d/hello-k8s-ai-overview", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if value := recorder.Header().Get("X-Frame-Options"); value != "" {
		t.Fatalf("X-Frame-Options = %q, want empty for /grafana/*", value)
	}

	request = httptest.NewRequest(http.MethodGet, "http://dashboard.example/api/v1/overview", nil)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if value := recorder.Header().Get("X-Frame-Options"); value != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want DENY for API paths", value)
	}
}
