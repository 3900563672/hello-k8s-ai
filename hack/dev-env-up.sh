#!/usr/bin/env bash
# 一键自愈 + 联调拉起（make env-up）：重启 Docker Desktop / WSL 后只跑本脚本恢复完整联调环境。
# 覆盖 #109：apiserver 端口自愈、PV tmpfs 遮罩自愈、port-forward 幂等重建、
# 本地后端（.runtime/start-backend.sh 密钥注入，不落明文到仓库）、前端 vite（代理到本地后端）。
#
# 用法:
#   bash hack/dev-env-up.sh                 # 自愈 + 拉起（后端 + 前端 + 转发）
#   DEV_ENV_SKIP_FRONTEND=1 bash hack/dev-env-up.sh   # 不起前端，只转发 + 后端
#   DEV_ENV_SKIP_HEAL=1 bash hack/dev-env-up.sh       # 跳过自愈，只拉起
# 环境变量: KUBE_CONTEXT / NAMESPACE / DEV_ENV_SKIP_FRONTEND / DEV_ENV_SKIP_HEAL
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

KUBE_CONTEXT="${KUBE_CONTEXT:-kind-hello-k8s-ai-dev}"
NAMESPACE="${NAMESPACE:-hello-k8s-ai-system}"
RUNTIME_DIR="$ROOT_DIR/.runtime"
KUBECTL="${KUBECTL:-kubectl}"

NODES=(hello-k8s-ai-dev-control-plane hello-k8s-ai-dev-worker hello-k8s-ai-dev-worker2 hello-k8s-ai-dev-worker3 hello-k8s-ai-dev-worker4)

log()  { printf '[env-up] %s\n' "$*"; }
warn() { printf '[env-up][warn] %s\n' "$*"; }

require_command() {
  command -v "$1" >/dev/null 2>&1 || { warn "缺少命令 $1"; exit 1; }
}

port_open() { # port_open <port>：本机是否在监听该端口（ss 检查，不建连，避免 WSL 网络退化时 hang）
  ss -tln 2>/dev/null | awk '{print $4}' | grep -qE "(:|\\.)$1$"
}

find_free_port() { # find_free_port <起始>：从起始端口向上找空闲（本机未被监听）
  local port="$1" i
  for ((i = 0; i < 20; i++)); do
    if ! port_open "$port" 2>/dev/null; then
      printf '%s' "$port"
      return 0
    fi
    port=$((port + 1))
  done
  printf '%s' "$1"
}

# ---------- 1. 自愈 ----------
heal_apiserver() {
  local cp
  cp="$(docker ps --filter "name=hello-k8s-ai-dev-control-plane" --format '{{.Names}}' 2>/dev/null | head -1)"
  if [[ -z "$cp" ]]; then
    warn "kind 集群未运行：先执行 make cluster-up"
    return 1
  fi
  if "$KUBECTL" --context "$KUBE_CONTEXT" get --raw /healthz >/dev/null 2>&1; then
    log "apiserver 可达，无需自愈"
    return 0
  fi
  log "apiserver 不可达 → docker restart $cp"
  docker restart "$cp" >/dev/null
  local i
  for i in {1..60}; do
    if "$KUBECTL" --context "$KUBE_CONTEXT" get --raw /healthz >/dev/null 2>&1; then
      log "apiserver 已恢复"
      return 0
    fi
    sleep 2
  done
  warn "apiserver 60s 内未恢复，请查看 $cp 日志"
}

