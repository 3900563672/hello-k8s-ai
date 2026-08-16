# 全库重复代码抽取重构（不改变业务行为）

- 变更日期：2026-08-16
- 关联问题：无（用户直接要求的代码质量整理）
- 变更级别：P2 代码质量
- 变更范围：控制器（`internal/controller`）、Simulator、Dashboard Backend Provider、Dashboard Frontend 配置页
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

把四块跨文件重复的代码收敛为共享实现，行为与 UI 完全不变：

- 控制器 9 处「Get → DeepCopy → 改 → Patch」状态更新收敛为 `internal/k8sutil.PatchWithRetry / PatchStatusWithRetry`，冲突重试统一走 client-go 默认退避。
- Simulator 的 `updateOwnedStatus` 复用同一 helper，删除第三份手写重试副本。
- Prometheus / Jaeger 两个 Provider 的 HTTP 客户端构造、URL 解析拼接、JSON GET 与错误文案收敛为 `dashboard/backend/internal/providers/httputil`。
- 前端 5 个配置表单的提交 / 模板 / 脏状态脚手架收敛为 `useConfigForm`；18 个数字输入收敛为 `ConfigNumberField`；引用型下拉收敛为 `ConfigRefSelect`；显示名称输入收敛为 `ConfigTextField`；5 个配置表的名称单元格收敛为 `NameCell` + 共享 `formatNumber`。

## 2. 关键行为

- 不改变任何业务逻辑、错误文案、默认值与校验规则。
- 唯一细微差异：patch helper 检测到对象没有任何实际变化时不再发 API（幂等优化）；Simulator 每 Tick 都会推进 ObservedAt，实际写入频率不变。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| internal/k8sutil | 新增共享 patch helper（RetryOnConflict / PatchWithRetry / PatchStatusWithRetry） |
| internal/controller | 9 处调用点改用共享 helper，删除本地副本 |
| simulator | updateOwnedStatus 改用共享 helper |
| dashboard/backend/internal/providers/httputil | 新增共享 HTTP 客户端 / URL / JSON GET 工具 |
| dashboard/backend/internal/providers | Prometheus、Jaeger 改用 httputil，删除本地重复实现 |
| dashboard/frontend/my-app | 配置表单与表格收敛到共享组件 / hook |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 实测结论与停止线

- 根模块 `go vet ./...`、`go test ./... -count=1` 全绿；Backend 模块 `go vet ./...`、`go test ./... -skip Grafana -count=1` 全绿；前端 `npm run check`（lint + 构建 + 状态校验）通过。
- 停止线：本轮只抽取重复代码，不做行为与视觉调整；Canvas 绘制（DrawCanvas / PreviewCanvas / PreviewCurve）与 timeline 数学因语义差异较大未抽取，保留原状。
