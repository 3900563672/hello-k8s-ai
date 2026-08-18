# Controller 架构

> 维护层：human | last-reviewed：2026-08-18 | 事实源：internal/controller/、internal/k8sutil/ 等

## 1. Manager 中实际注册的七个 Reconciler

| 文档简称 | Go 类型 | Primary resource |
| --- | --- | --- |
| Tenant Controller | `TenantModelPolicyReconciler` | TenantModelPolicy |
| SimulationClock Controller | `SimulationClockReconciler` | SimulationClock |
| SimulatorInstance Controller | `SimulatorInstanceReconciler` | SimulatorInstance |
| Traffic Controller | `TrafficReconciler` | Tenant |
| Performance Controller | `PerformanceCollectorReconciler` | Tenant |
| WorkerNode Controller | `WorkerNodeUsageReconciler` | WorkerNode |
| Orchestrator Controller | `OrchestratorReconciler` | Orchestrator |

不存在单独以 Tenant 为主资源、名为 `TenantController` 的第八个 Controller。旧称只是业务简称。

## 2. 通用 Reconcile 约定

- 所有 CR 都是 cluster-scoped，`request.Namespace` 通常为空。
- Reconcile 可重复，比较 desired/actual 后才 Patch。
- 通过 standard Condition 表达成功、等待和失败原因。
- Watch generation/status 的精确变化，周期 Requeue 处理时钟和漏事件。
- Controller 通过 API Server 协作，不互相调用。
- 业务指标记录 outcome/operation/duration；OTel Span 命名 `controller.<name>.reconcile`，日志带 traceID。
- Manager dev 模式开启 leader election，因此同一时刻只有一个 Manager 实例执行控制循环。

## 3. TenantModelPolicy Controller

### 输入与 Watch

- Primary：TenantModelPolicy。
- 读取：同 Tenant-Model 组合的全部 TenantModelPolicy、Tenant、Model、SimulatorInstance。
- Watch：SimulatorInstance 生命周期；Tenant/Model generation 变化。

### 决策

1. 聚合同一 tenant/model 的 policy。
2. 任意 Deny -> effective denied。
3. 否则至少一个显式 Allow -> allowed；没有 Allow 也 denied。
4. Tenant/Model 不存在或删除中 -> 不创建/删除实例。

### 输出与字段

- 确保 SimulatorInstance，名称为稳定的 `<tenant>-<model>`（过长时哈希截断）。
- 创建时写：tenantRef、modelRef、`replicas=0`、`traffic.qps=0`、`timeScale=1`、平台标签/注解、Tenant owner reference。后续倍速由 SimulationClock Controller 独占。
- 后续只维护自己拥有的 identity/refs/metadata，不覆盖 replicas/QPS/Status。
- denied 时删除对应 SimulatorInstance。
- 写 TenantModelPolicy Status Ready Condition。
- finalizer：`platform.study.com/tenant-model-policy`，删除路径确保实例清理。

### 与其他 Controller 的关系

它只决定实例“是否存在”。副本由 Orchestrator，QPS 由 Traffic，Deployment 由 SimulatorInstance Controller，性能由 Simulator。这个边界防止 policy 变化意外重置运行状态。

## 4. SimulationClock Controller

### 输入与 Watch

- Primary：集群唯一的 `SimulationClock/default`；对象缺失时以 1x 自动创建。
- Watch：SimulationClock generation，以及 SimulatorInstance 创建、删除或 `spec.timeScale` 变化。
- 不监听 Instance 的副本、QPS 或高频 Status，避免每个 Tick 触发全量扇出。

### 收敛

1. 防御性检查 `spec.rate` 在 1..20；CRD 同时执行 schema 校验。
2. 列出非删除中的全部 SimulatorInstance。
3. 对每个实例使用冲突重试和字段级 Patch，只修改 `spec.timeScale`。
4. 写回 observedGeneration、appliedRate、同步/总实例数和 Ready Condition。
5. 任一实例同步失败时 Ready=False 并返回错误，由 controller-runtime 退避重试。

倍速字段不进入 Deployment Pod template，因此更新不会滚动重启 Simulator。运行中的 Simulator 在下一个真实 Tick GET Instance 时读取新值。Status Ready 只证明 Kubernetes 字段收敛；运行时应用由 Simulator 指标/E2E 继续验证。

## 5. SimulatorInstance Controller

### 输入与 Watch

- Primary：SimulatorInstance。
- Owns：Deployment。
- 读取：Tenant、Model、WorkerNode、TenantNodePolicy、ModelNodePolicy、Deployment、同 Tenant 的实例。
- Watch：Deployment 状态，节点策略 generation。

### 节点候选算法