heal_pv_tmpfs() {
  # 旧集群 extraMounts bind 失效后 fallback 成 tmpfs 遮罩 named volume（SOP：docs/lessons/kind-pv-tmpfs-umount-sop.md）
  local n fixed=0
  for n in "${NODES[@]}"; do
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$n"; then
      if docker exec "$n" sh -c 'mount | grep -q " /var/lib/hello-k8s-ai-pv "' 2>/dev/null; then
        log "umount $n /var/lib/hello-k8s-ai-pv（tmpfs 遮罩 PV）"
        docker exec "$n" umount /var/lib/hello-k8s-ai-pv
        fixed=1
      fi
    fi
  done
  if (( fixed )); then
    log "触发 PVC 工作负载重建（kubelet 重新挂载）"
    "$KUBECTL" --context "$KUBE_CONTEXT" -n "$NAMESPACE" delete pod hello-k8s-ai-dashboard-postgresql-0 --ignore-not-found --wait=false >/dev/null 2>&1 || true
    "$KUBECTL" --context "$KUBE_CONTEXT" -n "$NAMESPACE" rollout restart deployment hello-k8s-ai-prometheus hello-k8s-ai-jaeger >/dev/null 2>&1 || true
  else
    log "无 PV tmpfs 遮罩残留"
  fi
}

# ---------- 2. port-forward（幂等，pid 文件 + 端口连通双检查） ----------
PF_PID_DIR="$RUNTIME_DIR/port-forward"

pf_pid_file() { printf '%s/%s.pid\n' "$PF_PID_DIR" "$1"; }

start_port_forward() {
  local key="$1" service="$2" port="$3" remote_port="$4"
  local pid_file log_file pid args actual_port running_port
  mkdir -p "$PF_PID_DIR"
  pid_file="$(pf_pid_file "$key")"
  log_file="$RUNTIME_DIR/port-forward-$key.log"

  if [[ -f "$pid_file" ]]; then
    pid="$(<"$pid_file")"
    args="$(ps -p "$pid" -o args= 2>/dev/null || true)"
    running_port="$(cat "${pid_file%.pid}.port" 2>/dev/null || true)"
    [[ -n "$running_port" ]] || running_port="$port"
    if [[ "$args" == *"port-forward"* && "$args" == *"$service"* ]] && port_open "$running_port"; then
      log "port-forward $key 已在运行（127.0.0.1:$running_port）"
      return 0
    fi
    rm -f -- "$pid_file" "${pid_file%.pid}.port"
  fi

  actual_port="$(find_free_port "$port")"
  if [[ "$actual_port" != "$port" ]]; then
    warn "$key 端口 $port 被占用，改用 $actual_port"
  fi

  nohup "$KUBECTL" --context "$KUBE_CONTEXT" -n "$NAMESPACE" \
    port-forward --address 127.0.0.1 "service/$service" \
    "$actual_port:$remote_port" >"$log_file" 2>&1 </dev/null &
  pid=$!
  printf '%s\n' "$pid" >"$pid_file"
  printf '%s\n' "$actual_port" >"${pid_file%.pid}.port"

  local i
  for ((i = 1; i <= 20; i++)); do
    if grep -q 'Forwarding from' "$log_file" 2>/dev/null; then
      log "port-forward $key → 127.0.0.1:$actual_port"
      return 0
    fi
    if ! kill -0 "$pid" 2>/dev/null; then
      warn "$key 启动失败：$(tail -n 2 "$log_file" 2>/dev/null | tr '\n' ' ')"
      rm -f -- "$pid_file" "${pid_file%.pid}.port"
      return 0
    fi
    sleep 0.5
  done
  warn "$key 尚未确认，请查看 $log_file"
}

pf_port() { # pf_port <key> <默认>：读取实际端口（幂等复用或探测）
  local pid_file="${PF_PID_DIR}/$1.pid" port_file="${PF_PID_DIR}/$1.port"
  if [[ -f "$port_file" ]]; then cat "$port_file"; else printf '%s' "$2"; fi
}

