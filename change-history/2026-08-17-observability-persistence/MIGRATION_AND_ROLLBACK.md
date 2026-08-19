# 迁移与回滚：可观测组件持久化与事件丢弃可观测化

## 数据迁移

- Prometheus：旧 `emptyDir` 数据不迁移。切换到 PVC 时 TSDB 从空开始，本次切换发生在 2026-08-17 12:54Z；12:54Z 之前的原始指标已丢失（原本就是易失存储，重启即丢）。从本次起，Pod 重建/重启不再丢数据。
- Jaeger：旧内存存储数据不迁移（进程内，重启即丢）。badger 首次启动建库，从 12:54Z 起的 Trace 持续保留（TTL 168h）。
- PostgreSQL：无变化（`resource_events` 新增 `TimelineGap` 类型行，无需迁移；旧数据不受影响）。

## 部署步骤（已执行）

```bash
docker build -t hello-k8s-ai-dashboard-backend:dev dashboard/backend
kubectl kustomize config/dev | kubectl apply -f -   # 创建 2 个 PVC、更新 configmap/deployment
kubectl -n hello-k8s-ai-system rollout restart deployment/hello-k8s-ai-dashboard-backend
# Jaeger/Prometheus 因单副本目录锁，重启必须：
kubectl -n hello-k8s-ai-system scale deploy hello-k8s-ai-jaeger --replicas=0
kubectl -n hello-k8s-ai-system scale deploy hello-k8s-ai-prometheus --replicas=0
# 等待 Pod 清空后：
kubectl -n hello-k8s-ai-system scale deploy hello-k8s-ai-jaeger --replicas=1
kubectl -n hello-k8s-ai-system scale deploy hello-k8s-ai-prometheus --replicas=1
```

## 回滚

代码回滚：恢复 `recorder.go` / `postgres.go` / `server.go` / `go.mod` 到上一版本（backend 镜像重新构建 + rollout）。`TimelineGap` 类型的行留在 `resource_events` 中无害（时间线 kind 白名单随代码回滚不再包含它，不会展示）。

清单回滚：恢复 `prometheus.yaml` / `jaeger.yaml` 后重新 apply。注意：

1. **PVC 保留**：回滚到 emptyDir 不会删除 PVC（`kubectl apply` 不删资源）；数据仍在 PVC 上，未来再切回 PVC 不丢数据。要彻底清理需显式 `kubectl delete pvc hello-k8s-ai-prometheus-data hello-k8s-ai-jaeger-data`（会丢当前历史，谨慎）。
2. **Jaeger 回滚到无 config**：恢复后无 badger 锁问题，但历史 Trace 仍保留在 PVC 上的 `/tmp/jaeger` 数据不会被读取（内存模式从空开始）。
3. 回滚后 retention 回到 24h：超过 24h 的旧指标会被 TSDB 按新 retention 清理。

## 风险与注意

- Jaeger/Prometheus 的**滚动更新（rollout restart）会 CrashLoop**：单副本 badger/TSDB 目录锁 + RWO PVC，新旧 Pod 同时挂载即冲突。必须 scale 0 → 1（清单注解与 KNOWN_PITFALLS 已记录）。这是既有 Deployment 单副本策略下的运维约束，不是数据丢失风险。
- 缩到 0 期间组件短暂不可用（数秒到十几秒），属预期。
- `desktop-worker6` 网络故障未恢复，保持 cordon；与此清单无关，另行跟进。