1. TenantNodePolicy：显式 Allow 构成基础集合，Deny 删除。
2. ModelNodePolicy：Deny 删除；若该 Model 存在任何 Allow，则再与其 Allow 集合相交；否则不额外收窄。
3. 结果为空时设置不可能匹配的 node affinity 值，保证 Pod Pending，而不是调度到未授权节点。

### 输出资源

在配置 namespace 创建/修补 `simulator-<instance>` Deployment：

- replicas = instance.spec.replicas。
- requiredDuringScheduling node affinity。
- Simulator image/pull policy/ServiceAccount。
- instance/tenant/model 环境变量和 identity labels/annotations。
- metrics/health 9090、readiness/liveness。
- non-root、安全上下文、资源配置、rolling update。
- OwnerReference 指向 cluster-scoped SimulatorInstance。

### 修改字段

- `SimulatorInstance.status.availableReplicas`
- `SimulatorInstance.status.phase`
- `SimulatorInstance.status.conditions`（Ready）
- **不写** score/performance/observedAt/reporterID/effectiveScore。

### TenantRuntime

确保每 Tenant 一个 TenantRuntime，并汇总同 Tenant Deployments 的 available replicas 到 `status.instanceCount`。phase 大致为：出现失败 -> Failed；期望尚未可用 -> Pending；可用/稳定 -> Running；空/不可判定 -> Unknown（以实现为准）。

### 删除

finalizer 先删除 Deployment、等待清理，再重算 TenantRuntime，最后移除 finalizer。这样 cluster-scoped CR 删除不会留下 namespaced 工作负载。

## 6. Traffic Controller

### 输入与 Watch

- Primary：Tenant。
- 读取：同 Tenant 的 SimulatorInstances。
- Watch：Instance 的 score、observedAt、phase、replicas、删除等相关变化。
- 周期：约 10 秒兜底。

### 样本资格

实例必须：不在删除、`replicas>0`、Phase Running、observedAt 在当前时间前后约 30 秒、score>0，才作为正权重。

### 分配算法

- Tenant.qps=0 -> 所有实例 qps=0。
- 所有有效 score 都为 0/无有效分数 -> 在可运行候选之间等权分配。
- 有正分数 -> 按 score 比例，零分实例分配 0。
- 使用 Largest Remainder 把浮点比例转为整数；稳定排序打破余数相同。
- 分配总和严格等于 Tenant.spec.qps。

### 输出字段

只写 `SimulatorInstance.spec.traffic.qps`。不写 Tenant、replicas、Status。

### 关系

Simulator 读取分配 QPS；性能变化更新 score 后再次触发 Traffic，因此形成反馈。Traffic 不直接看 Prometheus，CR Status 是控制输入。

## 7. PerformanceCollector Controller

### 输入与 Watch

- Primary：Tenant。
- 读取：同 Tenant SimulatorInstances、TenantPerformance。
- Watch：Instance 的 status/spec 相关变化和 TenantPerformance generation。
- 周期：约 10 秒。

### 有效样本

- Instance Phase Running。
- availableReplicas > 0。
- observedAt 在约 30 秒新鲜窗口。
- performance 中对应 TTFT/Queue 非空。

TTFT 与 Queue 分别收集，不能因一个指标缺失伪造另一个。

### 聚合

- 每个实例按 availableReplicas 加权。
- 使用基于加权中位数偏差的稳健均值，降低异常实例对总体结果的支配；不是简单算术平均。
- observedAt 取纳入样本的最新时间，sampleCount 是实例样本数。

### 输出

- 确保同名 TenantPerformance，Spec tenantRef，OwnerReference Tenant。
- 写 avgTTFT(ms)、avgQueue(requests)、observedAt、sampleCount、Phase Running/Stale、MetricsReady Condition。
- 没有新鲜样本 -> Stale，不把旧值标为 Running。

### 关系

Orchestrator 只对 Running TenantPerformance 做决策。该层把多实例噪声与扩缩容逻辑隔离。

## 8. WorkerNodeUsage Controller

### 输入与 Watch

- Primary：WorkerNode。
- 读取：Pod、Model。
- Watch：Pod 调度/阶段变化、Model generation。

### 算法

筛选已调度到 WorkerNode 同名节点、非终态、具有平台 Simulator identity labels/annotations 的 Pod；根据其 Model：

- usedGPU += model.spec.gpuUnits。
- usedConcurrency += model.spec.maxConcurrency。

它按 Pod 计数，因此 Deployment 副本天然重复累加；Succeeded/Failed Pod 不再占用。

### 输出

写 WorkerNode.status.usedGPU、usedConcurrency、UsageReady Condition，并更新 Prometheus gauges。

### 限制

