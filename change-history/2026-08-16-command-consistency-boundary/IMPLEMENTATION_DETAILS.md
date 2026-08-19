# 实现修改明细

## 1. 改动前状态

- 批量 apply 真实写入阶段第 N 个资源失败时直接返回错误，前 N-1 个已生效的资源对客户端不可见。
- `CompleteIdempotency` 写失败时记录留在 pending，相同 key 后续请求持续返回 `COMMAND_IN_PROGRESS`，直到 24 小时保留期过期。
- 审计使用原请求 context，客户端断开/超时可能导致 Kubernetes 已写入但 audit_log 缺记录。

## 2. 修改

- `dashboard/backend/internal/model/types.go`：`OperationResourceResult` 增加 `Error string (json:"error,omitempty")`。
- `dashboard/backend/internal/api/handlers_command.go`：
  - 新增 `resourceApplier` 接口与 `applyConfigurationBatch`，批量按序应用，遇错停止并返回"已成功结果 + 失败明细"；
  - `handleApplyConfiguration` 在失败时返回 `state=partial`、`meta.partial=true` 与 warnings；
  - `recordAudit` 使用 `context.WithTimeout(context.WithoutCancel(...), 5s)` 独立超时写审计。
- `dashboard/backend/internal/api/idempotency.go`：`CompleteIdempotency` 失败时调用 `ReleaseIdempotency` 释放占位，两次错误均记日志。
- `dashboard/frontend/my-app/src/api/endpoints/configApi.ts`、`controlPlaneApi.ts`：`OperationReceipt.results` 类型增加 `error?: string`。
- `docs/backend/API_DESIGN.md`：补充批量应用 partial 契约、幂等释放与审计超时说明。

## 3. 未做

- 未实现 OperationID 查询接口与后台收敛状态机（`convergence: pending` 仍无查询入口）。
- 未做跨 Kubernetes/PostgreSQL 强事务（分布式操作显式建模为 partial 契约）。
- 未改变 delete / 单资源接口的失败语义。
