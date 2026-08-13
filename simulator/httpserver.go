package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// newObservabilityServer 创建一个带 /metrics、/healthz、/readyz 的 HTTP 服务。
// gatherer 是 Prometheus 注册器，传 nil 就用默认的。
// 超时设得短，防止慢客户端堆积 goroutine。
func newObservabilityServer(bindAddress string, gatherer prometheus.Gatherer) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{
		EnableOpenMetrics: true, // 输出 OpenMetrics 格式
	}))
	mux.HandleFunc("/healthz", plainOK)
	mux.HandleFunc("/readyz", plainOK)
	return &http.Server{
		Addr:              bindAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,  // 防慢速攻击
		ReadTimeout:       10 * time.Second, // 整个请求体读完的最长时间
		WriteTimeout:      10 * time.Second, // 写完响应的最长时间
		IdleTimeout:       60 * time.Second, // keep-alive 空闲多久后断开
	}
}

// plainOK 返回 200 + "ok\n"，最简单的健康检查响应。
func plainOK(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte("ok\n"))
}

// serveObservability 启动监听，空地址或 "0" 表示不启动。
func serveObservability(server *http.Server) error {
	if server == nil || server.Addr == "" || server.Addr == "0" {
		return nil
	}
	err := server.ListenAndServe()
	// 正常关闭不报错
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// shutdownObservability 优雅关闭，最多等 5 秒。
func shutdownObservability(server *http.Server) error {
	if server == nil || server.Addr == "" || server.Addr == "0" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}
