# 源码地图

## 1. 根控制面

| 路径 | 事实/职责 | 相关文档 |
| --- | --- | --- |
| `PROJECT` | Kubebuilder version、API resource scaffold metadata | CRD 设计 |
| `go.mod` | Go/K8s/controller-runtime/OTel 版本 | 开发环境 |
| `api/v1/*_types.go` | CRD Spec/Status、validation/default/print columns | CRD 设计、字段所有权 |
| `cmd/main.go` | Manager flags、tracing、Controller 注册、leader election | Controller/配置参考 |
| `internal/controller/tenant_controller.go` | TenantModelPolicyReconciler | Controller 架构 |
| `internal/controller/simulatorinstance_controller.go` | Deployment/TenantRuntime | Controller/生命周期 |
| `internal/controller/traffic_controller.go` | Score-based QPS allocation | Controller/数据流 |
| `internal/controller/performancecollector_controller.go` | TenantPerformance | Controller/聚合 |
| `internal/controller/performance_calculator.go` | 稳健加权算法 | Controller |
| `internal/controller/workernodeusage_controller.go` | Pod -> WorkerNode usage | Controller |
| `internal/controller/orchestrator_*.go` | input、decision、score、executor/recovery | Controller |
| `internal/controller/policy_helpers.go` | Policy 交集/Allow/Deny | CRD/Controller |
| `internal/controller/metadata.go` | labels/annotations/name/plan metadata | 字段所有权 |
| `internal/controller/observability.go` | metrics/spans helpers | Prom/OTel |
| `internal/observability/tracing.go` | OTel provider/client instrumentation | OTel |
| `simulator/main.go` | flags、Lease leader、observability server | Simulator |
| `simulator/simulator.go` | Tick、score、Status patch | Simulator |
| `simulator/engine.go` | 离散事件引擎/Poisson/queue | Simulation Flow |
| `simulator/coldstart.go` | 冷启动曲线 | Simulation Flow |
| `simulator/metrics.go` | Simulator Prom metrics | Prom/Simulator |

## 2. Kubernetes 配置

| 路径 | 内容 |
| --- | --- |
| `config/crd/bases/` | 自动生成 10 CRD，禁止手改 |
| `config/rbac/role.yaml` | 自动生成 Manager RBAC，禁止手改 |
| `config/rbac/simulator_*` | Simulator SA/ClusterRole/Binding |
| `config/manager/manager.yaml` | Controller Deployment 基线 |
| `config/default/` | CRD/RBAC/Manager 默认组合 |
| `config/dev/` | 默认 + observability + dev patch |
| `config/demo/` | 不写死 Node 的静态演示 Model/Tenant/Policy/Orchestrator |
| `config/samples/` | 用户配置示例（只应用 Kustomization 中列出的 7 个） |
| `config/observability/` | OTel/Jaeger/Prom/Grafana |
| `config/network-policy/` | metrics 基础策略，非完整 production policy |
| `hack/local-cluster.sh` | Docker Desktop 一键构建、镜像导入、部署、验收、状态与停止 |
| `hack/cleanup-obsolete.sh` | 覆盖旧目录后的显式废弃文件清理 |

## 3. Dashboard Backend

| 路径 | 职责 |
| --- | --- |
| `dashboard/backend/cmd/server/main.go` | Backend entry |
| `internal/app/app.go` | dependency lifecycle |
| `internal/config/config.go` | env/default/validation |
| `internal/api/server.go` | routes |
| `internal/api/handlers_read.go` | health/config/traffic/metric/trace/overview |
| `internal/api/handlers_command.go` | apply/delete/qps/audit |
| `internal/api/idempotency.go` | mutation replay |
| `internal/api/middleware.go` | request/CORS/timeout/recovery/security |
| `internal/api/sse.go`, `events.go` | stream hub/notifications |
| `internal/kubernetes/client.go` | dynamic/typed/discovery clients |
| `internal/kubernetes/cache.go` | informers/sync/index/events |
| `internal/kubernetes/resources.go` | descriptors/GVR/watch list |
| `internal/kubernetes/mapper.go` | objects -> DTO |
| `internal/kubernetes/commands.go` | write allowlist/dry-run/gateway |
| `internal/readmodel/aggregator.go` | Configuration/Traffic/Workloads/Snapshot |
| `internal/providers/prometheus/client.go` | catalog/PromQL/query normalization |
| `internal/providers/jaeger/client.go` | Jaeger Query API/Span tree |
| `internal/store/migrations/001_initial.sql` | DB schema |
| `internal/store/postgres.go` | connection/migrate/query/write |
| `internal/store/recorder.go` | async informer event recorder |
| `internal/clock/clock.go` | rate=1 authoritative clock |
| `internal/model/types.go` | API read model DTO |

## 4. Frontend

| 路径 | 职责 |
| --- | --- |
| `dashboard/frontend/my-app/src/app/router.tsx` | `/config` `/traffic` `/trace` |
| `src/components/shared/Layout/MainLayout.tsx` | app shell + sync |
| `src/components/shared/TimeTravelBar/` | latest/historical UI |
| `src/components/features/config/` | Config page/forms/tables |
| `src/components/features/traffic/` | templates/canvas/overlay |
| `src/components/features/trace/DataOverviewPage.tsx` | Data View + Trace |
| `src/api/client.ts` | envelope/problem HTTP client |
| `src/api/endpoints/` | domain endpoints |
| `src/api/queries/` | TanStack Query |
| `src/hooks/useBackendSync.ts` | initial sync/SSE/poll |
| `src/stores/controlPlaneSlice.ts` | cluster/provider/capability state |
| `src/stores/timeSlice.ts` | replay selection/viewport |
| `src/stores/trafficSlice.ts` | in-memory traffic drafts |
| `src/types/` | API/frontend types |
| `vite.config.ts` | dev proxy/build |
| `nginx.conf` | SPA/API/SSE production serving |

## 5. Dashboard 部署

| 路径 | 内容 |
| --- | --- |
| `dashboard/deploy/kustomization.yaml` | Dashboard resource composition |
| `postgresql.yaml` | Service/StatefulSet/PVC；Secret 由部署脚本生成 |
| `backend.yaml` | Backend Deployment/Service/env/probes |
| `frontend.yaml` | Nginx Deployment/Service |
| `rbac.yaml` | Backend ClusterRole/Binding |
| `service-account.yaml` | Backend SA |

## 6. 测试

| 路径 | 范围 |
| --- | --- |
| `internal/controller/*_test.go` | Controller unit/envtest/integration |
| `simulator/*_test.go` | engine/HTTP/refactor |
| `internal/observability/*_test.go` | tracing |
| `test/e2e/` | isolated Kind E2E (需 tag/tooling) |
| `dashboard/backend/internal/**/*_test.go` | API/idempotency/mapper/providers/aggregator/store |
| `dashboard/frontend/my-app/verification/state-check.ts` | state contract smoke |

## 7. 已清理的历史材料

旧 `docs/observability.md`、`docs/validation.md`、`docs/yaml/` 和 Frontend `DOCS/` 与当前实现重复或冲突，已从交付包移除。对应信息由当前专题文档、`config/samples/` 和可执行验证命令维护。
