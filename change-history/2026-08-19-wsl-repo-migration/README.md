# WSL 回环案例整体迁移至独立仓库 wsl-loopback-stall

> 日期：2026-08-19 ｜ 决策：WSL 研究内容从 hello-k8s-ai 分离，独立成库（对外展示面）｜ 新仓库：[wsl-loopback-stall](https://github.com/3900563672/wsl-loopback-stall)

## 背景

WSL 回环案例研究已闭环（官方 issue #41383 发布放行 + 2.9.5 修复确认）。用户决定：研究内容与 hello-k8s-ai 解耦，独立仓库承载，便于对外展示与后续维护，避免本仓库继续累积非项目主题内容。

## 迁移内容

- **完整 commit 历史**：git filter-repo 按路径提取 18 个 WSL 相关 commit（作者/日期/消息保留，哈希重写），已推送新仓库 main。
- **文档归一重编号**：桌面工作区与 Documents 版本归一为 docs/ 01-24 全局唯一编号（公开面已清洗投稿内部材料），无交叉。
- **归档**：源仓库研究 issue 全文与 PR 索引、早期文档形态（archive/）；投稿内部材料仅本地保留，不入库。
- **工具与证据**：探针 Go 源码、观察脚本、直方图分析（tools/）；关键日志包与渲染截图（evidence/）；大文件清单（evidence/manifest.md）。
- **面试材料**：Documents/（gitignore，永不推送）。
- **观察窗口**：由新仓库 #1 跟踪（源 issue #83 已删除，内容本地归档）。

## 本仓库处理

- 既有 WSL 相关文档（docs/operations/WSL_LOOPBACK_CASE_STUDY.md、docs/lessons/*、docs/journal/*、change-history WSL 条目）**保留不动**，作为历史语境；新内容不再在本仓库沉淀 WSL 研究。
- 迁移后新仓库为 WSL 主题唯一事实源；本条目为迁移记录。

## 未验证项

- 新仓库 README/文档渲染（公开前复查）。
- 观察窗口（至 09-01）改由新仓库 #1 跟踪。
