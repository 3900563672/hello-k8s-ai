# 统一 Backend 命令幂等、批量应用与审计的一致性边界

- 变更日期：2026-08-16
- 关联问题：Fixes #17（Project Review issue-05）
- 变更级别：P1 一致性边界
- 变更范围：`dashboard/backend/internal/api`（批量应用、幂等、审计）、`internal/model`、前端类型同步
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

批量配置应用在真实写入阶段中途失败时，不再伪装成整体失败：返回 `state=partial` 的成功 envelope（`meta.partial=true`），`results` 同时包含已成功项与失败项明细（`convergence=failed` + `error`）。

幂等记录完成写失败时立即释放占位，同一 `Idempotency-Key` 不再被 pending 卡满 24 小时保留期；重放依赖 Kubernetes apply 幂等语义。

审计持久化改用独立于请求生命周期的超时上下文，客户端断开不再静默丢审计。

## 2. 关键行为

- dry-run 阶段失败仍整体拒绝（此时没有任何资源被写入）。
- 批量真实写入按序执行，遇错停止并返回 partial 契约；停止线：不做跨系统强事务与后台收敛状态机。
- 完成写失败 → 释放占位 + Warning 日志；最坏情况是客户端重试重复执行幂等 apply。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| Backend API | 批量 apply 的 partial 契约；幂等完成失败自动释放；审计独立超时 |
| API 契约 | `OperationResourceResult` 增加可选 `error` 字段 |
| 前端 | TS 类型同步 `error?: string`，消费逻辑不变 |
| 测试 | 新增完成写失败释放、批量中途失败、批量全成功三个用例 |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 实测结论与停止线

- `go test ./internal/api/` 相关用例全绿；`go vet ./...` 通过。
- 停止线：不引入 OperationID 查询接口与后台恢复状态机，不尝试跨 Kubernetes/PostgreSQL 强事务。
