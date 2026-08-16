# 实现修改明细

## 1. 改动前状态

- Project Review 建在用户级（`users/3900563672/projects/1`），用户期望出现在仓库 Projects 页面（`github.com/3900563672/hello-k8s-ai/projects`）。
- gh CLI 2.46 的 `project` 命令不支持仓库级 owner（报 `unknown owner type`），原文档命令速查基于 gh CLI。

## 2. 实现

- 关键 ID：仓库 `R_kgDOT3WkFA`、用户 `U_kgDODN0KGA`。
- 创建：`createProjectV2(input: { ownerId: 用户ID, repositoryId: 仓库ID, title: "Project Review" })` → 项目 ID `PVT_kwHODN0KGM4BgfyL`（number 2）。
- 字段：新项目默认 Status（Todo/In Progress/Done），用 `updateProjectV2Field` 更新为五态（To do / In review / Approved / In progress / Done），选项 ID 已写入 `docs/agents/PROJECT_REVIEW.md`。
- 迁移：`addProjectV2ItemById` × 8（issue #15–22）+ `updateProjectV2ItemFieldValue` 置 To do。
- 清理：`deleteProjectV2` 删除旧用户级项目（仅含迁移前的卡片副本，issue 本体不受影响）。
- 文档：`docs/agents/PROJECT_REVIEW.md` 头部时间戳、mermaid 标注仓库级、第 6 节命令速查改为 GraphQL 并附 ID 表。

## 3. 未做

- 未开始自动化执行任何条目。
- 未改动 8 个 issue 内容。
