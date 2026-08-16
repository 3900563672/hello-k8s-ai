# hello-k8s-ai 维护指南

## 开始前

1. 文档按读者分层：人类看 `docs/` 与根目录 README；能操作本机的 Agent 看本文件与 `docs/agents/`；只收打包内容的远程 AI 看 `docs/remote-ai/`。
2. 每次任务先读 `docs/agents/README.md` 与 `docs/agents/WORKFLOW.md`，按流程判断是否需要建 issue。
3. 动手前扫一遍 `docs/agents/KNOWN_PITFALLS.md`。
4. 涉及 CRD、Controller 或写 API 时，先核对 `docs/agents/PRINCIPLES.md` 与 `docs/kubernetes/FIELD_OWNERSHIP.md`。

源码和可执行清单优先于说明文档。没有运行证据时，不得把“清单中存在”写成“集群已就绪”。

## 文档维护边界

- `docs/` 是人类文档：Agent 默认不读、不改；需要背景时按需阅读，事实以源码、生成清单和可执行测试为准。
- Agent 只维护 `docs/agents/` 与 `change-history/`；修改人类文档需用户明确要求（或先列清单等授权）。
- 每次交付后按 `docs/agents/SYNC.md` 同步：追加 `change-history/` 条目、更新本目录受影响文档、重新生成上下文包（`make context-pack`）、列出人类文档待同步清单。

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
config/                       CRD、RBAC、Docker Desktop 开发栈、样例和可观测性清单
docs/                         当前工程文档
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

不要删除 `+kubebuilder:scaffold:*`、`+kubebuilder:rbac:*` 和 CRD 校验标记。

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

完整开发栈只复用当前 `docker-desktop` Context：

```bash
bash setup.sh
```

部署脚本不得创建、重置或删除 Kubernetes 集群，也不得调用旁边的 `minikserve-demo`。停止本项目使用 `make cluster-down`，它只把工作负载缩到 0，并保留 CRD、CR、Secret 和 PVC。

自动化 E2E 仍必须使用独立 Kind 集群 `hello-k8s-ai-test-e2e`，不能复用日常开发或共享集群。E2E 清理会删除该测试集群，执行前核对固定名称。

## 交付检查

- 不提交 `.env`、`node_modules/`、`bin/`、`dist/`、覆盖率文件、IDE 配置或下载附加文件。
- Frontend 只保留 `package-lock.json`，使用 npm。
- 文档链接必须指向现有文件；旧实现只能以明确的历史背景出现。
- 最终说明列出实际执行的命令、未验证范围和真实风险。
