# 测试报告

测试日期：2026-08-14

## 1. 结论

| 检查 | 结果 | 说明 |
| --- | --- | --- |
| `gofmt` | PASS | 本次修改的 Go 文件全部使用 Go 1.26.0 `gofmt`。 |
| 限定变更范围 `git diff --check` | PASS | 无新增尾随空格或 patch 格式错误。 |
| 放置相关定向测试 | PASS | 11 个顶层测试及 5 个非法 payload 子用例通过。 |
| `go test ./internal/controller` | PASS | Controller 包全部测试通过。 |
| `go test ./...` | PASS | 根 Go module 全部包通过。 |
| `go vet ./...` | PASS | 无 vet 错误。 |
| `go test -race ./internal/controller` | PASS | Controller 包 race detector 通过。 |
| E2E 编译 | PASS | `go test -tags=e2e ./test/e2e -run '^$'` 通过。 |
| Kind 实际 E2E | NOT RUN | 当前工作容器没有 Docker、kubectl、kind；没有伪造集群执行结论。 |

Go 工具启动时出现环境提示：`readlink /proc/self/exe` 无法启动 telemetry sidecar。Go 命令本身正常执行并以成功状态结束；该提示不来自项目代码，也不影响测试结果。

## 2. 定向测试结果

实际执行命令：

```bash
go test ./internal/controller -run 'Placement|ScalingPlan|DecideAt' -count=1 -v
```

通过的顶层用例：

1. `TestPlacementReservationCoversPodsNotYetMaterialized`
2. `TestScalingPlanIsIdempotentAcrossRetries`
3. `TestScalingPlanMigratesLegacyPodPlacement`
4. `TestPlacementRebalanceKeepsReplicaCount`
5. `TestSimulatorInstancePlacementCreatesNodePinnedDeployments`
6. `TestSimulatorInstanceRejectsPlacementReplicaMismatch`
7. `TestNodePlacementPlanRoundTripAndScaling`
8. `TestNodePlacementPlanRejectsInvalidPayloads`
9. `TestDecideAtScaleUpCooldownAndScaleDownFloor`
10. `TestDecideAtSupportsScaleToZeroAndMaximum`
11. `TestDecideAtRebalancesPlacementAfterPolicyChange`

非法计划子用例覆盖：未知版本、缺失主节点、空计划残留主节点、重复节点、零副本。

## 3. 覆盖矩阵

| 风险 | 测试证据 |
| --- | --- |
| 评分选中的节点在 Decision 中丢失 | 扩容纯函数断言 `Decision.NodeName`。 |
| 缩容不知道从哪个节点回收 | 缩容纯函数断言优先回收非主节点。 |
| replicas 与 placement 分两次写导致半完成状态 | 幂等 apply 测试断言一次资源更新后的 replicas 和 plan；重复 apply 不再次扩容。 |
| 旧实例无法升级 | 旧 Pod `spec.nodeName` 迁移测试恢复 node-a，再把新增副本放到 node-b。 |
| Pod 尚未创建时容量被再次使用 | 逻辑预留测试断言“1 个实际 Pod + 1 个计划差额”按 2 个副本计量。 |
| 多节点副本被错误写进一个 Deployment | 物化测试断言生成两个 Deployment，副本分别为 1 和 2。 |
| Scheduler 仍可选到其他 eligible node | fake-client 测试断言每个 Deployment required affinity 只有一个节点；Kind E2E 进一步断言实际 Pod nodeName。 |
| placement 节点删除后工作负载残留 | 缩容后断言次节点 Deployment 已删除。 |
| TenantRuntime 只统计主 Deployment | 聚合测试断言主/次节点 1+2 个可用副本合计为 3，并忽略 obsolete Deployment。 |
| placement 合计与 replicas 不一致仍创建 Pod | 错误计划测试断言返回错误且未创建 Deployment。 |
| 升级后所有旧实例立即重建 | 旧路径兼容测试断言没有 annotation 时仍是一个 Deployment 和完整 eligible set。 |
| Policy 收窄使已有 Pod 永久留在失效节点 | Rebalance 决策和执行测试断言 source node-a 迁到 node-b、总副本不变。 |
| Policy 收窄期间在失效节点反向扩容 | 物化测试允许已有失效节点 Deployment 从 2 缩到 1，并拒绝从 1 增回 2。 |
| Rebalance 污染扩缩容 cooldown/history | 集成测试断言 `Orchestrator.status.lastScaling` 保持为空。 |
| 多 Controller 逻辑存在数据竞争 | Controller 包 race detector 通过。 |

## 4. 全量根模块结果

实际执行：

```bash
go test ./... -count=1
go vet ./...
go test -race ./internal/controller -count=1
```

通过的包：

- `cmd`
- `internal/controller`
- `internal/observability`
- `simulator`
- `api/v1`、`test/utils` 无测试文件但成功编译

## 5. E2E 用例内容

`test/e2e/e2e_test.go` 新增实际 Scheduler 验证：

1. 读取 Controller Pod 所在节点，保证目标节点可调度；
2. 创建只允许该节点的 TenantNodePolicy、ModelNodePolicy；
3. 创建带单节点 placement annotation、replicas=1 的 SimulatorInstance；
4. 等待主 Deployment，并断言 required affinity 等于目标节点；
5. 等待 Pod 被 Scheduler 绑定，并断言 `spec.nodeName` 等于目标节点；
6. 用例结束删除测试 CR。

E2E 源码已带 `e2e` build tag 成功编译。当前容器缺少 Docker、kubectl、kind，不能启动 Kind，因此运行态结果必须由 CI 或目标机器执行：

```bash
make test-e2e
```

## 6. 目标集群部署后验收

代码测试证明控制逻辑与生成的 Deployment 约束正确；最终目标集群还应按 [DEPLOYMENT_AND_ROLLBACK.md](DEPLOYMENT_AND_ROLLBACK.md) 对比：

- SimulatorInstance placement；
- Deployment required affinity；
- Pod 实际 nodeName；
- WorkerNode 实际占用；
- Controller 的 Rebalance/blocked 日志和指标。

## 7. 后续 CI 结果

本报告记录的是首次提交前的验证边界，因此保留当时 Kind E2E 未执行的事实。首次提交后的 GitHub Actions 实际运行发现：

- `restricted` Pod Security Namespace 拒绝了缺少 `seccompProfile` 的 Simulator Pod 模板，放置用例最终因 Pod 列表为空超时；
- golangci-lint v2.12.2 的 `modernize` 规则要求逆序遍历使用 `slices.Backward`。

对应修复与复测证据见 [Orchestrator 放置修复的 CI 收敛](../2026-08-14-orchestrator-placement-ci-follow-up/README.md)。
