# 变更总览：文档体系重构——机械门禁、内容迁移与入口重写

> 日期：2026-08-18 ｜ 级别：P1 ｜ 对应 PR：[#33](https://github.com/3900563672/hello-k8s-ai/pull/33)、[#34](https://github.com/3900563672/hello-k8s-ai/pull/34)

## 为什么做

- 用户痛点：文档混杂（人类 / 本地 Agent / 远程 AI 共用同一份），AI 读不到自己需要的部分、Token 浪费；旧文档结构没有人强制维护，README 与代码漂移反复发生。
- 目标：三层读者各自独立入口、独立维护、互不串读；文档漂移由 CI 强制拦截；变更历史可追溯、可总览。

## 改成什么

1. **机械门禁（PR #33）**
   - 新增 `docs/MAP.yaml` 源码 → 文档映射；MAP 门禁强制"改源码必须同 PR 更新映射文档"（最长匹配）。
   - 新增 `hack/gen-docs.py` 幂等生成：README 时间线段、`docs/status.md`、`docs/remote-ai/llms.txt`、`docs/README` 所有权表。
   - `hack/check-docs.py` 扩展为 10 项检查（链接 / 白名单 / 行数 / MAP 门禁 / 新鲜度 / front-matter / change-history / journal / 孤儿）；`.github/workflows/docs.yml` 去 paths 过滤，PR 全量校验。
   - 根目录清理：`PROJECT_OVERVIEW.md`、`CHANGELOG.md` 归档至 `change-history/2026-08-13-initial-deployment/`；白皮书 PDF 移出版本控制（`.gitignore`）。
   - `docs/journal/`（15 条流水账）+ `docs/lessons/`（15 条蒸馏）；PROMPTING / SYNC / UI 问题清单并入 `WORKFLOW.md`；remote-ai 只留 README 单一入口；context-pack 默认 FULL。
2. **入口与元数据（PR #34）**
   - `README.md` 重写：新增"文档入口"分层表（人类 / Agent / 远程 AI），保留一键部署与访问表。
   - `AGENTS.md` 重写：命令前置 + 必须 / 先问 / 禁止三层行为准则，保留工程结构、不可破坏的边界、生成文件与交付检查；修复失效的 `SYNC.md` 引用。
   - 46 个 `docs/` 专题统一 front-matter（`维护层 | last-reviewed | 事实源`），`docs/status.md` 的 front-matter 由生成脚本自带；`BUILD_PDF.md` 明确 PDF 输出 `.runtime/` 不提交仓库。
   - `change-history/README.md` 升级为索引 + 两代格式规范。

## 关键行为

- 以后任何 PR：改源码必须同步映射文档（MAP 门禁为 error）；docs-only PR 同样必须过 `make docs-check` + `make docs-sync-check`。
- 新变更归档：中小改动单文件多节，大改动四件套；追加后运行 `make docs-sync`，README 时间线与 `docs/status.md` 自动更新，未同步会被 CI 拦截。
- 远程 AI 唯一入口 `docs/remote-ai/README.md`（包内先读 `CONTEXT_PACK.md`）；本地 Agent 入口 `AGENTS.md` + `docs/agents/`；人类入口 `docs/INDEX.md`。

## 验证

- `make docs-check`：全绿（front-matter 0 error / 0 warning；链接、白名单、MAP 门禁、新鲜度通过）。
- `make docs-sync-check`：提交后派生文件无漂移、工作区干净。
- `DOCS_CHECK_BASE=<PR1 前 commit>` 模拟 CI：MAP 门禁通过。
- PR #33 / #34 全部 CI 通过（文档检查、代码检查、E2E、镜像构建）。

## 回滚

- 两个 PR 均为文档与校验脚本改动，不涉及业务代码：回滚即 revert 对应 commit；派生文件重跑 `make docs-sync` 恢复。
- 根目录保留 `PROJECT_OVERVIEW_NEW.md`（用户明确要求给初学者保留）。
