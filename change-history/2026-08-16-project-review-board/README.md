# Project Review 看板与任务闭环

- 变更日期：2026-08-16
- 关联问题：无（用户直接要求建立 GitHub Issue / Project 看板 / project-review 审查记录之间的关联机制）
- 变更级别：P1 工程基建
- 变更范围：GitHub Project v2（新建 Project Review 看板）、gh 认证 scope、`docs/agents/`（新增 PROJECT_REVIEW.md、更新 README）、`project-review/README.md`
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

- 补齐 gh token 的 `project` scope（`gh auth refresh -s project`），现在可以读写 GitHub Projects。
- 创建用户级 GitHub Project v2 看板 **Project Review**（number 1），Status 字段配置为闭环五态：To do / In review / Approved / In progress / Done，每个状态带颜色与中文说明。
- 明确三者关联模型：`project-review/` 审查记录（静态事实源）→ GitHub Issue（执行单元）→ Project 看板（状态机），详细规则写入 `docs/agents/PROJECT_REVIEW.md`。
- `docs/agents/README.md` 阅读决策表新增"Project / 批量任务"一行；`project-review/README.md` 增加关联说明小节。
- **未导入任何 issue / 卡片内容**，等用户放行第一批后再填充。

## 2. 关键行为

- Agent 只操作 `Approved` 及之后的条目；`To do` / `In review` 一律不动代码。
- 每批上限 10 个；一批全部 `Done` 后归档（issue 关闭 + 卡片 `item-archive`），停下等用户放行下一批。
- 提交用 `Fixes #N` 自动关闭 issue，卡片移入 Done 并归档；`project-review/` 作为历史基线不修改。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| GitHub Project v2 | 新建 Project Review 看板（number 1），Status 五态 |
| gh 认证 | token 新增 `project` scope |
| docs/agents/ | 新增 PROJECT_REVIEW.md；README 决策表加一行并更新时间戳 |
| project-review/ | README 增加关联说明小节 |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 实测结论与停止线

- `gh project create` / `field-list` / GraphQL `updateProjectV2Field` 均实测成功，Status 五态已写入看板。
- gh 2.46 的 `field-create` 不允许使用保留名 "Status"，内置 Status 字段不可删除，最终用 GraphQL `updateProjectV2Field` 更新选项完成。
- **停止线**：关联机制到此为止；不导入第一批内容，等用户放行后进入自动化闭环。

## 6. 剩余未验证

- 卡片实际流转（`item-add` / `item-edit` / `item-archive`）在第一批 issue 导入后验证。
