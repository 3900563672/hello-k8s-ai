# 一键启动默认干净环境：预置配置模板与参数填写指南

- 变更日期：2026-08-16
- 关联问题：#23（design: 一键启动默认干净环境，预置配置模板与参数填写指南）
- 变更级别：P1
- 变更范围：`hack/local-cluster.sh`、`dashboard/backend/internal/app/app.go`、前端（预置模板 / 从模板新建 / `/guide` 填写指南页 / 时间栏与连接徽标小修）、`config/observability/grafana.yaml`、根 `README.md`、`docs/getting-started/LOCAL_RUN.md`、`change-history/`
- CRD 变化：无
- 数据库变化：无 Schema 变化；行为变化——空配置（无用户业务资源）不再写入历史快照

## 1. 完成结果

一键启动（`bash setup.sh` → `make cluster-up`）从“强制写入演示数据”改为“默认干净环境”：

- `hack/local-cluster.sh` 新增 `DEMO_ENABLED`（默认 false）：关闭时跳过 `deploy_demo`，并在验收阶段断言业务 CR 为空、`/replay` 无历史快照；开启时保持原演示链路与验收。
- Backend 快照持久化改为“空配置跳过”：没有模型/租户/节点/策略/编排器/实例等业务资源时不再每 30s 写入空快照，保证“没有运行就没有历史切面”；`SimulationClock/default` 与系统 Pod/物理节点不构成业务历史。
- 前端新增预置模板（模型/节点/租户/编排策略各 3 条 + 流量 3 条），模板只预填表单，提交与运行由用户决定，不产生任何 CR 或历史数据。
- “从模板新建”进入配置中心：模板库支持 pickMode（使用此模板），新建时用模板数据替代 `DEFAULT_*` 组装，仍走原有幂等写接口。
- 新增 `/guide` 填写指南页：字段含义、单位、默认值、系统参数速查表（用户可配置/系统常量/开发测试）、模拟条件下的取值建议；侧边栏新增“填写指南”入口，配置中心与流量模板空态增加指南链接。
- 顺手修复：时间栏在权威时间到达前不展示 1970 纪元时间（禁用精确跳转并提示）；配置中心“Backend 已连接”徽标改为跟随真实连接状态；Grafana 面板“查看 Trace”链接从不可达的 `localhost:16686` 改为 Dashboard 单入口 `/trace`。

## 2. 关键行为

- 干净环境下：`kubectl get tenants,models,...` 为空；PostgreSQL 业务表为空（`resource_events` 仍会记录真实系统 Lease/Node 心跳事件，属系统遥测而非假数据）；`/replay` 无 `snapshot-*`。
- `DEMO_ENABLED=true make cluster-up` 完全复现原演示链路（写入 sample CR、动态 WorkerNode/Policy、触发实例并全量验收）。
- 预置模板是内存级静态资源：删除后刷新页面会重新出现，不写入 Kubernetes，也不使用 localStorage。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| hack/local-cluster.sh | `DEMO_ENABLED` 开关、验收拆分（基础链路 + 演示链路）、`verify_clean_state`、usage |
| dashboard/backend | `persistSnapshot` 空配置跳过 + `snapshotHasBusinessData` 判定 |
| dashboard/frontend/my-app | `presetTemplates.ts`、templateSlice/trafficSlice 预置注入、ConfigPage/ConfigTabPanel 从模板新建、TemplateLibraryDialog pickMode、GuidePage、router/AppSidebar、FullscreenTimeline 1970 占位、ConfigPage 连接徽标 |
| config/observability | grafana.yaml 面板 Trace 链接改为单入口 |
| 文档 | README、LOCAL_RUN 同步干净环境语义与 `DEMO_ENABLED` |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 实测结论与停止线

- `npm run check`（lint + tsc + vite build + state verify）通过；Backend `go build`/`go vet` 通过；`bash setup.sh`（干净模式）全流程验收结果见 TEST_REPORT。
- 停止线：本次只做“干净环境 + 模板 + 指南页”，不引入后端配置 API、不改造 Traffic Overlay 提交（仍是部分实现）、不改动 CRD Schema。
