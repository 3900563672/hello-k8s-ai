# 配置参考

本文件记录关键默认值，最终依据仍是源码和清单。修改默认值时同时更新测试与对应专题文档。

## 1. Controller Manager flags/env

| Flag / Env | 默认/开发值 | 说明 |
| --- | --- | --- |
| `--metrics-bind-address` | flag默认 `0`；dev `:8443` | metrics endpoint。 |
| `--metrics-secure` | true | HTTPS + authn/authz filter。 |
| `--health-probe-bind-address` | `:8081` | health/readiness。 |
| `--leader-elect` | flag默认 false；dev启用 | Manager leader election。 |
| LeaderElectionID | `5e2d7bf4.platform.study.com` | 固定代码值。 |
| `--enable-http2` | false | 安全原因默认只 HTTP/1.1。 |
| `SIMULATOR_NAMESPACE` | dev Downward API namespace | 动态 Deployment namespace。 |
| `SIMULATOR_IMAGE` | flag fallback `simulator:latest`；dev `hello-k8s-ai-simulator:dev` | Simulator 镜像。 |
| `SIMULATOR_IMAGE_PULL_POLICY` | `IfNotPresent` | 必须 Always/IfNotPresent/Never。 |
| `SIMULATOR_SERVICE_ACCOUNT` | dev `hello-k8s-ai-simulator-sa` | Simulator Pod SA。 |
| `--simulator-metrics-port` | 9090 | 范围 1..65535。 |
| `APP_VERSION` | `dev` | OTel resource。 |
| `K8S_CLUSTER_NAME` | dev `docker-desktop` | OTel resource。 |
| `DEPLOYMENT_ENVIRONMENT` | dev `docker-desktop` | OTel resource。 |

OTel 通用：`OTEL_EXPORTER_OTLP_ENDPOINT`、`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`、`OTEL_EXPORTER_OTLP_INSECURE`、`OTEL_SDK_DISABLED`、`OTEL_TRACES_SAMPLER`、`OTEL_TRACES_SAMPLER_ARG`。`SIMULATOR_` 前缀变量由 Manager 注入到动态 Pod。

本地演示使用 Make 变量 `DEMO_MODEL_ABSOLUTE_SCORE`，默认 100。`make cluster-up` 会把它写入 `Model.spec.absoluteScore`；生产 Model 也必须显式提供自己的正整数基准分。升级时，脚本会把仍在旧 `status.absoluteScore` 中的正数复制到 Spec，但不会为完全缺失的模型猜测分数。

## 2. Simulator

| Flag/Env | 默认 | 说明 |
| --- | --- | --- |
| `--instance` / `SIMULATOR_INSTANCE_NAME` | 必需 | Cluster-scoped Instance 名。 |
| `--interval` | 5s | Tick，必须 >0。 |
| `--pod-name` / `POD_NAME` | 必需 | Lease identity/reporterID。 |
| `--pod-namespace` / `POD_NAMESPACE` | 必需 | Lease namespace。 |
| `--metrics-bind-address` | `:9090` | metrics/health；`0` 禁用。 |
| LeaseDuration | 15s | 代码固定。 |
| RenewDeadline | 10s | 代码固定。 |
| RetryPeriod | 2s | 代码固定。 |
| sample freshness | 约 30s | Controller 逻辑常量。 |

### Simulator 时间倍速

| 配置 | 默认/范围 | 说明 |
| --- | --- | --- |
| `SimulationClock/default.spec.rate` | 1；1..20 | 全局期望倍速，由用户或 Backend Clock API 写。 |
| `SimulatorInstance.spec.timeScale` | 1；1..20 | Clock Controller 派生，Simulator 每个真实 Tick 读取。 |
| Frontend 选项 | 1x、2x、5x、10x、20x | API 接受范围内任意整数，UI 提供常用档位。 |

倍速只影响 SimEngine 步长和冷启动累计模拟时间。`--interval`、Controller fallback、新鲜度、冷却、Lease 和采集周期仍是真实时间。

## 3. Backend HTTP

