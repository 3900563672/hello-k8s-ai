# 1. 问题标题

Backend 命令的幂等、批量应用与审计不是同一一致性边界

## 2. 当前状态描述

Backend 的写命令通过 `dashboard/backend/internal/api/idempotency.go` 强制要求 `Idempotency-Key`。中间件先在 PostgreSQL 创建 pending 记录，再执行 HTTP handler，最后把状态码和响应体写成 completed。这个设计能处理普通的重复提交，也会拒绝同一个 key 对应不同请求。

`dashboard/backend/internal/api/handlers_command.go` 的批量配置接口先对每个资源逐一执行 Kubernetes API Server dry-run，再按顺序执行真实 Apply。dry-run 可以提前发现静态校验错误，但真实写入仍是多个独立 Kubernetes 请求。如果第 N 个请求在实际执行阶段失败，前 N-1 个已经生效，handler 直接返回错误，没有返回已成功项清单，也没有回滚或后续收敛操作。

Kubernetes 变更和 PostgreSQL 幂等记录也不是原子操作。若 Kubernetes 已写成功，但 `CompleteIdempotency` 因数据库超时失败，中间件仍把成功响应发给当前客户端；数据库记录会保持 pending。相同 key 的后续请求会持续收到 `COMMAND_IN_PROGRESS`，直到 24 小时过期后才允许重新占用，此时再次执行可能产生新副作用。

审计写入在每个资源操作后同步调用，但失败只记录日志，不改变命令结果。审计使用原请求 context；连接中断或超时也可能使 Kubernetes 写入成功而 audit_log 缺记录。Backend 返回的 `convergence: pending` 没有对应的操作查询或后台收敛状态机。

## 3. 问题定位

当前实现提供了“请求去重”，但还没有提供“业务操作可恢复”。幂等键的数据库状态、Kubernetes 实际对象、审计记录和客户端回执可能分别处于不同阶段。

批量接口的名称和统一 OperationReceipt 容易让调用方理解为一个整体操作，但实际语义是“预校验后顺序尽力写入”。发生竞争、API Server 短暂故障或 context 超时时，调用方无法仅靠响应判断哪些资源已落地，也没有稳定接口查询最终状态。

这类问题在本地单用户、低并发且依赖稳定时不明显；自动化客户端重试、多 Backend 副本或数据库波动会放大风险。

## 4. 影响范围

- Backend API：批量 apply、delete、Tenant QPS 修改都经过同一幂等边界。
- Kubernetes：可能已经接受变更，但 Backend 把操作保留为 pending 或返回批次失败。
- PostgreSQL：command_idempotency 和 audit_log 不能完整代表实际副作用。
- Frontend/自动化客户端：无法查询 OperationID 的最终状态，也无法安全决定是否重试。
- Controller：可能正常收敛已经写入的部分配置，但用户以为整个批次失败。
- 测试：现有幂等测试覆盖重放、冲突和 pending 响应，未覆盖“Kubernetes 成功、完成记录失败”及批量中途失败。

## 5. 根本原因分析

根本原因不是缺少数据库事务，而是把跨 Kubernetes 和 PostgreSQL 的分布式操作当作一次同步 HTTP 调用完成。两套系统不可能通过普通本地事务实现原子提交，因此必须显式建模中间状态与恢复策略。

当前代码已经引入 OperationID、pending 状态和 convergence 字段，说明架构开始向操作模型演进，但尚未完成持久化操作状态、逐项结果和后台恢复链路。

## 6. 修改方向建议

- 明确批量接口语义：若不能保证原子性，应把逐项成功/失败、可重试性和最终收敛状态作为稳定契约返回并持久化。
- 以 OperationID 建立可查询的操作生命周期，记录每个资源的期望、实际结果和恢复状态，而不只缓存最终 HTTP 响应。
- 让幂等恢复能根据 Kubernetes 当前对象和目标意图判断“已完成、需继续、需人工处理”，避免 pending 只能等待过期。
- 审计记录应来自可信操作状态，并有独立超时、重试或补偿机制；审计持久化失败需要可观测告警。
- 对批量中途失败、数据库完成写失败、客户端断开、双 Backend 并发和 key 过期重放建立故障注入测试。
- 不建议尝试用跨系统强事务改写架构；应采用显式、可恢复的最终一致性模型。

## 7. 优先级

优先级：P1

建议在开放自动化写入、多用户并发或运行多个 Backend 副本前完成，否则重试和审计无法提供可靠保证。
