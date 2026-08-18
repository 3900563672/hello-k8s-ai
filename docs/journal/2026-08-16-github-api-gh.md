# GitHub API 与 gh

> 日期：2026-08-16 ｜ 触发者：本地 Agent ｜ 相关：原 docs/agents/KNOWN_PITFALLS.md 归档（2026-08-18 文档体系重构拆分为 journal/lessons）

### 2026-08-16 gh 偶发 TLS handshake timeout / EOF
- 现象：批量创建 label、编辑 issue 时部分请求失败。
- 解决：失败项重试即可；批量脚本要容忍失败，最后统一核对清单。
- 验证：重试后 17 个 label、5 个 issue 标签全部就位。

### gh 权限不足不等于无数据
- 现象：Projects v2 字段查询报权限错误；SSH keys 列表 404。
- 原因：token scopes 缺 `read:project`、`admin:public_key`。
- 解决：先查 `totalCount` 判断"有没有"；缺 scope 需 `gh auth refresh` 或重新授权，不要根据报错断定无数据。
- 验证：`projectsV2 totalCount=0` 确认真无 Projects。
### 2026-08-18 删除 issue：REST 无端点，必须 GraphQL
- 现象：临时 issue #57 需要删除；`gh api -X DELETE repos/{owner}/{repo}/issues/57` 报错（404/410）。
- 原因：GitHub REST API 根本没有"删除 issue"端点；只有 GraphQL mutation `deleteIssue`（需 issue 的 node_id，用 `gh api repos/{owner}/{repo}/issues/{n} --jq .node_id` 取）。
- 坑：`DeleteIssuePayload` 不含 `issue` 字段，查询里写 `issue { ... }` 会报 `Field 'issue' doesn't exist on type 'DeleteIssuePayload'`；只查 `repository { nameWithOwner }` 即可。
- 验证：mutation 返回 `{"repository":{"nameWithOwner":"..."}}`；再 GET 该 issue 返回 HTTP 410 "This issue was deleted"。
