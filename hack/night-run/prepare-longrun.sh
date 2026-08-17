#!/usr/bin/env bash
# 长时测试前准备：把 WorkerNode 并发容量放大到目标副本数（幂等，可重复执行）。
# 用法:
#   bash hack/night-run/prepare-longrun.sh                  # 默认目标 120 副本
#   TARGET_REPLICAS=200 bash hack/night-run/prepare-longrun.sh
# 之后启动流量剧本（另开终端）:
#   node hack/night-run/day-watch.mjs --baseline-qps 200 --peak-qps 350 \
#        --peak-minutes 15 --cycle-minutes 60 --interval 900
set -euo pipefail

TARGET_REPLICAS="${TARGET_REPLICAS:-120}"
CONTEXT="${CONTEXT:-docker-desktop}"
MODEL="${MODEL:-model-lite}"
MODEL_CONCURRENCY="$(kubectl --context "$CONTEXT" get models "$MODEL" -o jsonpath='{.spec.maxConcurrency}')"
MODEL_GPU="$(kubectl --context "$CONTEXT" get models "$MODEL" -o jsonpath='{.spec.gpuUnits}')"

NODES="$(kubectl --context "$CONTEXT" get workernodes -o jsonpath='{.items[*].metadata.name}')"
read -ra NODE_LIST <<< "$NODES"
NODE_COUNT="${#NODE_LIST[@]}"
if [ "$NODE_COUNT" -eq 0 ]; then
  echo "错误: 没有可用 WorkerNode" >&2
  exit 1
fi

# 每节点并发 = ceil(目标副本数 × 模型并发 / 节点数)，GPU 同理
PER_NODE_CONCURRENCY=$(( (TARGET_REPLICAS * MODEL_CONCURRENCY + NODE_COUNT - 1) / NODE_COUNT ))
PER_NODE_GPU=$(( (TARGET_REPLICAS * MODEL_GPU + NODE_COUNT - 1) / NODE_COUNT ))

echo "目标: $TARGET_REPLICAS 副本 × $MODEL_CONCURRENCY 并发 = 共 ${TARGET_REPLICAS}x${MODEL_CONCURRENCY} 并发"
echo "节点: ${NODE_LIST[*]} ($NODE_COUNT 个)，每个 maxConcurrency=$PER_NODE_CONCURRENCY gpu=$PER_NODE_GPU"
echo

for node in "${NODE_LIST[@]}"; do
  echo ">>> patch $node"
  kubectl --context "$CONTEXT" patch workernode "$node" --type merge -p "{\"spec\":{\"maxConcurrency\":$PER_NODE_CONCURRENCY,\"gpu\":$PER_NODE_GPU}}"
done

echo
echo "验证（剩余容量应为正且够放目标副本）:"
kubectl --context "$CONTEXT" get workernodes -o custom-columns=NAME:.metadata.name,CONC:.spec.maxConcurrency,USED:.status.usedConcurrency,GPU:.spec.gpu,USED_GPU:.status.usedGPU
echo
echo "容量上限: $((NODE_COUNT * PER_NODE_CONCURRENCY / MODEL_CONCURRENCY)) 副本（目标 $TARGET_REPLICAS）"
echo "接下来启动剧本: node hack/night-run/day-watch.mjs --baseline-qps 200 --peak-qps 350 --peak-minutes 15 --cycle-minutes 60 --interval 900"
