# 项目十大待优化问题

## 1. Orchestrator 选点结果没有约束实际 Pod 调度

严重程度：P0 / 严重

影响范围：资源调度正确性、容量控制、扩缩容稳定性

涉及模块：Orchestrator Controller、SimulatorInstance Controller、WorkerNodeUsage、Kubernetes Scheduler

一句话描述：调度算法选出了具体 WorkerNode，但节点信息在后续决策和执行中丢失，Pod 只被限制到“任一可用节点”。

## 2. Backend 写接口缺少可信的认证与授权边界

严重程度：P0 / 严重

影响范围：集群配置安全、租户隔离、审计可信度

涉及模块：Dashboard Backend、HTTP API、RBAC、Frontend、Kubernetes

一句话描述：任何能访问 Backend 的调用方都可使用其 ClusterRole 修改集群级业务 CR，审计身份还可由请求头直接伪造。

## 3. Model 绝对分没有系统内生产者（已处理）

严重程度：P0 / 严重

影响范围：新模型接入、首次扩容、调度闭环

涉及模块：Model CRD、Orchestrator Controller、Backend、部署脚本

一句话描述：2026-08-14 已将权威字段迁入必填的 `Model.spec.absoluteScore`，并补齐 Backend、Frontend、迁移兼容和测试。

## 4. CRD 引用关系缺少生命周期与唯一性约束

严重程度：P1 / 高

影响范围：Controller 收敛、资源清理、状态一致性、API 演进

涉及模块：全部关系型 CRD、TenantModelPolicy Controller、Orchestrator、SimulatorInstance

一句话描述：可变的名称引用、未约束的“一租户一个资源”假设和删除后的 NotFound 路径可能留下旧实例或使 Controller 持续报错。

## 5. Backend 命令的幂等、批量应用与审计不是同一一致性边界

严重程度：P1 / 高

影响范围：配置写入、失败恢复、操作追踪、客户端重试

涉及模块：Backend API、Kubernetes Gateway、PostgreSQL

一句话描述：Kubernetes 已部分写入而幂等或审计记录未完成时，客户端无法可靠判断和恢复操作结果。

## 6. 流量分配总量不变量没有端到端闭环

严重程度：P1 / 高

影响范围：Simulator 输入、Backend 聚合、Frontend 展示、扩缩容联动

涉及模块：Traffic Controller、SimulatorInstance、Backend Read Model、Frontend 流量工作台

一句话描述：缩到零的实例会保留旧 QPS，批量更新可能部分成功，Frontend 模板操作也只停留在内存预览。

## 7. Simulator 状态生命周期与 reporter Pod 生命周期错误耦合

严重程度：P1 / 高

影响范围：冷启动模拟、TTFT、队列、扩缩容反馈

涉及模块：Simulator、Leader election、SimulatorInstance Controller、Orchestrator

一句话描述：Leader 切换会重置整个实例池的冷启动和队列状态，而新增副本又不会获得独立冷启动状态。

## 8. Controller 事件映射存在集群级放大效应

严重程度：P1 / 高

影响范围：API Server 压力、Controller 延迟、规模扩展

涉及模块：WorkerNodeUsage、Orchestrator、TenantModelPolicy、Controller Runtime Cache

一句话描述：单个 Pod 或 Model 事件可能触发全部 WorkerNode 重算，每次重算又扫描全部 Pod 和 Model。

## 9. 历史回放与可观测性数据无法形成一致时间切面

严重程度：P1 / 高

影响范围：故障复盘、历史页面、指标与 Trace 保留、审计完整性

涉及模块：PostgreSQL、Recorder、Prometheus、OpenTelemetry、Jaeger、Backend Aggregation

一句话描述：Kubernetes Snapshot 可持久化，但指标和 Trace 使用短期易失存储，历史页面因此无法稳定重建同一时刻的完整视图。

## 10. CI 与交付基线无法证明完整系统可重复发布

严重程度：P1 / 高

影响范围：回归防护、发布可信度、多人协作、环境复现

涉及模块：GitHub Actions、Go 模块、Frontend、E2E、Docker、Kustomize、源码归档

一句话描述：CI 主要覆盖根 Go 模块，未验证 Backend、Frontend、四镜像和完整链路，当前归档本身也不是干净 Git 基线。
