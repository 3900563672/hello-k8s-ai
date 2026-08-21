#!/usr/bin/env bash
# 环境自检：开工 / 长跑前 30 秒检查环境层健康（磁盘 / Docker / 宿主 VM 残留 / WSL 回环 / 端口 / 内存 / tmpfs / dmesg）。
# 与 hack/preflight.sh 分工：doctor 不依赖集群可达（Docker 挂了也能给出明确 FAIL 项），
# preflight 在 doctor 通过后做集群与工作负载深度体检。
# 用法: bash hack/doctor.sh  ｜ 配套：make doctor
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

PASS=0
FAIL=0
WARN=0
ok()   { PASS=$((PASS+1)); printf '  PASS  %s\n' "$*"; }
bad()  { FAIL=$((FAIL+1)); printf '  FAIL  %s\n' "$*"; }
warn() { WARN=$((WARN+1)); printf '  WARN  %s\n' "$*"; }

echo "[doctor] 环境自检开始"

# ---------- 1. 磁盘（C: 盘 pagefile 曾吃 20GB+，见 docs/journal/2026-08-17-host-memory-governance.md） ----------
if command -v powershell.exe >/dev/null 2>&1; then
  CFREE=$(powershell.exe -NoProfile -Command "(Get-PSDrive C).Free/1GB" 2>/dev/null | tr -d '\r' | tail -1)
  if [[ "$CFREE" =~ ^[0-9.]+$ ]]; then
    if awk -v f="$CFREE" 'BEGIN{exit !(f < 15)}'; then
      bad "C: 盘可用 ${CFREE}GB（<15GB，pagefile / WPR 抓包有爆盘风险）"
    elif awk -v f="$CFREE" 'BEGIN{exit !(f < 25)}'; then
      warn "C: 盘可用 ${CFREE}GB（<25GB，长时抓包 / WSL 磁盘增长前建议清理）"
    else
      ok "C: 盘可用 ${CFREE}GB"
    fi
  else
    warn "无法读取 C: 盘可用空间（输出=$CFREE）"
  fi
fi
WSL_FREE=$(df -P / | awk 'NR==2{print $4/1024/1024}')
if awk -v f="$WSL_FREE" 'BEGIN{exit !(f < 5)}'; then
  bad "WSL 根分区可用 ${WSL_FREE}GB（<5GB）"
else
  ok "WSL 根分区可用 ${WSL_FREE}GB"
fi

# ---------- 2. Docker engine 与 kind 节点容器 ----------
if ! command -v docker >/dev/null 2>&1; then
  bad "docker 命令不存在"
elif ! docker info >/dev/null 2>&1; then
  bad "Docker engine 不可达（docker info 失败）——先启动 Docker Desktop"
else
  ok "Docker engine 可达"
  NODE_CONTAINERS=$(docker ps --filter "name=hello-k8s-ai-dev" --format '{{.Names}}' 2>/dev/null | wc -l)
  if (( NODE_CONTAINERS >= 5 )); then
    ok "kind 节点容器在跑（$NODE_CONTAINERS/5）"
  elif (( NODE_CONTAINERS > 0 )); then
    warn "kind 节点容器部分在跑（$NODE_CONTAINERS/5），恢复：docker start hello-k8s-ai-dev-*"
  else
    warn "kind 节点容器未运行（0/5）——集群当前处于关闭态；长跑前需启动"
  fi
fi

# ---------- 2.1 宿主 VM 残留（孤儿 vmwp/vmmemWSL 锁 vhdx，见 docs/operations/WSL_DOCKER_RESTART_SOP.md） ----------
if command -v powershell.exe >/dev/null 2>&1 && command -v wsl.exe >/dev/null 2>&1; then
  VMWP=$(powershell.exe -NoProfile -Command "(Get-Process vmwp -ErrorAction SilentlyContinue | Measure-Object).Count" 2>/dev/null | tr -d '\r' | tail -1)
  VMMEM=$(powershell.exe -NoProfile -Command "(Get-Process vmmemWSL -ErrorAction SilentlyContinue | Measure-Object).Count" 2>/dev/null | tr -d '\r' | tail -1)
  if timeout 8 wsl.exe -l -v >/dev/null 2>&1; then
    RUNNING=$(wsl.exe -l --running -q 2>/dev/null | tr -d '\0' | grep -c . || true)
    if [[ "$VMWP" =~ ^[0-9]+$ ]] && (( VMWP > 0 )) && ! docker info >/dev/null 2>&1; then
      bad "孤儿 vmwp 进程 ${VMWP} 个且 Docker 引擎不可达（疑似锁 ext4.vhdx）——先看 SOP：docs/operations/WSL_DOCKER_RESTART_SOP.md"
    elif [[ "$VMMEM" =~ ^[0-9]+$ ]] && (( VMMEM > 0 )) && (( RUNNING == 0 )) && ! docker info >/dev/null 2>&1; then
      bad "孤儿 vmmemWSL 进程 ${VMMEM} 个（无运行中 distro 且引擎不可达）——先看 SOP：docs/operations/WSL_DOCKER_RESTART_SOP.md"
    else
      ok "宿主 VM 进程正常（vmwp=${VMWP:-0}，vmmemWSL=${VMMEM:-0}，运行中 distro=${RUNNING}）"
    fi
  else
    bad "wsl.exe 无响应（wslservice 疑似僵尸化）——先看 SOP：docs/operations/WSL_DOCKER_RESTART_SOP.md"
  fi
fi

