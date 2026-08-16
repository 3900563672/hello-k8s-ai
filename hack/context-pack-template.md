# hello-k8s-ai 上下文包（CONTEXT_PACK）

> 生成时间：__GENERATED_AT__（UTC）
> 生成方式：`make context-pack`（hack/gen-context-pack.sh），源模板 `hack/context-pack-template.md`。
> 本文件是打包内容的**第一入口**。远程 AI 请先读本文件，再按需读包内其他文件；能操作本机的 Agent 不需要本包。

## 1. 项目一句话

基于 Kubernetes 的 AI 推理调度与仿真平台：React Frontend → Dashboard Backend → CRD → 7 个 Controller → Simulator → Prometheus / OpenTelemetry / Jaeger → Backend 聚合展示。Kubernetes API Server 是唯一事实源；PostgreSQL 只存历史快照、事件与审计。

## 2. 当前状态基线（生成时）

- 分支：`__BRANCH__`
- 最近提交：

__RECENT_COMMITS__

- Open Issues：

__OPEN_ISSUES__

- 若上面两项为空，说明生成环境无网络或 gh 未认证；以 git 记录和包内文档为准。

## 3. 仓库地图

__TREE__

## 4. 关键文件（事实源）

| 路径 | 内容 |
| --- | --- |
| `api/v1/` | CRD Go 类型与 Kubebuilder 标记 |
| `cmd/main.go` | Controller Manager 入口 |
| `internal/controller/` | 7 个 Controller |
| `simulator/` | Simulator、Lease 选主、Metrics 与 Trace |
| `dashboard/backend/` | Backend API、Kubernetes cache、数据库与 Provider |
| `dashboard/frontend/my-app/` | React 控制台 |
| `config/` | CRD、RBAC、开发栈、样例与可观测性清单 |
| `docs/` | 文档体系：人类（docs/）、Agent（docs/agents/）、远程 AI（docs/remote-ai/） |
| `change-history/` | 变更与决策记录（按日期） |

## 5. 约束边界（摘要）

- Kubernetes API Server 是当前状态的唯一事实源；PostgreSQL 不能反向驱动 Controller。
- Frontend 只调用 Backend；Backend 写接口只修改白名单 CR 的 Spec。
- Traffic Controller 写 `SimulatorInstance.spec.traffic.qps`；SimulationClock Controller 写 `spec.timeScale`；Orchestrator 写副本数、有效分数与扩缩计划；Simulator 写性能、可用副本与运行状态。
- 遥测失败不能阻止控制面或 Simulator 启动。
- 不编辑生成文件：`config/crd/bases/*.yaml`、`config/rbac/role.yaml`、`**/zz_generated.*.go`。
- 不宣称未验证的运行状态。
- 完整约束见 `docs/agents/PRINCIPLES.md` 与 `AGENTS.md`。

## 6. 文档分层

- 人类：根目录 `README.md`、`PROJECT_OVERVIEW_NEW.md`（初学者）、`docs/INDEX.md`。
- 本地 Agent（能操作本机）：`AGENTS.md`、`docs/agents/`。
- 远程 AI（你）：`docs/remote-ai/`（本包已包含全部）。

## 7. 验证命令速查（由本地 Agent 执行，你只能引用）

- Go 控制面：`make fmt` / `make vet` / `make test` / `make lint`
- Dashboard Backend：`gofmt -w . && go vet ./... && go test ./...`
- Frontend：`cd dashboard/frontend/my-app && npm run check`
- 清单：`kubectl kustomize config/dev`、`config/demo`、`dashboard/deploy`
- 文档与打包：`make docs-check`、`make context-pack`
- CI：代码检查 / 源码与部署验证 / E2E 测试三个 workflow