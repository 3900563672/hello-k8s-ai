# 1. 问题标题

CRD 引用关系缺少生命周期与唯一性约束

## 2. 当前状态描述

项目的业务 CRD 全部是 cluster-scoped，资源之间通过 `ObjectRef.name` 建立关系，例如 TenantModelPolicy 引用 Tenant 和 Model，Orchestrator 引用 Tenant，SimulatorInstance 同时引用 Tenant 和 Model。这些名称字段主要只有非空校验，没有不可变约束、引用存在性校验或组合唯一性校验。

Controller 内部却依赖更强的隐含不变量：

- `internal/controller/orchestrator_data.go` 要求每个 Tenant 恰好有一个 Orchestrator，多于一个就返回错误。
- `internal/controller/orchestrator_controller.go` 要求每个 Tenant 恰好有一个 TenantPerformance，多于一个同样报错。
- `api/v1/simulatorinstance_types.go` 的注释声明一个租户和一个模型只对应一个实例，TenantModelPolicy Controller 通过确定性名称实现这一点，但 CRD admission 并未禁止外部创建冲突或修改引用。
- `internal/controller/tenant_controller.go` 在策略更新时按新 `tenantRef/modelRef` 重算；Controller Runtime 的映射函数接收更新后的对象，没有对旧引用组合执行清理。
- SimulatorInstance、Orchestrator、TenantPerformance 等引用字段变化时，多个事件映射也主要依据新对象，旧租户的聚合状态可能不能立即重算。

Model 删除路径还有一个确定性缺口：TenantModelPolicy Controller 只有在读到“Model 存在且带 deletionTimestamp”时才删除对应 SimulatorInstance；如果 Model 已经 NotFound，它会把情况当作依赖未就绪并周期重试。SimulatorInstance 的 controller owner 是 Tenant 而不是 Model，因此 Kubernetes 垃圾回收也不会因 Model 删除而清理该实例。

## 3. 问题定位

CRD 对外允许的状态空间大于 Controller 能正确收敛的状态空间。用户可以合法提交多个 Orchestrator、多个 TenantPerformance 或修改关系引用，但 Controller 只能在运行时发现部分冲突，且没有统一修复策略。

引用更新尤其危险：资源从旧关系迁移到新关系后，新关系会收敛，旧关系产生的 SimulatorInstance、TenantRuntime 聚合或状态可能继续存在。删除依赖对象时，如果 Controller 只处理 deletionTimestamp 而不处理最终 NotFound，也可能留下永久孤儿。

这些问题通常不会在只创建一次、从不修改引用的样例 YAML 中出现，但会在长期运维、导入错误配置、恢复备份或通过 Backend 编辑资源时出现。

## 4. 影响范围

- CRD：Spec 的合法性和 Controller 实际假设不一致。
- TenantModelPolicy Controller：旧关系可能未被重新计算；Model 已删除后实例可能残留。
- SimulatorInstance Controller：引用变化会影响 Deployment 身份、TenantRuntime 聚合和 OwnerReference 语义。
- Orchestrator：重复资源会使整个租户的决策循环报错；引用迁移可能让旧租户无人管理。
- Backend：批量配置接口允许写入这些可变引用，但无法提前表达跨对象不变量。
- Frontend：可能看到重复、孤儿或长期 Pending 的资源，却缺少明确修复指引。
- 测试：已有 happy path 和部分冲突测试，但未覆盖所有旧引用清理、依赖彻底删除和重复资源组合。

## 5. 根本原因分析

当前数据模型使用 Kubernetes 对象名称模拟关系型约束，但没有在 admission、命名规则和 Controller 收敛逻辑之间选择一个统一的约束层。部分唯一性由确定性名称实现，部分只在 Reconcile 时检查，部分完全依赖操作人员约定。

另一个根因是事件驱动实现偏重“当前对象应该是什么”，对“对象从什么关系迁移而来”关注不足。Kubernetes update 事件同时提供 old/new 对象，但多数映射只消费 new 对象，使关系变更的反向清理没有稳定入口。

## 6. 修改方向建议

- 列出每一种引用字段的契约：是否允许修改、目标不存在时如何处理、删除目标后谁负责清理、是否允许跨租户迁移。
- 对不支持安全迁移的身份型引用使用 CRD 校验保持不可变；需要迁移的引用则必须同时对 old/new 两侧触发收敛。
- 把“一租户一个 Orchestrator”“一租户一个 TenantPerformance”“一租户一模型一个 SimulatorInstance”等不变量放到可执行的命名或 admission 规则中，避免运行后才报错。
- 统一处理 dependency NotFound 与 deletionTimestamp，保证依赖彻底删除后仍能清理派生资源。
- 为关系变更和删除建立矩阵测试，至少覆盖旧引用、新引用、重复资源、OwnerReference 和 Finalizer 交互。
- 先补契约和测试，再决定是否需要版本化 CRD；不需要重新设计全部 CRD。

## 7. 优先级

优先级：P1

建议在开放配置编辑、批量导入或升级 CRD 前处理，否则长期运行后会积累难以解释的孤儿状态。
