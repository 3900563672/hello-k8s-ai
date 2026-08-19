#!/usr/bin/env bash
# WSL → Windows 代理（FlClash 等）访问 GitHub：检测 + 配置 + 测速
# 用法：hack/wsl-github-proxy.sh [--check] [-h HOST] [-p PORT ...]
#   --check   只检测不修改 git 配置
#   -h HOST   代理主机（默认 127.0.0.1，WSL2.7+ localhost 共享可直接访问 Windows 代理）
#   -p PORT   代理端口列表（默认 7890 7897 10809 1080）
set -euo pipefail

HOST=127.0.0.1
PORTS=(7890 7897 10809 1080)
CHECK_ONLY=0

usage() {
  echo "用法: $0 [--check] [-h HOST] [-p PORT ...]"
  echo "  --check   只检测，不修改 git 配置"
  echo "  -h HOST   代理主机（默认 127.0.0.1）"
  echo "  -p PORT   代理端口（可多个，默认 7890 7897 10809 1080）"
  exit 0
}

while [ $# -gt 0 ]; do
  case "$1" in
    --check) CHECK_ONLY=1; shift ;;
    -h) HOST="$2"; shift 2 ;;
    -p) read -r -a PORTS <<< "$2"; shift 2 ;;
    --help) usage ;;
    *) echo "未知参数: $1"; usage ;;
  esac
done

find_proxy_port() {
  local port
  for port in "${PORTS[@]}"; do
    if timeout 5 curl -s -o /dev/null -w "%{http_code}" -x "http://${HOST}:${port}" \
        https://api.github.com --max-time 4 2>/dev/null | grep -q "200"; then
      echo "$port"
      return 0
    fi
  done
  return 1
}

echo "== 检测代理（${HOST}）=="
PORT="$(find_proxy_port)" || {
  echo "未检测到可用代理：请确认 Windows 侧 FlClash 已启动、混合端口已开启。"
  echo "提示：WSL2.7+ 用 127.0.0.1 直接访问 Windows localhost 代理，无需 Allow LAN。"
  exit 1
}
echo "可用代理: http://${HOST}:${PORT}"

if [ "$CHECK_ONLY" -eq 0 ]; then
  git config --global "http.https://github.com.proxy" "http://${HOST}:${PORT}"
  echo "已配置: git config --global http.https://github.com.proxy http://${HOST}:${PORT}"
fi

echo "== 测速 github.com =="
RESULT="$(timeout 15 curl -s -o /dev/null -w 'HTTP %{http_code}, 耗时 %{time_total}s' \
  -x "http://${HOST}:${PORT}" https://github.com --max-time 12)" || RESULT="失败"
echo "github.com 走代理: ${RESULT}"

CUR="$(git config --global --get "http.https://github.com.proxy" || true)"
echo "== 当前 git 代理配置: ${CUR:-（未配置）} =="
echo "完成。GitHub 操作建议用 https（SSH 不走 http 代理）。"
