#!/usr/bin/env bash
# AIOps 模板预置（演示用）：创建 10 模型 + 10 租户 + 10 节点 CR 及对应关系策略。
# 模板 id 与集群 CR 名一一对应（preset-model-001..010 / preset-tenant-001..010 / preset-node-001..010），
# 与 backend internal/aiops/command.go 的 TemplateCatalog 对齐；租户 qps 全部 0 = 空环境，无预置流量。
# 用法: bash hack/aiops-templates-seed.sh [--dry-run]
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

DRY_RUN=0
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=1
APPLY_CMD="kubectl apply -f -"
[[ "$DRY_RUN" == "1" ]] && APPLY_CMD="cat"

command -v kubectl >/dev/null || { echo "缺少 kubectl"; exit 2; }
kubectl cluster-info >/dev/null 2>&1 || { echo "集群不可用，请先 make cluster-up"; exit 1; }

OUT="$(mktemp)"
trap 'rm -f "$OUT"' EXIT

# 模型：gpuUnits / maxConcurrency / absoluteScore / coldStartMs（与模板目录 Description 一致）
MODELS=( "001|轻量在线推理|8|16|75|800" "002|标准在线推理|16|32|100|1500" "003|批量离线任务|32|64|60|5000" "004|高并发推荐|64|96|95|1200" "005|图像生成服务|40|24|85|3000" "006|语音实时转写|24|40|90|1000" "007|向量检索服务|12|48|88|900" "008|多模态理解|48|32|92|2500" "009|代码补全|20|64|87|1100" "010|长文本摘要|16|20|78|4000" )
# 节点：gpu / maxConcurrency
NODES=( "001|高并发 GPU 池|80|128" "002|标准 GPU 节点|32|48" "003|边缘轻量节点|8|16" "004|推理加速节点|48|96" "005|大显存节点|64|64" "006|训练节点|72|32" "007|弹性扩缩节点|40|80" "008|高可用节点|56|72" "009|通用计算节点|24|56" "010|混合负载节点|16|40" )
# 租户：priority / qps=0 / ttftThresholdMs / queueThreshold / ttftScaleDownThresholdMs / queueScaleDownThreshold
TENANTS=( "001|核心在线业务|P1|0|800|150|300|40" "002|一般在线业务|P3|0|500|100|200|30" "003|离线分析批|P5|0|2000|80|800|25" "004|实时风控|P1|0|300|200|120|60" "005|搜索服务|P2|0|400|120|160|35" "006|视频渲染批|P5|0|3000|60|1200|20" "007|交互式助手|P2|0|600|130|240|40" "008|数据管道|P4|0|1500|90|600|30" "009|模型微调任务|P4|0|2500|70|1000|22" "010|边缘推理|P3|0|700|110|280|33" )

MODEL_NAMES=() NODE_NAMES=() TENANT_NAMES=()
for entry in "${MODELS[@]}"; do IFS='|' read -r n _ _ _ _ <<<"$entry"; MODEL_NAMES+=("preset-model-$n"); done
for entry in "${NODES[@]}"; do IFS='|' read -r n _ _ <<<"$entry"; NODE_NAMES+=("preset-node-$n"); done
for entry in "${TENANTS[@]}"; do IFS='|' read -r n _ _ <<<"$entry"; TENANT_NAMES+=("preset-tenant-$n"); done

append() { printf '%s\n' "$1" >>"$OUT"; }

for entry in "${MODELS[@]}"; do
  IFS='|' read -r n name gpu conc score cold <<<"$entry"
  append "apiVersion: platform.study.com/v1"
  append "kind: Model"
  append "metadata:"
  append "  labels:"
  append "    app.kubernetes.io/name: hello-k8s-ai"
  append "    app.kubernetes.io/managed-by: aiops-templates-seed"
  append "  name: preset-model-$n"
  append "spec:"
  append "  displayName: $name"
  append "  gpuUnits: $gpu"
  append "  maxConcurrency: $conc"
  append "  absoluteScore: $score"
  append "  coldStartMs: $cold"
  append "---"
done

for entry in "${NODES[@]}"; do
  IFS='|' read -r n name gpu conc <<<"$entry"
  append "apiVersion: platform.study.com/v1"
  append "kind: WorkerNode"
  append "metadata:"
  append "  labels:"
  append "    app.kubernetes.io/name: hello-k8s-ai"
  append "    app.kubernetes.io/managed-by: aiops-templates-seed"
  append "  name: preset-node-$n"
  append "spec:"
  append "  displayName: $name"
  append "  gpu: $gpu"
  append "  maxConcurrency: $conc"
  append "---"
