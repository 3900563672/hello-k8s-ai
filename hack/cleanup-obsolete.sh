#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

[[ -f "$ROOT_DIR/Makefile" && -f "$ROOT_DIR/docs/README.md" ]] || {
  printf '拒绝清理：当前目录不像 hello-k8s-ai 项目。\n' >&2
  exit 1
}

# 仅列出已经确认废弃或可以重新生成的内容，不使用通配递归删除。
obsolete_paths=(
  ".idea"
  ".audit"
  "bin"
  "cover.out"
  "coverage.html"
  "output"
  "tmp"
  "config/kind"
  "dashboard/frontend/my-app/.audit"
  "dashboard/frontend/my-app/dist"
  "dashboard/frontend/my-app/node_modules"
  "dashboard/frontend/my-app/verification/dist"
  "simulator/.idea"
  "dashboard/frontend/my-app/DOCS"
  "dashboard/frontend/my-app/pnpm-lock.yaml"
  "dashboard/frontend/my-app/yarn.lock"
  "dashboard/frontend/my-app/bun.lock"
  "dashboard/frontend/my-app/bun.lockb"
  "docs/yaml"
  "docs/DOCUMENTATION_MIGRATION.md"
  "docs/OBSERVABILITY.md"
  "docs/VALIDATION.md"
  "docs/VERIFICATION_REPORT.md"
  "OBSERVABILITY.md"
  "VALIDATION_REPORT.md"
  "package-lock.json"
  "hello-k8s-ai-architecture-guide.pdf"
  "hello-k8s-ai-complete-overview.pdf"
)

removed=0
for relative_path in "${obsolete_paths[@]}"; do
  target="$ROOT_DIR/$relative_path"
  [[ -e "$target" || -L "$target" ]] || continue
  rm -rf -- "$target"
  printf '已清理：%s\n' "$relative_path"
  removed=$((removed + 1))
done

if (( removed == 0 )); then
  printf '没有发现需要清理的旧文件。\n'
else
  printf '共清理 %d 项。未触碰 .git、源码、CRD、数据库或集群数据。\n' "$removed"
fi
