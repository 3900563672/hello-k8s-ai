# 实现细节：可观测组件持久化与事件丢弃可观测化

## 1. 改动前状态

### Prometheus（`config/observability/prometheus.yaml`）

- Deployment 单副本，`prom/prometheus:v3.13.2`，数据卷 `emptyDir` 挂 `/prometheus`，`--storage.tsdb.retention.time=24h`。
- 容器 securityContext：`runAsUser/runAsGroup/fsGroup=65534`、`readOnlyRootFilesystem: true`。
- 配置在 `hello-k8s-ai-prometheus-config` configmap（scrape 任务 + recording rules + alerts）。

### Jaeger（`config/observability/jaeger.yaml`）

- Deployment 单副本，`cr.jaegertracing.io/jaegertracing/jaeger:2.20.0`，无 config 参数（默认内存存储），`emptyDir` 挂 `/tmp`。
- 容器 UID 10001 / GID 0（`runAsNonRoot: true`，未显式 fsGroup）。
- OTLP 默认绑定（无配置时工作正常，因为 Jaeger 内置默认配置绑 0.0.0.0）。

### Recorder（`dashboard/backend/internal/store/recorder.go`）

- `changes chan model.ResourceChange`（默认 4096，`PERSISTENCE_EVENT_BUFFER` 可调）在 informer 回调与 DB 写之间做缓冲。
- 缓冲满 → `dropped` 原子计数 + Error 日志；DB 写失败 → 只记日志；进程内计数无法被外部读取。

## 2. 实现内容

### P0-1 Prometheus PVC

- 新增 PVC `hello-k8s-ai-prometheus-data`：`storageClassName: standard`（local-path）、RWO、20Gi。
- Deployment volume `data` 由 `emptyDir` 改为 `persistentVolumeClaim`；retention 改 168h。
- 权限无需改动：既有 fsGroup 65534 与 local-path 的 fsGroup chown 机制配合，容器可写。

### P0-2 Jaeger badger 持久化

- 新增 configmap `hello-k8s-ai-jaeger-config`，内容基于 Jaeger v2.20.0 官方 `config-badger.yaml` 精简：
  - `service.extensions: [jaeger_storage, jaeger_query, healthcheckv2]`；
  - `jaeger_storage.backends.some_store.badger`：`directories: {keys: /tmp/jaeger/, values: /tmp/jaeger/}`、`ephemeral: false`、`ttl.spans: 168h`；
  - OTLP receiver 显式 `endpoint: 0.0.0.0:4317 / 0.0.0.0:4318`（不写时 Jaeger v2 只绑 127.0.0.1，collector 连不上——实测踩坑）；
  - telemetry metrics pull 8888 保持 Prometheus 抓取。
- 新增 PVC `hello-k8s-ai-jaeger-data`（standard、RWO、10Gi），挂载到 `/tmp`（badger 目录在其下）。
- Pod securityContext 增加 `fsGroup: 0`（镜像 UID 10001/GID 0：kubelet 把 PVC 目录 chown 到组 0 并 0770，容器组可写）。
- Deployment 增加 `--config=/etc/jaeger/config.yaml`，configmap 只读挂载 `/etc/jaeger`；`readOnlyRootFilesystem` 不变。
- Deployment 注解 `platform.study.com/restart-procedure: scale-to-zero`：单副本 badger + RWO PVC 的滚动更新会新旧 Pod 抢目录锁 CrashLoop，必须先缩 0 再扩 1。

### P1-1 Backend 指标

- `recorder.go` 新增两个 `promauto` 普通 Counter：
  - `hello_k8s_ai_dashboard_events_dropped_total`：缓冲满丢弃总数；
  - `hello_k8s_ai_dashboard_events_write_failures_total`：DB 写失败总数。
- 用普通 Counter 而非 CounterVec 的原因：CounterVec 在没有 label 实例时不会出现在 `/metrics`，静默期看不到 0 值；丢掉的 kind 已写在日志与 gap payload。
- `server.go` 注册 `GET /metrics`（promhttp，默认 registry）；只读、不参与写认证，走通用中间件链。
- Prometheus config 增加 scrape job `hello-k8s-ai-dashboard-backend`（targets `hello-k8s-ai-dashboard-backend:8080`）与告警 `HelloK8sAIDashboardEventsDropped`（5 分钟丢弃速率 > 0 持续 5 分钟）。

### P1-2 TimelineGap 记录

- `Recorder` 增加 `writeFailures atomic.Uint64` 与 `recordedDropped/recordedWriteFailures` 水位（只由 Run goroutine 读写）。
- `Run` 每处理完一条 change 调用 `recordGapIfNeeded`：当丢弃或写失败计数相对上次沉淀有增量时，向 `resource_events` 写一条 `TimelineGap` 事件（`Operation: "gap"`、`Ref: TimelineGap/recorder`、payload `{"dropped":N,"writeFailures":K}` 为增量）。
- gap 写入失败：只记日志、水位不前进，DB 恢复后下轮补记；不阻塞 informer 回调（仍在 Run goroutine 内）。
- `postgres.go` `ListTimeline` 的 businessKinds 增加 `TimelineGap`，`timelineItem` 增加专门分支：`domain=runtime`、`severity=attention`、`Weight=9`、`Source=postgresql/gap`。
- 时间线 API 无需改前端：未知 kind 的条目按 domain/severity 渲染，前端已有 `attention` 样式。

## 3. 关键决策

| 决策 | 原因 |
| --- | --- |
| retention 统一 168h | 与历史回放承诺一致；Prometheus/Jaeger/PostgreSQL 快照保留期不再三套口径 |
| badger 而非内存/ES | 单副本本地部署最轻量；官方支持，PVC 即持久 |
| `TimelineGap` 复用 `resource_events` | 无新表无迁移；时间线/审计查询天然复用 |
| 计数用普通 Counter | CounterVec 静默期不可见；指标名与告警规则直接对应 |
| 重启流程 scale 0→1 | 单副本 badger/TSDB 目录锁；滚动更新必然 CrashLoop |

## 4. 边界与约束

- 不改变 Controller/CRD/API 契约；`/api/v1/replay` 响应结构不变（新增条目类型）。
- `TimelineGap` 只说明"有事件丢失"，不恢复丢失事件本身；精确顺序恢复仍依赖快照周期。
- 遥测失败不阻止控制面（既有原则不变）：gap 写失败只记日志。
- 本机 `go get` 新依赖需 `GOSUMDB=off`（本机 sumdb 校验异常，见 KNOWN_PITFALLS）；go.sum 已入库，CI 正常校验。
