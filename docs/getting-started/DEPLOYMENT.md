# 部署架构

> 维护层：human | last-reviewed：2026-08-19 | 事实源：dashboard/backend/deploy/、config/default/ 等

## 1. 目标集群

本地部署目标为 Kind 多节点开发集群 `hello-k8s-ai-dev`（由 `make cluster-up` 自动创建）：

| 项目 | 约束 |
| --- | --- |
| kubectl Context | `kind-hello-k8s-ai-dev`（`make cluster-up` 自动创建并切换） |
| Namespace | `hello-k8s-ai-system` |
| Node | Docker Desktop 本地容器，全部 Ready、架构一致 |
| StorageClass | `standard` |
| 集群生命周期 | `make cluster-up` 自动创建/复用；`make kind-down` 显式删除（PVC 数据保留） |

自动化 E2E 使用隔离的 Kind 集群 `hello-k8s-ai-test-e2e`，与开发集群互不影响。

## 2. 完整拓扑

```mermaid
flowchart TB
  subgraph DD["Kind 开发集群 hello-k8s-ai-dev"]
    C["Controller Manager"] --> S["动态 Simulator Deployment"]
    S --> K["CRD Spec / Status"]
    O["OTel Collector"] --> J["Jaeger"]
    P["Prometheus"] --> G["Grafana"]
    PG["PostgreSQL + PVC"] --> B["Dashboard Backend"]
    K --> B
    P --> B
    J --> B
    B --> F["Dashboard Frontend"]
  end
```

静态入口仍保持两个清晰边界：

- `config/dev`：11 个 CRD、Controller/Simulator RBAC、Controller、OTel Collector、Jaeger、Prometheus、Grafana。
- `dashboard/deploy`：PostgreSQL、Backend、Frontend 与 Backend RBAC。

`config/demo` 包含静态 Model、Tenant、TenantModelPolicy、Orchestrator 和 `SimulationClock/default`（1x）；WorkerNode 与 Node Policy 由脚本按目标集群节点生成。

## 3. 镜像交付

旧部署使用 `controller:latest` 并让 Node 从 Docker Hub 拉取，导致明确的 `ImagePullBackOff`。当前流程改为：

1. 在 Docker Desktop Engine 构建四个项目镜像。
2. 在同一 Engine 拉取第三方运行镜像与工具镜像（含 `busybox`，供备份/恢复辅助 Pod 使用）。
3. 用 `docker image save` 生成临时镜像包。
4. 找到与 Kubernetes Node 同名的 Docker 容器。
5. 使用节点内 `ctr --namespace k8s.io images import` 导入全部镜像。
6. 导入成功后才 apply 清单。

镜像导入覆盖全部 Node，因此 Controller、Backend、Frontend 和 Simulator 无论被调度到哪个 Node，都不会因项目镜像不存在而回退到 Docker Hub。

Kind 节点容器内无法访问宿主代理，所有清单镜像必须使用 `imagePullPolicy: IfNotPresent`（镜像已由脚本导入节点）；任何 Always 策略都会在节点上拉取 registry 失败。

## 4. 部署顺序

| 顺序 | 动作 | 失败策略 |
| ---: | --- | --- |
| 1 | 只读检查 Context、Node、StorageClass、Docker Node 容器 | 立即停止，不修改集群 |
| 2 | 拉取与构建镜像 | 最多重试；失败不 apply |
| 3 | 镜像导入所有 Node | 任一 Node 失败则停止 |
| 4 | apply CRD、RBAC、Controller、可观测性 | 等待 CRD Established 和全部 rollout |
| 5 | 创建/复用 PostgreSQL Secret，部署 Dashboard | PVC 存在但 Secret 丢失时拒绝猜密码 |
| 6 | 写入动态演示配置 | 只使用非 control-plane Node |
| 7 | 验证完整数据链路 | 任一硬门失败则生成诊断；无 SimulationClock CR 的干净环境跳过收敛检查；复用保留 PVC 时历史快照仅警告 |
| 8 | 启动端口转发 | 端口冲突只警告，不推翻已成功部署 |


### AIOps 可选配置（#93/#94/#95/#110/#112/#118）