done

for entry in "${TENANTS[@]}"; do
  IFS='|' read -r n name pri qps ttft qthr tdown qdown <<<"$entry"
  append "apiVersion: platform.study.com/v1"
  append "kind: Tenant"
  append "metadata:"
  append "  labels:"
  append "    app.kubernetes.io/name: hello-k8s-ai"
  append "    app.kubernetes.io/managed-by: aiops-templates-seed"
  append "  name: preset-tenant-$n"
  append "spec:"
  append "  displayName: $name"
  append "  priority: $pri"
  append "  qps: $qps"
  append "  ttftThresholdMs: $ttft"
  append "  queueThreshold: $qthr"
  append "  ttftScaleDownThresholdMs: $tdown"
  append "  queueScaleDownThreshold: $qdown"
  append "---"
done

# 关系策略：模型-节点（每个模型绑主节点 + 下一号，环回）
for i in "${!MODEL_NAMES[@]}"; do
  m="${MODEL_NAMES[$i]}"; n1="${NODE_NAMES[$i]}"; n2="${NODE_NAMES[$(((i + 1) % 10))]}"
  for n in "$n1" "$n2"; do
    append "apiVersion: platform.study.com/v1"
    append "kind: ModelNodePolicy"
    append "metadata:"
    append "  labels:"
    append "    app.kubernetes.io/name: hello-k8s-ai"
    append "    app.kubernetes.io/managed-by: aiops-templates-seed"
    append "  name: mnp-$m-$n"
    append "spec:"
    append "  modelRef:"
    append "    name: $m"
    append "  nodeRef:"
    append "    name: $n"
    append "  effect: Allow"
    append "---"
  done
done

# 租户-节点（每个租户绑主节点 + 后两号，环回）
for i in "${!TENANT_NAMES[@]}"; do
  t="${TENANT_NAMES[$i]}"
  for off in 0 1 2; do
    n="${NODE_NAMES[$(((i + off) % 10))]}"
    append "apiVersion: platform.study.com/v1"
    append "kind: TenantNodePolicy"
    append "metadata:"
    append "  labels:"
    append "    app.kubernetes.io/name: hello-k8s-ai"
    append "    app.kubernetes.io/managed-by: aiops-templates-seed"
    append "  name: tnp-$t-$n"
    append "spec:"
    append "  tenantRef:"
    append "    name: $t"
    append "  nodeRef:"
    append "    name: $n"
    append "  effect: Allow"
    append "---"
  done
done

# 租户-模型（每个租户绑主模型 + 下一号，环回）
for i in "${!TENANT_NAMES[@]}"; do
  t="${TENANT_NAMES[$i]}"
  for off in 0 1; do
    m="${MODEL_NAMES[$(((i + off) % 10))]}"
    append "apiVersion: platform.study.com/v1"
    append "kind: TenantModelPolicy"
    append "metadata:"
    append "  labels:"
    append "    app.kubernetes.io/name: hello-k8s-ai"
    append "    app.kubernetes.io/managed-by: aiops-templates-seed"
    append "  name: tmp-$t-$m"
    append "spec:"
    append "  tenantRef:"
    append "    name: $t"
    append "  modelRef:"
    append "    name: $m"
    append "  effect: Allow"
    append "---"
  done
done

if [[ "$DRY_RUN" == "1" ]]; then
  echo "--- 预置清单（dry-run）：$(grep -c '^kind: Model$' "$OUT") 模型 + $(grep -c '^kind: WorkerNode$' "$OUT") 节点 + $(grep -c '^kind: Tenant$' "$OUT") 租户 + $(grep -c '^kind: .*Policy' "$OUT") 策略 ---"
  $APPLY_CMD <"$OUT" | tail -1
else
  echo "[seed] 预置模板资源（幂等，已存在将更新）..."
  $APPLY_CMD <"$OUT" | grep -c 'configured\|created\|unchanged' || true
  echo "[seed] 完成。校验："
  for kind in Model WorkerNode Tenant ModelNodePolicy TenantNodePolicy TenantModelPolicy; do
    echo "  $kind: $(kubectl get "$kind" --no-headers 2>/dev/null | wc -l)"
  done
fi
