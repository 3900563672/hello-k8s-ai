# 迁移与回滚

## 部署方式（必须）

```bash
docker build -t hello-k8s-ai-dashboard-backend:dev dashboard/backend
docker build -t hello-k8s-ai-dashboard-frontend:dev dashboard/frontend/my-app
kubectl kustomize config/dev | kubectl apply -f -          # 更新 deployment 引用（tag 不变）
kubectl -n hello-k8s-ai-system rollout restart deployment hello-k8s-ai-dashboard-backend
kubectl -n hello-k8s-ai-system rollout restart deployment hello-k8s-ai-dashboard-frontend
```

- 控制器未变更；仍遵守「dev 集群更新用 config/dev，不用 make deploy」的坑位。
- 无数据库迁移、无 CRD 变更；`resource_snapshots` 既有快照流直接可用。

## 回滚

- git revert 本提交后重新构建两个镜像并 rollout restart；`/segment` 路由随之消失，前端段面板不再挂载（DataOverviewPage 回退）。
- 行为差异仅在新增只读接口与页面区块，不影响调度 / 流量 / 扩缩容业务逻辑。

## 环境备注（2026-08-17 部署时发现）

- `desktop-worker6` 节点 DNS 不通（kindnet 网络故障，nginx 启动时 `host not found in upstream`），已 `kubectl cordon desktop-worker6` 规避并让前端调度到正常节点；恢复需在节点网络修复后 `kubectl uncordon desktop-worker6`。与本次代码无关，属环境故障。
- 前端镜像 tag 与旧镜像相同（`:dev`），apply 不触发滚动，必须 rollout restart。