| Env | 默认 | 说明 |
| --- | --- | --- |
| `HTTP_ADDRESS` | `:8080` | listen address。 |
| `HTTP_READ_TIMEOUT` | 15s | body read。 |
| `HTTP_READ_HEADER_TIMEOUT` | 5s | header。 |
| `HTTP_WRITE_TIMEOUT` | 30s | 普通响应；SSE 排除。 |
| `HTTP_IDLE_TIMEOUT` | 90s | keep-alive。 |
| `HTTP_SHUTDOWN_TIMEOUT` | 15s | graceful shutdown。 |
| `HTTP_MAX_BODY_BYTES` | 1MiB | 最小允许 1024。 |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173` | 逗号分隔。 |
| `LOG_LEVEL` | `info` | slog level。 |

## 4. Backend Kubernetes

| Env | 默认 | 说明 |
| --- | --- | --- |
| `KUBECONFIG` | 空 | 空时优先 in-cluster。 |
| `KUBE_CONTEXT` | 空 | kubeconfig context；注意不是文档旧写法 K8S_CONTEXT。 |
| `KUBE_CLIENT_QPS` | 50 | 必须 >0。 |
| `KUBE_CLIENT_BURST` | 100 | 必须 >0。 |
| `KUBE_CACHE_RESYNC_PERIOD` | 10m | informer resync。 |
| `KUBE_CACHE_SYNC_TIMEOUT` | 2m | readiness 初始同步窗口。 |

## 5. Backend Database/Persistence

| Env | 默认 | 说明 |
| --- | --- | --- |
| `DATABASE_URL` | local dashboard URL，sslmode=disable | 生产必须覆盖/保密/TLS。 |
| `DATABASE_REQUIRED` | true | required 时影响 readiness。 |
| `DATABASE_CONNECT_TIMEOUT` | 15s | 连接。 |
| `DATABASE_MAX_CONNECTIONS` | 20 | 必须 >= min。 |
| `DATABASE_MIN_CONNECTIONS` | 2 | pool。 |
| `DATABASE_MAX_CONNECTION_AGE` | 30m | pool recycle。 |
| `PERSISTENCE_EVENT_BUFFER` | 4096 | 最低 128。 |
| `SNAPSHOT_INTERVAL` | 30s | 必须为正；非法值回退默认。 |
| `SNAPSHOT_RETENTION` | 30d | dashboard 清单写 720h。 |

## 6. Provider

| Env | Prometheus 默认 | Jaeger 默认 |
| --- | --- | --- |
| URL | `http://hello-k8s-ai-prometheus:9090` | `http://hello-k8s-ai-jaeger:16686` |
| ENABLED | true | true |
| TIMEOUT | 6s | 8s |
| CACHE_TTL | 5s | 10s |
| MAX_WINDOW | 7d | 24h |

变量分别为 `PROMETHEUS_*` 和 `JAEGER_*`。

## 7. Backend identity

| Env | 默认/清单 | 说明 |
| --- | --- | --- |
| `K8S_CLUSTER_NAME` | 默认 `default`；dashboard清单 `docker-desktop` | bootstrap/metadata。 |
| `DEPLOYMENT_ENVIRONMENT` | 默认 `development`；清单 `docker-desktop` | 环境。 |

Controller、Simulator 与 Backend 的本地清单统一使用 `docker-desktop`，避免 Trace 和页面集群维度分裂。

## 8. Frontend

| Env | 默认 | 说明 |
| --- | --- | --- |
| `VITE_API_BASE_URL` | `/api/v1` | 浏览器 API base。 |
| `VITE_API_PROXY_TARGET` | `http://localhost:8080` | Vite dev `/api` proxy。 |
| `VITE_APP_TITLE` | `调度控制台` | 页面标题（按实际使用确认）。 |

Nginx listen 8080，`/api/` proxy Backend service，read timeout 1h、buffering/cache off；SPA fallback `index.html`；assets immutable 1y。

## 9. CRD 默认

| CRD/字段 | 默认 |
| --- | ---: |
| Model.performance.prefillBaseMs | 50 |
| Model.performance.prefillPerTokenUs | 500 |
| Model.performance.decodePerTokenMs | 20 |
| Tenant.ttftThresholdMs / down | 必填，无默认 |
| Tenant.queueThreshold / down | 必填，无默认 |
| Orchestrator scaleUp/down cooldown | 60 / 120 s |
| Orchestrator min / max replicas | 1 / 必填 |

## 10. 配置解析注意

Backend 的 duration/int/bool 环境变量解析失败时多数回退默认，而不是启动失败；只有最终 `validate()` 检查部分跨字段/最小值。这提高容错但可能掩盖拼写错误。生产建议启动日志输出脱敏后的有效配置，并对显式非法值 fail fast。
