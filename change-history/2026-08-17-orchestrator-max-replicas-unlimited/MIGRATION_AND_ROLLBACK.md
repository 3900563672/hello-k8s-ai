# 迁移与回滚

## 1. 迁移

代码与 CRD 部署后，已有 Orchestrator 配置不会自动变化：

```bash
# 放开当前集群的副本上限（0 = 不限制）
kubectl patch orchestrator orch-core -n hello-k8s-ai-system \
  --type merge -p '{"spec":{"maxReplicas":0}}'
```

或在前端“配置 → 编排策略”把最大副本数改为 0（表单已支持并带说明）。

新建的 Orchestrator 默认即为 0（前端默认值与核心预置模板已改）。

## 2. 回滚

- 代码回滚：还原本次提交即可；旧 CRD 要求 `maxReplicas >= 1`，若集群里存在 `maxReplicas: 0` 的 CR，回滚后会被新 CRD 拒绝写入，需先把 CR 改回正整数。
- 配置回滚：把 `orch-core.spec.maxReplicas` 改回原值 10，或任意正整数即恢复封顶行为；`scaleUpCooldownSeconds` 等其余字段不受影响。

## 3. 风险

- 0 = 无限制后，副本会一直扩到节点/模型容量为止；模拟器无背压（队列只积压不拒绝），极端 QPS 下队列与 TTFT 仍会持续升高，这是既有设计，本次未改动。
