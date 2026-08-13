# 设计哲学

## 1. 声明式意图优先于命令式编排

用户表达“租户允许哪些模型、QPS 是多少、扩缩容上下限是什么”，而不是直接创建 Pod、指定某个节点或写性能状态。Controller 持续把声明式意图与实际状态对齐，因此重启、重复事件和短暂故障不会改变业务语义。

为什么重要：调度系统的中间步骤很多。如果页面直接依次调用“建实例、建 Deployment、改流量”，任一步失败都会留下不可解释的半状态；CRD + Reconcile 让每一步都有持久意图和可恢复入口。

## 2. API Server 是 Controller 的消息总线和状态边界

六个 Controller 通过 CR 的 Spec/Status 间接协作，不互相调用。这样每个 Controller 可以独立测试、重启和重放；同时也要求字段所有权非常严格。

代价是最终一致：用户提交后只得到“意图已接受”，不是“所有 Pod 已 Ready”。前端必须展示 desired、observed、新鲜度和 Condition，而不是把 HTTP 200 当作运行完成。

## 3. 一个字段只应有一个语义所有者

`SimulatorInstance` 是协作中心，但它不是共享可写字典：

- Orchestrator 写 `replicas/effectiveScore`。
- Traffic 写 `traffic.qps`。
- SimulatorInstance Controller 写 Deployment 状态。
- Simulator Leader 写性能和运行分数。

为什么重要：controller-runtime 的 Status Patch 仍可能遇到冲突；多个 writer 如果没有字段边界，会互相覆盖、产生振荡和难以复现的故障。

## 4. 派生数据不应变成第二份真相

Backend 为页面计算 free GPU、sample age、SLO 偏差、关联拓扑和来源新鲜度，但不把这些展示派生值写回 CR。PostgreSQL 保存历史快照，也不反向驱动最新对象。

只有当某个派生值需要参与控制决策并具备明确所有者、更新频率、容错语义时，才应升级为 Status 字段或新 CRD。

## 5. 新鲜度是业务语义，不只是 UI 标签

Traffic 与 PerformanceCollector 都拒绝过期 Simulator 样本。当前窗口约为 30 秒，并允许有限未来偏差。没有这个规则，已经失联的实例仍可能收到流量或影响扩缩容。

Dashboard 同样应展示 `observedAt`、查询时间和数据源；“有数值”不代表“可用于决策”。

## 6. 可恢复执行优先于一次性事务幻想

Orchestrator 的扩缩计划涉及 Replica Spec、effectiveScore 和自身 Status，无法形成一个跨对象 ACID 事务。实现通过实例注解持久化 pending scale plan：先保存可恢复意图，再完成各步，最后清理注解。重启后优先恢复 pending plan。

同理，Backend 批量配置虽然先对所有对象 dry-run，但实际写入仍是顺序发生的；文档明确它不是跨对象原子事务。未来若业务要求全有或全无，应设计更高层 Command CR，而不是靠 HTTP 批次承诺不存在的事务。

## 7. Simulator 是受控近似，不是真实模型

Simulator 用固定 token 数、Poisson 到达、带噪声服务时间和并发服务器近似推理负载。它适合验证控制算法和可观测链路，不适合宣称预测真实模型成本或 SLO。

可扩展方向是让模型参数、随机种子、工作负载分布和 SimulationRun 都显式版本化，从而让实验可复现；不能只在前端把时钟乘以倍率。

## 8. 遥测故障不能阻断控制面

OTel 未配置或导出失败时使用 no-op/批处理失败路径，Controller 与 Simulator 仍应工作。Prometheus/Jaeger 对 Dashboard readiness 也属于可选依赖：缺失时返回 partial/warning，而不是让配置读取整体不可用。

这个原则的另一面是：必须为遥测缺失本身提供健康信号，否则“控制面可用”会掩盖“不可诊断”。

## 9. 历史浏览必须诚实

当前历史是定时 JSON snapshot：它能回答“最近一次采集到的状态是什么”，不能回答两个快照之间每个事件的精确顺序，也不能重新驱动随机仿真。`at` 查询无快照时返回不可用，而不是用当前数据假装过去。

真正的回放需要版本化输入、确定性时钟/随机种子、事件日志和快照边界，这是未来单独的架构阶段。
