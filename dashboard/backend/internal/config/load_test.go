package config

import (
	"testing"
	"time"
)

// TestLoadEnvOverrides 覆盖 Load() 全部分区解析（#142 config 门禁）。
func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("HTTP_ADDRESS", ":9090")
	t.Setenv("HTTP_READ_TIMEOUT", "3s")
	t.Setenv("HTTP_MAX_BODY_BYTES", "4096")
	t.Setenv("CORS_ALLOWED_ORIGINS", " http://a,http://b , ")
	t.Setenv("TRUST_REMOTE_USER_HEADER", "true")
	t.Setenv("KUBE_CONTEXT", "kind-test")
	t.Setenv("KUBE_CLIENT_QPS", "12.5")
	t.Setenv("KUBE_CLIENT_BURST", "25")
	t.Setenv("KUBE_CACHE_RESYNC_PERIOD", "5m")
	t.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db?sslmode=disable")
	t.Setenv("DATABASE_REQUIRED", "false")
	t.Setenv("DATABASE_MAX_CONNECTIONS", "7")
	t.Setenv("DATABASE_MIN_CONNECTIONS", "2")
	t.Setenv("DATABASE_CONNECT_TIMEOUT", "4s")
	t.Setenv("PERSISTENCE_EVENT_BUFFER", "256")
	t.Setenv("SEGMENT_BURST_REPLICA_DELTA", "3")
	t.Setenv("SEGMENT_ERROR_RATE_THRESHOLD", "0.02")
	t.Setenv("SEGMENT_TTFT_THRESHOLD_MS", "1500")
	t.Setenv("GRAFANA_URL", "http://grafana:3000")
	t.Setenv("GRAFANA_ENABLED", "true")
	t.Setenv("GRAFANA_TIMEOUT", "2s")
	t.Setenv("PROMETHEUS_URL", "http://prom:9090")
	t.Setenv("PROMETHEUS_ENABLED", "true")
	t.Setenv("PROMETHEUS_TIMEOUT", "3s")
	t.Setenv("JAEGER_URL", "http://jaeger:16686")
	t.Setenv("JAEGER_ENABLED", "true")
	t.Setenv("JAEGER_TIMEOUT", "4s")
	t.Setenv("AIOPS_ENABLED", "true")
	t.Setenv("AIOPS_OPENAI_API_KEY", "sk-test")
	t.Setenv("AIOPS_MODEL", "deepseek-v4-flash")
	t.Setenv("AIOPS_TIMEOUT", "30s")
	t.Setenv("AIOPS_MAX_TOKENS_PER_CALL", "1000")
	t.Setenv("AIOPS_CHAT_MODELS", "model-a,model-b")
	t.Setenv("AIOPS_CHAT_MAX_MESSAGE_LEN", "8000")
	t.Setenv("AIOPS_CHAT_RATE_PER_MINUTE", "10")
	t.Setenv("AIOPS_POLL_INTERVAL", "7s")
	t.Setenv("AIOPS_DAILY_MAX_CALLS", "500")
	t.Setenv("AIOPS_DAILY_MAX_TOKENS", "3000000")
	t.Setenv("AIOPS_ALERT_THRESHOLD", "50")
	t.Setenv("AIOPS_ALERT_CONSECUTIVE", "4")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("DEPLOYMENT_ENVIRONMENT", "staging")
	t.Setenv("K8S_CLUSTER_NAME", "demo-cluster")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTP.Address != ":9090" || cfg.HTTP.MaxBodyBytes != 4096 || !cfg.HTTP.TrustRemoteUser {
		t.Fatalf("HTTP 解析异常: %+v", cfg.HTTP)
	}
	if len(cfg.HTTP.AllowedOrigins) != 2 || cfg.HTTP.AllowedOrigins[0] != "http://a" || cfg.HTTP.AllowedOrigins[1] != "http://b" {
		t.Fatalf("CORS 解析异常: %+v", cfg.HTTP.AllowedOrigins)
	}
	if cfg.Kubernetes.Context != "kind-test" || cfg.Kubernetes.QPS != 12.5 || cfg.Kubernetes.Burst != 25 {
		t.Fatalf("Kubernetes 解析异常: %+v", cfg.Kubernetes)
	}
	if cfg.Database.URL == "" || cfg.Database.Required || cfg.Database.MaxConnections != 7 || cfg.Database.MinConnections != 2 {
		t.Fatalf("Database 解析异常: %+v", cfg.Database)
	}
	if cfg.Persistence.EventBuffer != 256 || cfg.Persistence.SegmentBurstReplicaDelta != 3 ||
		cfg.Persistence.SegmentErrorRateThreshold != 0.02 || cfg.Persistence.SegmentTTFTThresholdMS != 1500 {
		t.Fatalf("Persistence 解析异常: %+v", cfg.Persistence)
	}
	if !cfg.Grafana.Enabled || cfg.Grafana.URL != "http://grafana:3000" || cfg.Grafana.Timeout != 2*time.Second {
		t.Fatalf("Grafana 解析异常: %+v", cfg.Grafana)
	}
	if !cfg.Prometheus.Enabled || cfg.Prometheus.URL != "http://prom:9090" || cfg.Prometheus.Timeout != 3*time.Second {
		t.Fatalf("Prometheus 解析异常: %+v", cfg.Prometheus)
	}
	if !cfg.Jaeger.Enabled || cfg.Jaeger.URL != "http://jaeger:16686" || cfg.Jaeger.Timeout != 4*time.Second {
		t.Fatalf("Jaeger 解析异常: %+v", cfg.Jaeger)
	}
	if !cfg.AIOps.Enabled || cfg.AIOps.OpenAIAPIKey != "sk-test" || cfg.AIOps.Model != "deepseek-v4-flash" ||
		cfg.AIOps.Timeout != 30*time.Second || cfg.AIOps.MaxTokensPerCall != 1000 {
		t.Fatalf("AIOps 解析异常: %+v", cfg.AIOps)
	}
	if len(cfg.AIOps.ChatModels) != 2 || cfg.AIOps.ChatModels[0] != "model-a" ||
		cfg.AIOps.ChatMaxMessageLen != 8000 || cfg.AIOps.ChatRatePerMinute != 10 ||
		cfg.AIOps.PollInterval != 7*time.Second || cfg.AIOps.DailyMaxCalls != 500 ||
		cfg.AIOps.DailyMaxTokens != 3000000 || cfg.AIOps.AlertThreshold != 50 || cfg.AIOps.AlertConsecutive != 4 {
		t.Fatalf("AIOps 扩展解析异常: %+v", cfg.AIOps)
	}
	if cfg.Environment != "staging" || cfg.ClusterName != "demo-cluster" {
		t.Fatalf("环境解析异常: env=%s cluster=%s", cfg.Environment, cfg.ClusterName)
	}
}

