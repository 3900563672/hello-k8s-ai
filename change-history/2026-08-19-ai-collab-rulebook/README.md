# Agent 对外协作规则落地：AI 产出自查 + 披露 trailer + lessons 触发式升级

> 日期：2026-08-19

## 背景与决策

- 调研 curl（CONTRIBUTE.md AI 政策）、GitLab（AI-Assisted trailer 规范）、WordPress Gutenberg（`.agents/skills/` 触发式 AGENTS.md 体系）、Temporal（Codex 并行 backlog 案例）等开源 AI 协作实践，确定三条可落地规则，全部写入 agent 层文档（`docs/agents/` + `docs/lessons/`）。
- 决策：不推翻现有 `AGENTS.md` + `docs/agents/` 体系（已是主流形态，对标 Gutenberg / WSL）；只做"强制层"增强，规则直接进工作流，不留口头债。

## 实现摘要

- `docs/agents/WORKFLOW.md`：
  - 第 1 节开工：lessons 扫描改为按任务类型匹配 **Use when 触发条件**（索引在 lessons/README.md）。
  - 第 5 节新增两条硬规则：**AI 产出自查（curl 黄金标准）**——"别人看得出这是 AI 写的，就要再打磨"；**AI 披露 trailer（GitLab 式）**——`AI-Assisted: yes` + `AI-Tools: <工具名>`，外部有披露政策的项目贡献时使用。
  - 第 5 节新增"提交信息措辞自然化"规则：提交信息直接写做了什么，避免"落地 / 赋能 / 纯粹化"等 AI 化套话。
  - 8.6 交付检查清单新增 AI 产出自查勾选项。
- `docs/agents/README.md`：开工顺序第 3 步同步改为触发条件匹配。
- `docs/lessons/README.md`：升级为**触发式规则库**（Gutenberg `.agents/skills/` 风格），新增 Use when 触发条件速查表（21 条）、模板强制"触发条件"字段、与 FAILURE_REGISTRY 的分工说明。
- 新建 `docs/lessons/process-ai-collaboration-disclosure.md`：curl 黄金标准 + GitLab trailer 完整规则与验证方法。
- 20 个既有 lesson 全部补充"触发条件（Use when）"元数据行（与失败模式注册表 + 强制扫描合流）。

## 测试与验证

- `make lint-md`：markdownlint 覆盖 docs/agents / journal / lessons / remote-ai / change-history / README（见命令输出）。
- `make docs-check`：相对链接与图片路径校验（见命令输出）。
- `make docs-sync` + `make docs-sync-check`：派生文件无漂移（gen-docs.py 跳过 lessons，无生成物变化）。
- 抽查全部 lessons 文件无 UTF-8 BOM、21 个文件均含"触发条件（Use when）"。

## 迁移与回滚

- 纯文档规则变更，无代码 / 部署影响，无回滚需求；如规则不妥，`git revert` 该提交即可。