# ---------- 3. 生成本地后端启动脚本（密钥只进 .runtime，gitignored） ----------
render_backend_script() {
  local db_port prom_port jae_port api_key base_url model
  mkdir -p "$RUNTIME_DIR"
  db_port="$(pf_port postgresql 5432)"
  prom_port="$(pf_port prometheus 9090)"
  jae_port="$(pf_port jaeger 16686)"
  api_key="$("$KUBECTL" --context "$KUBE_CONTEXT" -n "$NAMESPACE" get secret hello-k8s-ai-dashboard-aiops -o jsonpath='{.data.openai-api-key}' 2>/dev/null | base64 -d 2>/dev/null || true)"
  base_url="$("$KUBECTL" --context "$KUBE_CONTEXT" -n "$NAMESPACE" get deployment hello-k8s-ai-dashboard-backend -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="AIOPS_OPENAI_BASE_URL")].value}' 2>/dev/null || true)"
  model="$("$KUBECTL" --context "$KUBE_CONTEXT" -n "$NAMESPACE" get deployment hello-k8s-ai-dashboard-backend -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="AIOPS_MODEL")].value}' 2>/dev/null || true)"
  base_url="${base_url:-https://api.openai.com/v1}"
  model="${model:-gpt-4o-mini}"

  cat >"$RUNTIME_DIR/start-backend.sh" <<EOF
#!/usr/bin/env bash
# 由 hack/dev-env-up.sh 生成（勿手改；.runtime 不入库）。
cd "$ROOT_DIR/dashboard/backend"
export HTTP_ADDRESS='127.0.0.1:18080'
export DATABASE_URL='postgres://dashboard:dashboard@127.0.0.1:${db_port}/dashboard?sslmode=disable'
export DATABASE_REQUIRED='true'
export PROMETHEUS_URL='http://127.0.0.1:${prom_port}'
export JAEGER_URL='http://127.0.0.1:${jae_port}'
export AIOPS_ENABLED='true'
export AIOPS_OPENAI_API_KEY='${api_key}'
export AIOPS_OPENAI_BASE_URL='${base_url}'
export AIOPS_MODEL='${model}'
exec go run ./cmd/server
EOF
  chmod 700 "$RUNTIME_DIR/start-backend.sh"
  chmod 700 "$RUNTIME_DIR"
  log "已生成 $RUNTIME_DIR/start-backend.sh（密钥注入，chmod 700）"
}

# ---------- 4. 启动本地后端（幂等：pid 文件 + 端口归属双检查） ----------
start_backend() {
  local pid_file="$RUNTIME_DIR/backend.pid" pid args listening_pid i
  if [[ -f "$pid_file" ]]; then
    pid="$(<"$pid_file")"
    args="$(ps -p "$pid" -o args= 2>/dev/null || true)"
    if [[ "$args" == *"cmd/server"* ]] && port_open 18080; then
      log "本地后端已在运行（PID $pid，127.0.0.1:18080）"
      return 0
    fi
    rm -f -- "$pid_file"
  fi
  # 端口归属检查：18080 已被本项目的 server 监听（孤儿进程场景）→ 复用并回写 pid
  listening_pid="$(ss -tlnp 2>/dev/null | grep ':18080 ' | grep -oE 'pid=[0-9]+' | head -1 | cut -d= -f2 || true)"
  if [[ -n "$listening_pid" ]] && kill -0 "$listening_pid" 2>/dev/null; then
    args="$(ps -p "$listening_pid" -o args= 2>/dev/null || true)"
    if [[ "$args" == *"/server"* || "$args" == *"cmd/server"* ]]; then
      printf '%s\n' "$listening_pid" >"$pid_file"
      log "本地后端已在运行（孤儿进程复用，PID $listening_pid，127.0.0.1:18080）"
      return 0
    fi
  fi
  render_backend_script
  log "启动本地后端（go run ./cmd/server，首次编译较慢）"
  setsid nohup bash "$RUNTIME_DIR/start-backend.sh" >"$RUNTIME_DIR/backend.log" 2>&1 </dev/null &
  pid=$!
  printf '%s\n' "$pid" >"$pid_file"
  for ((i = 1; i <= 90; i++)); do
    if curl -sf --max-time 5 --noproxy '*' "http://127.0.0.1:18080/api/v1/health/ready" >/dev/null 2>&1; then
      log "本地后端 READY（PID $pid）"
      return 0
    fi
    if ! kill -0 "$pid" 2>/dev/null; then
      warn "本地后端启动失败：$(tail -n 5 "$RUNTIME_DIR/backend.log" 2>/dev/null | tr '\n' ' ')"
      return 1
    fi
    sleep 1
  done
  warn "本地后端 90s 未 READY，请查看 $RUNTIME_DIR/backend.log"
}