// TestEnvHelpers 覆盖 env/duration/integer/decimal/boolean/csv 解析辅助（#142 config 门禁）。
func TestEnvHelpers(t *testing.T) {
	t.Setenv("T_HELPER_STR", "  abc  ")
	if got := env("T_HELPER_STR", "fallback"); got != "abc" {
		t.Fatalf("env trim: got %q", got)
	}
	if got := env("T_HELPER_MISSING", "fallback"); got != "fallback" {
		t.Fatalf("env fallback: got %q", got)
	}
	t.Setenv("T_HELPER_DUR", "90s")
	if got := duration("T_HELPER_DUR", time.Second); got != 90*time.Second {
		t.Fatalf("duration: got %v", got)
	}
	t.Setenv("T_HELPER_DUR_BAD", "nope")
	if got := duration("T_HELPER_DUR_BAD", time.Second); got != time.Second {
		t.Fatalf("duration bad: got %v", got)
	}
	t.Setenv("T_HELPER_DUR_ZERO", "0s")
	if got := duration("T_HELPER_DUR_ZERO", time.Second); got != time.Second {
		t.Fatalf("duration zero: got %v", got)
	}
	if got := duration("T_HELPER_MISSING", time.Second); got != time.Second {
		t.Fatalf("duration missing: got %v", got)
	}
	t.Setenv("T_HELPER_INT", "42")
	if got := integer("T_HELPER_INT", 1); got != 42 {
		t.Fatalf("integer: got %d", got)
	}
	t.Setenv("T_HELPER_INT_BAD", "x")
	if got := integer("T_HELPER_INT_BAD", 1); got != 1 {
		t.Fatalf("integer bad: got %d", got)
	}
	if got := integer("T_HELPER_MISSING", 7); got != 7 {
		t.Fatalf("integer missing: got %d", got)
	}
	t.Setenv("T_HELPER_DEC", "3.14")
	if got := decimal("T_HELPER_DEC", 0); got != 3.14 {
		t.Fatalf("decimal: got %v", got)
	}
	t.Setenv("T_HELPER_DEC_BAD", "x")
	if got := decimal("T_HELPER_DEC_BAD", 1.5); got != 1.5 {
		t.Fatalf("decimal bad: got %v", got)
	}
	t.Setenv("T_HELPER_BOOL", "true")
	if got := boolean("T_HELPER_BOOL", false); !got {
		t.Fatalf("boolean: got %v", got)
	}
	t.Setenv("T_HELPER_BOOL_BAD", "yes")
	if got := boolean("T_HELPER_BOOL_BAD", true); !got {
		t.Fatalf("boolean bad should fallback: got %v", got)
	}
	if got := boolean("T_HELPER_MISSING", true); !got {
		t.Fatalf("boolean missing: got %v", got)
	}
	t.Setenv("T_HELPER_CSV", " a ,b,, c ")
	gotCSV := csv("T_HELPER_CSV", nil)
	if len(gotCSV) != 3 || gotCSV[0] != "a" || gotCSV[1] != "b" || gotCSV[2] != "c" {
		t.Fatalf("csv: got %v", gotCSV)
	}
	fallbackCSV := csv("T_HELPER_MISSING", []string{"x", "y"})
	if len(fallbackCSV) != 2 || fallbackCSV[0] != "x" {
		t.Fatalf("csv fallback: got %v", fallbackCSV)
	}
}
