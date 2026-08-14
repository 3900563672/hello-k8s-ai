# 部署、观察与回滚

## 1. 升级前提

本次没有 CRD、数据库或 Frontend 迁移。覆盖源码后，按项目现有方式重新构建并部署 Controller 镜像即可。不要只重启旧镜像；旧二进制不包含 placement 执行逻辑。

升级前建议保存当前对象：

```bash
kubectl get simulatorinstances -o yaml > /tmp/simulatorinstances-before-placement-fix.yaml
kubectl -n hello-k8s-ai-system get deployment,pod -o yaml > /tmp/simulator-workloads-before-placement-fix.yaml
```

## 2. 升级行为

- replicas=0 的实例：首次扩容时直接建立 placement。
- 已有稳定 Pod 的实例：升级后保持旧单 Deployment；下一次扩缩容时从实际 Pod 落点迁移。
- rollout 中、Pod 尚未调度或观察 Pod 数不等于 replicas 的实例：Orchestrator 暂停新扩缩容，等待稳定。
- Policy 已收窄且现有 placement 失效：优先逐副本 Rebalance；目标无容量时阻塞，不绕过 Policy。

## 3. 运行观察

### 查看计划

```bash
INSTANCE='<SimulatorInstance 名称>'
kubectl get simulatorinstance "$INSTANCE" \
  -o go-template='{{index .metadata.annotations "platform.study.com/node-placements"}}{{"\n"}}'
```

### 对比 Deployment 约束与 Pod 落点

```bash
kubectl -n hello-k8s-ai-system get deployment \
  -l "platform.study.com/instance=$INSTANCE" \
  -o custom-columns='NAME:.metadata.name,DESIRED:.spec.replicas,NODES:.spec.template.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms[0].matchExpressions[0].values'

kubectl -n hello-k8s-ai-system get pod \
  -l "platform.study.com/instance=$INSTANCE" \
  -o custom-columns='POD:.metadata.name,NODE:.spec.nodeName,PHASE:.status.phase'
```

验收条件：

1. placement 副本合计等于 SimulatorInstance `spec.replicas`；
2. Deployment desired 副本合计等于 `spec.replicas`；
3. 每个已迁移 Deployment 的 NODES 只有一个值；
4. Pod 的 NODE 等于所属 Deployment 的唯一 NODES 值；
5. `status.availableReplicas` 等于各节点 Deployment available replicas 合计；
6. WorkerNode usedGPU/usedConcurrency 最终与实际非终态 Pod 一致。

## 4. 常见升级状态

| 现象 | 含义 | 处理 |
| --- | --- | --- |
| annotation 暂时为空 | 旧实例尚未发生扩缩容。 | 无需手工补写；保持兼容路径。 |
| `no_feasible_placement` | 没有满足 Policy 和逻辑容量的扩容目标。 | 检查 WorkerNode 容量、Model 需求和 Policy。 |
| `placement_rebalance_blocked` | 现有节点已失效，但没有合法且有容量的迁移目标。 | 增加合法容量或调整 Policy；不要手改 placement。 |
| Instance `DeploymentReconcileFailed` | placement 无法解析、合计不一致或目标节点不合法。 | 查看 Controller 日志和 annotation；优先恢复备份，不要同时改 replicas 和 placement。 |
| 一个 Instance 出现多个 Deployment | 正常的逐节点物化结果。 | 按 instance identity 聚合，不按单一名称判断。 |

## 5. 安全回滚

旧版 Controller 不认识额外的节点 Deployment。直接切回旧镜像会把主 Deployment 恢复为总副本数，却不会清理额外 Deployment，可能临时重复运行副本。因此回滚前必须先把对象恢复到旧单 Deployment 形态。

推荐短维护窗口执行：

1. 暂停产生新的配置或流量变更。
2. 保持新版 Controller 运行，删除 placement annotation，让新版 Instance Controller 自动合并为旧单 Deployment并清理额外 Deployment。

```bash
kubectl annotate simulatorinstances --all \
  platform.study.com/node-placements-
```

3. 确认每个 Instance 只剩一个 Deployment，且主 Deployment desired 副本等于 Instance replicas。
4. 立即部署旧版 Controller 镜像。
5. 再次核对 Pod 总数、TenantRuntime 和 WorkerNode Status。

如果 Orchestrator 在步骤 2 到步骤 4 之间又触发扩缩容，可能重新写入 placement。此时重复步骤 2，并缩短切换窗口。对于严格维护窗口，可先把 Controller Deployment 缩到 0，手工清理额外 `*-node-<12位哈希>` Deployment 并把主 Deployment replicas 恢复为 Instance replicas，再启动旧镜像。

## 6. 数据恢复边界

- 删除 annotation 不删除 CR，也不修改 Tenant/Model/Policy/数据库历史。
- placement 是可由实际 Pod 分布重新建立的内部执行状态，但不要在 Pod 分布不稳定时手工构造。
- pending plan 仍按原恢复机制处理；不要只删除 pending plan 而保留一半完成的 replicas/placement。
- 回滚后历史审计中已经记录的 ScaleUp/ScaleDown 不需要改写；Rebalance 本来就不写扩缩容历史。
