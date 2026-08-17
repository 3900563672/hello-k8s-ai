# 实现细节：宿主工具链恢复

## 改动前状态

- `C:\Users\hh\.codex\config.toml`：`[model_providers.deepseek] notify` 指向 `cua_node/2f053e67fec2d258\bin\node_modules\@oai\sky\bin\windows\codex-computer-use.exe`，该目录在应用更新后已不存在（现有运行时为 `cua_node/1cb4becc994cbb02`）。
- 日志证据（`C:\Users\hh\.codex\logs_2.sqlite`）：
  - `2026-08-17T15:51:06-15Z` 连续 3 条 `exec_command failed ... CreateProcess { message: "Rejected(\"Failed to create unified exec process: helper_unknown_error: setup refresh had errors\")" }`；
  - `windowsSandbox/setupStart` 于 15:50:36Z 发起，`cap_sid` 于 15:51:02Z 生成，失败窗口正好落在沙箱初始化期间；
  - 重启早期 `Failed to connect to github.com:443 after 21070 ms`，导致应用反复重启（15:33-15:50 共 8 个实例）。

## 实施步骤（已执行）

1. **诊断**：读 config.toml，验证所有引用路径；确认 `2f053e67fec2d258` 目录不存在、`1cb4becc994cbb02\bin\node_repl.exe` 与 `codex-computer-use.exe` 存在。
2. **修复**：仅替换 notify 路径中的运行时哈希（`2f053e67fec2d258` → `1cb4becc994cbb02`），保留 UTF-8 无 BOM；先备份 `config.toml.bak-20260817-2359`。
3. **模型核验**：用 `auth.json` 中的 key 调 `https://api.deepseek.com/models`，当前仅 `deepseek-v4-flash` / `deepseek-v4-pro`；实测 `deepseek-chat` 仍可用（返回 `deepseek-v4-flash`），自动化任务不受影响。
4. **环境恢复**：启动 Docker Desktop → 引擎 5 秒就绪 → 内置 K8s 节点约 40 秒全 Ready → `rollout restart` controller-manager（1/1）→ `make cluster-open`（8080/18080）→ 验收。
5. **沉淀**：KNOWN_PITFALLS.md 新增 3 条；本条目四件套；更新 change-history/README.md 索引与 SYNC.md 时间戳。

## 涉及文件（仅文档）

- `docs/agents/KNOWN_PITFALLS.md`（新增"宿主与工具链（2026-08-18）"主题）
- `change-history/README.md`（索引表新增一行）
- `docs/agents/SYNC.md`（头部时间戳）
- `change-history/2026-08-18-host-toolchain-recovery/`（本条目）
