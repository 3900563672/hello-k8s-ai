# 修复 Grafana 运行中内存打满导致探针失败与组件意外停止

- 变更日期：2026-08-16
- 关联问题：Fixes #4
- 变更级别：P1 运行稳定性
- 变更范围：config/observability/grafana.yaml（资源限额）
- CRD 变化：无
- 数据库变化：无

## 1. 问题与根因

集群启动后运行一段时间，Grafana 出现探针间歇失败（`context deadline exceeded` / `503`），
日志持续出现 `http: Handler timeout`、`/api/datasources`、`/api/annotations` 等请求超时。

实测证据（docker-desktop 集群）：

- cgroup 水位：`memory.current ≈ 383MiB` / `memory.max = 384MiB`（占满 99.7%）。
- 探针事件：Liveness/Readiness probe failed（context deadline exceeded、HTTP 503），
  且滚动更新完成后仍持续出现，说明不是启动瞬态。
- Grafana 日志：大量 timeout / context canceled / 请求 8-10s 超时。

结论：384MiB 内存上限对 Grafana 13 过小，运行期内存打满引发 GC 抖动与请求超时，
探针失败后触发容器重启，表现为“运行中意外停止”。

## 2. 修复内容

config/observability/grafana.yaml：

- `limits.memory`：384Mi → 1024Mi
- `requests.memory`：128Mi → 256Mi
- `limits.cpu`：500m → 1000m
- `requests.cpu`：50m → 100m

其余组件水位实测无超限（PostgreSQL 59%、Prometheus 28%、Jaeger 23%、frontend 25%），
本次不改，避免无关变更。

## 3. 验证结果

- `kubectl apply -k config/observability` + rollout 成功。
- 滚动后 Grafana `restartCount=0`，无新探针告警。
- cgroup 水位稳定在约 547MiB / 1GiB（53%），留有充足余量。
