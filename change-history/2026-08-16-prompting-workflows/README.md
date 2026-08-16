# 提示词工作流体系：人类 / 本地 Agent / 远程 AI 三份协议

- 变更日期：2026-08-16
- 关联问题：无（用户要求的协作体系优化）
- 变更级别：P1 协作与文档体系
- 变更范围：`docs/`（人类层）、`docs/agents/`、`docs/remote-ai/`、根 `README.md`、`change-history/`
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

把"怎么给 AI 下任务"从零散经验沉淀为三份可复制的提示词协议，按读者分层：

- 人类侧新增 [AI_COLLABORATION.md](../../docs/getting-started/AI_COLLABORATION.md)：任务五要素、五个可复制提示词模板（探索 / 最小改动 / 分批 / GitHub / 远程 AI）、好例子与坏例子、交付审核清单、协作节奏建议。
- 本地 Agent 侧新增 [PROMPTING.md](../../docs/agents/PROMPTING.md)：任务五要素解析与默认假设、澄清协议、开工陈述模板、任务转交模板（给 Agent / 给远程 AI）、交付检查清单。
- 远程 AI 侧新增 [PROMPTING.md](../../docs/remote-ai/PROMPTING.md)：收任务三步、产出组织规则、固定交接格式、红线（不假装验证）。

## 2. 关键行为

- 工作流主链不变（WORKFLOW.md 仍是唯一流程主链），本次只补"提示词"这一段，并在各层 README / WORKFLOW / SYNC 中互相引用。
- 三层文档时间戳统一更新为本次条目；SYNC.md 第 6 节"可复用提示词"从一段话升级为按对象取用的协议入口。
- 上下文包自动包含新增的 `docs/agents/PROMPTING.md` 与 `docs/remote-ai/PROMPTING.md`（`cp -r docs/agents docs/remote-ai`），远程 AI 无需额外操作。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| docs/getting-started | 新增 AI_COLLABORATION.md（人类入口） |
| docs/agents | 新增 PROMPTING.md；README / WORKFLOW / SYNC 增加引用 |
| docs/remote-ai | 新增 PROMPTING.md；README / WORKFLOW 增加引用 |
| docs/ | README / INDEX 登记新入口 |
| 根 README | 首段增加 AI 协作入口 |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 实测结论与停止线

- `make docs-check`（Markdown 链接检查）通过；上下文包重新生成成功。
- 停止线：本次只建提示词协议与入口，不重写各层 WORKFLOW 主链；后续按实际使用反馈迭代模板措辞。