这是业务容量推算，不读取 NVIDIA device plugin 或真实 GPU 利用率。Pod 已调度但尚未 Ready 仍可能计入资源占用，符合调度 reservation 视角。

## 9. Orchestrator Controller

### 输入与 Watch

- Primary：Orchestrator；`MaxConcurrentReconciles=1`。
- 读取：Tenant、唯一 TenantPerformance、同 Tenant SimulatorInstances、Model、WorkerNode、三类 Policy。
- Watch：TenantPerformance、SimulatorInstance、WorkerNode；Tenant/Policy/Model generation。
- 周期：不超过约 10 秒兜底。

### 前置条件

- Orchestrator 对应 Tenant 存在。
- 同 Tenant 恰有一个 Orchestrator 和一个 TenantPerformance。
- TenantPerformance Phase Running。
- Model `spec.absoluteScore` 有效，且 WorkerNode 剩余容量足以选择扩容目标。

### 决策规则

1. 计算 floor/ceiling：min/max；qps=0 且 allowScaleToZero 可 floor=0。
2. 当前总副本低于 floor -> 扩容。
3. TTFT > up **或** Queue > up -> 扩容。
4. qps=0 -> 倾向缩容到允许的 floor。
5. TTFT < down **且** Queue < down -> 缩容。
6. 其余 no-op。
7. 扩/缩分别检查独立 cooldown。
8. 扩容决策按 `maxScaleUpBatch` 步长补副本（默认 10，0=默认）：队列缺口大时按缺口换算后截断到步长，配合 cooldown 形成批次节奏。

### 放置与分数

- Policy 过滤合法 Model/WorkerNode 组合。
- 剩余容量 = WorkerNode Spec - Status used。
- GPU 和 concurrency 是硬门；不足则不可选。
- effectiveScore = Model.spec.absoluteScore × cold start weight，weight 下限约 0.7。
- Model 分数更新后，Orchestrator 会刷新已有副本的 effectiveScore；休眠实例首次初始化仍要求存在可行节点。
- 扩容选择高分且可容纳目标；缩容优先副本较多、effectiveScore 较低的实例，保持确定性排序。
- 旧 Model 同时缺少 Spec 与旧 Status 分数时，决策原因为 `model_absolute_score_missing`，Orchestrator Ready Condition 为 `ModelScoreMissing`；不再伪装成容量不足。

### 输出字段/资源

- `SimulatorInstance.spec.replicas`
- 扩容时 `SimulatorInstance.status.effectiveScore`
- `Orchestrator.status.lastScaling/lastScaleUpTime/lastScaleDownTime/conditions`
- 实例 annotations：trigger hash、`platform.study.com/pending-scale-plan`

### 可恢复执行

Controller 先将 pending plan JSON 与 replica 更新持久化，再完成 effectiveScore/Orchestrator Status，最后清理 annotation。下次 Reconcile 先恢复 pending plan。trigger SHA256 由输入版本、配置、资源等确定，避免同一状态重复执行。

### 关系

Orchestrator 只决定副本，不创建 Deployment；SimulatorInstance Controller 把 replicas 收敛为 Deployment。新 Pod 冷启动由 Simulator 反映到 score，Traffic 再调整流量，形成闭环。

## 10. 关系矩阵

| Controller | 输入 CR | 输出资源 | 写的核心字段 | 下游 |
| --- | --- | --- | --- | --- |
| TenantModelPolicy | Policy/Tenant/Model | SimulatorInstance | refs/initial Spec、Policy Condition | Instance Controller/Orchestrator |
| SimulationClock | Clock/Instance 生命周期 | SimulatorInstance、Clock Status | spec.timeScale、同步 Condition | Simulator/Frontend |
| SimulatorInstance | Instance/Policies/Nodes | Deployment、TenantRuntime | availableReplicas/phase/Ready | Simulator、Dashboard、Orchestrator |
| Traffic | Tenant/Instance Score | Instance | spec.traffic.qps | Simulator |
| Performance | Instance Performance | TenantPerformance | avg metrics/phase | Orchestrator |
| WorkerUsage | Pod/Model | WorkerNode Status | used GPU/concurrency | Orchestrator |
| Orchestrator | TenantPerformance/Policy/Capacity | Instance/Orchestrator Status | replicas/effectiveScore/scaling | Instance Controller/Simulator |

## 11. 故障原则

- 缺依赖：Condition + requeue，不创建越权资源。
- Status 冲突：重新读取/patch，不用全对象 Update 覆盖他人字段。
- 无候选节点：Pod 保持不可调度并可诊断，不绕过 Policy。
- 样本过期：Traffic/Performance 忽略；不把最后值永久当真。
- 遥测失败：记录但不阻断核心 Reconcile。
- 删除：finalizer 必须能重试；不能因临时 API 错误直接放弃清理。
