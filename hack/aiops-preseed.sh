#!/usr/bin/env bash
# AIOps 历史预生成（演示用）：批量创建并完成切面实验，让分析 worker 自动产出 AI 分析历史。
# 前置：集群已起、AIOps 已启用且已配置 API Key（make env-up 后按 docs/aiops/AIOPS_OVERVIEW 开启）。
# 用法: bash hack/aiops-preseed.sh [数量]   默认 3 条；API 地址用 AIOPS_API_BASE 覆盖；租户用 PRESEED_TENANT 覆盖。
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COUNT="${1:-3}"
API="${AIOPS_API_BASE:-http://127.0.0.1:8080/api/v1}"
TENANT="${PRESEED_TENANT:-tenant-sample}"
STAMP="$(date +%Y%m%d-%H%M%S)"

[[ "$COUNT" =~ ^[1-9][0-9]*$ ]] || { echo "数量必须是正整数"; exit 2; }

echo "[preseed] 目标 $COUNT 条历史分析（API: $API，租户: $TENANT）"

# 1. 就绪检查
curl -sf --max-time 5 "$API/health/ready" >/dev/null \
  || { echo "后端未就绪：请先启动集群与 Dashboard（make env-up / make cluster-open）。"; exit 1; }

# 2. AIOps 启用与 Key 检查（避免创建了实验却没有分析）
SETTINGS="$(curl -sf --max-time 5 "$API/aiops/settings")" || { echo "读取 AIOps 配置失败。"; exit 1; }
ENABLED="$(jq -r '.data.enabled' <<<"$SETTINGS")"
KEYED="$(jq -r '.data.keyConfigured' <<<"$SETTINGS")"
if [[ "$ENABLED" != "true" || "$KEYED" != "true" ]]; then
  echo "AIOps 未启用或未配置 API Key（enabled=$ENABLED, keyConfigured=$KEYED）。"
  echo "请在 Dashboard 右下角 AI 助手 → 设置中打开开关并填入 Key（或检查 .runtime/aiops.env）。"
  exit 1
fi

# 3. 创建 → 开始 → 完成 实验（complete/fail 自动入队分析）
for i in $(seq 1 "$COUNT"); do
  NAME="预生成演示 $i ($STAMP)"
  CREATE="$(curl -sf --max-time 10 -X POST "$API/experiments" \
    -H 'Content-Type: application/json' \
    -H "Idempotency-Key: preseed-create-$i-$STAMP" \
    -d "{\"tenant\":\"$TENANT\",\"name\":\"$NAME\"}")" || { echo "创建实验 $i 失败"; exit 1; }
  SEG="$(jq -r '.data.segment.segmentId' <<<"$CREATE")"
  [[ -n "$SEG" && "$SEG" != "null" ]] || { echo "创建实验 $i 响应异常"; exit 1; }
  curl -sf --max-time 10 -X POST "$API/experiments/$SEG/start" -H "Idempotency-Key: preseed-start-$SEG" >/dev/null \
    || { echo "开始实验 $SEG 失败"; exit 1; }
  curl -sf --max-time 10 -X POST "$API/experiments/$SEG/complete" -H "Idempotency-Key: preseed-complete-$SEG" >/dev/null \
    || { echo "完成实验 $SEG 失败"; exit 1; }
  echo "  已创建并完成: $SEG"
done

# 4. 轮询分析完成（worker 异步处理，含 LLM 调用；上限 5 分钟）
echo "[preseed] 等待分析完成（worker 异步处理中，最长 5 分钟）..."
DEADLINE=$((SECONDS + 300))
DONE=0
while (( SECONDS < DEADLINE )); do
  DONE="$(curl -sf --max-time 5 "$API/aiops/analyses?status=completed&limit=100" \
    | jq '.data | length' 2>/dev/null || echo 0)"
  (( DONE >= COUNT )) && break
  sleep 10
done

echo "[preseed] 已完成分析 $DONE 条（目标 $COUNT）"
if (( DONE == 0 )); then
  echo "提示：若一直为 0，请检查 AIOps 开关/Key、日配额（AIOPS_DAILY_MAX_*）以及后端日志。"
fi
curl -sf --max-time 5 "$API/aiops/analyses?limit=$COUNT" | jq -r \
  '.data[] | "  \(.segmentId)  \(.status)  attempts=\(.attempts // 0)  \(.error // "")"'