# 跨平台文件写入卫生：UTF-8 BOM 与 Git 执行位噪音（Windows ↔ WSL）

> 提升日期：2026-08-19 ｜ 来源：2026-08-19 仓库体检 CI 修复与直方图运行期间排障 ｜ 适用对象：本地 Agent

## 现象

- 251 个仓库文件整体出现 `old mode 100755 / new mode 100644` 的"幽灵差异"（0 行内容变化）：Windows Git 经 `\\wsl.localhost\...` UNC 访问 WSL 仓库时，把全部可执行文件的执行位判定为丢失，一旦 `git add` 会整体污染提交。
- PowerShell `Set-Content -Encoding UTF8` 写入仓库文档后，文件头被加上 UTF-8 BOM（`EF BB BF`）；`hack/gen-docs.py` 的 `h1_of()` 按 "# "（井号+空格）开头匹配 H1 失败，`llms.txt` 条目回退成裸文件名（如 `TROUBLESHOOTING.md`），`make docs-sync-check` 在 CI 直接失败。

## 根因

- 跨文件系统访问（9P/UNC）下 exec bit 的 stat 结果不稳定，Windows Git 在 UNC 上把不可靠的执行位报告为丢失（仓库本地 `core.fileMode` 为 true 时尤甚）。
- PowerShell 5.1 `Set-Content -Encoding UTF8` 必然写 BOM；`gen-docs.py` 用无 BOM 的 UTF-8 读取，首行 `# 标题` 变成 `\ufeff# 标题`，所有按前缀匹配的解析都会静默失败（标题回退成文件名只是最温和的症状）。

## 可复用规则

- 仓库内任何文件写入一律在 WSL 侧用 Python `io.open(path, 'w', encoding='utf-8')`（无 BOM）；禁止 PowerShell `Set-Content` / `Out-File` 直接改写仓库文件。PowerShell 7+ 的 `-Encoding utf8NoBOM` 只可用于写临时脚本，不改仓库文件。
- Windows Git 操作该仓库前，确认 `core.fileMode false`（已写入 `.git/config`）；若再出现整批 `100755 → 100644`：`git config core.fileMode false`，并在 WSL 内 `git ls-files -s | awk '$1 == "100755" {print $4}' | xargs -r chmod +x` 恢复真实执行位。
- `docs-sync-check` 失败且差异只有 `docs/remote-ai/llms.txt` 时，先查源文档 H1 是否被 BOM 污染，再 `make docs-sync`。
- 提交前抽查：`head -c 3 <新写/改写的 .md> | xxd` 无 `efbbbf`；`git diff --summary` 无 `mode change` 噪音。

## 验证方法

- BOM：`head -c 3 docs/operations/TROUBLESHOOTING.md | xxd`（期望无 `efbbbf`）；`make docs-sync-check` 全绿。
- mode：未改动时 `git status --short` 为空；`git diff --summary | grep 'mode change'` 无输出。
