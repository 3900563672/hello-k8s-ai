#!/usr/bin/env bash
# #50 迁移恢复：PostgreSQL（psql）+ Prometheus TSDB + Jaeger badger。
# 前提：新集群 make cluster-up 已部署完成，三套 PVC 已 Bound。
# 用法：BACKUP_DIR=/var/tmp/hello-k8s-ai-backup-* bash hack/kind/restore-data.sh
# 环境变量：KUBE_CONTEXT / NAMESPACE / BACKUP_DIR
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

KUBE_CONTEXT="${KUBE_CONTEXT:-kind-hello-k8s-ai-dev}"
NAMESPACE="${NAMESPACE:-hello-k8s-ai-system}"
BACKUP_DIR="${BACKUP_DIR:-$(ls -dt /var/tmp/hello-k8s-ai-backup-* 2>/dev/null | head -1)}"
[[ -d "$BACKUP_DIR" && -f "$BACKUP_DIR/dashboard.sql" ]] || {
  echo "错误：找不到备份目录（需含 dashboard.sql）。BACKUP_DIR=$BACKUP_DIR" >&2
  exit 1
}

kube() { kubectl --context "$KUBE_CONTEXT" "$@"; }
step() { printf '\n[restore] %s\n' "$*"; }

step "1/3 PostgreSQL 恢复 <- $BACKUP_DIR/dashboard.sql"
kube -n "$NAMESPACE" exec "statefulset/hello-k8s-ai-dashboard-postgresql" -i -- \
  psql -U dashboard -d dashboard -v ON_ERROR_STOP=0 < "$BACKUP_DIR/dashboard.sql"

restore_pvc_dir() {
  local name="$1" claim="$2" src_path="$3" in_file="$4"
  step "恢复 $name <- $in_file"
  kube -n "$NAMESPACE" scale "deployment/$name" --replicas=0 >/dev/null
  kube -n "$NAMESPACE" rollout status "deployment/$name" --timeout=120s >/dev/null 2>&1 || true
  kube -n "$NAMESPACE" apply -f - >/dev/null <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: restore-$name
  namespace: $NAMESPACE
spec:
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: $claim
  - name: in
    emptyDir: {}
  containers:
  - name: unpack
    image: busybox
    command: ["sh", "-c", "tar xzf /in/data.tar.gz -C $src_path && sleep 3600"]
    volumeMounts:
    - name: data
      mountPath: $src_path
    - name: in
      mountPath: /in
  restartPolicy: Never
EOF
  kubectl --context "$KUBE_CONTEXT" -n "$NAMESPACE" wait --for=condition=Ready "pod/restore-$name" --timeout=120s >/dev/null
  kubectl --context "$KUBE_CONTEXT" cp "$in_file" "$NAMESPACE/restore-$name:/in/data.tar.gz"
  kubectl --context "$KUBE_CONTEXT" -n "$NAMESPACE" exec "restore-$name" -- sh -c "du -sh $src_path"
  kubectl --context "$KUBE_CONTEXT" -n "$NAMESPACE" delete pod "restore-$name" --wait=false >/dev/null
  kube -n "$NAMESPACE" scale "deployment/$name" --replicas=1 >/dev/null
  kube -n "$NAMESPACE" rollout status "deployment/$name" --timeout=300s >/dev/null
}

restore_pvc_dir hello-k8s-ai-prometheus hello-k8s-ai-prometheus-data /prometheus "$BACKUP_DIR/prometheus.tar.gz"
restore_pvc_dir hello-k8s-ai-jaeger hello-k8s-ai-jaeger-data /tmp/jaeger "$BACKUP_DIR/jaeger.tar.gz"

step "完成。验证：make preflight / Grafana 面板 / /api/v1/replay"