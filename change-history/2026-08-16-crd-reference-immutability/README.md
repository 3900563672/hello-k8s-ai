# 策略 CRD 引用不可变校验

- 变更日期：2026-08-16
- 关联问题：Fixes #16（Project Review issue-04，第一步）
- 变更级别：P1 CRD 契约
- 变更范围：`api/v1/tenantmodelpolicy_types.go`、`api/v1/tenantnodepolicy_types.go`、生成的 CRD 清单
- CRD 变化：新增 x-kubernetes-validations（引用不可变）
- 数据库变化：无

## 1. 完成结果

审查指出身份型引用（TenantModelPolicy / TenantNodePolicy 的 tenantRef / modelRef / nodeRef）只有非空校验，运行中可被修改，容易产生派生资源与引用不一致的孤儿状态。

本次用 CRD 原生 CEL 校验（`self.name == oldSelf.name`）把四个身份型引用标记为**不可变**：修改引用会被 API Server 拒绝（message：`xx引用不可变，变更请删除重建`），避免运行中改引用导致派生资源混乱。其余不变量（一租户一 Orchestrator、唯一性等）保持 Controller 现有检查，不扩大本轮改动。

## 2. 关键行为

- TenantModelPolicy：`spec.tenantRef.name`、`spec.modelRef.name` 不可变。
- TenantNodePolicy：`spec.tenantRef.name`、`spec.nodeRef.name` 不可变。
- 变更引用需删除重建（保留现有删除/清理链路）。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| api/v1 | 两个 policy 类型增加 XValidation 标记 |
| config/crd/bases | 两个 CRD 清单增加 x-kubernetes-validations |
| 控制器 | 无代码变化（校验由 API Server admission 执行） |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 实测结论与停止线

- `make manifests generate YEAR=2026` 生成差异仅含预期 4 个文件；CRD 中 x-kubernetes-validations 内容核对无误。
- 停止线：本轮只做引用不可变；"一租户一 Orchestrator" admission 化、依赖删除清理矩阵测试留待后续单独评估。