`AIOPS_ENABLED` 默认 false：不启动 worker、不触发分析入队；`/aiops/settings` 路由始终注册，保证面板能重新打开运行时开关。开启需同时提供 `AIOPS_OPENAI_API_KEY`（Secret 注入，backend.yaml 含示例注释）。核心参数：`AIOPS_MODEL`、`AIOPS_OPENAI_BASE_URL`、`AIOPS_TIMEOUT`、`AIOPS_MAX_TOKENS_PER_CALL`、`AIOPS_MAX_CALLS_PER_ANALYSIS`、`AIOPS_MAX_ATTEMPTS_PER_ANALYSIS`、`AIOPS_POLL_INTERVAL`、`AIOPS_STALE_REQUEUE_INTERVAL`；M3 时间聚合（#95）：`AIOPS_WINDOW_INTERVAL`（默认 15m）、`AIOPS_WINDOW_GRANULARITY`（默认 2h）、`AIOPS_ALERT_THRESHOLD`（默认 40）、`AIOPS_ALERT_CONSECUTIVE`（默认 3）；对话浮窗（#110）：`AIOPS_CHAT_MODELS`、`AIOPS_CHAT_MAX_MESSAGE_LEN`（默认 4000）、`AIOPS_CHAT_RATE_PER_MINUTE`（默认 6）。运行时开关由面板 `POST /aiops/settings` 控制（仅服务端内存，重启恢复部署级配置）。关闭即完全停用，不影响其它功能。日配额保护（#124）：`AIOPS_DAILY_MAX_CALLS`（默认 300 次/24h）与 `AIOPS_DAILY_MAX_TOKENS`（默认 200 万/24h）超限时对话 429、分析不再入队。演示预生成：开启后可用 `bash hack/aiops-preseed.sh [数量]` 批量创建并完成切面实验，自动产出 AI 分析历史（见 [AIOPS_OVERVIEW](../aiops/AIOPS_OVERVIEW.md) 第 8 节）。
模板预置：`bash hack/aiops-templates-seed.sh` 幂等创建 10 模型 + 10 租户（qps=0 空环境）+ 10 节点及关系策略，模板 id 与 AIOps 模板目录一一对应，AI 一句话起实验前建议先跑一次。完整参数见 [CONFIGURATION_REFERENCE](../reference/CONFIGURATION_REFERENCE.md) 第 11 节。

## 5. 工作负载与存储

| 工作负载 | 镜像 | 存储 |
| --- | --- | --- |
| Controller Manager | `hello-k8s-ai-controller:dev` | 无 |
| Simulator | `hello-k8s-ai-simulator:dev` | CR Status / Lease |
| PostgreSQL 17 | `postgres:17-alpine` | `standard`，10Gi RWO PVC |
| Dashboard Backend | `hello-k8s-ai-dashboard-backend:dev` | 无本地卷，历史写 PostgreSQL |
| Dashboard Frontend | `hello-k8s-ai-dashboard-frontend:dev` | 无 |
| Prometheus / Jaeger | 固定版本 | PVC（20Gi / 10Gi RWO）、单副本 |
| Grafana | 固定版本 | 无持久卷（开发型） |

PostgreSQL 密码不再写死在 Git。首次部署生成随机密码，后续复用 Kubernetes Secret。

Docker Desktop / WSL 重启后，跑 make env-up（hack/dev-env-up.sh，#109）一键自愈并拉起完整联调环境：apiserver/PV tmpfs 自愈、port-forward 幂等重建、本地后端（密钥从集群 Secret 注入，不入库）与前端 vite（代理到本地后端）。幂等，可反复执行。

## 5.1 数据备份与恢复（Kind 底座）

底座迁移（`#50`）配套提供备份/恢复脚本，操作对象是当前集群（默认 `kind-hello-k8s-ai-dev`，迁移期用 `KUBE_CONTEXT=docker-desktop`）：

- `bash hack/kind/backup-data.sh`：PostgreSQL `pg_dump` + Prometheus TSDB + Jaeger badger 打包到 `/var/tmp/hello-k8s-ai-backup-<时间戳>/`。
- `BACKUP_DIR=<目录> bash hack/kind/restore-data.sh`：先部署新底座（`make cluster-up`），再恢复三套数据。

PVC 数据落在节点容器 `/var` named volume（Docker 数据盘 vhdx，WSL/Docker 重启不丢）；`kind-5node.yaml` 不使用宿主 hostPath（Docker Desktop 下位于 VM 根文件系统、非持久，历史上曾导致重启后 PVC 数据面故障，见 `docs/lessons/kind-hostpath-docker-desktop-rootfs.md`）。**删除集群重建后旧数据不会自动挂回**：先备份再重建，最后用 `restore-data.sh` 显式恢复。

注意事项：

- 备份/恢复期间对应 Deployment 会缩到 0（Prometheus / Jaeger），结束后自动恢复。
- 备份用 `/out/done` 标志轮询后才 cp；恢复先 cp 再由 `kubectl exec` 同步解包，不会复制或解包半成品。
- 恢复解包前会清空目标目录，避免与新集群初始化数据混合（否则 Prometheus TSDB 报 `segments are not sequential`）。
- 恢复完成后验证：`make preflight`、Grafana 面板有历史数据、`/api/v1/replay` 可查旧切面。

## 5.2 宿主层重启与自检（WSL/Docker）

PVC 数据在节点容器 `/var` named volume（Docker 数据盘 vhdx），WSL/Docker 重启不丢；但宿主层重启有安全顺序与残留检查要求（孤儿 vmwp/vmmemWSL 锁 vhdx 会导致引擎起不来，系统服务卡死时强杀会崩系统）。完整 SOP 见 [docs/operations/WSL_DOCKER_RESTART_SOP.md](../operations/WSL_DOCKER_RESTART_SOP.md)，要点：

