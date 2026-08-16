# 实现修改明细

## 1. 背景（改动前状态）

- `hack/local-cluster.sh up` 固定执行 `deploy_demo`：apply `config/demo`（tenant-sample、model-sample、tenantmodelpolicy-sample、orchestrator-sample、simulationclock/default），并按真实 Worker Node 动态创建 WorkerNode/NodePolicy；`verify_data_flow` 依赖 `tenant-sample` 与 `snapshot-*` 验收。
- Backend `persistSnapshot`（`dashboard/backend/internal/app/app.go`）每 `SnapshotInterval`（默认 30s）无条件写 `resource_snapshots`，即使集群没有业务资源也会产生 `snapshot-*` 记录。
- 前端 `templateSlice`/`trafficSlice` 模板数组初始为空；新建资源走 `DEFAULT_*`；无参数说明页面。
- 时间栏初始 timestamp 为 1970；ConfigPage 连接徽标恒显示“Backend 已连接”；Grafana 面板 Trace 链接写死 `http://localhost:16686`（未暴露）。

## 2. 改动清单

### 2.1 hack/local-cluster.sh

- 新增环境变量 `DEMO_ENABLED`（默认 false）。
- `run_up`：仅 `DEMO_ENABLED=true` 时执行 `deploy_demo`，否则打印“保持干净环境”。
- `verify_data_flow` 拆分：
  - 基础链路（总是执行）：Backend ready、`/clock` 倍速能力、SimulationClock `1|1|True`、Frontend HTML。
  - 演示链路（`DEMO_ENABLED=true`）：`/configuration` 含 tenant-sample、instance timeScale=1、`/replay` 有 snapshot、Prometheus 两个指标、Jaeger services。
  - 干净模式（否则）：`verify_clean_state`——断言 `kubectl get tenants,models,orchestrators,simulatorinstances,workernodes` 为空；`/replay` 不含 `snapshot-`。
- `usage` 增加 `DEMO_ENABLED` / `DEMO_MODEL_ABSOLUTE_SCORE` 说明。

### 2.2 dashboard/backend/internal/app/app.go

- `persistSnapshot` 开头新增 `snapshotHasBusinessData` 判定：无业务资源时记 Debug 日志并 return，不写快照、不 upsert resource_states。
- 新增 `snapshotHasBusinessData(snapshot model.CurrentSnapshot) bool`：Models / WorkerNodes / Tenants / Policies（三种）/ Orchestrators / SimulatorInstances / TenantPerformance / TenantRuntimes 任一非空即视为有业务数据；`SimulationClock/default` 与系统工作负载不构成业务历史。

### 2.3 前端 dashboard/frontend/my-app

- 新增 `src/lib/constants/presetTemplates.ts`：模型模板（轻量在线推理 8G/16/75/800ms、标准 16G/32/100/1500ms、批量 32G/64/60/5000ms，性能画像 50/500/20）、节点模板（80G/128、32G/48、8G/16）、租户模板（P1/20 QPS 800/150/300/40、P3/10 QPS 500/100/200/30、P5/0 QPS 2000/300/800/60）、编排模板（核心 60/120/false/1..10、弹性 30/90/true/1..20、保守 120/240/false/1..4）、流量模板（平稳 10 QPS/300s、脉冲 0-50-0、斜坡 0→25/300s）。所有值通过对应 zod schema。
- `src/types/config.types.ts`：`ConfigTemplate<T>` 增加可选 `preset?: boolean`。
- `src/stores/templateSlice.ts`：四个模板数组初始值注入预置模板；add/remove 语义不变。
- `src/stores/trafficSlice.ts`：`templates` 初始值注入预置流量模板。
- `src/components/shared/dialogs/TemplateLibraryDialog.tsx`：预置模板卡片加“预置”Badge；新增 `pickMode`（隐藏删除、按钮文案“使用此模板”、标题“从模板新建”）。
- `src/components/features/config/ConfigPage.tsx`：
  - 新增 `CreateTemplateSource` 状态与 `openCreateFromTemplate`/`applyCreateTemplate`：模板选择 → 预填名称进 CreateDialog → `confirmCreate` 用模板 data 组装资源（model/node/tenant/orchestrator 各自分支），成功后清空模板源；普通新建不受影响（`openCreate` 重置模板源）。
  - 连接徽标改为 `connectionMeta[workspace.cluster.connectionStatus]` 三元渲染（已连接/连接中/未连接），历史只读分支保留。
  - 四个资源 Tab 传入 `onCreateFromTemplate`。
- `src/components/features/config/components/ConfigTabPanel.tsx`：工具栏新增“从模板新建”次级按钮（仅 `onCreateFromTemplate` 存在时显示）；空态增加同入口与“查看填写指南”链接；脏表单确认丢弃后继续模板新建（`pendingTemplateCreate`）。
- 新增 `src/components/features/guide/GuidePage.tsx`：标识符规则、模型、租户、节点、编排策略、策略、流量、系统参数速查表（22 项，含归属标注）、模拟填写建议、预置模板一览；复用 `ConfigFormSection`。
- `src/app/router.tsx`：新增 `/guide` 懒加载路由。
- `src/components/shared/Layout/AppSidebar.tsx`：navigation 增加“填写指南”（BookOpen）。
- `src/components/features/traffic/TemplateLibrary.tsx`：空态增加“查看填写指南”链接。
- `src/components/shared/TimeTravelBar/FullscreenTimeline.tsx`：无权威时间（1970 占位）时精确跳转输入框置空禁用并提示“等待数据…”。

### 2.4 config/observability/grafana.yaml

- 面板 Trace 链接 `http://localhost:16686` → `http://localhost:8080/trace`，标题改为“查看 Trace 明细”（与单入口架构一致）。

### 2.5 文档

- 根 `README.md`：一键启动步骤 7/8 改为干净语义；新增“干净环境 + 模板 + 指南页”能力行；常用命令增加 `DEMO_ENABLED` 示例。
- `docs/getting-started/LOCAL_RUN.md`：步骤 6-8 改为 `DEMO_ENABLED` 语义；补充干净环境说明与填写指南入口。

## 3. 不做的事（范围外）

- 不引入后端配置 API / 不改 CRD Schema（`api/v1/` 无改动）。
- 不改 Traffic Overlay 提交（`PATCH /tenants/{name}/traffic` 部分实现现状保留）。
- 不删除 `resource_events` 中的系统 Lease/Node 心跳事件（真实遥测）。