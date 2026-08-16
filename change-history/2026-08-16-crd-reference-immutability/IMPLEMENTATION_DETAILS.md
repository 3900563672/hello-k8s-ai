# 实现修改明细

## 1. 改动前状态

- `TenantModelPolicySpec` / `TenantNodePolicySpec` 的引用字段只有 `MinLength=1` 非空校验。
- Controller 依赖隐含不变量（如一租户一 Orchestrator），但引用本身可被运行中修改，造成派生资源与引用不一致。

## 2. 修改

- `api/v1/tenantmodelpolicy_types.go`：`TenantRef`、`ModelRef` 增加 `+kubebuilder:validation:XValidation:rule="!has(oldSelf) || self.name == oldSelf.name"`，message 提示"租户/模型引用不可变，变更请删除重建"。
- `api/v1/tenantnodepolicy_types.go`：`TenantRef`、`NodeRef` 同样处理。
- 规则用 `!has(oldSelf) ||` 短路：CEL 的 `oldSelf` 在创建对象时为 null，直接 `oldSelf.name` 会让创建请求被拒；更新时 oldSelf 为对象，比较引用是否变化。
- 重新生成：`make manifests generate YEAR=2026`（controller-gen v0.21.0），仅 2 个 CRD 清单 + 2 个类型文件变化。

## 3. 未做

- 未把"一租户一 Orchestrator / TenantPerformance / SimulatorInstance"不变量 admission 化（需要 webhook，改动面大）。
- 未做依赖删除清理矩阵测试（后续单独评估）。
- 未改 SimulatorInstance 的 tenantRef/modelRef（可能存在合法迁移场景，需先确认契约）。
