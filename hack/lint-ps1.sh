#!/usr/bin/env bash
# PSScriptAnalyzer 静态检查仓库内 .ps1（Windows interop 环境专用；非 Windows 自动跳过）
# 配套：make lint-ps1 ｜ 规则：告警即失败，禁止带告警提交
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

PS1_FILES="$(find . -name '*.ps1' -not -path './.git/*' -not -path './node_modules/*')"
if [ -z "$PS1_FILES" ]; then
  echo "无 .ps1 文件，跳过"
  exit 0
fi

if ! command -v powershell.exe >/dev/null 2>&1; then
  echo "WARN: powershell.exe 不可用（非 Windows 环境），跳过 PSScriptAnalyzer"
  exit 0
fi

if ! powershell.exe -NoProfile -Command 'if (-not (Get-Module -ListAvailable PSScriptAnalyzer)) { exit 1 }' >/dev/null 2>&1; then
  echo "缺少 PSScriptAnalyzer：请在 Windows PowerShell 执行 Install-Module -Name PSScriptAnalyzer -Scope CurrentUser -Force" >&2
  exit 1
fi

FAIL=0
while IFS= read -r f; do
  [ -n "$f" ] || continue
  rel="${f#./}"
  out="$(powershell.exe -NoProfile -Command "Invoke-ScriptAnalyzer -Path '$rel' -Severity Warning | Out-String -Width 200" 2>&1 | tr -d '\r' || true)"
  if [ -n "$out" ]; then
    echo "[$rel]"
    echo "$out"
    FAIL=1
  fi
done <<<"$PS1_FILES"

if [ "$FAIL" -eq 1 ]; then
  echo "PSScriptAnalyzer 发现告警，请修复后再提交" >&2
  exit 1
fi
echo "PSScriptAnalyzer OK"
