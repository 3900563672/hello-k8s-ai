# 升级与回滚

## 1. 迁移

- 无代码、CRD、数据库变化，不存在数据迁移。
- 看板为外部状态（GitHub Project v2），随仓库提交同步演进，文档已记录命令速查。

## 2. 回滚

- 文档回滚：`git revert <本提交>` 即可恢复 docs/agents/ 与 project-review/README.md 的改动。
- 看板回滚（如需删除）：`gh project delete 1 --owner 3900563672`。
- 字段回滚（恢复默认三态）：GraphQL `updateProjectV2Field` 将 Status 选项改回 Todo / In Progress / Done。
- 权限回滚：无（token scope 只增不减，不影响既有功能）。

## 3. 风险与注意

- `project` scope 提升的是 GitHub Projects 读写权限，不涉及仓库代码权限（repo scope 原本已有）。
- 看板删除后不可恢复，删除前需确认没有未归档的卡片。
- 当前看板为空（0 items），第一批导入前处于安全状态。
