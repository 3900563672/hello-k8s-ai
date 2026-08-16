# 测试报告

## 1. 环境

- WSL Ubuntu，`gh 2.46.0`，GitHub 账号 `3900563672`。

## 2. 执行的验证与真实结果

| 命令 | 结果 |
| --- | --- |
| `gh auth status` | 已登录；scopes：`gist, project, read:org, repo` |
| `gh project list --owner 3900563672` | 返回 1 条：`1 Project Review open`（此前为空） |
| `gh project field-list 1 --owner 3900563672` | Status 字段存在，选项为五态，与预期一致 |
| GraphQL `updateProjectV2Field` | 返回 options 含 id/name/color，五态写入成功 |
| `python3 hack/check-docs.py`（make docs-check） | 通过（Markdown 相对链接与图片路径检查） |

## 3. 未验证项

- 卡片实际流转（`item-add` / `item-edit` / `item-archive`）：需在第一批 issue 导入后验证。
- 看板网页视图：无浏览器会话，未截图确认；字段与选项已通过 API 核实。
- 用户侧权限体验：`project` scope 为 gh OAuth 授权，网页端 Projects 界面依赖登录态，未验证。
