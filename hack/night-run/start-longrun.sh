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

# 运行前体检（长跑强制 sleep-guard；FAIL 项中止启动）
if ! PREFLIGHT_REQUIRE_GUARD=1 bash hack/preflight.sh; then
  echo "ERROR: 运行前体检未通过，长跑不启动（先修复上方 FAIL 项）。" >&2
  exit 1
fi

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
