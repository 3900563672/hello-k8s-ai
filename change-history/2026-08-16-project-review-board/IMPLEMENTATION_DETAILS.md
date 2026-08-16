# 实现修改明细

## 1. 改动前状态

- gh token scopes：`gist, read:org, repo`，缺少 `project`，`gh project list` 报 `missing required scopes [read:project]`。
- GitHub 账号下没有任何 Projects 项目（`gh project list` 为空）。
- 仓库 `project-review/` 已有 10 条静态审查记录（issue-01～10），与线上 Issue / 看板没有关联。
- `docs/agents/` 只有单任务流程（WORKFLOW.md），没有"看板 + 批量闭环"规则。

## 2. 权限

- `gh auth refresh -s project`：OAuth 设备流，用户浏览器确认，token 追加 `project` scope（读写 Projects）。
- 说明：GitHub OAuth 不存在 `write:project` scope（只有 `read:project` 与 `project`），首次尝试 `-s write:project` 报 `invalid_scope`。

## 3. 看板创建与字段

- `gh project create --owner 3900563672 --title "Project Review"` → number 1，url：`https://github.com/users/3900563672/projects/1`。
- 内置 Status 字段（Todo / In Progress / Done）不可删除（`deleteProjectV2Field` 仅限自定义字段），且名称 "Status" 为保留名（`createProjectV2Field` 报错）。
- 最终使用 GraphQL mutation `updateProjectV2Field` + `singleSelectOptions` 更新内置 Status 字段为五态：

| 选项 | 颜色 | 说明 |
| --- | --- | --- |
| To do | GRAY | 待办：已提出，等待人工审核 |
| In review | YELLOW | 审核中：等用户查看并决定是否放行 |
| Approved | PURPLE | 已批准：允许 Agent 开始执行 |
| In progress | BLUE | 执行中：Agent 正在开发与验证 |
| Done | GREEN | 完成：issue 已关闭并归档 |

## 4. 文档

- 新增 `docs/agents/PROJECT_REVIEW.md`：三者关联模型（mermaid）、编号与命名规则、状态机、批量闭环流程（每批 10 个、只动 Approved）、gh 命令速查、与 WORKFLOW.md 的衔接。
- 更新 `docs/agents/README.md`：阅读决策表新增"Project / 批量任务"一行，头部时间戳指向新 change-history 条目。
- 更新 `project-review/README.md`：新增"与 GitHub Issue / 看板的关联（2026-08-16 更新）"小节，说明审查记录如何进入执行闭环。

## 5. 未做

- 未创建任何 issue、未向看板添加任何卡片（用户明确"先别急着放"）。
- 未修改业务代码、CRD、配置与测试。
