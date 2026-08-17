# 同步协议（SYNC）

> 维护层：agents ｜ 最后同步：2026-08-17 ｜ 对应变更：change-history/2026-08-17-stability-recovery/
> 目的：代码或行为变更后，让三层文档与 change-history 时间线保持一致，避免漂移。

## 1. 谁维护什么

| 层 | 内容 | 维护者 |
| --- | --- | --- |
| human | `docs/` 专题（叙事 / 教程 / 白皮书） | 人；Agent 仅按用户明确要求代笔 |
| agents | `docs/agents/`（工作流 / 踩坑 / 原则 / 本文件） | 本地 Agent，人审核 |
| remote-ai | `docs/remote-ai/` + 上下文包 | 人 + Agent 代笔；远程 AI 通过交付物提建议 |
| 时间线 | `change-history/` | Agent 每次交付追加条目 |

## 2. 触发条件

- 任何代码、CRD、API、行为、部署方式的变更交付后。
- 文档自身结构或规则大改后。
- 发现文档漂移（README 或 `docs/` 描述与当前行为不一致，例如访问地址、架构边界、CRD 语义）。

## 3. 同步步骤（Agent 每次交付后执行）

1. 追加 `change-history/YYYY-MM-DD-<主题>/` 条目（README / IMPLEMENTATION_DETAILS / TEST_REPORT / MIGRATION_AND_ROLLBACK），日期用 UTC 日期。
   - UI / 面板视觉改动：条目下附 `screenshots/before-<page>.png` 与 `after-<page>.png` 成对快照（约定见 `docs/agents/UI_VERIFICATION.md`「快照约定」）。
   - 详略规范：README 一页内概述"为什么改、改成什么、关键行为"；三个细节文件完整记录背景（改动前状态）、实现（文件与逻辑）、验证（命令与真实结果）、回滚与风险；禁止简写成一行结论，无验证证据写"未验证"。
2. 更新 `docs/agents/` 受影响文档：踩了新坑 → `KNOWN_PITFALLS.md`；契约或原则变化 → `PRINCIPLES.md`；流程变化 → `WORKFLOW.md`。
3. 更新 `docs/remote-ai/`：远程 AI 的阅读、产出或交接方式受影响时。
4. 重新生成上下文包：`make context-pack`（`CONTEXT_PACK.md` 顶部的生成时间即新时间戳）。
5. 核对 README 与受影响人类文档：本次改动导致 README / `docs/` 描述过期时，能直接修的直接修并纳入本次提交（访问方式、架构、行为类必须修）；其余过期项列"人类文档待同步清单"，交给用户决定：自己改，或授权 Agent 代笔。
6. 提交信息与最终汇报中注明对应 `change-history/` 条目。

## 4. 时间戳规则

- 每个受维护文档头部维护一行元数据：

```text
> 维护层：<human|agents|remote-ai> ｜ 最后同步：YYYY-MM-DD ｜ 对应变更：change-history/<条目>/
```

- 变更只影响本层时，只更新本层时间戳；跨层影响按第 3 节执行。

## 5. 给远程 AI 的同步义务

- 交付物必须带日期，并声明"基于的包生成时间"（`CONTEXT_PACK.md` 顶部）。
- 发现包内文档与源码不一致时，作为交付物列出差异，不静默按文档写代码。
- 对 `docs/agents/`、`docs/remote-ai/`、`change-history/` 的建议可以直接给；对 `docs/` 人类文档的建议单独标注"人类文档"，由用户转交。

## 6. 提示词工作流（发给任何 AI）

提示词协议已独立成文，按对象取用：

- 人类下达任务：抄 [docs/getting-started/AI_COLLABORATION.md](../getting-started/AI_COLLABORATION.md) 的模板 A-E。
- 本地 Agent 执行：按 [docs/agents/PROMPTING.md](PROMPTING.md) 解析五要素，开工陈述复述理解，交付走第 7 节检查清单。
- 转交其他本地 Agent：用 PROMPTING.md 第 5 节转交模板。
- 转交远程 AI：打包后附 PROMPTING.md 第 6 节模板；远程 AI 按 [docs/remote-ai/PROMPTING.md](../remote-ai/PROMPTING.md) 执行。

一行版：> 你是 <本地 Agent | 远程 AI>。本次任务：<任务>。请先读 <你的层入口>（本地 Agent：`AGENTS.md` + `docs/agents/README.md` + `docs/agents/PROMPTING.md`；远程 AI：包内 `CONTEXT_PACK.md` + `docs/remote-ai/PROMPTING.md`），按对应 WORKFLOW 执行；交付后按 SYNC 协议同步，并给出时间戳与 change-history 条目。涉及人类文档的改动先列出清单，不要直接改。

## 7. CI 轮询节奏

- 推送后每 30 秒轮询一次 run 结论（`gh run list` / `gh run view --json jobs`），**不要 sleep 到固定大间隔**。
- 预期耗时：普通 job 3-6 分钟；E2E / 镜像构建最慢，冷缓存首次更久；最多等到 10 分钟再停下排查。
- 失败先取 `gh run view <run-id> --log-failed` 定位原因，不盲改重推；docs-only 提交只触发"文档检查"。
