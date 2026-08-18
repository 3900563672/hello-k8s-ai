#!/usr/bin/env bash
# 运行前体检：一键启动 / 长时值守前检查环境与集群健康。
# 用法:
#   bash hack/preflight.sh                 # 默认体检（FAIL 返回 1）
#   PREFLIGHT_REQUIRE_GUARD=1 bash hack/preflight.sh   # 长跑场景：sleep-guard 未开视为 FAIL
#   PREFLIGHT_SKIP_WINDOWS=1 bash hack/preflight.sh    # 无 Windows interop 环境跳过内存检查
# 环境变量: KUBE_CONTEXT / NAMESPACE / PREFLIGHT_REQUIRE_GUARD / PREFLIGHT_SKIP_WINDOWS
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

KUBE_CONTEXT="${KUBE_CONTEXT:-kind-hello-k8s-ai-dev}"
NAMESPACE="${NAMESPACE:-hello-k8s-ai-system}"
REQUIRE_GUARD="${PREFLIGHT_REQUIRE_GUARD:-0}"
SKIP_WINDOWS="${PREFLIGHT_SKIP_WINDOWS:-0}"

PASS=0
FAIL=0
WARN=0

ok()  { PASS=$((PASS+1)); printf '  PASS  %s\n' "$*"; }
bad() { FAIL=$((FAIL+1)); printf '  FAIL  %s\n' "$*"; }
warn() { WARN=$((WARN+1)); printf '  WARN  %s\n' "$*"; }

echo "[preflight] 开始体检（context=$KUBE_CONTEXT）"

# ---------- 1. kubectl 与集群可达 ----------
if ! command -v kubectl >/dev/null 2>&1; then
  bad "kubectl 不存在"
  echo "[preflight] 结果：$FAIL 项失败（kubectl 缺失，无法继续）"
  exit 1
fi
if ! kubectl --context "$KUBE_CONTEXT" cluster-info >/dev/null 2>&1; then
  bad "集群不可达（kubectl --context $KUBE_CONTEXT cluster-info 失败）"
  echo "[preflight] 结果：$PASS 通过 / $FAIL 失败 / $WARN 警告"
  exit 1
fi
ok "集群可达"

# ---------- 2. 节点状态 ----------
NODES=$(kubectl --context "$KUBE_CONTEXT" get nodes --no-headers 2>/dev/null | wc -l)
NOT_READY=$(kubectl --context "$KUBE_CONTEXT" get nodes --no-headers 2>/dev/null | grep -v ' Ready' | grep -v 'Ready,SchedulingDisabled' | wc -l || true)
CORDONED=$(kubectl --context "$KUBE_CONTEXT" get nodes --no-headers 2>/dev/null | grep -c 'Ready,SchedulingDisabled' || true)
if (( NODES == 0 )); then
  bad "没有可用节点"
elif (( NOT_READY > 0 )); then
  bad "存在 NotReady 节点（$NOT_READY 个）"
else
  ok "节点就绪（$NODES 个）"
fi
if (( CORDONED > 0 )); then
  warn "存在 cordon 节点（$CORDONED 个）"
fi

# ---------- 3. 系统组件健康 ----------
COMPONENTS=$(kubectl --context "$KUBE_CONTEXT" get deploy,statefulset -n "$NAMESPACE" \
  -o jsonpath='{range .items[*]}{.kind}/{.metadata.name} {.status.replicas} {.status.availableReplicas}{"\n"}{end}' 2>/dev/null || true)
if [[ -z "$COMPONENTS" ]]; then
  if ! kubectl --context "$KUBE_CONTEXT" get crd simulatorinstances.platform.study.com >/dev/null 2>&1; then
    warn "命名空间 $NAMESPACE 为空且 CRD 未安装（首次部署，跳过组件健康检查）"
  else
    bad "命名空间 $NAMESPACE 内没有 Deployment/StatefulSet（CRD 已存在但工作负载缺失）"
  fi
else
  while read -r kindname spec avail; do
    [[ -z "$kindname" ]] && continue
    if (( spec == 0 )); then
      warn "$kindname 未启动（replicas=0）"
    elif (( avail == 0 )); then
      bad "$kindname 不可用（replicas=$spec available=$avail）"
    elif (( avail < spec )); then
      warn "$kindname 部分可用（replicas=$spec available=$avail）"
    else
      ok "$kindname 就绪（$avail/$spec）"
    fi
  done <<<"$COMPONENTS"
fi

# ---------- 4. PVC ----------
PVC_BAD=0
while read -r name status; do
  [[ -z "$name" ]] && continue
  if [[ "$status" == "Bound" ]]; then
    ok "PVC $name Bound"
  else
    bad "PVC $name 状态=$status（历史数据卷异常）"
    PVC_BAD=1
  fi
done < <(kubectl --context "$KUBE_CONTEXT" get pvc -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $1, $2}')
(( PVC_BAD == 0 )) || true

