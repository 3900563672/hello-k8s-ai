# 切面为第一公民：生命周期 + 混合采样 + 分层存储（#51）

> 日期：2026-08-18

## 为什么做

- 历史切面此前是"临时拼装"：`GET /segment` 按 start/end 现场查快照 + Prometheus + Jaeger，切面本身没有身份、没有生命周期、没有持久归档，AI OPS 无法把"一次调度"当独立实体查询。
- 事件风暴风险：0→几百 Pod 时若把 Pod 个体事件全部入库会爆炸（实测样例一次 `SuccessfulCreate` 聚合 count=232）。
- 采样策略缺失：固定频率采样要么丢细节（平静期浪费），要么被事件淹没（高频期）。用户明确要求：时间驱动 + 事件驱动混合，且参数进配置不硬编码。

## 改成什么

1. **数据模型（迁移 004）**：`segments`（生命周期 pending→running→completed/failed，配置/起点/终点快照 + 摘要）、`segment_events`（六类：decision/alert/error/gap/burst/phase_change）、`segment_metrics`（1min 桶 min/avg/max/p95，幂等 upsert）、`trace_index.segment_id`（幂等加列 + 部分索引）。全部 `IF NOT EXISTS`，可重复应用。
2. **生命周期 API**（`internal/api/handlers_experiment.go`）：`POST /experiments`（pending + 配置快照定格）→ `POST /experiments/{id}/start`（running + 起点快照）→ `POST /experiments/{id}/complete|fail`（终点快照 + 摘要 + Jaeger Trace 关联）；`GET /experiments`（status 过滤）与 `GET /experiments/{id}`（详情）。写接口走既有幂等与写认证。
3. **混合采样器**（`internal/segment.Sampler`）：独立 goroutine 轮询 running 切面；基线 30s / 高保真 5s / 平静退出 60s；事件分类（Orchestrator `lastScaling` 扩缩决策按指纹去重、SimulatorInstance spec 变化、TimelineGap→gap、副本曲线→phase_change、变化量≥阈值→burst、错误率均值超阈值→error、TTFT p95 超阈值→alert）；重叠查询按秒去重防重复计入；关键事件进入高保真窗口。后端重启自动恢复对残留 running 切面的采样（自愈），终态自动冲刷内存分桶。
4. **配置**：`SEGMENT_BASELINE_INTERVAL`（30s）/ `SEGMENT_BURST_INTERVAL`（5s）/ `SEGMENT_QUIESCENCE_WINDOW`（60s）/ `SEGMENT_BURST_REPLICA_DELTA`（5）/ `SEGMENT_ERROR_RATE_THRESHOLD`（0.05）/ `SEGMENT_TTFT_THRESHOLD_MS`（2000），均带校验。
5. **前端**：DataOverview 页新增 ExperimentPanel——创建/开始/完成/失败实验、10s 轮询列表、详情展示事件序列/指标分桶/关联 Trace，样式沿用既有深色面板风格。

## 关键行为

- 切面=一次调度实验的不可变归档：起点与终点各有完整全局快照，中间是事件序列与 1min 指标分桶，因此"没有数据"与"数据过期丢失"可区分。
- Pod 个体事件不进切面，只记录群体演化（副本曲线、事件计数、指标聚合），防事件风暴。
- 采样器只处理 `running` 状态；生命周期只能由 API 推进，采样器不写 status。
- 实验名/租户校验：必填、≤63 字符、无控制字符（纯 DB 记录，不映射 K8s 资源名）。

## 验证

- `go build ./...` / `go vet ./...` / `go test ./...` 全绿（新增 `internal/segment` 单测 5 个、`handlers_experiment_test.go` 5 个；含 fake informer cache 的真实链路测试）。
- `make lint`：0 issues。
- 前端 `npm run check`（eslint + tsc + vite build + SSR state-check）通过。
- `make docs-check` 通过；`make docs-sync` 已重新生成派生文档。
- 本机环境遗留（与本次改动无关）：WSL 回环 TCP 当前整体不可用（同进程 listen→dial 被拒），`TestGrafanaProxyPreservesSubPathAndForwards` / `TestGrafanaProxyRootPath` 两个纯环境性测试本地失败；CI（GitHub Actions）无此问题。原因待查（疑似 Docker/WSL 磁盘迁移后网络栈残留），不影响本次交付。
- 未验证：真实集群上的长时间实验归档（≥2h）与 AI OPS 查询切面全貌，属后续长跑轮次。

## 回滚

- 代码：`git revert` 对应 commit。迁移 004 全部幂等；如需删表：`DROP TABLE segment_metrics, segment_events, segments; ALTER TABLE trace_index DROP COLUMN segment_id;`（删除前确认无正在运行的实验）。
- 前端：移除 ExperimentPanel 引用与 `experiment*` API/query 文件即可；后端不部署时写接口返回 404，读接口不受影响。