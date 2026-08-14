# 1. 问题标题

流量分配总量不变量没有端到端闭环

## 2. 当前状态描述

`internal/controller/traffic_controller.go` 负责把 `Tenant.spec.qps` 按 SimulatorInstance 的实时 Score 分配到各实例的 `spec.traffic.qps`。分配算法使用 Largest Remainder 方法，纯函数层面可以保证参与分配的实例之和等于请求 QPS。

问题出现在参与集合和写回阶段。`collectTrafficInstances` 会跳过 `spec.replicas == 0` 的实例，`distributeTrafficForTenant` 只更新收集到的实例。如果一个实例在有流量时曾得到 QPS，随后被 Orchestrator 缩到零，它会被下一次分配排除，旧的 `spec.traffic.qps` 不会被清零。当所有实例都为零时，Controller 直接按 no_instances 返回，同样不会清理旧值。

Backend 的 `dashboard/backend/internal/readmodel/aggregator.go` 会把租户下所有 SimulatorInstance 的 `spec.traffic.qps` 相加，不会排除零副本实例。因此 stale QPS 会让 `allocatedQPS` 大于 `requestedQPS`，`allocationBalanced` 变为 false；历史 Snapshot 也会保存这个不一致切面。

对多个活跃实例的写回是逐项 Update。任一 Update 失败后，Controller 会继续更新其他实例并最终返回聚合错误。在下一次 Reconcile 前，集群会处于部分新分配、部分旧分配的可见状态。

Frontend 还有另一层断点：Backend 已提供 `PATCH /api/v1/tenants/{name}/traffic`，但 `trafficApi.ts` 只有读取方法。`TrafficPage.tsx` 的“应用”操作调用 Zustand `addOverlay`，模板和 Overlay 明确只保存在内存中。这对预览工具是合理的，但当前交互没有把预览与真实流量变更形成清晰、可执行的闭环。

## 3. 问题定位

算法保证的是“本轮活跃集合的分配和”，系统展示和 Simulator 消费的却是“所有实例当前保存的 QPS”。参与集合变化时没有先处理退出集合，导致局部正确不等于全局正确。

部分更新期间的短暂不一致本身可以通过最终一致性接受，但当前没有分配版本、期望总量或阶段状态，Simulator 和 Backend 只能读取每个实例的独立值，无法区分完整分配与半完成分配。

Frontend 工作台使用“应用”语义，却只创建本地 Overlay。刷新页面后状态消失，也不会触发 Tenant QPS 或 Controller 分配。若产品定位只是沙盘预览，应在契约和界面上明确；若定位是流量控制，则功能链路尚未完成。

## 4. 影响范围

- Traffic Controller：缩容、缩到零和写回失败时无法持续保证总量不变量。
- Simulator：零副本期间不产生性能数据，但实例保存的 assigned QPS 仍可能是旧值；恢复副本后会短暂消费旧流量。
- Backend：聚合值和 `allocationBalanced` 会暴露错误总量，并将其写入 PostgreSQL Snapshot。
- Frontend：模板“应用”不是后端命令，用户可能把预览误认为真实变更。
- Orchestrator：扩缩容改变参与集合，直接触发该问题。
- 测试：现有测试主要验证分配函数，缺少“实例从 1 缩到 0”“全部缩到 0”“中途 Update 失败”和 UI 到 PATCH 的链路测试。

代码可以确定 stale QPS 路径存在；归档的单样例成功日志未覆盖多实例缩容，因此不能据此判断线上已经产生错误总量。

## 5. 根本原因分析

根本原因是流量分配被实现为多个实例字段的独立写入，却没有定义租户级分配快照。Controller 把“不参与分配”理解成“不更新”，而业务不变量实际要求退出集合的值归零。

Frontend 的原因则是预览工作区和控制面命令在迭代中并行存在：远端查询使用 TanStack Query，本地模板使用 Zustand，这一分层本身合理，但缺少从确认预览到提交真实 Tenant QPS 的产品契约。

## 6. 修改方向建议

- 每次分配都以租户下完整实例集合为边界，对退出活跃集合的实例显式归零。
- 明确部分写入期间的可见语义；可引入租户级分配版本、期望总量或收敛 Condition，使消费者能判断一组值是否属于同一轮。
- 保留现有 Largest Remainder 算法，重点修复集合变化和失败恢复，不需要替换算法。
- 明确 Frontend 工作台是“只读沙盘”还是“控制入口”。若是控制入口，应在用户确认后调用现有命令 API，并处理 resourceVersion、Idempotency-Key、回执和收敛状态；若只是沙盘，应避免使用会暗示已生效的文案。
- 增加跨层测试，验证扩缩容后 `requestedQPS == sum(all instance QPS)`，并覆盖 Backend 展示和页面刷新行为。

## 7. 优先级

优先级：P1

建议在多模型分流、频繁缩到零或把流量工作台开放给用户前处理。