# ---------- 5. 启动前端 vite（幂等，端口自动探测） ----------
start_frontend() {
  local pid_file="$RUNTIME_DIR/vite.pid" pid old_port i
  if [[ -f "$pid_file" ]]; then
    pid="$(<"$pid_file")"
    old_port="$(cat "$RUNTIME_DIR/vite.port" 2>/dev/null || true)"
    if kill -0 "$pid" 2>/dev/null && [[ -n "$old_port" ]] && port_open "$old_port"; then
      log "前端 vite 已在运行（PID $pid，127.0.0.1:$old_port）"
      return 0
    fi
    rm -f -- "$pid_file" "$RUNTIME_DIR/vite.port"
  fi
  port="$(find_free_port 5173)"
  if [[ "$port" != "5173" ]]; then
    warn "前端端口 5173 被占用，改用 $port"
  fi
  log "启动前端 vite（http://0.0.0.0:$port，代理 → 本地后端 18080）"
  (cd dashboard/frontend/my-app && setsid nohup env VITE_API_PROXY_TARGET='http://127.0.0.1:18080' \
    npm run dev -- --host 0.0.0.0 --port "$port" --strictPort >"$RUNTIME_DIR/vite.log" 2>&1 </dev/null & echo $! >"$pid_file")
  sleep 1
  printf '%s' "$port" >"$RUNTIME_DIR/vite.port"
  for ((i = 1; i <= 30; i++)); do
    if curl -sf --max-time 5 --noproxy '*' "http://127.0.0.1:$port/" >/dev/null 2>&1; then
      log "前端 READY（http://127.0.0.1:$port）"
      return 0
    fi
    sleep 1
  done
  warn "前端 30s 未就绪，请查看 $RUNTIME_DIR/vite.log"
}

# ---------- 6. 输出 ----------
print_urls() {
  local ip db_port prom_port jae_port vite_port
  ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  db_port="$(pf_port postgresql 5432)"
  prom_port="$(pf_port prometheus 9090)"
  jae_port="$(pf_port jaeger 16686)"
  vite_port="$(cat "$RUNTIME_DIR/vite.port" 2>/dev/null || printf 5173)"
  printf '\n==== 联调环境就绪 ====\n'
  printf '  Frontend (vite)  http://%s:%s\n' "$ip" "$vite_port"
  printf '  Backend health   http://127.0.0.1:18080/api/v1/health/ready\n'
  printf '  PostgreSQL      127.0.0.1:%s\n' "$db_port"
  printf '  Prometheus      127.0.0.1:%s\n' "$prom_port"
  printf '  Jaeger          127.0.0.1:%s\n' "$jae_port"
  printf '  （Windows 浏览器访问用 http://%s:%s，WSL 内用 127.0.0.1）\n' "$ip" "$vite_port"
}

# ---------- main ----------
require_command docker
require_command "$KUBECTL"
require_command curl

if [[ "${DEV_ENV_SKIP_HEAL:-0}" != "1" ]]; then
  heal_apiserver
  heal_pv_tmpfs
fi

start_port_forward postgresql hello-k8s-ai-dashboard-postgresql 5432 5432
start_port_forward prometheus hello-k8s-ai-prometheus 9090 9090
start_port_forward jaeger hello-k8s-ai-jaeger 16686 16686

start_backend

if [[ "${DEV_ENV_SKIP_FRONTEND:-0}" != "1" ]]; then
  start_frontend
fi

print_urls
