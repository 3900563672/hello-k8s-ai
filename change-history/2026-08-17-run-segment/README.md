# 时间段切面（Run Segment）：起点/终点全局状态 + 区间指标与 Trace

- 变更日期：2026-08-17（Asia/Shanghai 20:00~20:40；UTC 12:00~12:40）
- 关联问题：[#27](https://github.com/3900563672/hello-k8s-ai/issues/27)（design: 建立时间段切面）
- 变更级别：P1 历史分析与可观测性
- 变更范围：`dashboard/backend/internal/api/handlers_segment.go`（新增）、`dashboard/backend/internal/model/types.go`、`dashboard/frontend/my-app/src/components/features/trace/SegmentPanel.tsx`（新增）及相关类型/API/queries
- CRD 变化：无 ｜ 数据库变化：无（复用 `resource_snapshots`，无迁移）

## 1. 为什么做

历史"切面"此前是**时刻点**：`/overview?at=T` 返回 T 之前最近的一个 30 秒快照，指标/Trace 只回看以该点为终点的 15 分钟窗口。用户做故障复盘与容量分析时需要的是一次调度/实验的**时间段**：起点全局状态 + 终点全局状态 + 区间指标与 Trace。点查询回答不了"从什么状态开始、到什么状态结束、中间发生了什么"。

## 2. 完成结果

1. **新接口 `GET /api/v1/segment?start=<RFC3339>&end=<RFC3339>`**（可选 tenant/model/instance 过滤）：返回起点快照（start 之前最近）、终点快照（end 之前最近）、`[start,end]` 区间 5 项指标、区间 Trace、freshness 与覆盖告警；窗口上限 24 小时，`start < end` 必校验。
2. **不伪造数据**：任一端无快照或存储不可用时 `availability=unavailable` + 明确中文告警（"起点之前没有持久化快照…"），不返回伪造数据（遵守 PRINCIPLES「历史不能冒充当前」）。
3. **段级覆盖告警**：针对段起点/终点分别计算 Prometheus/Jaeger 保留窗口告警，区分"段内没有数据"与"数据超出保留窗口已丢失"。
4. **前端段面板**（trace 页新增"时间段切面（Run Segment）"区块）：从时间轴快照列表选择起点/终点 → 请求 `/segment` → 展示起点→终点 8 项状态对比（Tenant/Model/WorkerNode/Simulator/副本/QPS/Pod Ready/Deployment Ready）、区间指标曲线、区间 Trace 与告警；时间轴"标记起点/终点"的视觉交互留到后续 UI 阶段。
5. 无数据库迁移、无 CRD/Controller 改动；`/overview` 点查询保持不变。

## 3. 影响文件

| 文件 | 变更 |
| --- | --- |
| `dashboard/backend/internal/api/handlers_segment.go` | 新增：段查询 handler、参数解析、覆盖告警 |
| `dashboard/backend/internal/api/handlers_segment_test.go` | 新增：参数校验/覆盖告警/unavailable 分支测试 |
| `dashboard/backend/internal/api/server.go` | 注册 `GET /api/v1/segment` |
| `dashboard/backend/internal/model/types.go` | 新增 `SegmentSnapshot` / `SegmentOverview` |
| `dashboard/frontend/my-app/src/components/features/trace/SegmentPanel.tsx` | 新增：段面板 |
| `dashboard/frontend/my-app/src/components/features/trace/DataOverviewPage.tsx` | 挂载 SegmentPanel |
| `dashboard/frontend/my-app/src/types/trace.types.ts` | 新增 Segment 类型 |
| `dashboard/frontend/my-app/src/api/endpoints/traceApi.ts` / `queries/traceQueries.ts` | fetchSegment / useSegment |
| `docs/agents/KNOWN_PITFALLS.md`、`PRINCIPLES.md`、`docs/data-flow/TIME_AND_REPLAY.md`、`docs/backend/API_DESIGN.md`、`docs/reference/API_EXAMPLES.md` | 沉淀与同步 |

## 4. 验证摘要

- backend：`gofmt` / `go vet` / 新增单元测试全过；本机仅已知 Grafana proxy 2 个 502（以 CI 为准）。
- frontend：`npm run typecheck`、`npm run check`（lint + build + verify:state）全过。
- 控制面：`make fmt` / `make vet` / `make test` / `bin/golangci-lint run`（0 issues）。
- 真机（docker-desktop）：部署 backend + frontend 后，`/segment` 实测 available（起点 05:59:56Z → 终点 09:59:55Z）、5 指标 series、50 条 Trace；缺参/start>end 返回 400；2026-08-01 无快照返回 unavailable + 告警。
- 前端页面：CDP 无头 Chrome 打开 trace 页，段面板渲染、起点/终点下拉各 1000+ 选项；交互选中起点/终点并点击"分析时间段"后，起点状态/终点状态/区间指标/区间 Trace/时长全部渲染，无 console error。

## 5. 未验证 / 风险

- Prometheus 数据卷是 emptyDir：本次验证时段内 Prometheus 于 11:41Z 重启过，06:00Z-10:00Z 原始指标已丢失（已知部署坑，见 issue-09 后续方向"按环境拆分可观测性存储"）；段接口如实返回空指标 + 告警，行为正确，但"长跑全时段指标曲线"需在 Prometheus 未重启的窗口下复核。
- `errorRate` 的 `or on() vector(0)` 空集保护会让"区间内无原始数据"显示为常量 0 系列（121 个 0 点）——这是既有 PromQL 语义，不是段接口引入；解读时注意。
- 前端时间轴交互（在时间轴上直接标记起点/终点）留到 UI 阶段；当前用下拉选择。
- 段查询最大窗口 24h；更长区间需分多次查询。
