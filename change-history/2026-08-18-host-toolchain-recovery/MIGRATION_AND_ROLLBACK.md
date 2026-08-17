# 迁移与回滚：宿主工具链恢复

## 迁移说明

- 本次无数据库、CRD、API 或部署清单变更，无需数据迁移。
- 机器侧 config.toml 的修复不影响仓库内容；应用重启后自动读取新路径。

## 回滚

- config.toml：备份位于 `C:\Users\hh\.codex\config.toml.bak-20260817-2359`，如需回滚直接覆盖回去（注意备份是修改前的完整文件）。
- 文档：`git checkout -- docs/agents/KNOWN_PITFALLS.md docs/agents/SYNC.md change-history/README.md` 并删除本条目目录即可。

## 风险

- 若未来 Codex 应用再次更新运行时目录，notify 路径可能再次过期；下次遇到 `helper_unknown_error` 先查 `C:\Users\hh\AppData\Local\OpenAI\Codex\runtimes\cua_node` 下的实际版本号再改 config。
- 若 Docker Desktop 设置中 Kubernetes 被手动关闭，重启后节点容器不会自动恢复，需要用户在 Settings → Kubernetes 重新启用（本次为自动恢复，未触发该路径）。
