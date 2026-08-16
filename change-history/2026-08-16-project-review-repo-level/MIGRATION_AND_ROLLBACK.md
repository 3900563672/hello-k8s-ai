# 升级与回滚

## 1. 迁移

- 无代码、CRD、数据库变化。
- 8 张卡片从旧用户级项目迁移到仓库级项目；issue 内容不变。

## 2. 回滚

- 文档回滚：`git revert <本提交>`。
- 看板回滚：如需恢复用户级项目，用 `createProjectV2(ownerId: 用户ID)` 重建并重新 `addProjectV2ItemById`。
- 已删除的用户级项目无法恢复（GitHub 不提供项目回收站），但 issue 与卡片内容均在仓库级项目与 GitHub issue 中。

## 3. 风险与注意

- 仓库级项目的 API 地址显示为 `users/.../projects/2`，网页入口在仓库 Projects 页面，两者一致指向同一项目。
- 项目 ID 与选项 ID 若项目重建会变化，文档已注明"以实际查询为准"。