# ---------- 5. 端口占用 ----------
port_check() {
  local port="$1" label="$2"
  if (exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null; then
    exec 3>&- 3<&- || true
    if pgrep -f "kubectl.*port-forward.*$port" >/dev/null 2>&1; then
      ok "$label 端口 $port 由 port-forward 监听"
    else
      warn "$label 端口 $port 被其他进程占用（不是本项目的 port-forward）"
    fi
  else
    warn "$label 端口 $port 未监听（启动后会自动建立转发）"
  fi
}
port_check 18080 "WSL 内脚本"
port_check 8080 "Windows 侧 Dashboard"

# ---------- 6. 内存水位（Windows 宿主，经 WSL interop） ----------
if (( SKIP_WINDOWS == 1 )) || ! command -v powershell.exe >/dev/null 2>&1; then
  warn "跳过 Windows 内存检查（SKIP_WINDOWS=1 或 powershell.exe 不可用）"
else
  FREE_GB=$(powershell.exe -NoProfile -Command "(Get-CimInstance Win32_OperatingSystem).FreePhysicalMemory/1MB" 2>/dev/null | tr -d '\r' | tail -1)
  if [[ "$FREE_GB" =~ ^[0-9.]+$ ]]; then
    if awk -v f="$FREE_GB" 'BEGIN{exit !(f < 1.0)}'; then
      bad "Windows 空闲内存 ${FREE_GB}GB（<1GB，内存压力极大，先清理负载再启动）"
    elif awk -v f="$FREE_GB" 'BEGIN{exit !(f < 3.0)}'; then
      warn "Windows 空闲内存 ${FREE_GB}GB（<3GB，长跑/大负载前建议先清理）"
    else
      ok "Windows 空闲内存 ${FREE_GB}GB"
    fi
  else
    warn "无法读取 Windows 空闲内存（输出=$FREE_GB）"
  fi
  VMMEM_MB=$(powershell.exe -NoProfile -Command "(Get-Process vmmemWSL -ErrorAction SilentlyContinue).WorkingSet64/1MB" 2>/dev/null | tr -d '\r' | tail -1)
  if [[ "$VMMEM_MB" =~ ^[0-9.]+$ ]]; then
    if awk -v f="$VMMEM_MB" 'BEGIN{exit !(f > 11500)}'; then
      warn "WSL VM 内存 ${VMMEM_MB}MB（>11.5GB，接近 12GB 上限，10 节点开销大）"
    else
      ok "WSL VM 内存 ${VMMEM_MB}MB"
    fi
  fi
fi

# ---------- 7. 长跑残留负载 ----------
RESIDUAL=$(kubectl --context "$KUBE_CONTEXT" get simulatorinstances -o jsonpath='{range .items[*]}{.metadata.name} replicas={.spec.replicas}{"\n"}{end}' 2>/dev/null || true)
if [[ -n "$RESIDUAL" ]]; then
  while read -r line; do
    [[ -z "$line" ]] && continue
    case "$line" in
      *"replicas=0") ok "SimulatorInstance $line（已暂停）" ;;
      *) warn "SimulatorInstance $line（残留负载，长跑/大负载前确认内存余量；停止请删 TenantModelPolicy）" ;;
    esac
  done <<<"$RESIDUAL"
else
  ok "无 SimulatorInstance 残留负载"
fi

# ---------- 8. sleep-guard ----------
if command -v bash >/dev/null 2>&1 && [[ -f hack/night-run/sleep-guard.sh ]]; then
  guard=$(bash hack/night-run/sleep-guard.sh status 2>/dev/null | tail -1)
  case "$guard" in
    *guard=on*) ok "sleep-guard 已开启（$guard）" ;;
    *)
      if (( REQUIRE_GUARD == 1 )); then
        bad "sleep-guard 未开启（长跑必需：bash hack/night-run/sleep-guard.sh on 需要 UAC 确认）"
      else
        warn "sleep-guard 未开启（$guard；长跑前必须开启）"
      fi
      ;;
  esac
else
  warn "sleep-guard.sh 不存在，跳过"
fi


# ---------- 9. WSL 回环中继（新端口首连被拒） ----------
if ! command -v go >/dev/null 2>&1; then
  warn "go 不存在，跳过 WSL 回环探针（hack/wsl-loopback-probe）"
else
  PROBE_OUT=$(go run ./hack/wsl-loopback-probe 2>/dev/null || true)
  case "$PROBE_OUT" in
    *"RESULT: FAIL"*)
      bad "WSL 回环中继降级（新端口首连全失败）：整机重启或 wsl --shutdown 后复测（影响运行中发行版与 Docker Desktop K8s，需用户同意）"
      ;;
    *"RESULT: WARN"*)
      warn "WSL 回环中继疑似降级（间歇性首连失败）；本地测试可先自连一次完成端口注册"
      ;;
    *"RESULT: PASS"*)
      ok "WSL 回环中继正常（新端口首连通过）"
      ;;
    *"RESULT: SKIP"*)
      ok "非 WSL 环境，跳过回环中继检查"
      ;;
    *)
      warn "WSL 回环探针输出异常：$PROBE_OUT"
      ;;
  esac
fi

echo "----------------------------------------"
echo "[preflight] 结果：$PASS 通过 / $FAIL 失败 / $WARN 警告"
if (( FAIL > 0 )); then
  echo "[preflight] 存在失败项，请先修复再启动。"
  exit 1
fi
exit 0
