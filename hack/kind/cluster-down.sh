#!/usr/bin/env bash
# 删除 Kind 开发集群。
# 注意：PVC 数据在节点容器 /var named volume（Docker 数据盘 vhdx），删除集群不丢数据；
#       但重建后不会自动挂回，需按 hack/kind/restore-data.sh 从备份显式恢复。
# 用法：bash hack/kind/cluster-down.sh
set -Eeuo pipefail

CLUSTER="${DEV_KIND_CLUSTER:-hello-k8s-ai-dev}"
KIND="${KIND:-kind}"

"$KIND" delete cluster --name "$CLUSTER"
echo "已删除 Kind 集群 $CLUSTER（PVC 数据保留在节点 /var 数据卷，重建后需 restore-data.sh 显式恢复）"
