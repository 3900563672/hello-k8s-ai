#!/usr/bin/env bash
# 生成远程 AI 上下文包：CONTEXT_PACK.md + 源码 + AI 文档 + tar.gz
# 输出到 .runtime/context-pack/（已被 .gitignore 忽略，不提交）
# 用法：make context-pack           默认包（不含 docs/ 人类专题）
#       make context-pack FULL=1    全量包（包含 docs/ 人类专题）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/.runtime/context-pack"
PKG="$OUT/hello-k8s-ai-context-pack"
rm -rf "$PKG"
mkdir -p "$PKG/docs"

# 动态数据
GENERATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
BRANCH="$(git -C "$ROOT" branch --show-current)"
RECENT_COMMITS="$(git -C "$ROOT" log --oneline -10 | sed 's/^/  - /')"
OPEN_ISSUES="$(gh issue list --state open --limit 20 -R 3900563672/hello-k8s-ai 2>/dev/null | sed 's/^/  /' || echo '  （无法读取：生成环境无 gh 或未认证）')"
TREE="$(cd "$ROOT" && find api cmd internal simulator dashboard config docs change-history test -maxdepth 2 -type d 2>/dev/null | sort | sed 's/^/  /')"
MODE="$([ "${FULL:-0}" = "1" ] && echo "full（包含 docs/ 人类专题）" || echo "default（不含 docs/ 人类专题）")"

# 用模板渲染 CONTEXT_PACK.md
python3 - "$ROOT/hack/context-pack-template.md" "$PKG/CONTEXT_PACK.md" "$GENERATED_AT" "$BRANCH" "$RECENT_COMMITS" "$OPEN_ISSUES" "$TREE" "$MODE" <<'PYEOF'
import sys
tpl, out, generated, branch, commits, issues, tree, mode = sys.argv[1:9]
text = open(tpl, encoding="utf-8").read()
text = (text.replace("__GENERATED_AT__", generated)
            .replace("__BRANCH__", branch)
            .replace("__RECENT_COMMITS__", commits)
            .replace("__OPEN_ISSUES__", issues)
            .replace("__TREE__", tree)
            .replace("__MODE__", mode))
open(out, "w", encoding="utf-8").write(text)
PYEOF

# 复制入口文件与根构建文件
cp "$ROOT/AGENTS.md" "$ROOT/README.md" "$ROOT/PROJECT_OVERVIEW_NEW.md" "$ROOT/CHANGELOG.md" \
   "$ROOT/go.mod" "$ROOT/go.sum" "$ROOT/Makefile" "$ROOT/Dockerfile" "$ROOT/setup.sh" \
   "$ROOT/PROJECT" "$ROOT/.golangci.yml" "$ROOT/.custom-gcl.yml" "$PKG/" 2>/dev/null || true

# 复制源码（事实源）
cp -r "$ROOT/api" "$ROOT/cmd" "$ROOT/internal" "$ROOT/simulator" "$ROOT/config" "$ROOT/test" "$PKG/" 2>/dev/null || true
cp -r "$ROOT/dashboard" "$PKG/" 2>/dev/null || true

# 文档：默认只带 AI 两层；FULL=1 时带全部
if [ "${FULL:-0}" = "1" ]; then
  cp -r "$ROOT/docs" "$PKG/"
else
  cp -r "$ROOT/docs/agents" "$ROOT/docs/remote-ai" "$PKG/docs/"
fi

# 时间线（全部层共享）
cp -r "$ROOT/change-history" "$PKG/"

# 打包 PKG 全部内容
tar -czf "$OUT/hello-k8s-ai-context-pack.tar.gz" \
  --exclude="node_modules" --exclude="__pycache__" --exclude=".runtime" \
  -C "$PKG" .

echo "上下文包已生成（模式：$MODE）："
echo "  CONTENT_PACK: $PKG/CONTEXT_PACK.md"
echo "  目录副本:     $PKG/"
echo "  压缩包:       $OUT/hello-k8s-ai-context-pack.tar.gz"