# 分层文档维护边界与同步协议

- 变更日期：2026-08-16
- 关联问题：无（用户直接指示）
- 变更级别：P1 文档体系与 AI 协作方式
- 变更范围：docs、AGENTS.md、hack、Makefile、change-history
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

三层文档不再共享同一批人类专题：`docs/` 明确为纯人类文档（Agent 与远程 AI 默认不读、默认不接收）；`docs/agents/` 由本地 Agent 独立维护；`docs/remote-ai/` 与上下文包由人 + Agent 维护、远程 AI 通过交付物提建议。新增 `docs/agents/SYNC.md` 同步协议（时间戳 + change-history 时间线 + 人类文档待同步清单）。上下文包默认排除人类专题（`make context-pack FULL=1` 可选全量），体积从约 13MB 降到约 1.4MB。

## 2. 维护边界

| 层 | 内容 | 维护者 |
| --- | --- | --- |
| human | `docs/` 专题 | 人；Agent 仅按明确要求代笔 |
| agents | `docs/agents/` | 本地 Agent，人审核 |
| remote-ai | `docs/remote-ai/` + 上下文包 | 人 + Agent；远程 AI 提建议 |
| 时间线 | `change-history/` | Agent 每次交付追加 |

## 3. 同步协议要点

- 每次交付后：追加 change-history 条目 → 更新 agents 受影响文档 → 更新 remote-ai（如受影响）→ 重新生成上下文包 → 列出人类文档待同步清单。
- 文档头部元数据：`维护层 / 最后同步 / 对应变更`。
- 可复用提示词见 `docs/agents/SYNC.md` 第 6 节，可直接发给任何 AI。

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)
- [测试报告](TEST_REPORT.md)