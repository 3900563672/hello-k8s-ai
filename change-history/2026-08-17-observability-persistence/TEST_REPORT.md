# 测试报告：可观测组件持久化与事件丢弃可观测化

> 验证环境：docker-desktop 集群（WSL2），2026-08-17 UTC 12:50~13:40；backend 镜像 `hello-k8s-ai-dashboard-backend:dev`（本机构建）。

## 单元测试

```bash
cd dashboard/backend
gofmt -l .        # 空（通过）
go vet ./...      # 通过
go test ./...     # internal/api 仅 2 个已知 Grafana proxy 502（本机网络，以 CI 为准）；其余全 ok
```

新增 `internal/store/recorder_test.go` 3 个用例：

- `TestRecorderDropsWhenBufferFullAndRecordsGap`：buffer=1 发 2 条 → `Dropped()==1`；`recordGapIfNeeded` 写 1 条 `TimelineGap`，payload `dropped=1/writeFailures=0`；再次调用不重复写。
- `TestRecorderGapWriteFailureDoesNotAdvanceWatermark`：DB 故障期间 gap 不落库、水位不前进；恢复后补记 1 条且 delta 正确。
- `TestTimelineItemMarksGapAsAttention`：`timelineItem` 对 `TimelineGap` 分类为 `runtime/attention`、`Source=postgresql/gap`。

## 清单渲染

```bash
kubectl kustomize config/dev >/tmp/kustomize-dev.yaml     # 2 个 PVC、jaeger 注解、backend 抓取任务均出现
kubectl kustomize config/demo >/tmp/kustomize-demo.yaml   # 通过
kubectl kustomize dashboard/deploy >/tmp/kustomize-dashboard.yaml  # 通过
```

## 真机部署

```bash
docker build -t hello-k8s-ai-dashboard-backend:dev dashboard/backend
kubectl kustomize config/dev | kubectl apply -f -
kubectl kustomize dashboard/deploy | kubectl apply -f -
kubectl -n hello-k8s-ai-system rollout restart deployment/...  # prometheus/jaeger/backend
```

- PVC：`hello-k8s-ai-prometheus-data` 20Gi Bound、`hello-k8s-ai-jaeger-data` 10Gi Bound（standard local-path，WaitForFirstConsumer）。
- Jaeger badger 启动日志：`All 0 tables opened`、`/tmp/jaeger/` 生成 `000001.vlog / 00001.mem / DISCARD / KEYREGISTRY / LOCK`；Query 16686 / OTLP `[::]:4317/4318` 就绪。
- otel-collector：滚动期短暂 `connection refused` 后自愈（重试机制），13:02:24 后再无导出失败；Jaeger `/api/services` 返回 `hello-k8s-ai-controller / hello-k8s-ai-simulator / jaeger`。

## 关键验收：重启后数据保留

### Prometheus

重启前：`count(up)` = 205；`up{job="prometheus"}[30m]` 首样本 1786971281。

执行 `scale 0 → 1` 重启后：

- `count(up)` = **205**（不变）；
- `up{job="prometheus"}[40m]` 首样本 **1786971281**（重启前数据仍在），`HAS_PRE_RESTART_DATA: True`。

### Jaeger

重启前记录 Trace `53a4ca260552b149c7dbeae1daa744d4`；`scale 0 → 1` 重启后：

- `GET /api/traces/53a4ca260552b149c7dbeae1daa744d4` → **FOUND, spans=1**（badger PVC 持久化生效）；
- `/api/services` 三个服务完整。

### Backend `/metrics`

```text
# HELP hello_k8s_ai_dashboard_events_dropped_total ...
hello_k8s_ai_dashboard_events_dropped_total 0
# HELP hello_k8s_ai_dashboard_events_write_failures_total ...
hello_k8s_ai_dashboard_events_write_failures_total 0
```

Prometheus target `hello-k8s-ai-dashboard-backend`：`up`（抓取正常）。

## API 回归

| 接口 | 结果 |
| --- | --- |
| `GET /api/v1/health/live` | 200 |
| `GET /api/v1/health/ready` | 200（database available，resource_events=302854） |
| `GET /api/v1/replay` | 200（timeline 正常） |
| `GET /api/v1/overview` | 200 |

## 环境（P2）验证

- `kubectl get node desktop-worker6`：`Ready,SchedulingDisabled` + unschedulable taint 仍在。
- 调度 busybox 到 worker6（带 toleration）执行 nslookup：`connection timed out; no servers could be reached`（`hello-k8s-ai-dashboard-postgresql` 与 `kubernetes.default` 均超时）。
- kindnet（worker6）日志持续出现 `lookup desktop-control-plane: i/o timeout`。
- 结论：网络未恢复，**保持 cordon**，不 uncordon。

## 未验证

- 真实缓冲丢弃（4096 容量在生产很难触发）未在真机制造；计数器增量与 TimelineGap 写入由单元测试覆盖。
- CI（lint / E2E / 部署验证）以推送后 workflow 结果为准。
- 前端无改动，未跑前端检查。