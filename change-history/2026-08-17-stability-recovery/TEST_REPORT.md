# 测试报告：运行前体检、工具链自检与 Prometheus 告警

## 1. 环境

- 宿主：Windows + WSL2 Ubuntu；集群：`docker-desktop`（Docker Desktop 内置 K8s，10 节点，worker6 cordon）。
- 验证时间：2026-08-17 14:30-15:00（UTC+8）。
- 基线状态：8 个系统组件 Running，0 SimulatorInstance / 无策略（干净）。

## 2. 执行的命令与真实结果

### 2.1 `bash hack/preflight.sh`

结果：`19 通过 / 0 失败 / 1 警告`（WARN = worker6 cordon），exit=0。

关键 PASS 项：集群可达；节点就绪（10 个）；controller-manager / dashboard-backend / dashboard-frontend / grafana / jaeger / otel-collector / prometheus 7 个 Deployment 就绪；postgresql StatefulSet 就绪；3 个 PVC 全部 Bound；18080 与 8080 由 port-forward 监听；Windows 空闲内存 3.5GB；WSL VM 内存 8.4GB；无 SimulatorInstance 残留负载；sleep-guard guard=on。

### 2.2 `make selfcheck`

- bash 语法：OK（全部 `*.sh` 通过 `bash -n`）；
- Node 语法：OK（`hack/*.mjs` 通过 `node --check`）；
- 清单渲染：config/dev、config/demo、dashboard/deploy 三个 kustomize build 全部 OK；
- `selfcheck 通过`。

### 2.3 Prometheus 部署验证（`kubectl apply -k config/dev` + rollout）

- 首次 `rollout restart` 失败复现：新 Pod CrashLoopBackOff，日志 `Fatal error: opening storage failed: lock DB directory: resource temporarily unavailable`——单副本 + RWO PVC 滚动更新 TSDB 锁冲突；
- 修复为 `strategy: Recreate` 后：`deployment "hello-k8s-ai-prometheus" successfully rolled out`；
- cAdvisor 抓取：`up{job="kubernetes-cadvisor"}` 全部 10 节点（desktop-control-plane + desktop-worker..worker9，含 cordon 的 worker6）= 1；
- 指标落地：`count(container_memory_working_set_bytes{namespace=~"hello-k8s-ai.*"})` = 24 条序列；
- 规则加载（`GET /api/v1/rules`）：`HelloK8sAIContainerMemoryHigh` 与 `HelloK8sAIContainerRestarted` 均在 `hello-k8s-ai-alerts` 组内（status: success）。

### 2.4 CI（推送 d4dc41c，即本项工作的前序提交）

- 代码检查：success（gocyclo 圈复杂度问题已在此前修复）；
- 源码与部署验证：success（上一版 679a3eb 的同类 job 失败原因是 GitHub 429 限流下载 setup-go，基础设施问题，非代码问题）；
- E2E 测试：success。

## 3. 未验证

- 告警触发路径（内存 >85% 持续 10 分钟、容器重启突变）：规则已加载、指标源已确认，未做人为触发演练（用户未安排 1 小时验证窗口）；
- 长跑脚本 `start-longrun.sh` 的强制 preflight 拦截：仅静态检查接入点与 `PREFLIGHT_REQUIRE_GUARD=1` 分支逻辑，未在真实长跑中触发 FAIL 路径。
