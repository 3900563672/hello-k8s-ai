# 测试报告

## 1. 环境

- WSL Ubuntu；gh 2.46.0；GitHub 账号 3900563672。

## 2. 执行的验证与真实结果

| 命令/操作 | 结果 |
| --- | --- |
| `repository.projectsV2` 查询 | 返回 Project Review（number 2），确认仓库级关联 |
| `updateProjectV2Field` | Status 五态写入成功（含选项 ID） |
| `addProjectV2ItemById` × 8 | 全部成功，item ID 已记录 |
| `updateProjectV2ItemFieldValue` × 8 | 全部成功 |
| 看板查询（items + fieldValues） | 8 张卡片全部 `To do` |
| `deleteProjectV2`（旧用户级项目） | 成功 |
| `make docs-check` | 提交前验证 |

## 3. 未验证项

- 浏览器视角的仓库 Projects 页面（无浏览器会话，已通过 API 确认仓库关联）。
- 自动化执行链路（未开始，等用户放行）。
