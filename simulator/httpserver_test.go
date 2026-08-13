package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// 测试 /healthz, /readyz, /metrics 三个可观测性端点是否返回预期内容
func TestObservabilityServerEndpoints(t *testing.T) {
	// 创建一个独立的注册表，避免与其他测试的指标混在一起
	registry := prometheus.NewRegistry()
	metrics := newSimulatorMetrics(registry)
	// 写一个已知值进去，后面检查 /metrics 输出时用到
	metrics.assignedQPS.Set(12)
	// 创建服务器，绑定随机端口
	server := newObservabilityServer(":0", registry)

	// 每个端点的预期响应
	tests := []struct {
		path        string
		contentType string
		body        string
	}{
		{path: "/healthz", contentType: "text/plain", body: "ok\n"},
		{path: "/readyz", contentType: "text/plain", body: "ok\n"},
		{path: "/metrics", contentType: "application/openmetrics-text", body: "hello_k8s_ai_simulator_assigned_qps 12"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.path == "/metrics" {
				request.Header.Set("Accept", "application/openmetrics-text")
			}
			response := httptest.NewRecorder()
			server.Handler.ServeHTTP(response, request)

			// 状态码必须是 200
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			// Content-Type 应该包含预期的媒体类型
			if !strings.Contains(response.Header().Get("Content-Type"), test.contentType) {
				t.Fatalf("content type = %q, want it to contain %q", response.Header().Get("Content-Type"), test.contentType)
			}
			// 响应体应该包含预期的内容
			if !strings.Contains(response.Body.String(), test.body) {
				t.Fatalf("body did not contain %q: %s", test.body, response.Body.String())
			}
		})
	}
}
