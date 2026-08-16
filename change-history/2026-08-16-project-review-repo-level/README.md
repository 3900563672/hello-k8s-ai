# Project Review 看板迁移到仓库级

- 变更日期：2026-08-16
- 关联问题：无（用户要求看板出现在仓库 Projects 页面）
- 变更级别：P1 工程基建
- 变更范围：GitHub Project v2（新建仓库级 Project Review、删除误建的用户级项目）、`docs/agents/PROJECT_REVIEW.md`
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

- 之前误建为用户级项目（`users/3900563672/projects/1`），用户希望从仓库页面 `github.com/3900563672/hello-k8s-ai/projects` 直接看到。
- 通过 GraphQL `createProjectV2(ownerId + repositoryId)` 创建**仓库级** Project Review（number 2），8 张卡片（issue #15–22）全部迁移并置为 `To do`，旧用户级项目已删除。
- `docs/agents/PROJECT_REVIEW.md` 更新：看板 URL、关键 ID 表（Project / Status 字段 / 五个选项）、命令速查从 gh CLI 改为 GraphQL（gh CLI 的 project 命令不支持仓库级项目）。

## 2. 关键行为

- 看板操作统一走 GraphQL：`addProjectV2ItemById` 加卡片、`updateProjectV2ItemFieldValue` 设状态。
- 查看板入口：仓库 Projects 页面；API 地址仍为 `users/.../projects/2`，属 GitHub 展示方式。
- 归档流程不变：issue 关闭 + 状态置 `Done`。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| GitHub Project v2 | 仓库级 Project Review（number 2）；旧用户级项目删除 |
| docs/agents/PROJECT_REVIEW.md | URL、ID 表、GraphQL 命令速查 |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 实测结论与停止线

- 仓库级项目创建、字段五态、8 张卡片迁移、状态设置全部实测成功；旧项目删除成功。
- 停止线：看板位置与命令已对齐；仍未开始自动化执行（等用户放行）。
