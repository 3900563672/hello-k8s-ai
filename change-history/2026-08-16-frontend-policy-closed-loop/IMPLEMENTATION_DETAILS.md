# 实现细节：策略管理前端

## 新增文件

| 文件 | 说明 |
| --- | --- |
| src/lib/validations/policy.schema.ts | 三类策略的引用必填校验（zod superRefine）与预览 |
| src/components/features/config/tables/PolicyTable.tsx | 策略列表（引用关系 / 类型 / 效果） |
| src/components/features/config/forms/PolicyForm.tsx | 编辑表单（按类型渲染租户/模型/节点选择） |
| src/components/shared/dialogs/PolicyCreateDialog.tsx | 新建对话框（类型 + 引用 + 效果 + 标识预览） |

## 修改文件

- src/types/config.types.ts：Policy / PolicySpec / PolicyKind / PolicyEffect 类型；
  `ConfigurationReadModel.policies` 沿用 Backend 已有的 `tenantModel/tenantNode/modelNode` 结构。
- src/api/endpoints/configApi.ts：`getPolicies`（合并三类）、`createPolicy/updatePolicy/deletePolicy`
  （kind 映射到 `TenantModelPolicy/TenantNodePolicy/ModelNodePolicy`，均已在 Backend 可写白名单）。
- src/api/queries/configQueries.ts：`usePolicies` 与三个 mutation hook。
- src/components/features/config/ConfigPage.tsx：新增「策略」页签、选择/批量删除、
  新建对话框接线；总资源数、加载与错误态同步纳入策略。

## 关键设计

- 策略没有 displayName 字段，`displayName` 用引用关系生成（如 `tenant-sample → model-sample`），
  满足既有 ConfigTable/ConfigTabPanel 的通用约束。
- ModelNodePolicy 无 tenantRef，TenantModelPolicy/TenantNodePolicy 无 nodeRef/modelRef；
  请求体按类型只携带合法字段，避免触发 Backend 白名单校验失败。
- 新建时系统标识按 `tenant-model` / `tenant-node` / `model-node` 生成并保证唯一
  （同名追加 `-2`），与 Controller 的实例命名约定一致。
- 未改动 Backend 与 Controller：读写白名单、读模型（`/configuration` 的 `policies`）均已就绪。
