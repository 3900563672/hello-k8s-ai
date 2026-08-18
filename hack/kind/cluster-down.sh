#!/usr/bin/env bash
# 删除 Kind 开发集群。
# 注意：PVC 数据在 /var/lib/hello-k8s-ai-pv（docker_data.vhdx 内），删除集群不丢数据，
#       下次 cluster-up 重建后自动挂回。
# 用法：bash hack/kind/cluster-down.sh
set -Eeuo pipefail

CLUSTER="${DEV_KIND_CLUSTER:-hello-k8s-ai-dev}"
KIND="${KIND:-kind}"

"$KIND" delete cluster --name "$CLUSTER"
echo "已删除 Kind 集群 $CLUSTER（PVC 数据保留在 /var/lib/hello-k8s-ai-pv）"