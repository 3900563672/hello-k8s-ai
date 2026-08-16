# Orchestrator 放置修复的 CI 收敛

日期：2026-08-14  
关联问题：Fixes #6  
性质：P0 修复的 CI follow-up

## 1. 结果

本次处理首次真实 Kind E2E 和 golangci-lint 暴露的两个阻断：

1. Simulator Deployment 已创建，但 Namespace 启用了 `restricted` Pod Security，Pod 模板缺少 `seccompProfile: RuntimeDefault`，ReplicaSet 无法创建 Pod；
2. `scaleDownPlacementNode` 的逆序下标循环不符合项目当前 golangci-lint v2.12.2 `modernize` 规则。

修复后，Simulator Pod 模板满足当前 `restricted` 基线；测试环境会构建并加载真实 Simulator 镜像，不再依赖不存在的 `simulator:latest`；lint 返回 `0 issues`。

## 2. 为什么 Deployment 存在但 Pod 为零

CI 已先成功读取 `simulator-placement-e2e-instance` Deployment 的 node affinity，证明 Controller 的放置物化已经执行。随后两分钟内 Pod 列表一直为空。

Simulator 容器原本已有 `runAsNonRoot`、`allowPrivilegeEscalation: false`、只读根文件系统和 `drop: ALL`，但 Pod/Container 都没有声明 seccomp profile。E2E 在部署前给 Namespace 设置了：

```text
pod-security.kubernetes.io/enforce=restricted
```

在该策略下，ReplicaSet 创建 Pod 时会被准入拒绝，所以不会出现 Pending 或 ImagePullBackOff Pod。镜像问题本身只会影响容器启动，不会解释“Pod 对象为零”。

## 3. 修改范围

| 文件 | 修改目的 |
| --- | --- |
| `internal/controller/simulatorinstance_controller.go` | 为所有 Simulator Pod 模板设置 Pod 级 `RuntimeDefault` seccomp profile。 |
| `internal/controller/controller_integration_test.go` | 断言逐节点 Deployment 同时具备精确 affinity 和安全上下文。 |
| `internal/controller/placement_plan.go` | 使用 `slices.Backward` 保持原有逆序缩容语义并通过 modernize。 |
| `test/e2e/e2e_suite_test.go` | 构建并加载独立 Simulator 镜像。 |
| `test/e2e/e2e_test.go` | 配置测试镜像、检查 seccomp、稳定读取 Pod 列表并等待 Pod Ready。 |
| `change-history/` | 保存本次原因、实现、影响和验证证据。 |

没有修改 CRD、字段所有权、放置计划格式、评分算法、Backend 或 Frontend。

## 4. 运行影响

- Controller 下一次 Reconcile 会把安全上下文写入现有 Simulator Deployment Pod template，Kubernetes 会按现有 RollingUpdate 策略更新 Pod。
- 节点选择、placement annotation、Deployment 命名、副本数和资源核算逻辑不变。
- `RuntimeDefault` 使用容器运行时默认 seccomp 配置，不需要额外 RuntimeClass 或宿主机文件。
- E2E 新增 Simulator 镜像构建，首次无缓存执行会增加构建时间，但消除了外部镜像拉取的不确定性。

## 5. 回滚

本次没有数据迁移和 CRD 变更。需要回滚时可恢复上述三个生产代码/测试文件并重新发布 Controller。已经写入 Deployment 的 `RuntimeDefault` 安全配置可以保留，不影响旧 Controller 工作。

## 6. 详细记录

- [实现细节](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
