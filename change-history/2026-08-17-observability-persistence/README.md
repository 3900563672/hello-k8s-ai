# 可观测组件持久化与事件丢弃可观测化（P0/P1）

- 变更日期：2026-08-17（Asia/Shanghai 21:00~21:50；UTC 13:00~13:50）
- 关联问题：[#28](https://github.com/3900563672/hello-k8s-ai/issues/28)（design: 可观测组件持久化与事件丢弃可观测化（PVC + gap 记录））
- 变更级别：P0 可观测性持久化
- 变更范围：`config/observability/prometheus.yaml`、`config/observability/jaeger.yaml`、`dashboard/backend/internal/store/recorder.go`、`dashboard/backend/internal/store/postgres.go`、`dashboard/backend/internal/api/server.go`、`dashboard/backend/go.mod`、`dashboard/backend/internal/store/recorder_test.go`（新增）
- CRD 变化：无 ｜ 数据库变化：无（`TimelineGap` 复用 `resource_events` 表，无迁移）

## 1. 为什么做

Prometheus 与 Jaeger 此前数据卷都是 `emptyDir`：Pod 重建后历史指标与 Trace 全部丢失。PostgreSQL 的 Snapshot 可以长期保留，但配套的性能数据与 Trace 会随 Pod 重启归零——历史页面只能部分重建，用户无法区分"当时没有数据"与"数据已丢失"。issue-09 归档日志已证明这发生在 Prometheus 重启（11:41Z 丢当天历史）等真实场景。

事件历史也不是无损的。`recorder.go` 为避免阻塞 informer 使用有限 channel：缓冲区满直接丢弃（只有日志 + 进程内计数），数据库写失败只记日志。丢失没有成为可查询的系统状态，历史消费者不知道时间线存在缺口。

## 2. 完成结果

1. **Prometheus 持久化**：新增 PVC `hello-k8s-ai-prometheus-data`（20Gi、RWO、`standard` local-path），数据卷由 `emptyDir` 改为 PVC；retention 从 24h 提到 168h（与 Jaeger 一致，匹配历史回放承诺）。
2. **Jaeger 持久化**：新增 badger 存储配置（configmap + `--config=/etc/jaeger/config.yaml`）与 PVC `hello-k8s-ai-jaeger-data`（10Gi），`ephemeral: false`、spans TTL 168h；OTLP receiver 显式绑定 `0.0.0.0:4317/4318`；`fsGroup: 0` 解决镜像 UID 10001/GID 0 对 local-path PVC 的写权限。
3. **Backend 自描述指标**：新增 `GET /metrics`（promhttp），暴露 `hello_k8s_ai_dashboard_events_dropped_total` 与 `hello_k8s_ai_dashboard_events_write_failures_total` 两个计数器；Prometheus 增加 backend 抓取任务与 `HelloK8sAIDashboardEventsDropped` 告警规则。
4. **时间线 gap 记录**：Recorder 在丢弃/写失败水位变化时向 `resource_events` 写 `TimelineGap` 事件（payload 携带 dropped/writeFailures 增量），并加入 `/api/v1/replay` 时间线（`attention` 级别条目）；gap 写入失败不阻塞主循环，水位不前进、恢复后补记。
5. **环境（P2）**：worker6 复测 DNS 仍全超时（busybox 实测 `connection timed out`），保持 cordon 并记录；不强行 uncordon。
6. 单测：新增 `recorder_test.go`（缓冲丢弃、gap 记录、写失败水位不前进、TimelineGap 时间线分类）。

## 3. 影响文件

| 文件 | 变更 |
| --- | --- |
| `config/observability/prometheus.yaml` | PVC + retention 168h + backend 抓取任务 + 丢弃告警 |
| `config/observability/jaeger.yaml` | badger 配置 configmap + PVC + `--config` + fsGroup + OTLP 0.0.0.0 + 重启流程注解 |
| `dashboard/backend/internal/store/recorder.go` | 丢弃/写失败计数器 + TimelineGap 记录 |
| `dashboard/backend/internal/store/postgres.go` | `TimelineGap` 进时间线 kind 白名单 + 条目渲染 |
| `dashboard/backend/internal/api/server.go` | 注册 `GET /metrics` |
| `dashboard/backend/internal/store/recorder_test.go` | 新增：丢弃/gap/水位/时间线分类测试 |
| `dashboard/backend/go.mod` / `go.sum` | 新增 `prometheus/client_golang` |

## 4. 验证摘要

- backend：`gofmt` / `go vet` / 单元测试全过；本机仅已知 Grafana proxy 2 个 502（以 CI 为准）。
- 清单：`kubectl kustomize config/dev`、`config/demo`、`dashboard/deploy` 全部渲染通过。
- 真机（docker-desktop）：Prometheus/Jaeger 写入数据后各做一次"缩 0 扩 1"重启——`count(up)` 从 205 保持 205、重启前 30 分钟采样仍在；重启前的 Trace `53a4ca26...` 重启后仍可查询（FOUND, spans=1）；backend `/metrics` 两个计数器可见（0 值）；`/api/v1/replay`、`/overview`、`health` 回归 200。
- 详情见 `TEST_REPORT.md`。

## 5. 未验证 / 风险

- `desktop-worker6` kindnet 网络故障持续（DNS 到 API Server 全超时），保持 cordon；建议后续从 kindnet/kube-proxy 日志与节点容器侧排查，未获用户批准前不重启节点。
- 切换 PVC 时旧 `emptyDir` 数据未迁移（新存储从空开始），属预期；从本次起重启不再丢历史。
- Jaeger badger 单副本 + RWO PVC：滚动更新会新旧 Pod 抢目录锁 CrashLoop，重启/升级必须先 `scale 0` 再 `scale 1`（清单已加注解，坑位见 KNOWN_PITFALLS）。
- 事件丢弃计数用普通 Counter（非 CounterVec）：Vec 无 label 实例时不出现在 `/metrics`，普通 Counter 保证静默期可见 0 值；丢掉的 kind 记录在日志与 gap payload。
- 本机未跑前端（无改动）；CI 全量结果以推送后 workflow 为准。
