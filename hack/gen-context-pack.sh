#!/usr/bin/env bash
# 生成远程 AI 上下文包：CONTEXT_PACK.md + 关键文件副本 + tar.gz
# 输出到 .runtime/context-pack/（已被 .gitignore 忽略，不提交）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/.runtime/context-pack"
PKG="$OUT/hello-k8s-ai-context-pack"
rm -rf "$PKG"
mkdir -p "$PKG/docs" "$PKG/change-history"

# 动态数据
GENERATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
BRANCH="$(git -C "$ROOT" branch --show-current)"
RECENT_COMMITS="$(git -C "$ROOT" log --oneline -10 | sed 's/^/  - /')"
OPEN_ISSUES="$(gh issue list --state open --limit 20 -R 3900563672/hello-k8s-ai 2>/dev/null | sed 's/^/  /' || echo '  （无法读取：生成环境无 gh 或未认证）')"
TREE="$(cd "$ROOT" && find api cmd internal simulator dashboard config docs change-history test -maxdepth 2 -type d 2>/dev/null | sort | sed 's/^/  /')"

# 用模板渲染 CONTEXT_PACK.md
python3 - "$ROOT/hack/context-pack-template.md" "$PKG/CONTEXT_PACK.md" "$GENERATED_AT" "$BRANCH" "$RECENT_COMMITS" "$OPEN_ISSUES" "$TREE" <<'PYEOF'
import sys
tpl, out, generated, branch, commits, issues, tree = sys.argv[1:8]
text = open(tpl, encoding="utf-8").read()
text = (text.replace("__GENERATED_AT__", generated)
            .replace("__BRANCH__", branch)
            .replace("__RECENT_COMMITS__", commits)
            .replace("__OPEN_ISSUES__", issues)
            .replace("__TREE__", tree))
open(out, "w", encoding="utf-8").write(text)
PYEOF

# 复制关键入口文件与文档（源码树由 tar 步骤整体带上）
cp "$ROOT/AGENTS.md" "$ROOT/README.md" "$ROOT/PROJECT_OVERVIEW_NEW.md" "$ROOT/CHANGELOG.md" "$ROOT/go.mod" "$PKG/"
cp -r "$ROOT/docs" "$PKG/"
cp -r "$ROOT/change-history" "$PKG/"

# 打包整个工作树（排除构建/运行/体积类产物，与 .gitignore 保持一致）
tar -czf "$OUT/hello-k8s-ai-context-pack.tar.gz" \
  --exclude=".git" --exclude=".runtime" --exclude=".idea" --exclude=".verify" \
  --exclude="bin" --exclude="dist" --exclude="output" --exclude="tmp" \
  --exclude="project-review" --exclude="node_modules" \
  --exclude="cover.out" --exclude="hello-k8s-ai.tar.gz" --exclude="*.pdf" \
  -C "$ROOT" . -C "$PKG" CONTEXT_PACK.md

echo "上下文包已生成："
echo "  CONTENT_PACK: $PKG/CONTEXT_PACK.md"
echo "  目录副本:     $PKG/"
echo "  压缩包:       $OUT/hello-k8s-ai-context-pack.tar.gz"