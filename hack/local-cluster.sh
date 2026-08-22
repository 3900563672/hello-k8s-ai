#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

KUBE_CONTEXT="${KUBE_CONTEXT:-kind-hello-k8s-ai-dev}"
NAMESPACE="${NAMESPACE:-hello-k8s-ai-system}"
CONTAINER_TOOL="${CONTAINER_TOOL:-docker}"
KUBECTL="${KUBECTL:-kubectl}"

MANAGER_IMG="${MANAGER_IMG:-hello-k8s-ai-controller:dev}"
SIMULATOR_IMG="${SIMULATOR_IMG:-hello-k8s-ai-simulator:dev}"
BACKEND_IMG="${BACKEND_IMG:-hello-k8s-ai-dashboard-backend:dev}"
FRONTEND_IMG="${FRONTEND_IMG:-hello-k8s-ai-dashboard-frontend:dev}"
DEMO_MODEL_ABSOLUTE_SCORE="${DEMO_MODEL_ABSOLUTE_SCORE:-100}"
DEMO_ENABLED="${DEMO_ENABLED:-false}"

RUNTIME_DIR="${RUNTIME_DIR:-$ROOT_DIR/.runtime}"
mkdir -p "$RUNTIME_DIR"

ACTION="${1:-help}"
LOG_FILE="$RUNTIME_DIR/${ACTION}-$(date -u +%Y%m%dT%H%M%SZ).log"
CURRENT_STEP="初始化"
IMAGE_ARCHIVE=""
NODE_ARCHIVE="/var/tmp/hello-k8s-ai-images.tar"

declare -a ALL_NODES=()
declare -a WORKER_NODES=()
declare -a RUNTIME_IMAGES=()
declare -A NODE_CONTAINERS=()

log() {
  printf '[hello-k8s-ai] %s\n' "$*"
}

warn() {
  printf '[hello-k8s-ai] 警告：%s\n' "$*" >&2
}

fail() {
  printf '[hello-k8s-ai] 错误：%s\n' "$*" >&2
  return 1
}

step() {
  CURRENT_STEP="$1"
  printf '\n[hello-k8s-ai] === %s ===\n' "$CURRENT_STEP"
}

kube() {
  "$KUBECTL" --context "$KUBE_CONTEXT" "$@"
}

cleanup_temp() {
  if [[ -n "$IMAGE_ARCHIVE" && -f "$IMAGE_ARCHIVE" ]]; then
    rm -f -- "$IMAGE_ARCHIVE"
  fi
}

collect_diagnostics() {
  local diagnostic_file="$RUNTIME_DIR/last-failure.log"
  {
    printf '失败步骤：%s\n' "$CURRENT_STEP"
    printf '采集时间：%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf 'Context：%s\n\n' "$KUBE_CONTEXT"

    kube get nodes -o wide 2>&1 || true
    printf '\n'
    kube -n "$NAMESPACE" get deployment,statefulset,pod,service,pvc,lease -o wide 2>&1 || true
    printf '\n'
    kube -n "$NAMESPACE" get events --sort-by=.lastTimestamp 2>&1 | tail -n 120 || true
    printf '\nController 日志：\n'
    kube -n "$NAMESPACE" logs deployment/hello-k8s-ai-controller-manager \
      --all-containers --tail=160 2>&1 || true
    printf '\nBackend 日志：\n'
    kube -n "$NAMESPACE" logs deployment/hello-k8s-ai-dashboard-backend \
      --all-containers --tail=160 2>&1 || true
  } >"$diagnostic_file"
  warn "诊断信息已写入 $diagnostic_file"
}

on_error() {
  local exit_code=$?
  local line_number="${1:-unknown}"
  trap - ERR
  # shellcheck disable=SC1111  # 中文全角引号为有意使用
  warn "步骤“$CURRENT_STEP”失败（脚本第 $line_number 行，退出码 $exit_code）。"
  if command -v "$KUBECTL" >/dev/null 2>&1; then
    collect_diagnostics
  fi
  exit "$exit_code"
}

trap cleanup_temp EXIT
trap 'on_error $LINENO' ERR

require_command() {
  local command_name="$1"
  command -v "$command_name" >/dev/null 2>&1 || fail "未找到命令：$command_name"
}

