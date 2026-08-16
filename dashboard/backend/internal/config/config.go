package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTP        HTTPConfig
	Kubernetes  KubernetesConfig
	Database    DatabaseConfig
	Prometheus  ProviderConfig
	Jaeger      ProviderConfig
	Persistence PersistenceConfig
	LogLevel    slog.Level
	ClusterName string
	Environment string
}

type HTTPConfig struct {
	Address           string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	AllowedOrigins    []string
	MaxBodyBytes      int64
}

type KubernetesConfig struct {
	Kubeconfig       string
	Context          string
	QPS              float32
	Burst            int
	ResyncPeriod     time.Duration
	CacheSyncTimeout time.Duration
}

type DatabaseConfig struct {
	URL              string
	Required         bool
	ConnectTimeout   time.Duration
	StartupRetries   int
	StartupBackoff   time.Duration
	MaxConnections   int32
	MinConnections   int32
	MaxConnectionAge time.Duration
}

type ProviderConfig struct {
	URL       string
	Timeout   time.Duration
	CacheTTL  time.Duration
	MaxWindow time.Duration
	Enabled   bool
}

type PersistenceConfig struct {
	EventBuffer       int
	SnapshotInterval  time.Duration
	SnapshotRetention time.Duration
}

func Load() (Config, error) {
	logLevel := new(slog.Level)
	if err := logLevel.UnmarshalText([]byte(env("LOG_LEVEL", "info"))); err != nil {
		return Config{}, fmt.Errorf("parse LOG_LEVEL: %w", err)
	}

	cfg := Config{
		HTTP: HTTPConfig{
			Address:           env("HTTP_ADDRESS", ":8080"),
			ReadTimeout:       duration("HTTP_READ_TIMEOUT", 15*time.Second),
			ReadHeaderTimeout: duration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			WriteTimeout:      duration("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:       duration("HTTP_IDLE_TIMEOUT", 90*time.Second),
			ShutdownTimeout:   duration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
			AllowedOrigins:    csv("CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173"}),
			MaxBodyBytes:      int64(integer("HTTP_MAX_BODY_BYTES", 1<<20)),
		},
		Kubernetes: KubernetesConfig{
			Kubeconfig:       strings.TrimSpace(os.Getenv("KUBECONFIG")),
			Context:          strings.TrimSpace(os.Getenv("KUBE_CONTEXT")),
			QPS:              float32(decimal("KUBE_CLIENT_QPS", 50)),
			Burst:            integer("KUBE_CLIENT_BURST", 100),
			ResyncPeriod:     duration("KUBE_CACHE_RESYNC_PERIOD", 10*time.Minute),
			CacheSyncTimeout: duration("KUBE_CACHE_SYNC_TIMEOUT", 2*time.Minute),
		},
		Database: DatabaseConfig{
			URL:              env("DATABASE_URL", "postgres://dashboard:dashboard@localhost:5432/dashboard?sslmode=disable"),
			Required:         boolean("DATABASE_REQUIRED", true),
			ConnectTimeout:   duration("DATABASE_CONNECT_TIMEOUT", 15*time.Second),
			StartupRetries:   integer("DATABASE_STARTUP_RETRIES", 6),
			StartupBackoff:   duration("DATABASE_STARTUP_BACKOFF", 5*time.Second),
			MaxConnections:   int32(integer("DATABASE_MAX_CONNECTIONS", 20)),
			MinConnections:   int32(integer("DATABASE_MIN_CONNECTIONS", 2)),
			MaxConnectionAge: duration("DATABASE_MAX_CONNECTION_AGE", 30*time.Minute),
		},
		Prometheus: ProviderConfig{
			URL:       env("PROMETHEUS_URL", "http://hello-k8s-ai-prometheus:9090"),
			Timeout:   duration("PROMETHEUS_TIMEOUT", 6*time.Second),
			CacheTTL:  duration("PROMETHEUS_CACHE_TTL", 5*time.Second),
			MaxWindow: duration("PROMETHEUS_MAX_WINDOW", 7*24*time.Hour),
			Enabled:   boolean("PROMETHEUS_ENABLED", true),
		},
		Jaeger: ProviderConfig{
			URL:       env("JAEGER_URL", "http://hello-k8s-ai-jaeger:16686"),
			Timeout:   duration("JAEGER_TIMEOUT", 8*time.Second),
			CacheTTL:  duration("JAEGER_CACHE_TTL", 10*time.Second),
			MaxWindow: duration("JAEGER_MAX_WINDOW", 24*time.Hour),
			Enabled:   boolean("JAEGER_ENABLED", true),
		},
		Persistence: PersistenceConfig{
			EventBuffer:       integer("PERSISTENCE_EVENT_BUFFER", 4096),
			SnapshotInterval:  duration("SNAPSHOT_INTERVAL", 30*time.Second),
			SnapshotRetention: duration("SNAPSHOT_RETENTION", 30*24*time.Hour),
		},
		LogLevel:    *logLevel,
		ClusterName: env("K8S_CLUSTER_NAME", "default"),
		Environment: env("DEPLOYMENT_ENVIRONMENT", "development"),
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg Config) validate() error {
	var failures []error
	if cfg.HTTP.Address == "" {
		failures = append(failures, errors.New("HTTP_ADDRESS must not be empty"))
	}
	if cfg.HTTP.MaxBodyBytes < 1024 {
		failures = append(failures, errors.New("HTTP_MAX_BODY_BYTES must be at least 1024"))
	}
	if cfg.Kubernetes.QPS <= 0 || cfg.Kubernetes.Burst <= 0 {
		failures = append(failures, errors.New("Kubernetes client QPS and burst must be positive"))
	}
	if cfg.Database.Required && strings.TrimSpace(cfg.Database.URL) == "" {
		failures = append(failures, errors.New("DATABASE_URL is required when DATABASE_REQUIRED=true"))
	}
	if cfg.Database.MaxConnections < cfg.Database.MinConnections {
		failures = append(failures, errors.New("DATABASE_MAX_CONNECTIONS must be >= DATABASE_MIN_CONNECTIONS"))
	}
	if cfg.Persistence.EventBuffer < 128 {
		failures = append(failures, errors.New("PERSISTENCE_EVENT_BUFFER must be at least 128"))
	}
	return errors.Join(failures...)
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func duration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func integer(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func decimal(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func boolean(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func csv(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return append([]string(nil), fallback...)
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}
