# gh issue view 被 Projects classic 弃用报错 + worktree 工作区隔离

> 日期：2026-08-21 ｜ 会话：hello-k8s-ai-fixer ｜ 关联：PR #145、AI_COORDINATION.md

- `gh issue view <n> --repo ...` 在本仓库直接报错（GraphQL: Projects (classic) is being deprecated... repository.issue.projectCards），但 `gh issue list` 正常；改用 REST API（`curl https://api.github.com/repos/.../issues/<n>` + jq）或 `--json` 指定字段可绕过。
- 多 AI 并行时用 `git worktree add` 隔离工作区；但 linked worktree 的 `.git` 是指针文件，`echo pattern >> .git/info/exclude` 会失败，需用 `git rev-parse --git-path info/exclude` 拿到真实 gitdir 再写（否则 node_modules 符号链接会漏进 git status）。
- 在共享仓库根目录放根级 `AI_COORDINATION.md` 会触发 `hack/check-docs.py` 根目录 md 白名单门禁：新根级 md 必须同步加入白名单（并更新 docs/operations/TROUBLESHOOTING.md 对应行），MAP 门禁还会要求同步命中映射的人类文档。
- 经验：CI 的 docs-check 对「本提交 diff」做 MAP 门禁，本地验证必须先 commit（或 amend）再跑 `DOCS_CHECK_BASE=origin/main make docs-check`，只看工作树会误判。
