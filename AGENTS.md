# hello-k8s-ai 维护指南

## 常用命令（开工前先看）

```bash
make fmt && make vet && make test && make lint   # Go 控制面
make docs-check && make docs-sync-check          # 文档一致性与生成物漂移
make docs-sync && make context-pack              # 重生成派生文件 / 远程 AI 上下文包
make cluster-status / cluster-open / cluster-urls  # 集群状态 / 端口转发 / 访问地址
make cluster-down                                # 停止工作负载，保留集群、CRD、CR、Secret 与 PVC
bash setup.sh                                    # 完整开发栈部署（Kind 开发集群 hello-k8s-ai-dev）
```

## 三层行为准则

### 必须（Always）

1. 每次任务先读 `docs/agents/README.md` 与 `docs/agents/WORKFLOW.md`，按流程判断是否需要建 issue。
2. 动手前扫 `docs/agents/FAILURE_REGISTRY.md` 末尾 3 条 + `docs/journal/` 与 `docs/lessons/`（失败模式注册表 + 踩坑流水账与蒸馏规则），命中先读证据链。
3. 涉及 CRD、Controller 或写 API 时，先核对 `docs/agents/PRINCIPLES.md` 与 `docs/kubernetes/FIELD_OWNERSHIP.md`。
4. 涉及 GitHub Issue / Project 看板 / 批量任务时，先读 `docs/agents/PROJECT_REVIEW.md`（看板状态机与闭环规则，只动 `Approved` 条目）。
5. 源码和可执行清单优先于说明文档；没有运行证据时，不得把"清单中存在"写成"集群已就绪"。
6. 每次交付后按 `docs/agents/WORKFLOW.md` 第 9 节同步：追加 `change-history/` 条目、更新受影响文档、重跑 `make docs-sync` 与 `make docs-check`、列出人类文档待同步清单。
7. 脚本类改动（`.sh`/`.ps1`/`.mjs`）与 Agent 文档改动必须过静态检查：`make lint-sh`（shellcheck）、`make lint-ps1`（PSScriptAnalyzer）、`make lint-md`（markdownlint）——已并入 `make lint` / `make verify` 与 CI。
8. 开工与长跑前先 `make doctor` 环境自检（磁盘 / Docker / WSL 回环 / 端口 / 内存 / tmpfs / dmesg），FAIL 项修复后再继续。
9. 一切皆异步：预计超过 ~30s 的等待必须并行推进其他有用工作（先查证预期时长，再查历史/沉淀/维护 issue），禁止空转死等；长等待一律后台化并汇报“等什么/预计多久/期间在做什么”。
10. 默认不截图：页面/UI 效果由用户自己在浏览器查看（把本地服务地址告诉用户即可），禁止为验证 UI 主动截图；截图仅在用户明确要求、或必须视觉确认（文档/绘图/设计产物）时使用。前端验证用 `npm run check` + DOM 断言/控制台错误检查完成。

### 先问（Ask）

1. 大改、重构、删除文件或调整部署脚本（`setup.sh`、Makefile 目标）前，先说明方案与影响面。
2. 人类文档 `docs/` 默认不读、不改；需要改动时先列出清单再动手。例外：README 与 `docs/` 中访问方式、架构、行为描述因本次改动过期时，必须同步更新并纳入本次提交（文档漂移检查是强制步骤，不得只归档不改文档）。
3. 动 Kubernetes 集群（重启节点、cordon/uncordon、删资源）、发起长时运行、合并 PR 或直接推送 main 前，先确认。
4. 修改"不可破坏的边界"（见下）涉及的任何语义前，先说明影响面与回滚方案。

### 禁止（Never）

1. 不手工修改生成文件：`config/crd/bases/*.yaml`、`config/rbac/role.yaml`、`api/v1/zz_generated.deepcopy.go`、`PROJECT`；不删除 `+kubebuilder:scaffold:*`、`+kubebuilder:rbac:*` 与 CRD 校验标记。
2. 不执行 `wsl --shutdown`、不强杀 Docker Desktop、不动代理配置（除非用户明确同意）。
3. 部署脚本不得重置、删除或切换日常开发集群（`hello-k8s-ai-dev`），也不得调用旁边的 `minikserve-demo`；`make cluster-up` 只允许创建/复用固定 Kind 集群 `hello-k8s-ai-dev`（幂等），不得操作 docker-desktop 内置 K8s；自动化 E2E 只使用独立 Kind 集群 `hello-k8s-ai-test-e2e`，不复用日常开发或共享集群。
4. 遥测失败不能阻止控制面或 Simulator 启动；不把 Frontend 的 pause/seek 或确定性回放扩展进 `SimulationClock`。
5. 不提交 `.env`、`node_modules/`、`bin/`、`dist/`、覆盖率文件、IDE 配置或下载附加文件；Frontend 只保留 `package-lock.json`（npm）。

## 工程结构

