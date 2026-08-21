#!/usr/bin/env bash
# AIOps 一键启用（#136 治本）：集群部署后快速打开 AIOps 并接入 DeepSeek。
# 用法:
#   bash hack/aiops-enable.sh                     # 读 .runtime/aiops.env 里的 Key
#   AIOPS_OPENAI_API_KEY=sk-xxx bash hack/aiops-enable.sh   # 显式传入 Key（优先）
# 说明: 不落盘任何 Key；写入 Deployment env，仅进程内存持有；日配额保护默认 300 次/200 万 token。
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

NAMESPACE="${NAMESPACE:-hello-k8s-ai-system}"
DEPLOY="hello-k8s-ai-dashboard-backend"
BASE_URL="${AIOPS_OPENAI_BASE_URL:-https://api.deepseek.com/v1}"
MODEL="${AIOPS_MODEL:-deepseek-v4-flash}"

# 1. 前置检查
command -v kubectl >/dev/null || { echo "缺少 kubectl"; exit 1; }
kubectl -n "$NAMESPACE" get deployment "$DEPLOY" >/dev/null 2>&1 \
  || { echo "未找到 $NAMESPACE/$DEPLOY，请先 make cluster-up"; exit 1; }

# 2. Key 来源：环境变量优先，其次 .runtime/aiops.env
API_KEY="${AIOPS_OPENAI_API_KEY:-}"
if [[ -z "$API_KEY" && -f .runtime/aiops.env ]]; then
  API_KEY="$(sed -n "s/^AIOPS_OPENAI_API_KEY=//p" .runtime/aiops.env | head -1)"
fi
if [[ -z "$API_KEY" ]]; then
  echo "未找到 API Key：请设置 AIOPS_OPENAI_API_KEY 或 .runtime/aiops.env"; exit 1
fi

# 3. 写入 Deployment 环境变量（幂等）
echo "[aiops-enable] 启用 AIOps: enabled=true base=$BASE_URL model=$MODEL"
kubectl -n "$NAMESPACE" set env "deployment/$DEPLOY" \
  AIOPS_ENABLED=true \
  "AIOPS_OPENAI_BASE_URL=$BASE_URL" \
  "AIOPS_MODEL=$MODEL" \
  "AIOPS_OPENAI_API_KEY=$API_KEY" >/dev/null

# 4. 等待滚动
kubectl -n "$NAMESPACE" rollout status "deployment/$DEPLOY" --timeout=180s

# 5. 验证设置接口
echo "[aiops-enable] 验证 /aiops/settings ..."
for _ in $(seq 1 15); do
  OUT="$(curl -sf --max-time 3 http://127.0.0.1:8080/api/v1/aiops/settings 2>/dev/null || true)"
  if [[ -n "$OUT" ]]; then
    echo "$OUT" | jq -r ".data | \"enabled=\\(.enabled) keyConfigured=\\(.keyConfigured) model=\\(.model) base=\\(.baseUrl)\"" 2>/dev/null || echo "$OUT"
    echo "[aiops-enable] 完成：AIOps 已启用。"
    exit 0
  fi
  sleep 2
done
echo "[aiops-enable] 警告：设置接口暂不可达（可能 port-forward 未就绪），请稍后验证 make cluster-status"