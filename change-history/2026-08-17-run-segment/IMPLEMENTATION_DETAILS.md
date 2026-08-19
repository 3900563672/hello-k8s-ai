# 实现细节

## 1. 数据模型（dashboard/backend/internal/model/types.go）

- `SegmentSnapshot`：`SnapshotID` / `CapturedAt` / `Configuration` / `Traffic` / `Workloads`（与 `CurrentSnapshot` 同构，补快照 ID）。
- `SegmentOverview`：`Availability` / `Start` / `End` / `StartSnapshot` / `EndSnapshot` / `Metrics` / `Traces` / `Freshness`。段查询与点查询（`Overview`）互补，均为只读聚合，不写任何状态。

## 2. 段查询 handler（handlers_segment.go）

- 路由：`GET /api/v1/segment`（server.go 注册），与 `/overview` 平级。
- 参数：`start` / `end` 必填（RFC3339Nano，归一化 UTC）；`start < end`；窗口 ≤ `maxSegmentWindow`（24h）。非法参数返回 400 `INVALID_SEGMENT_WINDOW`。
- 快照：`server.segmentSnapshot()` 复用 `store.SnapshotAt`（"at 之前最近"），对存储不可用 / 无快照 / 解码失败分别返回中文告警；两端都有快照才置 `availability=available`，否则返回 unavailable + 告警（不伪造，遵守 PRINCIPLES 第 6 条）。
- 指标：与 overview 同一 catalog 的 5 项（ttft / queue / qps / errorRate / tickLatency），`QueryRange(Start: start, End: end)`，支持 tenant/model/instance/node 过滤；并发查询 + mutex 聚合，错误进 warnings（与 handleOverview 同构）。
- Trace：`jaeger.Search({Start: start, End: end, Limit: 50})`；查询成功后 `indexTraces` 顺带写 trace_index（与 overview 一致）。
- 覆盖告警：`segmentCoverageWarnings(start, end, now)`——Prometheus 保留窗口看起点；Jaeger 配置保留窗口看终点、内存模式看终点是否超过 15 分钟回看窗口。与 `historyCoverageWarnings`（点模式）并存。
- 响应经 `writeData` 统一信封（partial = 有 warnings），附 sourceVersions。

## 3. 前端段面板（SegmentPanel.tsx）

- 位置：trace 页（DataOverviewPage）header 之下、Overview 内容之上；独立组件文件。
- 选择：起点/终点两个 `<select>`，选项来自 timeStore 的 snapshots（时间轴项，按时间升序；终点只列起点之后的项），保证取点必然有快照。
- 查询：`useSegment(SegmentQuery | null)`，queryKey 含 start/end/过滤项，`enabled: query !== null`，`staleTime: Infinity`（历史数据不变）。
- 展示：
  - 段信息头（起点/终点时间、时长、快照 ID）；
  - 起点/终点 8 项状态对比（Tenant/Model/WorkerNode/Simulator 数量、副本总数、QPS、Pod Ready、Deployment Ready），数值变化高亮；
  - 5 项区间指标 ECharts 曲线（复用 overview 同款暗色样式）；
  - 区间 Trace 列表（最多 50 条）+ 覆盖告警条；
  - 空时间轴 / 未选择 / 请求中 / 失败四态均有占位。
- 与页面风格一致（暗色 `#05070A` 系、`border-white/[0.07]`、`text-[8px~10px]` 密度），复用 lucide 图标与 `Intl` 格式化。

## 4. 行为边界

- 不改变 `/overview` 点查询语义；`/replay` / `/replay/frame` 不动。
- 不引入数据库表；段只是对既有快照流、Prometheus 区间与 Jaeger 区间的只读组合。
- 段查询不做"按事件重新执行"或"确定性回放"，与 AGENTS.md 中 SimulationClock 边界一致。
