package api

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

// waitBackendReady 等待 httptest 后端可达后再发请求。
//
// 本机 WSL 环境下新端口存在注册时序竞态（bind 返回先于 Windows listener
// 就绪 ~200ms，t+0 拨号必 refused；退化窗口内可长达数分钟，见
// docs/operations/WSL_LOOPBACK_CASE_STUDY.md），直接请求会把环境竞态误报成
// 代理 502。先轮询拨号到成功再进入断言：WSL 退化窗口内超时则跳过（环境
// 问题，CI 原生 Linux 不受影响，仍严格失败），避免本地误报。
func waitBackendReady(t *testing.T, rawURL string, timeout time.Duration) {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse backend url: %v", err)
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, dialErr := net.DialTimeout("tcp", parsed.Host, 200*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	if onWSL() {
		t.Skipf("backend %s 在 %v 内不可达：本机 WSL 新端口注册退化窗口（见 docs/operations/WSL_LOOPBACK_CASE_STUDY.md，修复方向 = 升级 WSL 2.9.5+）", rawURL, timeout)
	}
	t.Fatalf("backend %s 在 %v 内不可达", rawURL, timeout)
}

// onWSL 判断当前是否运行在 WSL 内（/proc/version 含 microsoft 内核标识）。
func onWSL() bool {
	version, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(version)), "microsoft")
}

func TestGrafanaProxyPreservesSubPathAndForwards(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte("<html>grafana panel</html>"))
	}))
	defer backend.Close()
	waitBackendReady(t, backend.URL, 8*time.Second)

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
	waitBackendReady(t, backend.URL, 8*time.Second)

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
