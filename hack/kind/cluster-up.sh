#!/usr/bin/env bash
# 开发/演示底座：创建 Kind 集群（幂等）并安装 local-path 持久化存储。
# 用法：bash hack/kind/cluster-up.sh
# 环境变量：DEV_KIND_CLUSTER / KIND_NODE_IMAGE / KIND / KUBECTL
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

KIND="${KIND:-kind}"
KUBECTL="${KUBECTL:-kubectl}"
CLUSTER="${DEV_KIND_CLUSTER:-hello-k8s-ai-dev}"
CTX="kind-$CLUSTER"
KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.36.1@sha256:3489c7674813ba5d8b1a9977baea8a6e553784dab7b84759d1014dbd78f7ebd5}"

command -v "$KIND" >/dev/null 2>&1 || {
  echo "错误：kind 未安装。参考 docs/operations/ 安装 Kind。" >&2
  exit 1
}

if "$KIND" get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  echo "Kind 集群 $CLUSTER 已存在，跳过创建"
else
  echo "创建 Kind 集群 $CLUSTER（1 control-plane + 4 worker，节点收敛 5）..."
  "$KIND" create cluster --name "$CLUSTER" \
    --image "$KIND_NODE_IMAGE" \
    --config "$ROOT_DIR/hack/kind/kind-5node.yaml"
fi

echo "安装 local-path 持久化存储（PVC 数据 -> /var/lib/hello-k8s-ai-pv）..."
"$KUBECTL" --context "$CTX" apply -f "$ROOT_DIR/hack/kind/local-path-provisioner.yaml"
"$KUBECTL" --context "$CTX" apply -f "$ROOT_DIR/hack/kind/storageclass-standard.yaml"
"$KUBECTL" --context "$CTX" -n local-path-storage rollout status \
  deployment/local-path-provisioner --timeout=180s >/dev/null

"$KUBECTL" config use-context "$CTX" >/dev/null

NODES=$("$KUBECTL" --context "$CTX" get nodes --no-headers | wc -l)
echo "OK：context=$CTX，节点 $NODES 个，StorageClass standard 就绪"