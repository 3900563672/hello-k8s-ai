# AI 协作协调文件（AI_COORDINATION.md）

> 维护层：agent ｜ 创建：2026-08-21 ｜ 事实源：docs/agents/WORKFLOW.md、docs/agents/PROJECT_REVIEW.md、docs/agents/PRINCIPLES.md
> 本文件是**多 AI 会话在同一仓库并行工作时的前台公告板**：登记各会话身份、工作区、文件所有权与当前进度，防止互相踩踏。
> 规则：本仓库任何 AI 会话开工前先读本文件，并在「§2 会话登记表」登记自己的行（也可维护自己的 AI_COORDINATION.md 并在表内留链接）。未登记不得认领他人范围内的文件。

## 1. 为什么有这个文件

多个 AI 会话正在 hello-k8s-ai 上并行工作（写测试、修 bug、做覆盖率）。Git 分支隔离了提交，但共享同一工作区时未提交文件会互相踩踏；issue 表达「做什么」，本文件表达「谁在做、在哪做、边界在哪」。

## 2. 会话登记表（每个会话填一行）

| 会话名 | 角色 | 工作区 / 分支 | 写权限范围 | 当前任务 | 状态 |
| --- | --- | --- | --- | --- | --- |
| hello-k8s-ai-fixer | 协调者 + 修复 | `/root/hello-k8s-ai-fix`（git worktree）；分支 `codex/fix-replay-viewport-2026-08-21` | `dashboard/frontend/my-app/src/components/shared/TimeTravelBar/timelineMath.ts`、`src/stores/timeSlice.ts`、这两个文件的测试中 #140/#141 相关断言、本文件、对应 change-history | 修复 #140/#141 | 2026-08-21 进行中 |
| hello-k8s-ai-frontend-tests | 前端测试 | `/root/hello-k8s-ai` 主工作区；分支 `codex/frontend-tests` | `dashboard/frontend/my-app` 测试脚手架（vitest 配置、`src/test/`、各 `*.test.ts(x)`）、`.github/workflows/test.yml` | #142/#143 前端测试补齐 | 2026-08-21 已交付（并入 PR #144） |
| hello-k8s-ai-backend-tests | 后端测试 | `/root/hello-k8s-ai` 主工作区；分支 `codex/frontend-tests` | `dashboard/backend/` 测试、`hack/cover-gaps.py`、`hack/coverage-check.py`、`.github/workflows/test.yml` 的 coverage job、`Makefile` 的 `coverage` target | #142 后端覆盖率 + 门禁（5 包全绿） | 2026-08-21 已交付（并入 PR #144） |

## 3. 工作区规则（防踩踏）

1. **一个会话一个工作区**：优先 `git worktree add` 独立目录；禁止两个会话在同一工作区同时编辑。
2. 主工作区 `/root/hello-k8s-ai` 当前被 frontend-tests 占用（有未提交文件），其他会话不得在此提交或切换分支。
3. `git status` 出现自己没碰过的文件时不要 `git add -A`，只添加自己范围内的路径。
4. 分支命名 `codex/<主题>-YYYY-MM-DD`；提交信息带 `Fixes #N`；不推送/合并 main（合并由用户执行）。
5. 脚本类改动（`.sh`/`.ps1`/`.mjs`）过 `make lint-sh`/`make lint-ps1`；markdown 改动过 `make lint-md`；文件一律 LF + UTF-8（WSL 侧写入，见 process-cross-platform-file-hygiene）。

## 4. 当前文件所有权（重叠风险点）

| 路径 | 所有者 | 说明 |
| --- | --- | --- |
| `timelineMath.ts` / `timeSlice.ts` | 暂无固定 | 改前在对应 issue 评论声明，避免并发冲突 |
| `timelineMath.test.ts` / `timeSlice.test.ts` | **fixer**（#140/#141 相关断言以 fixer 分支为准） | frontend-tests 提交时跳过这两个文件中的 #140/#141 断言，或先合 fixer 分支再 rebase |
| vitest 脚手架（`vitest.config.ts`、`src/test/`、package.json 测试脚本与依赖） | frontend-tests | fixer 分支含最小副本仅用于本地/CI 验证，合入时以先合入者为准 |
| `dashboard/backend/`、`hack/cover-*.py` | 后端测试会话 | 与 frontend-tests 零重叠 |

## 5. 协作协议（要点）

- 消息/认领：agent-bus（团队 `hello-k8s-ai`，本仓库专属；kueue 等外部事务仍在 `codex-bus`）。
- 发现 bug：按 WORKFLOW.md 建 `bug:` issue 并挂 Project Review 看板；看板状态机见 PROJECT_REVIEW.md（只动 Approved 及之后）。
- 修复认领：issue 评论声明 → 看板 `In progress` → 交付 PR `Fixes #N` → 看板 `Done`。
- 收工：更新本文件状态行；长任务每完成一个阶段同步一次。

## 6. 协调待办（给所有会话）

- [x] frontend-tests：本文件 §2 行已更新；#140/#141 断言移交 fixer（§4），其余测试已并入 PR #144。
- [x] 后端测试会话：登记身份/工作区/范围（已登记并在 PR #144 交付）。
- [ ] 任一 PR 合入 main 时把本文件同步进仓库（谁先合谁带）。
