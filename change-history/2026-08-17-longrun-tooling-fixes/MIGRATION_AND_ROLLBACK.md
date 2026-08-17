# 迁移与回滚

## 部署方式（必须）

```bash
docker build -t hello-k8s-ai-dashboard-backend:dev dashboard/backend
kubectl kustomize config/dev | kubectl apply -f -          # 更新 Prometheus configmap 与 backend deployment
kubectl -n hello-k8s-ai-system rollout restart deployment hello-k8s-ai-prometheus
kubectl -n hello-k8s-ai-system rollout restart deployment hello-k8s-ai-dashboard-backend
```

- 控制器未变更；仍遵守「dev 集群更新用 config/dev，不用 make deploy」的坑位。
- `day-watch.mjs` 是脚本，随代码提交生效，无需部署。

## 回滚

- `day-watch.mjs` / `client.go`：git revert 后重新构建 backend 镜像并执行上述部署命令。
- `prometheus.yaml`：revert 后重新 apply configmap + rollout restart prometheus。
- 行为差异仅在可观测性与总结口径，不影响调度 / 流量 / 扩缩容业务逻辑。

## 数据与产物

- `.runtime/longrun/<日期>/` 新增 `meta.json` 与 `metric-samples/`；旧 run 无 meta.json 时 `--resummarize` 会拒绝，需手工补写（见 TEST_REPORT 正式 run 的做法）。
- 已为 2026-08-17 正式 run 补写 meta.json 并重生成 summary.md（20 轮口径）。