```text
api/v1/                       CRD Go 类型与 Kubebuilder 标记
cmd/main.go                   Controller Manager 入口
internal/controller/          7 个 Controller
internal/observability/       Controller Trace 接入
simulator/                    Simulator、Lease 选主、Metrics 与 Trace
dashboard/backend/            Backend API、Kubernetes cache、数据库与 Provider
dashboard/frontend/my-app/    React 控制台
dashboard/deploy/             Dashboard 与 PostgreSQL 清单
config/                       CRD、RBAC、Kind 开发栈（兼容 docker-desktop 旧 Context）、样例和可观测性清单
docs/                         人类文档（Agent 默认不读、不改）
docs/agents/                  本地 Agent 手册与工作流
docs/remote-ai/               远程 AI 唯一入口（收打包内容）
change-history/               变更归档（两代格式，见 README）
```

## 不可破坏的边界

- Kubernetes API Server 是当前状态的唯一事实源。
- PostgreSQL 只保存历史快照、资源事件、审计和幂等记录，不能反向驱动 Controller。
- Frontend 只调用 Dashboard Backend，不直接访问 Kubernetes、Prometheus、Jaeger 或 PostgreSQL。
- Backend 写接口只修改白名单 CR 的 Spec，不写 Controller 所有的 Status。
- Traffic Controller 写 `SimulatorInstance.spec.traffic.qps`。
- SimulationClock Controller 写 `SimulatorInstance.spec.timeScale`；Simulator 每个 Tick 动态读取，不因倍速变化重建 Pod。
- Orchestrator 写副本数、有效分数和扩缩计划；Simulator 写性能、可用副本与运行状态。
- 遥测失败不能阻止控制面或 Simulator 启动。
- Backend/Controller 的逻辑时间仍是墙钟；`SimulationClock.spec.rate` 只加速 Simulator 离散事件引擎，不能扩展成 Frontend 伪造的 pause/seek 或确定性回放。

## 生成文件

下列文件由工具生成，不要手工修改：

- `config/crd/bases/*.yaml`
- `config/rbac/role.yaml`
- `api/v1/zz_generated.deepcopy.go`
- `PROJECT`

修改 `api/v1/*_types.go` 或 Kubebuilder 标记后执行：

```bash
make manifests
make generate
```

检查生成差异，确认只包含预期的 Schema、RBAC 或 DeepCopy 变化。

## 修改原则

- 先确认问题可以复现或由代码直接证明，再修改。
- 保持现有架构；不要为风格统一做无关重构。
- Reconcile 必须幂等，写入冲突使用最新 ResourceVersion 重试。
- 保留 OwnerReference、finalizer、Watch 和字段索引的既有语义。
- 新日志使用结构化键值，避免把对象名或错误文本放入 Metrics Label。
- 注释说明约束、原因和非明显逻辑，不复述代码。
- 普通说明使用中文；Kubernetes、Controller、CRD、API、Prometheus、OpenTelemetry、Jaeger 等技术名词保留英文。

## 验证

Go 控制面：

```bash
make fmt
make vet
make test
make lint
```

环境与脚本层：`make doctor`（环境自检）；`make lint-sh` / `make lint-md` / `make lint-ps1`（静态检查，已并入 `make lint` 与 `make verify`）。

Dashboard Backend：

```bash
cd dashboard/backend
gofmt -w .
go vet ./...
go test ./...
```

Frontend：

```bash
cd dashboard/frontend/my-app
npm ci
npm run check
```

清单：

```bash
kubectl kustomize config/dev >/tmp/hello-k8s-ai-control.yaml
kubectl kustomize config/demo >/tmp/hello-k8s-ai-demo.yaml
kubectl kustomize dashboard/deploy >/tmp/hello-k8s-ai-dashboard.yaml
```

若当前环境缺少命令或集群，只记录未验证项，不复用旧结果冒充本次验证。

## 本地部署与 E2E

完整开发栈使用 Kind 开发集群 `hello-k8s-ai-dev`（默认 context `kind-hello-k8s-ai-dev`，`make cluster-up` 幂等创建/复用；兼容 docker-desktop 旧 Context）：

```bash
bash setup.sh        # = ./hack/cleanup-obsolete.sh + make cluster-up
```

`make cluster-up` 只允许创建/复用固定 Kind 集群 `hello-k8s-ai-dev`，不得重置、删除或切换集群，也不得调用旁边的 `minikserve-demo`。停止本项目使用 `make cluster-down`，它只把工作负载缩到 0，并保留集群、CRD、CR、Secret 与 PVC；删除开发集群用 `make kind-down`（PVC 数据保留在节点 /var 数据卷，重建后按 hack/kind/restore-data.sh 显式恢复）。

自动化 E2E 必须使用独立 Kind 集群 `hello-k8s-ai-test-e2e`，不能复用日常开发集群（`hello-k8s-ai-dev`）或共享集群。E2E 清理会删除该测试集群，执行前核对固定名称。

## 交付检查

- 不提交 `.env`、`node_modules/`、`bin/`、`dist/`、覆盖率文件、IDE 配置或下载附加文件。
- Frontend 只保留 `package-lock.json`，使用 npm。
- 文档链接必须指向现有文件；旧实现只能以明确的历史背景出现。
- 最终说明列出实际执行的命令、未验证范围和真实风险。
