# Agent 进化：从"文本沉淀"到"机器强制"（静态检查 + 失败模式注册表 + make doctor）

> 日期：2026-08-19 ｜ 级别：P0 ｜ 关联：docs/agents/FAILURE_REGISTRY.md、AGENTS.md、WORKFLOW.md

## 为什么做

- 复盘结论：反复犯错的根因不是"没沉淀"，而是沉淀是**文本建议**不是**机器强制**——每次任务都是新上下文，提醒会被跳过、被淹没、会过期。
- 目标：把"犯过的低级错"变成机器拦截（静态检查），把"同类路径的坑"变成开工提醒（失败模式注册表 + 强制扫描），把"跑起来才爆"变成"跑之前 30 秒自检"（make doctor）。

## 改成什么

1. **静态检查接入 make 与 CI（三件套）**
   - `make lint-sh`：shellcheck 全部 `*.sh`（原 selfcheck 只有 `bash -n` 语法层，shellcheck 补语义层）。
   - `make lint-md`：markdownlint-cli2 检查 Agent 文档层（docs/agents、docs/remote-ai、docs/journal、docs/lessons、change-history、README.md），配置 `.markdownlint-cli2.yaml`（关中文不适用规则，保留 MD012/024/025/028/031/034/037/038/047/056 结构规则）。
   - `make lint-ps1`：PSScriptAnalyzer 检查仓库 `.ps1`（新 `hack/lint-ps1.sh` 包装；非 Windows 环境自动跳过）。
   - 三件套并入 `make lint` / `make verify`；CI 侧 lint.yml 安装工具并执行，docs.yml 增加 markdownlint。
   - 存量清零：shellcheck 修复 6 处告警（preflight/local-cluster/restore-data/sleep-guard），markdownlint 机械+语义修复 249 个文件至 0 告警，sleep-guard.ps1 修复 5 条 PSScriptAnalyzer 告警（函数改名 + ShouldProcess）。

2. **失败模式注册表**：新建 `docs/agents/FAILURE_REGISTRY.md`，13 条 FR（现象/触发条件/根因/必须动作/证据链/状态），状态分 active/guarded；WORKFLOW 第 1 节与 docs/agents/README 开工顺序强制"扫末尾 3 条"。

3. **环境自检**：新建 `hack/doctor.sh` + `make doctor`（磁盘 / Docker engine 与 kind 节点 / WSL 回环探针 / 端口 / 内存 / tmpfs 残留 / dmesg ENOBUFS），不依赖集群，与 preflight 分工（doctor=环境层，preflight=运行前深度体检）。

4. **流程强制化**：AGENTS.md 新增第 7/8 条必须项（脚本/文档改动必须过静态检查；开工与长跑前 make doctor）；WORKFLOW 8.6 增加 done-check 三问（实跑命令？输出证据？未验证范围？）。

## 关键行为

- `make lint` 现在 = golangci-lint + shellcheck + markdownlint + PSScriptAnalyzer；`make selfcheck` = lint-sh + lint-md + node 语法/单测 + 回环探针 + 清单渲染；`make verify` 全量。
- 工具缺失时不静默跳过：lint-sh/lint-md 报安装命令并 exit 1（机器强制，不允许"没装就绕过去"）。
- 新增 .sh/.md/.ps1 改动若带告警，本地 `make verify` 与 CI 都会拦下。

## 验证

- `make lint-sh` / `make lint-md` / `make lint-ps1`：全部 OK（shellcheck 0 告警、markdownlint 0/249、PSScriptAnalyzer 0）。
- `make doctor`：11 PASS / 0 FAIL（C: 103GB、Docker 可达、5/5 节点、回环 PASS、端口正常、内存 8.8GB、无 tmpfs 残留、无 ENOBUFS）。
- `make docs-sync`：派生文件无意外差异（README 仅补尾换行）；docs-check 见下。
- sleep-guard.ps1 功能回归：`status` 输出 `guard=on` 与改前一致。
- 未验证：CI 全流程（需推送后看 run）；golangci-lint 全量（Go 代码未改，`make lint` 依赖链路已本地跑通）。

## 回滚

- 删除 `.markdownlint-cli2.yaml`、`docs/agents/FAILURE_REGISTRY.md`、`hack/doctor.sh`、`hack/lint-ps1.sh` 与 change-history 条目，`git revert` Makefile/AGENTS/WORKFLOW/CI 改动即可；存量 markdown 机械修复为白噪声（尾换行/空行），可保留。
- 工具依赖：shellcheck（apt）、markdownlint-cli2（npm -g）、PSScriptAnalyzer（Windows PowerShell 模块）——缺失时 lint-sh/lint-md/lint-ps1 会给出安装指引并失败，不影响其他 target。