retry() {
  local attempts="$1"
  shift
  local count=1
  while (( count <= attempts )); do
    if "$@"; then
      return 0
    fi
    if (( count == attempts )); then
      return 1
    fi
    warn "命令失败，3 秒后重试（$count/$attempts）：$*"
    sleep 3
    count=$((count + 1))
  done
}

wait_for_text() {
  local description="$1"
  local expected="$2"
  local attempts="$3"
  shift 3
  local output=""
  local count
  for ((count = 1; count <= attempts; count++)); do
    output="$({ "$@"; } 2>/dev/null || true)"
    if [[ "$output" == *"$expected"* ]]; then
      log "$description：通过"
      return 0
    fi
    sleep 3
  done
  warn "$description 的最后响应：${output:0:800}"
  fail "$description 未在等待时间内通过"
}

service_proxy() {
  local service="$1"
  local port_name="$2"
  local path="$3"
  kube get --raw \
    "/api/v1/namespaces/$NAMESPACE/services/http:$service:$port_name/proxy$path"
}

check_context_and_cluster() {
  require_command "$CONTAINER_TOOL"
  require_command "$KUBECTL"
  require_command awk
  require_command base64
  require_command grep
  require_command od
  require_command sed
  require_command tee

  "$CONTAINER_TOOL" info >/dev/null

  local current_context
  current_context="$($KUBECTL config current-context)"
  [[ "$current_context" == "$KUBE_CONTEXT" ]] || fail \
    "当前 kubectl Context 是 $current_context，期望 $KUBE_CONTEXT。为避免部署到错误集群，已停止。"
  case "$KUBE_CONTEXT" in
    docker-desktop|kind-hello-k8s-ai-dev) ;;
    *) fail \
      "这套本地部署只允许 docker-desktop 或 kind-hello-k8s-ai-dev Context，实际为 $KUBE_CONTEXT。" ;;
  esac

  kube get --raw=/readyz >/dev/null
  kube get storageclass standard >/dev/null || fail \
    "集群缺少 StorageClass standard，PostgreSQL PVC 无法绑定。"

  mapfile -t ALL_NODES < <(kube get nodes -o name | sed 's#^node/##')
  ((${#ALL_NODES[@]} > 0)) || fail "目标集群没有 Node。"

  local node ready
  while read -r node ready; do
    [[ -n "$node" ]] || continue
    [[ "$ready" == "True" ]] || fail "Node $node 当前不是 Ready。"
  done < <(kube get nodes -o jsonpath='{range .items[*]}{.metadata.name}{" "}{range .status.conditions[?(@.type=="Ready")]}{.status}{end}{"\n"}{end}')

  mapfile -t WORKER_NODES < <(
    kube get nodes \
      --selector='!node-role.kubernetes.io/control-plane,!node-role.kubernetes.io/master' \
      -o name | sed 's#^node/##'
  )
  ((${#WORKER_NODES[@]} > 0)) || fail "没有可供 Simulator 使用的 Worker Node。"

  local architectures
  architectures="$(kube get nodes -o jsonpath='{range .items[*]}{.status.nodeInfo.architecture}{"\n"}{end}' | sort -u)"
  [[ "$(printf '%s\n' "$architectures" | sed '/^$/d' | wc -l)" -eq 1 ]] || fail \
    "当前一键部署要求所有 Node 架构一致，检测到：$architectures"
  case "$architectures" in
    amd64|arm64) TARGET_PLATFORM="linux/$architectures" ;;
    *) fail "暂不支持 Node 架构：$architectures" ;;
  esac

  local container_state
  for node in "${ALL_NODES[@]}"; do
    container_state="$($CONTAINER_TOOL inspect --type container --format '{{.State.Running}}' "$node" 2>/dev/null || true)"
    [[ "$container_state" == "true" ]] || fail \
      "无法访问 Node 对应的 Docker 容器 $node，不能可靠导入本地镜像。"
    "$CONTAINER_TOOL" exec "$node" ctr version >/dev/null
    NODE_CONTAINERS["$node"]="$node"
  done

  local docker_resources cpu_count memory_bytes
  docker_resources="$($CONTAINER_TOOL info --format '{{.NCPU}} {{.MemTotal}}')"
  read -r cpu_count memory_bytes <<<"$docker_resources"
  if (( cpu_count < 4 )); then
    warn "Docker Desktop 仅分配了 $cpu_count 个 CPU，首次构建可能较慢。"
  fi
  if (( memory_bytes < 4294967296 )); then
    warn "Docker Desktop 可用内存低于 4 GiB，完整栈可能出现调度或 OOM 问题。"
  fi

  if kube get crd orchestratorconfigs.platform.study.com >/dev/null 2>&1; then
    warn "发现旧 CRD orchestratorconfigs.platform.study.com；部署不会自动删除，以免误删未知数据。"
  fi

  log "Context：$KUBE_CONTEXT"
  log "Node：${#ALL_NODES[@]} 个（Worker ${#WORKER_NODES[@]} 个）"
  log "镜像平台：$TARGET_PLATFORM"
}

pull_runtime_images() {
  RUNTIME_IMAGES=(
    "$MANAGER_IMG"
    "$SIMULATOR_IMG"
    "$BACKEND_IMG"
    "$FRONTEND_IMG"
    "postgres:17-alpine"
    "busybox"
    "prom/prometheus:v3.13.2"
    "otel/opentelemetry-collector-contrib:0.158.0"
    "cr.jaegertracing.io/jaegertracing/jaeger:2.20.0"
    "grafana/grafana:13.1.3"
  )

  local image
  for image in "${RUNTIME_IMAGES[@]:4}"; do
    if "$CONTAINER_TOOL" image inspect "$image" >/dev/null 2>&1; then
      log "运行镜像已存在，跳过拉取：$image"
      continue
    fi
    log "拉取运行镜像：$image"
    retry 3 "$CONTAINER_TOOL" pull --platform "$TARGET_PLATFORM" "$image"
  done
}

build_project_images() {
  # 四个镜像互相独立：并行构建（本机 CPU 充足时显著缩短启动时间）。
  local image_pids=()
  build_one() {
    local label="$1"
    shift
    log "构建 $label"
    if ! retry 2 env DOCKER_BUILDKIT=1 "$CONTAINER_TOOL" build \
      --platform "$TARGET_PLATFORM" --build-arg "GOPROXY=${DOCKER_GOPROXY:-https://goproxy.cn,direct}" "$@"; then
      fail "构建 $label 失败"
    fi
  }

  build_one "Controller：$MANAGER_IMG" \
    --target manager --tag "$MANAGER_IMG" "$ROOT_DIR" &
  image_pids+=("$!")
  build_one "Simulator：$SIMULATOR_IMG" \
    --target simulator --tag "$SIMULATOR_IMG" "$ROOT_DIR" &
  image_pids+=("$!")
  build_one "Dashboard Backend：$BACKEND_IMG" \
    --tag "$BACKEND_IMG" "$ROOT_DIR/dashboard/backend" &
  image_pids+=("$!")
  build_one "Dashboard Frontend：$FRONTEND_IMG" \
    --tag "$FRONTEND_IMG" "$ROOT_DIR/dashboard/frontend/my-app" &
  image_pids+=("$!")

  local pid
  for pid in "${image_pids[@]}"; do
    wait "$pid"
  done
}

import_images_to_nodes() {
  IMAGE_ARCHIVE="$(mktemp "$RUNTIME_DIR/images.XXXXXX.tar")"
  log "生成节点镜像包（首次执行会占用一些时间）"
  "$CONTAINER_TOOL" image save --output "$IMAGE_ARCHIVE" "${RUNTIME_IMAGES[@]}"

  local node container
  for node in "${ALL_NODES[@]}"; do
    container="${NODE_CONTAINERS[$node]}"
    log "导入镜像到 Node：$node"
    "$CONTAINER_TOOL" cp "$IMAGE_ARCHIVE" "$container:$NODE_ARCHIVE" >/dev/null
    if ! "$CONTAINER_TOOL" exec "$container" \
      ctr --namespace k8s.io images import "$NODE_ARCHIVE" >/dev/null; then
      "$CONTAINER_TOOL" exec "$container" rm -f -- "$NODE_ARCHIVE" >/dev/null 2>&1 || true
      return 1
    fi
    "$CONTAINER_TOOL" exec "$container" rm -f -- "$NODE_ARCHIVE"
    "$CONTAINER_TOOL" exec "$container" ctr --namespace k8s.io images list -q |
      grep -F "$MANAGER_IMG" >/dev/null || fail \
        "Node $node 导入后仍找不到 Controller 镜像。"
  done
}

wait_for_crds() {
  local crd
  local crds=(
    models.platform.study.com
    workernodes.platform.study.com
    tenants.platform.study.com
    tenantmodelpolicies.platform.study.com
    tenantnodepolicies.platform.study.com
    modelnodepolicies.platform.study.com
    simulationclocks.platform.study.com
    simulatorinstances.platform.study.com
    tenantperformances.platform.study.com
    tenantruntimes.platform.study.com
    orchestrators.platform.study.com
  )
  for crd in "${crds[@]}"; do
    kube wait --for=condition=Established "crd/$crd" --timeout=120s >/dev/null
  done
}

restart_and_wait_deployment() {
  local deployment="$1"
  local procedure
  procedure="$(kube -n "$NAMESPACE" get "deployment/$deployment" \
    -o jsonpath='{.metadata.annotations.platform\.study\.com/restart-procedure}' 2>/dev/null || true)"
  if [[ "$procedure" == "scale-to-zero" ]]; then
    # 单副本 + RWO PVC（如 Jaeger badger）：滚动更新新旧 Pod 抢目录锁会 CrashLoop，
    # 必须按清单声明的流程先缩到 0 再扩回 1。
    kube -n "$NAMESPACE" scale "deployment/$deployment" --replicas=0 >/dev/null
    kube -n "$NAMESPACE" rollout status "deployment/$deployment" --timeout=120s >/dev/null
    kube -n "$NAMESPACE" scale "deployment/$deployment" --replicas=1 >/dev/null
    kube -n "$NAMESPACE" rollout status "deployment/$deployment" --timeout=360s >/dev/null
  else
    kube -n "$NAMESPACE" rollout restart "deployment/$deployment" >/dev/null
    kube -n "$NAMESPACE" rollout status "deployment/$deployment" --timeout=360s
  fi
}

ensure_database_secret() {
  local secret_name="hello-k8s-ai-dashboard-postgresql"
  local pvc_name="data-hello-k8s-ai-dashboard-postgresql-0"

  if kube -n "$NAMESPACE" get "secret/$secret_name" >/dev/null 2>&1; then
    local database_url password
    database_url="$(kube -n "$NAMESPACE" get "secret/$secret_name" -o jsonpath='{.data.DATABASE_URL}')"
    password="$(kube -n "$NAMESPACE" get "secret/$secret_name" -o jsonpath='{.data.POSTGRES_PASSWORD}')"
    [[ -n "$database_url" && -n "$password" ]] || fail \
      "现有 PostgreSQL Secret 缺少 DATABASE_URL 或 POSTGRES_PASSWORD。"
    if [[ "$(printf '%s' "$password" | base64 -d)" == "change-me-before-production" ]]; then
      warn "现有数据库仍使用旧占位密码。为避免破坏已有 PVC，本次保留；建议后续单独轮换。"
    fi
    log "沿用现有 PostgreSQL Secret，避免重部署破坏数据库连接。"
    return 0
  fi

  if kube -n "$NAMESPACE" get "pvc/$pvc_name" >/dev/null 2>&1; then
    fail "发现 PostgreSQL PVC 但 Secret 丢失，无法安全推断原密码。请先恢复 Secret。"
  fi

  local password
  password="$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')"
  kube -n "$NAMESPACE" create secret generic "$secret_name" \
    --from-literal=POSTGRES_DB=dashboard \
    --from-literal=POSTGRES_USER=dashboard \
    --from-literal="POSTGRES_PASSWORD=$password" \
    --from-literal="DATABASE_URL=postgres://dashboard:$password@hello-k8s-ai-dashboard-postgresql:5432/dashboard?sslmode=disable" \
    --dry-run=client -o yaml | kube apply -f - >/dev/null
  kube -n "$NAMESPACE" label "secret/$secret_name" \
    app.kubernetes.io/name=hello-k8s-ai-dashboard-postgresql \
    app.kubernetes.io/part-of=hello-k8s-ai \
    app.kubernetes.io/managed-by=local-deployer --overwrite >/dev/null
  log "已生成本地 PostgreSQL 随机密码并保存为 Kubernetes Secret。"
}

apply_worker_resources() {
  local node="$1"
  kube apply -f - >/dev/null <<EOF
apiVersion: platform.study.com/v1
kind: WorkerNode
metadata:
  name: $node
  labels:
    app.kubernetes.io/part-of: hello-k8s-ai
    app.kubernetes.io/managed-by: local-deployer
spec:
  displayName: 节点 $node
  gpu: 8000
  maxConcurrency: 160
---
apiVersion: platform.study.com/v1
kind: TenantNodePolicy
metadata:
  name: tenant-sample-$node
  labels:
    app.kubernetes.io/part-of: hello-k8s-ai
    app.kubernetes.io/managed-by: local-deployer
spec:
  tenantRef:
    name: tenant-sample
  nodeRef:
    name: $node
  effect: Allow
---
apiVersion: platform.study.com/v1
kind: ModelNodePolicy
metadata:
  name: model-sample-$node
  labels:
    app.kubernetes.io/part-of: hello-k8s-ai
    app.kubernetes.io/managed-by: local-deployer
spec:
  modelRef:
    name: model-sample
  nodeRef:
    name: $node
  effect: Allow
EOF
}

wait_for_demo_runtime() {
  local instance="tenant-sample-model-sample"
  local deployment="simulator-$instance"
  local count replicas observed_at

  for ((count = 1; count <= 90; count++)); do
    if kube get "simulatorinstance/$instance" >/dev/null 2>&1; then
      replicas="$(kube get "simulatorinstance/$instance" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
      if [[ "$replicas" =~ ^[0-9]+$ ]] && (( replicas >= 1 )); then
        break
      fi
    fi
    sleep 2
  done
  if [[ "${replicas:-0}" =~ ^[0-9]+$ ]] && (( replicas >= 1 )); then
    :
  else
    fail "演示 SimulatorInstance 没有扩到至少 1 个副本。"
  fi

  kube -n "$NAMESPACE" rollout status "deployment/$deployment" --timeout=360s

  for ((count = 1; count <= 60; count++)); do
    observed_at="$(kube get "simulatorinstance/$instance" -o jsonpath='{.status.observedAt}' 2>/dev/null || true)"
    [[ -n "$observed_at" ]] && break
    sleep 2
  done
  [[ -n "${observed_at:-}" ]] || fail "Simulator 已运行，但没有写回 observedAt。"
  log "Simulator 状态已写回：$observed_at"
}

deploy_control_plane() {
  kube apply -k "$ROOT_DIR/config/dev"
  wait_for_crds
  migrate_legacy_model_scores

  local deployment
  local deployments=(
    hello-k8s-ai-controller-manager
    hello-k8s-ai-otel-collector
    hello-k8s-ai-jaeger
    hello-k8s-ai-prometheus
    hello-k8s-ai-grafana
  )
  for deployment in "${deployments[@]}"; do
    restart_and_wait_deployment "$deployment"
  done
}

deploy_dashboard() {
  ensure_database_secret
  kube apply -k "$ROOT_DIR/dashboard/deploy"

  kube -n "$NAMESPACE" rollout status \
    statefulset/hello-k8s-ai-dashboard-postgresql --timeout=360s
  restart_and_wait_deployment hello-k8s-ai-dashboard-backend
  restart_and_wait_deployment hello-k8s-ai-dashboard-frontend
}

migrate_legacy_model_scores() {
  local records name spec_score legacy_score
  records="$(kube get models.platform.study.com \
    -o jsonpath='{range .items[*]}{.metadata.name}{"|"}{.spec.absoluteScore}{"|"}{.status.absoluteScore}{"\n"}{end}' \
    2>/dev/null || true)"

  while IFS='|' read -r name spec_score legacy_score; do
    [[ -n "$name" ]] || continue
    if [[ ! "$spec_score" =~ ^[1-9][0-9]*$ && "$legacy_score" =~ ^[1-9][0-9]*$ ]]; then
      kube patch model.platform.study.com "$name" --type=merge \
        --patch "{\"spec\":{\"absoluteScore\":$legacy_score}}" >/dev/null
      log "已迁移旧 Model 能力基准分：$name -> spec.absoluteScore=$legacy_score"
    fi
  done <<<"$records"
}

deploy_demo() {
  [[ "$DEMO_MODEL_ABSOLUTE_SCORE" =~ ^[1-9][0-9]*$ ]] || fail \
    "DEMO_MODEL_ABSOLUTE_SCORE 必须是正整数。"

  kube apply -k "$ROOT_DIR/config/demo"
  local node
  for node in "${WORKER_NODES[@]}"; do
    apply_worker_resources "$node"
  done

  kube patch model.platform.study.com model-sample --type=merge \
    --patch "{\"spec\":{\"absoluteScore\":$DEMO_MODEL_ABSOLUTE_SCORE}}" >/dev/null
  kube annotate orchestrator.platform.study.com orchestrator-sample \
    platform.study.com/local-deploy-trigger="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --overwrite >/dev/null

  wait_for_demo_runtime
}

verify_data_flow() {
  wait_for_text "Backend readiness 与 PostgreSQL" '"status":"ready"' 30 \
    service_proxy hello-k8s-ai-dashboard-backend http /api/v1/health/ready
  wait_for_text "Backend Simulator 倍速能力" '"simulatorAcceleration":true' 30 \
    service_proxy hello-k8s-ai-dashboard-backend http /api/v1/clock
  if [[ -n "$(kube get simulationclock --no-headers 2>/dev/null)" ]]; then
    wait_for_text "SimulationClock 配置收敛" '1|1|True' 30 \
      kube get simulationclock/default \
      -o 'jsonpath={.spec.rate}{"|"}{.status.appliedRate}{"|"}{.status.conditions[?(@.type=="Ready")].status}'
  else
    log "无 SimulationClock CR，跳过收敛检查（干净环境；创建后自动收敛）"
  fi
  wait_for_text "Frontend 页面" '<!doctype html>' 20 \
    service_proxy hello-k8s-ai-dashboard-frontend http /

  if [[ "$DEMO_ENABLED" == "true" ]]; then
    wait_for_text "Backend Kubernetes 聚合" 'tenant-sample' 30 \
      service_proxy hello-k8s-ai-dashboard-backend http /api/v1/configuration
    kube wait --for=jsonpath='{.spec.timeScale}'=1 \
      simulatorinstances --all --timeout=90s >/dev/null
    log "SimulatorInstance 倍速同步：通过"
    wait_for_text "PostgreSQL snapshot" 'snapshot-' 30 \
      service_proxy hello-k8s-ai-dashboard-backend http /api/v1/replay
    wait_for_text "Prometheus Simulator 指标" 'hello_k8s_ai_simulator_leader' 40 \
      service_proxy hello-k8s-ai-prometheus http \
      '/api/v1/query?query=hello_k8s_ai_simulator_leader'
    wait_for_text "Prometheus Simulator 倍速指标" 'hello_k8s_ai_simulator_time_scale' 40 \
      service_proxy hello-k8s-ai-prometheus http \
      '/api/v1/query?query=hello_k8s_ai_simulator_time_scale'
    wait_for_text "OpenTelemetry 到 Jaeger" 'hello-k8s-ai-' 40 \
      service_proxy hello-k8s-ai-jaeger query /api/services
  else
    verify_clean_state
  fi
}

verify_clean_state() {
  local leftover replay
  leftover="$(kube get tenants,models,orchestrators,simulatorinstances,workernodes \
    -o name 2>/dev/null || true)"
  leftover="$(printf '%s\n' "$leftover" | grep -v '/preset-' || true)"
  if [[ -n "$leftover" ]]; then
    fail "干净环境断言失败：仍存在业务 CR：$leftover"
  fi
  log "干净环境断言：业务 CR 为空（#131 预置模板 preset-* 已豁免）"

  replay="$(service_proxy hello-k8s-ai-dashboard-backend http /api/v1/replay 2>/dev/null || true)"
  if [[ "$replay" == *'snapshot-'* ]]; then
    warn "干净环境断言：/replay 含历史快照（复用保留的 PostgreSQL PVC，非本次部署产生）"
  else
    log "干净环境断言：无历史快照"
  fi
}

port_forward_pid_file() {
  printf '%s/port-forward-%s.pid\n' "$RUNTIME_DIR" "$1"
}

start_port_forward() {
  local key="$1"
  local service="$2"
  local local_port="$3"
  local remote_port="$4"
  local pid_file log_file pid args
  pid_file="$(port_forward_pid_file "$key")"
  log_file="$RUNTIME_DIR/port-forward-$key.log"

  if [[ -f "$pid_file" ]]; then
    pid="$(<"$pid_file")"
    args="$(ps -p "$pid" -o args= 2>/dev/null || true)"
    # 进程在 + 端口真实监听 + HTTP 探活通过才算"已在运行"；否则清理残留 PID 文件并重建。
    # #137：pod 滚动重建后旧转发进程退出，仅端口探测会在半开窗口误判复用。
    local http_ok=0
    if command -v curl >/dev/null 2>&1; then
      if curl -sf -o /dev/null --max-time 2 "http://127.0.0.1:$local_port/"; then
        http_ok=1
      fi
    fi
    if [[ "$args" == *"port-forward"* && "$args" == *"$service"* ]] &&
      (exec 3<>"/dev/tcp/127.0.0.1/$local_port") 2>/dev/null &&
      { [[ "$http_ok" -eq 1 ]] || ! command -v curl >/dev/null 2>&1; }; then
      log "$key 端口转发已在运行（PID $pid）。"
      return 0
    fi
    rm -f -- "$pid_file"
  fi

  nohup "$KUBECTL" --context "$KUBE_CONTEXT" -n "$NAMESPACE" \
    port-forward --address 127.0.0.1 "service/$service" \
    "$local_port:$remote_port" >"$log_file" 2>&1 </dev/null &
  pid=$!
  printf '%s\n' "$pid" >"$pid_file"

  local count
  for ((count = 1; count <= 20; count++)); do
    if grep -q 'Forwarding from' "$log_file" 2>/dev/null; then
      log "$key 已映射到 127.0.0.1:$local_port"
      return 0
    fi
    if ! kill -0 "$pid" 2>/dev/null; then
      warn "$key 端口转发启动失败：$(tail -n 2 "$log_file" 2>/dev/null | tr '\n' ' ')"
      rm -f -- "$pid_file"
      return 0
    fi
    sleep 0.5
  done
  warn "$key 端口转发尚未确认，请查看 $log_file"
}

open_ports() {
  require_command "$KUBECTL"
  require_command grep
  require_command nohup
  require_command ps

  # 可观测性收敛到 Dashboard 单入口：Grafana 经 /grafana 反代，
  # Prometheus / Jaeger 由 Backend 代理（/api/v1/metrics、/api/v1/traces）。
  start_port_forward dashboard hello-k8s-ai-dashboard-frontend 8080 80
  # WSL 内脚本专用端口：Windows 侧 localhost:8080 由 dllhost 转发宿主占用，
  # WSL 内访问 8080 会与之冲突（时好时坏），脚本一律走 18080（见 docs/journal/2026-08-16-cluster-and-deploy.md）。
  start_port_forward dashboard-internal hello-k8s-ai-dashboard-frontend 18080 80
  print_urls
}

stop_port_forwards() {
  local key pid_file pid args
  for key in dashboard dashboard-internal grafana prometheus jaeger; do
    pid_file="$(port_forward_pid_file "$key")"
    [[ -f "$pid_file" ]] || continue
    pid="$(<"$pid_file")"
    args="$(ps -p "$pid" -o args= 2>/dev/null || true)"
    if [[ "$args" == *"kubectl"* && "$args" == *"port-forward"* ]]; then
      kill "$pid" 2>/dev/null || true
    fi
    rm -f -- "$pid_file"
  done
}

apply_aiops_config() {
  # #136：部署后自动恢复 AIOps 配置并输出状态提示（.runtime/aiops.env 存在即自动启用）。
  if [[ -f .runtime/aiops.env ]]; then
    log "检测到 .runtime/aiops.env：自动启用 AIOps（#136 自动恢复）..."
    if bash hack/aiops-enable.sh; then
      log "AIOps 自动启用完成"
    else
      warn "AIOps 自动启用失败，可手动运行 bash hack/aiops-enable.sh"
    fi
  else
    log "未检测到 .runtime/aiops.env：AIOps 保持关闭。启用：bash hack/aiops-enable.sh"
  fi
  local settings enabled keyed
  settings="$(service_proxy hello-k8s-ai-dashboard-backend http /api/v1/aiops/settings 2>/dev/null || true)"
  if [[ -n "$settings" ]]; then
    enabled="$(printf '%s' "$settings" | sed -n 's/.*"enabled":\([a-z]*\).*/\1/p')"
    keyed="$(printf '%s' "$settings" | sed -n 's/.*"keyConfigured":\([a-z]*\).*/\1/p')"
    log "AIOps 状态：enabled=$enabled keyConfigured=$keyed"
  else
    warn "AIOps 状态接口不可达（稍后可用 make cluster-status 复查）"
  fi
}

print_urls() {
  printf '\n访问地址：\n'
  printf '  Dashboard   http://localhost:8080\n'
  printf '  Grafana     http://localhost:8080/grafana\n'
}

show_status() {
  require_command "$KUBECTL"
  kube get nodes -o wide
  printf '\n'
  kube -n "$NAMESPACE" get deployment,statefulset,pod,service,pvc,lease -o wide
  printf '\n'
  kube get models,workernodes,tenants,tenantmodelpolicies,tenantnodepolicies,\
modelnodepolicies,simulationclocks,simulatorinstances,tenantperformances,tenantruntimes,orchestrators

  local ready
  ready="$(service_proxy hello-k8s-ai-dashboard-backend http /api/v1/health/ready 2>/dev/null || true)"
  if [[ "$ready" == *'"status":"ready"'* ]]; then
    log "Backend API：READY"
  else
    warn "Backend API：未确认 READY"
  fi
  print_urls
}

scale_if_exists() {
  local kind="$1"
  local name="$2"
  if kube -n "$NAMESPACE" get "$kind/$name" >/dev/null 2>&1; then
    kube -n "$NAMESPACE" scale "$kind/$name" --replicas=0 >/dev/null
  fi
}

stop_stack() {
  require_command "$KUBECTL"
  stop_port_forwards

  scale_if_exists deployment hello-k8s-ai-controller-manager
  kube -n "$NAMESPACE" scale deployment \
    --selector='platform.study.com/managed-by=simulator-instance-controller' \
    --replicas=0 >/dev/null 2>&1 || true

  local deployment
  for deployment in \
    hello-k8s-ai-dashboard-backend \
    hello-k8s-ai-dashboard-frontend \
    hello-k8s-ai-otel-collector \
    hello-k8s-ai-jaeger \
    hello-k8s-ai-prometheus \
    hello-k8s-ai-grafana; do
    scale_if_exists deployment "$deployment"
  done
  scale_if_exists statefulset hello-k8s-ai-dashboard-postgresql

  log "项目工作负载已停止；集群、CRD、CR、Secret 与 PostgreSQL PVC 均保留。"
}

run_up() {
  step "部署前检查"
  check_context_and_cluster

  step "运行前体检"
  if ! bash hack/preflight.sh; then
    fail "运行前体检未通过，先修复再启动（见上方 FAIL 项）。"
  fi

  step "准备第三方运行镜像"
  pull_runtime_images

  step "构建四个项目镜像"
  build_project_images

  step "把镜像导入全部 Kubernetes Node"
  import_images_to_nodes

  step "部署 Controller 与可观测性"
  deploy_control_plane

  step "部署 PostgreSQL、Backend 与 Frontend"
  deploy_dashboard

  if [[ "$DEMO_ENABLED" == "true" ]]; then
    step "按现有 Worker Node 写入演示配置"
    deploy_demo
  else
    log "演示数据已关闭（DEMO_ENABLED=false），保持干净环境。"
  fi

  step "验证完整数据链路"
  verify_data_flow

  step "启动本地访问端口"
  open_ports

  step "AIOps 配置恢复与状态提示（#136）"
  apply_aiops_config

  printf '\n[hello-k8s-ai] 完整系统部署并验收通过。\n'
  log "部署日志：$LOG_FILE"
}

usage() {
  cat <<'EOF'
用法：hack/local-cluster.sh <up|status|open|urls|down>

  up      构建、导入镜像、部署并验收（默认不写演示数据）
  status  查看 Kubernetes 资源与 Backend 健康状态
  open    启动 Dashboard 本地端口转发（可观测性经单入口访问）
  urls    打印访问地址
  down    停止工作负载，保留集群、CRD、CR 和数据库 PVC

环境变量：
  DEMO_ENABLED=true                  在 up 时写入演示配置（模型/租户/节点/策略/实例）
  DEMO_MODEL_ABSOLUTE_SCORE=100      演示模型绝对分数（正整数）
EOF
}

case "$ACTION" in
  up)
    # 用普通管道保留终端输出和日志，避免依赖 /dev/fd 的进程替换。
    trap - ERR
    set +e
    (
      set -Eeuo pipefail
      trap cleanup_temp EXIT
      trap 'on_error $LINENO' ERR
      run_up
    ) 2>&1 | tee -a "$LOG_FILE"
    up_status=${PIPESTATUS[0]}
    exit "$up_status"
    ;;
  status) show_status ;;
  open) open_ports ;;
  urls) print_urls ;;
  down) stop_stack ;;
  help|-h|--help) usage ;;
  *) usage; fail "未知操作：$ACTION" ;;
esac
