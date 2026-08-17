#!/usr/bin/env bash
# 一键启动长时运行（默认跑到本地 18:00 自动停止，恢复 35qps）。
# 用法:
#   bash hack/night-run/start-longrun.sh                # 默认 200/350 剧本，--until 18:00
#   BASELINE_QPS=150 PEAK_QPS=300 bash hack/night-run/start-longrun.sh
#   UNTIL=20:00 bash hack/night-run/start-longrun.sh
# 环境变量: BASELINE_QPS / PEAK_QPS / PEAK_MINUTES / CYCLE_MINUTES / INTERVAL /
#           UNTIL(本地 HH:MM) / FINAL_QPS / TENANT
set -euo pipefail
cd "$(dirname "$0")/../.."

BASELINE_QPS="${BASELINE_QPS:-200}"
PEAK_QPS="${PEAK_QPS:-350}"
PEAK_MINUTES="${PEAK_MINUTES:-15}"
CYCLE_MINUTES="${CYCLE_MINUTES:-60}"
INTERVAL="${INTERVAL:-900}"
UNTIL="${UNTIL:-18:00}"
FINAL_QPS="${FINAL_QPS:-35}"
TENANT="${TENANT:-tenant-core}"

RUN_DIR=".runtime/longrun/$(date +%F)"
mkdir -p "$RUN_DIR"

# 停掉旧的 day-watch（幂等，避免双剧本打架）
for pid in $(pgrep -f 'hack/night-run/day-watch.mjs' || true); do
  echo ">>> stop old day-watch pid=$pid"
  kill "$pid" 2>/dev/null || true
done
sleep 2

# 前置快查：18080 与 sleep-guard（脚本内还有完整 preflight，这里只提示）
if ! curl -s -m 5 -o /dev/null http://localhost:18080/api/v1/health/live; then
  echo "WARN: Backend 18080 不可达，脚本会继续启动（keepalive 会尝试恢复端口转发）" >&2
fi
guard="$(bash hack/night-run/sleep-guard.sh status 2>/dev/null | tail -1)"
echo "sleep-guard: $guard"
case "$guard" in
  *guard=on*) ;;
  *) echo "WARN: sleep-guard 未开启，长跑可能被宿主机休眠打断（需要 UAC 点确认）" >&2 ;;
esac

setsid nohup node hack/night-run/day-watch.mjs --loop --interval "$INTERVAL" \
  --until "$UNTIL" \
  --baseline-qps "$BASELINE_QPS" --peak-qps "$PEAK_QPS" \
  --peak-minutes "$PEAK_MINUTES" --cycle-minutes "$CYCLE_MINUTES" \
  --final-qps "$FINAL_QPS" --tenant "$TENANT" \
  < /dev/null >> "$RUN_DIR/day-watch.log" 2>&1 &

PID=$!
echo "longrun started: pid=$PID"
echo "剧本: baseline=${BASELINE_QPS}qps peak=${PEAK_QPS}qps cycle=${CYCLE_MINUTES}min(peak ${PEAK_MINUTES}min) interval=${INTERVAL}s"
echo "结束: 本地 ${UNTIL}（自动恢复 ${FINAL_QPS}qps 并生成 summary.md）"
echo "日志: $RUN_DIR/day-watch.log"
echo "产物: $RUN_DIR/rounds/ 与 $RUN_DIR/snapshots/"
