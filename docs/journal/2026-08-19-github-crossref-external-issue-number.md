# 2026-08-19 GitHub 交叉引用泄露：外部 issue 编号进入公开仓库

> 日期：2026-08-19 ｜ 触发者：本地 Agent（WSL 回环案例研究）｜ 相关：docs/agents/WORKFLOW.md 第 5 节、change-history/2026-08-18-wsl-loopback-case-study/

## 现象

- 微软 WSL 回环 issue（编号见桌面 `WSL/` 研究文档，公开仓库内一律不写）的时间线上，出现了本仓库的 cross-referenced 事件，来源是本仓库的 PR 与演练 issue——我们从未在对方 issue 里发言，但对方维护者与第三方都能看到这些来源，等于把"研究微软内部 bug"这件事暴露出去。
- 用户观察到：演练 issue 删除后，对应事件从对方时间线消失；PR 无法删除只能 close。经 API 复核确认：**删除源 issue 事件才消失；close 不会移除事件**（close 仅改状态，推断未实测，因 PR 已不打算删除）。

## 上下文

- 触发机制：只要 PR 标题/正文、issue 标题/正文/评论、提交信息、提交 diff 中出现"井号 + 纯数字"（`#<数字>`），GitHub 自动在对应 issue 时间线登记一条 cross-reference，来源指向本仓库，第三方可见。URL 链接不触发，完整编号才触发。
- 本仓库 PR #59 原标题含"WSL `#<编号>`"字样，演练 issue #57/#58 标题同样含外部编号，均触发了对方时间线的引用事件。

## 处理（2026-08-19 凌晨）

- PR #59 改名（无编号）、正文去编号、分支重写为单提交 `4417f6e`（force-push），diff 与提交信息零外部编号。
- 演练 issue #57/#58 已删除；issue #60 标题同步去编号；全仓 main 分支已复查零残留。
- 本地备份分支 `backup/pre-cleanup` 仍含旧带编号内容，**禁止推送**；`Documents/`（gitignore）不受限，但其中外部编号仅用于本地引用。

## 结论（规矩，2026-08-19 起生效）

1. 公开仓库任何内容（PR/issue 标题与正文、评论、提交信息、提交 diff）**禁止出现外部 issue 的完整编号**（`#<数字>`）；需要指代时用描述语（如"微软 WSL 回环 issue，编号见桌面文档"）或贴 URL。
2. 只有删除源才能移除 cross-reference 事件；close 不移除。
3. 提交前检查 `git diff` 与提交信息不含外部编号（已并入 WORKFLOW.md 第 5 节提交前检查）。
