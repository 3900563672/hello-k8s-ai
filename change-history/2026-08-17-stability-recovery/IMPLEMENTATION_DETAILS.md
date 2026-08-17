# 实现细节：运行前体检、工具链自检与 Prometheus 内存/重启告警

## 1. 改动前状态

- `hack/local-cluster.sh run_up` 只有 `check_context_and_cluster`（context 与集群可达），没有节点/PVC/内存/残留负载检查；端口冲突、内存不足都是部署后才炸。
- `hack/night-run/start-longrun.sh` 只有 18080 可达性 + `sleep-guard.sh status` 两个"提示"级前置快查（WARN 不阻塞），长跑仍可能带着 FAIL 级环境问题启动。
- 工具链脚本没有统一语法检查：`make verify` 只覆盖 Go/Frontend/清单渲染，`hack/*.mjs` 与 `*.sh` 不在内（`snapshot.mjs` 漏定义 `sleep` 上线跑即前科）。
- Prometheus 只抓业务指标：controller / otel / jaeger / backend / simulator 与自身；RBAC 只有 pods 与 `/metrics`；规则里没有容器内存与重启告警。
- Prometheus Deployment 默认 RollingUpdate：单副本时新 Pod 先起、旧 Pod 后停，两个 Pod 同时挂载同一 RWO PVC。

## 2. 实现内容

### 2.1 `hack/preflight.sh`（新增，~200 行）

统一体检，8 组检查，`PASS/FAIL/WARN` 三态，任一 FAIL 返回 1：

1. kubectl 与集群可达（`cluster-info`）；
2. 节点状态：数量、NotReady、cordon（cordon 为 WARN，worker6 已知故障）；
3. 命名空间内 Deployment/StatefulSet：replicas=0 为 WARN、available=0 为 FAIL、部分可用为 WARN；
4. PVC 全部 `Bound`（历史数据卷异常为 FAIL）；
5. 端口占用：18080（WSL 脚本）与 8080（Windows Dashboard），由本项目 port-forward 监听为 PASS，被他人占用或未监听为 WARN；
6. Windows 宿主内存（经 WSL interop 调 powershell.exe）：空闲 <1GB FAIL、<3GB WARN、>11.5GB 的 WSL VM 内存 WARN；
7. SimulatorInstance 残留负载：replicas=0（已暂停）为 PASS，活跃实例为 WARN（停止姿势：删 TenantModelPolicy）；
8. sleep-guard：`guard=on` 为 PASS；未开启时默认 WARN，`PREFLIGHT_REQUIRE_GUARD=1` 下为 FAIL。

环境变量：`KUBE_CONTEXT`（默认 docker-desktop）、`NAMESPACE`（默认 hello-k8s-ai-system）、`PREFLIGHT_REQUIRE_GUARD`、`PREFLIGHT_SKIP_WINDOWS`（无 Windows interop 环境跳过内存检查）。

### 2.2 接入点

- `hack/local-cluster.sh run_up`：`check_context_and_cluster` 后执行 `bash hack/preflight.sh`，FAIL 即 `fail` 中止；
- `hack/night-run/start-longrun.sh`：删除原 18080/sleep-guard 快查，改为 `PREFLIGHT_REQUIRE_GUARD=1 bash hack/preflight.sh`，FAIL 直接 exit 1。

### 2.3 `Makefile selfcheck`（已并入 `verify`）

- `bash -n` 全部 `*.sh`（排除 node_modules）；
- `node --check` 全部 `hack/*.mjs`；
- `kustomize build` config/dev、config/demo、dashboard/deploy 三个渲染。

### 2.4 Prometheus 告警（`config/observability/`）

- `prometheus-rbac.yaml`：ClusterRole 增加 `nodes`（get/list/watch）与 `nodes/proxy`（get）——cAdvisor 经 API Server proxy 抓取需要；
- `prometheus.yaml` 抓取配置新增 `kubernetes-cadvisor` job：
  - `kubernetes_sd_configs role: node`，`__address__` relabel 到 `kubernetes.default.svc:443`，`__metrics_path__` relabel 到 `/api/v1/nodes/${1}/proxy/metrics/cadvisor`；
  - Bearer token 用 Pod SA 文件，`insecure_skip_verify`；
- `rules.yml` 新增两条告警（`hello-k8s-ai-alerts` 组）：
  - `HelloK8sAIContainerMemoryHigh`：`container_memory_working_set_bytes / container_spec_memory_limit_bytes > 0.85`（仅 `hello-k8s-ai.*` 命名空间），`for: 10m`；
  - `HelloK8sAIContainerRestarted`：`changes(container_start_time_seconds[10m]) > 0`（同命名空间过滤；kube-state-metrics 未部署，用 cAdvisor 的 start_time 突变近似重启事件），`for: 1m`；
- `prometheus.yaml` Deployment：`strategy.type: Recreate`，规避单副本 + RWO PVC 滚动更新 TSDB 锁冲突。

## 3. 影响文件

| 文件 | 变更 |
| --- | --- |
| `hack/preflight.sh` | 新增：运行前体检 |
| `hack/local-cluster.sh` | run_up 接入 preflight |
| `hack/night-run/start-longrun.sh` | 前置快查替换为强制 preflight（REQUIRE_GUARD=1） |
| `Makefile` | 新增 `selfcheck` target，并入 `verify` |
| `config/observability/prometheus-rbac.yaml` | 加 nodes / nodes-proxy 权限 |
| `config/observability/prometheus.yaml` | cAdvisor job + 2 条告警 + Recreate 策略 |
| `hack/night-run/README.md` | 更新启动前 preflight 描述 |
| `docs/agents/KNOWN_PITFALLS.md` | 新增 3 条坑位 |
| `docs/agents/RESILIENCE.md` | 告警矩阵 |
| `docs/agents/WORKFLOW.md` | 验证节加 preflight / selfcheck |

## 4. 未验证 / 风险

- preflight 的 Windows 内存检查只在 WSL interop 可用时执行（本机已验证），纯 Linux 环境走 `PREFLIGHT_SKIP_WINDOWS=1` 跳过；
- 告警阈值（85%、10 分钟）是经验值，未经过长期运行调参；误报/漏报在后续长跑中校准；
- `HelloK8sAIContainerRestarted` 会把首次部署的新容器也计为"重启"（start_time 出现即变化），属于可接受噪音；
- 1 小时整体验证用户明确表示本次没有时间做，未执行；以真实启动 + 指标可见 + CI 全绿作为本次验收。
