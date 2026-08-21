# Codex 桌面版 WSL UNC 工作区沙箱故障 + auto-review 模型错配修复与沉淀（2026-08-21）

> 日期：2026-08-21 ｜ 关联：docs/lessons/codex-wsl-unc-workspace-setup-refresh.md

## 为什么做

- Codex 桌面版出现三连故障：① 工作区为 WSL UNC 路径（`\\wsl.localhost\...`）的会话所有命令统一失败（`helper_unknown_error: setup refresh had errors`）；② 会话从「最近」掉出项目分组；③ 恢复后需要写沙箱外路径时被自动审批拒绝（`Automatic approval review failed: ... you passed gpt-5.6-luna`）。
- 根因：沙箱 setup refresh 向工作区及 `.git` 写 Windows ACL，WSL 9P 文件系统不支持 `GetSecurityInfo`（错误码 1）→ 命令通道全挂、项目关联不登记；工作区受限会话走 auto-review 时，审查模型默认名 `gpt-5.6-luna` 与 DeepSeek provider（仅支持 deepseek-v4-*）不匹配 → 审批必拒。

## 改成什么

1. 沉淀 lesson：`docs/lessons/codex-wsl-unc-workspace-setup-refresh.md`（现象 / 根因 / 恢复方法 / 教训，含沙箱日志证据行与 auto-review 附注）。
2. 修复 auto-review 模型：`~/.codex/config.toml` 顶层新增 `auto_review_model_override = "deepseek-v4-flash"`（官方 PR #23767 配置项），修改前已备份 `config.toml.bak-2026-08-21`。
3. 现场恢复：受影响会话拖回项目分组即恢复命令通道（实测有效）；两个大会话 JSONL（142MB / 71MB）已备份至 `C:\Users\hh\codex-session-backup\`（哈希一致），会话文件本身未做任何修改。

## 关键行为

- 本条目为环境 / 工具侧沉淀，未触碰集群、运行时组件与前端。
- `auto_review_model_override` 只影响审查子代理的模型选择，不改变沙箱边界、批准策略与网络 / 文件系统限制。
- 会话 JSONL 工作区路径保持原样，避免破坏性编辑。

## 验证

- 拖回项目后当前会话命令通道恢复；沙箱日志对照（`C:\Users\hh\.codex\.sandbox\sandbox.2026-08-21.log`）：C 盘工作区会话 `errors=[]`，UNC 工作区会话持续报 ACL 失败。
- 备份与源文件哈希一致。
- config.toml 改动为单行新增，读回校验正常。

## 回滚

- 删除 `~/.codex/config.toml` 的 `auto_review_model_override` 行（备份 `config.toml.bak-2026-08-21`）即恢复原状。
- 删除 lesson 与本条目即可，无运行时影响。