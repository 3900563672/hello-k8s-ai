# Codex 桌面版：WSL UNC 工作区沙箱 setup refresh 失败 + 项目分级丢失 + auto-review 模型错配（2026-08-21）

## 现象

- 工作区为 `\\wsl.localhost\...`（WSL UNC 路径）的 Codex 桌面版会话，所有命令（exec_command / node_repl）统一失败：
  `Failed to create unified exec process: helper_unknown_error: setup refresh had errors`
- 同一批会话在桌面版「最近」列表中全部掉出项目分组，无法按项目归类。
- C 盘工作区（`C:\...`）的新会话完全正常。

## 根因

- Codex Windows 沙箱每次执行命令前做 setup refresh，会向工作区及 `.git` 写入 Windows ACL（ACE check / write ACE / deny ACE）。
- WSL 9P 文件系统（`\\wsl.localhost\...`）不支持 `GetSecurityInfo / GetNamedSecurityInfoW`（错误码 1 = ERROR_INVALID_FUNCTION），ACL 操作全部失败 → setup refresh 报错退出 → 命令被拒。
- 沙箱日志：`C:\Users\hh\.codex\.sandbox\sandbox.2026-08-21.log`
  证据行：`write ACE check failed on \\?\UNC\wsl.localhost\...: GetSecurityInfo failed ... 1`、`setup error: setup refresh had errors`。
- 项目分级丢失同源：会话工作区是 UNC 路径时，桌面版项目关联（`.codex-global-state.json` 的 local-projects）不登记该路径，会话无法归入项目。
- 对照：C 盘工作区会话的 setup refresh 日志 `errors=[]`，完全正常。

## 恢复方法（实测有效）

1. 在 Codex 桌面版中，把「最近」里受影响的会话**拖回对应项目**（重新建立项目关联）。
2. 会话命令通道即恢复（实测立即生效，无需重启）。
3. 若拖回无效，备选方案：把会话 JSONL 中所有 `\\wsl.localhost\Ubuntu\root\hello-k8s-ai` 替换为 Windows 本地路径（如 `C:\Users\hh\hello-k8s-ai`），并把仓库复制到对应本地路径后重启 Codex。
   - 会话文件位置：`C:\Users\hh\.codex\sessions\<年>\<月>\<日>\rollout-*.jsonl`
   - 替换前必须先备份（`.pre-fix`）；必须在 Codex 完全退出后修改（文件被占用会读失败）。

## 教训

- 「所有命令突然全挂 + 报 setup refresh」→ 先看沙箱日志（`C:\Users\hh\.codex\.sandbox\`）定位是哪个路径的 ACL 操作失败，而不是怀疑 WSL / Docker / 项目本身。
- 工作区放 WSL UNC 路径的会话有结构性风险：沙箱 ACL 阶段无法在 9P 上工作。长期建议工作区用 Windows 本地路径。
- 会话历史永不丢：界面可见、磁盘 JSONL 在（`C:\Users\hh\.codex\sessions\`），只是命令通道可能被环境卡死。
- 修改会话 JSONL 前必须备份 + 逐行 JSON 校验 + 可回滚。

## 2026-08-21 附：自动审批拒绝高风险操作（auto-review）

- 现象：需要写沙箱外路径的命令被自动审批拒绝：`Automatic approval review failed: The supported API model names are deepseek-v4-pro... but you passed gpt-5.6-luna`。
- 根因：审批审查器（auto-review）配置的审查模型名与其服务支持的模型不匹配（审查端点只支持 deepseek-v4-*，配置传了 gpt-5.6-luna）。
- 处理：把审查模型改为 `deepseek-v4-pro`（或关闭 auto-review 改人工批准）。
- 教训：升级 / 更换 provider 后，审批审查模型配置不会自动跟随，需显式核对。
