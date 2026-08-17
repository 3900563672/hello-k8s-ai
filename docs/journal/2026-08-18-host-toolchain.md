# 宿主与工具链（2026-08-18）

> 日期：2026-08-18 ｜ 触发者：本地 Agent ｜ 相关：原 docs/agents/KNOWN_PITFALLS.md 归档（2026-08-18 文档体系重构拆分为 journal/lessons）

### 2026-08-18 Codex 应用更新后 notify 路径过期，exec 报 helper_unknown_error
- 现象：整机重启后 Codex 桌面应用反复重启（15 分钟内 8 个实例）；exec 工具报 `helper_unknown_error: setup refresh had errors`（cmd.exe / pwsh.exe 全部被拒）；回合结束尝试启动不存在的 `codex-computer-use.exe`。
- 原因：应用自动更新运行时（`cua_node/2f053e67fec2d258` → `1cb4becc994cbb02`），`C:\Users\hh\.codex\config.toml` 里 `[model_providers.deepseek] notify` 仍是旧路径；重启后沙箱初始化窗口（`windowsSandbox/setupStart` → `cap_sid` 生成，约 30-60 秒）内 exec 一律失败；重启早期 github.com 443 超时（代理未就绪）导致应用反复重启。
- 解决：notify 路径改为现有运行时 `1cb4becc994cbb02`（与顶层 notify 一致），改前备份 `config.toml.bak-20260817-2359`；沙箱初始化完成后 exec 自愈。
- 验证：notify 目标文件存在；exec 与 node_repl 正常；DeepSeek API 实测 `deepseek-chat` 仍是别名（返回 `deepseek-v4-flash`），`/models` 当前仅 `deepseek-v4-flash` / `deepseek-v4-pro`。
- 备注：`[windows] sandbox = "elevated"` 是官方合法值（elevated/unelevated），不是错误；`thread_tools` 未知 feature 警告与远程插件 401 均为无害噪音。

### 2026-08-18 整机重启后环境恢复顺序（Docker Desktop / 内置 K8s / 端口转发）
- 现象：重启后 `docker-desktop` WSL 发行版 Stopped；引擎起来后 kubectl 一度报 `nodes is forbidden`（RBAC 未就绪）；controller-manager Pod Error（`dial tcp 10.96.0.1:443: i/o timeout`）；`localhost:8080` 无监听（port-forward 进程随重启消失）。
- 原因：Docker Desktop 引擎需手动启动；内置 K8s 节点容器逐个拉起约 40-60 秒（期间 API server 可连但 RBAC/controller 未就绪）；controller-manager 在 API 未就绪时启动即退出；端口转发是前台进程，重启即丢。
- 解决顺序：启动 Docker Desktop → 等 `docker version` server 非空 → 等 `kubectl get nodes` 全 Ready（约 40s）→ `kubectl -n hello-k8s-ai-system rollout restart deploy/hello-k8s-ai-controller-manager` → `make cluster-open`（恢复 8080/18080 单入口）→ `/api/v1/health/ready` 验收。
- 验证：10 worker 节点 Ready；controller/backend/frontend/可观测组件全 1/1；ready API 200；PostgreSQL 历史完整（34.6 万事件、2223 快照）。
- 备注：Ubuntu 发行版重启后自动 Running；Kind 集群（e2e/minikserve）容器自动恢复；`make cluster-down` 只缩负载不删 CR，重启后 CR 全在。

### 2026-08-18 gocyclo 圈复杂度 31 > 30：给高复杂度函数加分支前先评估
- 现象：CI lint 失败 `gocyclo: cyclomatic complexity 31 > 30`（`internal/controller/simulatorinstance_placement.go`）。
- 原因：`reconcileDeploymentObjects` 原有复杂度 ~28（计划解码/校验/物化多职责叠加），修"replicas=0 与放置计划失配死锁"时内联"暂停清理"分支（多条件 && 链 + Update 错误处理）后越过阈值。
- 解决：提取 `clearPlacementPlanWhenPaused` helper（提交 `d4dc41c`），主函数只留一行调用，逻辑零变化。
- 验证：CI 全绿；`TestSimulatorInstancePauseClearsPlacementPlan` 通过。
- 备注：改这类接近阈值的函数前先 `gocyclo` 评估；超阈值优先按职责拆函数，不用 `//nolint` 掩盖。
