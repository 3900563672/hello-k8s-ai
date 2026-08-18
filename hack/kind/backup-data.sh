#!/usr/bin/env bash
# #50 迁移前备份：PostgreSQL（pg_dump）+ Prometheus TSDB + Jaeger badger。
# 用法：KUBE_CONTEXT=docker-desktop bash hack/kind/backup-data.sh
# 环境变量：KUBE_CONTEXT / NAMESPACE / BACKUP_DIR
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

KUBE_CONTEXT="${KUBE_CONTEXT:-docker-desktop}"
NAMESPACE="${NAMESPACE:-hello-k8s-ai-system}"
BACKUP_DIR="${BACKUP_DIR:-/var/tmp/hello-k8s-ai-backup-$(date +%Y%m%d-%H%M%S)}"
mkdir -p "$BACKUP_DIR"

kube() { kubectl --context "$KUBE_CONTEXT" "$@"; }
step() { printf '\n[backup] %s\n' "$*"; }

step "1/3 PostgreSQL pg_dump -> $BACKUP_DIR/dashboard.sql"
kube -n "$NAMESPACE" exec "statefulset/hello-k8s-ai-dashboard-postgresql" -- \
  pg_dump -U dashboard -d dashboard --clean --if-exists > "$BACKUP_DIR/dashboard.sql"
echo "dashboard.sql: $(wc -l < "$BACKUP_DIR/dashboard.sql") 行"

backup_pvc_dir() {
  local name="$1" claim="$2" src_path="$3" out_file="$4"
  step "备份 $name（PVC $claim -> $out_file）"
  kube -n "$NAMESPACE" scale "deployment/$name" --replicas=0 >/dev/null
  kube -n "$NAMESPACE" rollout status "deployment/$name" --timeout=120s >/dev/null 2>&1 || true
  kube -n "$NAMESPACE" apply -f - >/dev/null <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: backup-$name
  namespace: $NAMESPACE
spec:
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: $claim
  - name: out
    emptyDir: {}
  containers:
  - name: pack
    image: busybox
    imagePullPolicy: IfNotPresent
    command: ["sh", "-c", "tar czf /out/data.tar.gz -C $src_path . && touch /out/done && sleep 3600"]
    volumeMounts:
    - name: data
      mountPath: $src_path
    - name: out
      mountPath: /out
  restartPolicy: Never
EOF
  kubectl --context "$KUBE_CONTEXT" -n "$NAMESPACE" wait --for=condition=Ready "pod/backup-$name" --timeout=120s >/dev/null
  # tar 完成标志：wait Ready 只保证容器启动，不代表打包结束；轮询 /out/done 再 cp。
  for _ in $(seq 1 60); do
    kubectl --context "$KUBE_CONTEXT" -n "$NAMESPACE" exec "backup-$name" -- sh -c 'test -f /out/done' >/dev/null 2>&1 && break
    sleep 5
  done
  kubectl --context "$KUBE_CONTEXT" -n "$NAMESPACE" exec "backup-$name" -- sh -c 'test -f /out/done' >/dev/null || {
    echo "错误：backup-$name 打包超时（300s）" >&2
    kubectl --context "$KUBE_CONTEXT" -n "$NAMESPACE" delete pod "backup-$name" --wait=false >/dev/null
    exit 1
  }
  kubectl --context "$KUBE_CONTEXT" -n "$NAMESPACE" exec "backup-$name" -- sh -c 'ls -la /out/data.tar.gz'
  kubectl --context "$KUBE_CONTEXT" cp "$NAMESPACE/backup-$name:/out/data.tar.gz" "$out_file"
  kubectl --context "$KUBE_CONTEXT" -n "$NAMESPACE" delete pod "backup-$name" --wait=false >/dev/null
  kube -n "$NAMESPACE" scale "deployment/$name" --replicas=1 >/dev/null
  kube -n "$NAMESPACE" rollout status "deployment/$name" --timeout=300s >/dev/null
  ls -la "$out_file"
}

backup_pvc_dir hello-k8s-ai-prometheus hello-k8s-ai-prometheus-data /prometheus "$BACKUP_DIR/prometheus.tar.gz"
backup_pvc_dir hello-k8s-ai-jaeger hello-k8s-ai-jaeger-data /tmp/jaeger "$BACKUP_DIR/jaeger.tar.gz"

step "完成：$BACKUP_DIR"
ls -la "$BACKUP_DIR"
