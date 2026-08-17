# 迁移与回滚

## 部署方式（必须）

```bash
# 更新 dev 集群 controller（不要用 make deploy，会丢 SIMULATOR_IMAGE env）
kubectl kustomize config/dev | kubectl apply -f -
kubectl -n hello-k8s-ai-system rollout restart deployment hello-k8s-ai-controller-manager
```

## 回滚

- 代码回滚到上一个提交（`8c208f0` 之前）后重新构建镜像并执行上述部署命令即可；CRD/数据库无变化，无需迁移。
- 行为差异仅在扩容决策：旧版每次 +1，新版按缺口批量（最多 10）。回滚后批量能力消失，其他语义不变。

## 长时测试前配置（14:00 计划）

- WorkerNode `spec.maxConcurrency` 按目标副本数放大：目标 120 副本 × 模型 maxConcurrency 16 ÷ 节点数 2 = 每节点 960；gpu 同步放大（模型 gpuUnits 8 × 120 ÷ 2 = 480）。
- 流量剧本按容量公式选择 QPS：QPS × 平均服务时长 4.3s ÷ 16 = 目标副本数，避免重演"容量远小于负载"。