# ---------- 3. WSL 回环中继（新端口首连被拒坑，见 docs/lessons/process-wsl-loopback-fresh-listen-refused.md） ----------
if ! command -v go >/dev/null 2>&1; then
  warn "go 不存在，跳过 WSL 回环探针（hack/wsl-loopback-probe）"
else
  PROBE_OUT=$(go run ./hack/wsl-loopback-probe 2>/dev/null || true)
  case "$PROBE_OUT" in
    *"RESULT: FAIL"*) bad "WSL 回环中继降级（新端口首连全失败）" ;;
    *"RESULT: WARN"*) warn "WSL 回环中继疑似降级（间歇性首连失败）" ;;
    *"RESULT: PASS"*) ok "WSL 回环中继正常" ;;
    *"RESULT: SKIP"*) ok "非 WSL 环境，跳过回环中继检查" ;;
    *) warn "WSL 回环探针输出异常：$PROBE_OUT" ;;
  esac
fi

# ---------- 4. 端口占用 ----------
port_check() {
  local port="$1" label="$2"
  if (exec 3<>"/dev/tcp/127.0.0.1/$port") 2>/dev/null; then
    exec 3>&- 3<&- || true
    if pgrep -f "kubectl.*port-forward.*$port" >/dev/null 2>&1; then
      ok "$label 端口 $port 由 port-forward 监听"
    else
      warn "$label 端口 $port 被其他进程占用（非本项目 port-forward）"
    fi
  else
    warn "$label 端口 $port 未监听（启动后会自动建立转发）"
  fi
}
port_check 18080 "WSL 内脚本"
port_check 8080 "Windows 侧 Dashboard"

# ---------- 5. 内存水位（Windows 宿主） ----------
if ! command -v powershell.exe >/dev/null 2>&1; then
  warn "跳过 Windows 内存检查（powershell.exe 不可用）"
else
  FREE_GB=$(powershell.exe -NoProfile -Command "(Get-CimInstance Win32_OperatingSystem).FreePhysicalMemory/1MB" 2>/dev/null | tr -d '\r' | tail -1)
  if [[ "$FREE_GB" =~ ^[0-9.]+$ ]]; then
    if awk -v f="$FREE_GB" 'BEGIN{exit !(f < 1.0)}'; then
      bad "Windows 空闲内存 ${FREE_GB}GB（<1GB，先清理负载）"
    elif awk -v f="$FREE_GB" 'BEGIN{exit !(f < 3.0)}'; then
      warn "Windows 空闲内存 ${FREE_GB}GB（<3GB，长跑前建议清理）"
    else
      ok "Windows 空闲内存 ${FREE_GB}GB"
    fi
  else
    warn "无法读取 Windows 空闲内存（输出=$FREE_GB）"
  fi
  VMMEM_MB=$(powershell.exe -NoProfile -Command "(Get-Process vmmemWSL -ErrorAction SilentlyContinue).WorkingSet64/1MB" 2>/dev/null | tr -d '\r' | tail -1)
  if [[ "$VMMEM_MB" =~ ^[0-9.]+$ ]]; then
    if awk -v f="$VMMEM_MB" 'BEGIN{exit !(f > 11500)}'; then
      warn "WSL VM 内存 ${VMMEM_MB}MB（>11.5GB，接近 12GB 上限）"
    else
      ok "WSL VM 内存 ${VMMEM_MB}MB"
    fi
  fi
fi

# ---------- 6. kind 节点 tmpfs 残留挂载（恢复 SOP，见 docs/journal/2026-08-18-docker-bind-pvc-loss.md） ----------
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  TMPFS_BAD=0
  for n in hello-k8s-ai-dev-control-plane hello-k8s-ai-dev-worker hello-k8s-ai-dev-worker2 hello-k8s-ai-dev-worker3 hello-k8s-ai-dev-worker4; do
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$n"; then
      if docker exec "$n" sh -c 'mount | grep -q " /var/lib/hello-k8s-ai-pv "' 2>/dev/null; then
        warn "$n 仍挂载 tmpfs 到 hello-k8s-ai-pv（恢复 SOP：umount 后 chown）"
        TMPFS_BAD=1
      fi
    fi
  done
  (( TMPFS_BAD == 0 )) && ok "kind 节点无 tmpfs 残留挂载（或节点未运行）"
fi

# ---------- 7. dmesg 关键错误（ENOBUFS，见 docs/lessons/process-kubectl-enobufs.md） ----------
if command -v dmesg >/dev/null 2>&1; then
  ENOB=$(dmesg 2>/dev/null | grep -ci "ENOBUFS" || true)
  if (( ENOB > 0 )); then
    warn "dmesg 出现 ENOBUFS（${ENOB} 次，100+ Pod 高负载症状）"
  else
    ok "dmesg 无 ENOBUFS"
  fi
fi

# ---------- 8. kind apiserver 可达性（kind create 后端口映射注册丢失，docker restart 自愈，见 docs/journal/2026-08-21-kind-pv-rootfix.md） ----------
if command -v kubectl >/dev/null 2>&1 && command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  CP=$(docker ps --filter "name=hello-k8s-ai-dev-control-plane" --format '{{.Names}}' 2>/dev/null | head -1)
  if [[ -n "$CP" ]]; then
    if kubectl get --raw /healthz >/dev/null 2>&1; then
      ok "kind apiserver 可达"
    else
      bad "kind apiserver 不可达（节点容器在跑）——自愈：docker restart hello-k8s-ai-dev-control-plane"
    fi
  else
    warn "kind 集群未运行，跳过 apiserver 可达性检查"
  fi
fi

echo "----------------------------------------"