1. 先优雅退出 Docker Desktop，再 `wsl --shutdown`，最后重新启动 Docker Desktop 并拉起 Ubuntu。
2. 重启后先 `make doctor`（含宿主 VM 残留检查）再继续业务链路。

## 6. 自动验收门

| 门 | 自动检查 |
| --- | --- |
| Kubernetes | 11 个 CRD Established，Controller rollout |
| CR/Controller | 演示 SimulatorInstance 出现且副本至少为 1 |
| Simulator | Deployment Ready，`status.observedAt` 非空 |
| 时间倍速 | Clock desired/applied=1、Ready，Instance timeScale=1，Simulator 指标存在 |
| Metrics | Prometheus 查询到 `hello_k8s_ai_simulator_leader` |
| Trace | Jaeger `/api/services` 出现 `hello-k8s-ai-*` |
| Database | Backend ready，`/replay` 返回 `snapshot-*` |
| Backend | `/configuration` 返回 `tenant-sample` |
| Frontend | Service 代理返回页面 HTML |
| 环境 | `make doctor` 环境自检通过（磁盘 / Docker 引擎 / WSL 回环 / 端口冲突 / 内存 / tmpfs / dmesg / kind apiserver 共 8 类检查）；`make preflight` 通过（含 WSL 回环探针 `hack/wsl-loopback-probe`：单轮语义（新端口注册时延测量 + Windows 侧 curl 校验 + dmesg 计数），非 WSL 自动跳过） |

| 文档 | `make docs-check`（全仓库文档门禁：MAP 映射 / 链接 / front-matter / change-history 门禁）；`make docs-sync-check`（README 时间线段、`docs/status.md`、`llms.txt`、所有权表必须与已提交内容一致） |

派生文档生成器（`hack/gen-docs.py`）只统计 git 已跟踪的 `change-history/` 条目：未提交目录不会进入 README 时间线段与 `docs/status.md`，多会话共享工作树时互不污染；`make docs-sync-check` 失败先检查工作树是否有未提交变更（含其它会话的批次），提交后重新生成即可。

change-history 门禁：非文档源码改动（后端/前端/脚本/CI/测试）必须在同一次提交中新增 `change-history/YYYY-MM-DD-*/README.md`，或在提交信息显式引用既有条目（`change-history: <条目名>`）；纯文档提交（`docs/`、`change-history/`、根文档）豁免。

这些检查通过后，脚本才输出“完整系统部署并验收通过”。

## 7. 生产边界

当前方案针对受控本机开发环境。Prometheus/Jaeger/Grafana 未持久化，PostgreSQL 单副本，未配置 OIDC、用户级授权、TLS、完整 NetworkPolicy、备份或 HA。不要把本地一键成功写成生产就绪。

## 8. 部署约束与静态工作负载（原 operations/CLUSTER_INFORMATION 第 5/6 节，2026-08-18 并入）

### 8.1 仓库部署约束

| 项目 | 声明值 |
| --- | --- |
| Context | `kind-hello-k8s-ai-dev` |
| Namespace | `hello-k8s-ai-system` |
| StorageClass | `standard` |
| 镜像交付 | Docker build/save + 每 Node `ctr -n k8s.io images import` |
| 动态 WorkerNode | 从非 control-plane Kubernetes Node 名生成 |
| 数据库 | PostgreSQL 17，10Gi PVC，随机 Secret |
| 停止语义 | 工作负载缩到 0，保留集群/CRD/CR/Secret/PVC |
| 文档门禁 | 根目录 Markdown 白名单（含 SECURITY.md）与 MAP 同步由 hack/check-docs.py 强制（make docs-check） |
| WSL 网络 | WSL 内访问 GitHub 慢/断连时先跑 `bash hack/wsl-github-proxy.sh --check`；代理仅配 github.com，详见 [../lessons/process-wsl-github-proxy.md](../lessons/process-wsl-github-proxy.md) |

### 8.2 部署后应出现的静态工作负载

| Kind | Name | 副本 |
| --- | --- | ---: |
| Deployment | `hello-k8s-ai-controller-manager` | 1 |
| Deployment | `hello-k8s-ai-otel-collector` | 1 |
| Deployment | `hello-k8s-ai-jaeger` | 1 |
| Deployment | `hello-k8s-ai-prometheus` | 1 |
| Deployment | `hello-k8s-ai-grafana` | 1 |
| StatefulSet | `hello-k8s-ai-dashboard-postgresql` | 1 |
| Deployment | `hello-k8s-ai-dashboard-backend` | 1 |
| Deployment | `hello-k8s-ai-dashboard-frontend` | 1 |

Controller 还会按 SimulatorInstance 动态创建 `simulator-<instance>` Deployment。
