package api

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

// newGrafanaProxy 把 /grafana/* 反向代理到 Grafana（sub-path 部署）。
//
// 前端通过 Backend 相对路径 /grafana/... 加载 Grafana 面板，不感知 Grafana 地址，
// 避免跨域与独立端口转发。Grafana 侧需要 GF_SERVER_SERVE_FROM_SUB_PATH=true 且
// GF_SERVER_ROOT_URL 指向本入口（见 config/observability/grafana.yaml）。
func newGrafanaProxy(logger *slog.Logger, cfg config.ProviderConfig) http.Handler {
	target, err := url.Parse(cfg.URL)
	if err != nil {
		logger.Error("解析 Grafana URL 失败", "url", cfg.URL, "error", err)
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writeProblem(writer, request, http.StatusServiceUnavailable, "GRAFANA_UNAVAILABLE", "Grafana 配置无效", false, nil)
		})
	}

	proxy := &httputil.ReverseProxy{
		Rewrite: func(proxyRequest *httputil.ProxyRequest) {
			// Grafana 以 sub-path 部署（GF_SERVER_SERVE_FROM_SUB_PATH=true），
			// /grafana 前缀必须原样保留：剥离后 Grafana 会把面板页 301 回外部入口。
			proxyRequest.SetURL(target)
			proxyRequest.Out.Host = ""
		},
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, err error) {
			logger.Warn("Grafana 反向代理请求失败", "path", request.URL.Path, "error", err)
			writeProblem(writer, request, http.StatusBadGateway, "GRAFANA_PROXY_ERROR", "Grafana 暂不可用", false, nil)
		},
	}
	return proxy
}
