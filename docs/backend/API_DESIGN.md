# API 设计

> 维护层：human | last-reviewed：2026-08-21 | 事实源：dashboard/backend/internal/api/

Base path：`/api/v1`。当前 API 是面向 Dashboard 的内部稳定契约；尚未生成正式 OpenAPI 文档，也没有公开版本兼容承诺。

## 1. 通用响应

成功：

```json
{
  "data": {},
  "meta": {
    "requestId": "...",
    "servedAt": "2026-08-12T14:00:00Z",
    "partial": false,
    "warnings": [],
    "sourceVersions": {}
  }
}
```

失败：

```json
{
  "error": {
    "code": "RESOURCE_VERSION_CONFLICT",
    "message": "resource changed since it was read",
    "retryable": true,
    "details": {}
  },
  "meta": {
    "requestId": "...",
    "servedAt": "2026-08-12T14:00:00Z"
  }
}
```

`partial=true` 不是 HTTP 失败：通常表示 Prometheus/Jaeger 等可选 section 不可用。调用方必须展示 warnings。

## 2. 路由总表

### 健康、能力与引导

| Method | Path | 用途 |
| --- | --- | --- |
| GET | `/health/live` | 进程存活；不代表 cache/DB/provider ready。 |
| GET | `/health/ready` | cache 和 required PostgreSQL 就绪。Prom/Jaeger 非硬门。 |
| GET | `/capabilities` | commands、history、metrics、traces、simulation run 等能力。 |
| GET | `/bootstrap` | 集群、服务、provider、clock 和页面初始元数据。 |
| GET | `/clock` | authoritative server/actual/logical time，以及 Simulator desired/applied rate、同步计数和 capabilities。 |
| PATCH | `/clock/rate` | 设置 `SimulationClock/default.spec.rate`（1..20）；需要 Idempotency-Key、resourceVersion 和审计。 |

### Configuration

| Method | Path | 参数/语义 |
| --- | --- | --- |
| GET | `/configuration` | 可选 `at=RFC3339`；latest cache 或 historical snapshot。 |
| POST | `/configuration:apply` | 批量 create/update，支持 dry-run；需要 Idempotency-Key。 |
| DELETE | `/configuration/{kind}/{name}` | 可选 `dryRun=true`；If-Match/resourceVersion；需要 Idempotency-Key。 |

可写 Kind：Model、WorkerNode、Tenant、TenantModelPolicy、TenantNodePolicy、ModelNodePolicy、Orchestrator。允许字段是服务端 Spec allowlist；metadata.name 需要符合 Kubernetes 名称。

### Traffic

| Method | Path | 参数/语义 |
| --- | --- | --- |
| GET | `/traffic` | `at`、`tenant`；返回请求、分配、实例性能/状态。 |
| PATCH | `/tenants/{name}/traffic` | 修改 Tenant.spec.qps；需要 Idempotency-Key 和并发保护。 |

PATCH 修改的是 Tenant 总请求 QPS。Traffic Controller 再写各 SimulatorInstance 的分配；API 不允许用户直接编辑 `SimulatorInstance.spec.traffic.qps`。

### Metrics

| Method | Path | 参数 |
| --- | --- | --- |
| GET | `/metrics` | 与 `/metrics/query` 等价的查询入口。 |
| GET | `/metrics/query` | `metricId,start,end,step,tenant,model,instance,node`。 |

限制：窗口最多 7 天；step 5s..1h；metricId 必须在服务端 catalog。当前 catalog：

| metricId | 单位 | 意义 |
| --- | --- | --- |
| `simulator.ttft` | ms | Simulator TTFT。 |
| `simulator.queue` | requests | 队列深度。 |
| `simulator.qps` | req/s | 分配 QPS。 |
| `simulator.errorRate` | ratio | Simulator 错误率。 |
| `simulator.tickLatency` | ms, p95 | Tick 延迟。 |
| `simulator.timeScale` | x | 当前 Simulator reporter 实际采用的倍速。 |
| `controller.errorRate` | ratio | Reconcile 错误比例。 |
| `controller.reconcileLatency` | duration | Reconcile 延迟。 |
| `worker.gpuUsed` | units | 业务 GPU 使用量。 |

响应不暴露任意 PromQL，以防止查询注入和高成本查询。

### Traces

| Method | Path | 参数/语义 |
| --- | --- | --- |
| GET | `/traces` | `start,end,service,operation,tenant,model,instance,minDuration,maxDuration,limit`。 |
| GET | `/traces/{traceID}` | 单个 Trace 的规范化 Span tree。 |

搜索窗口最多 24h，limit 1..100。Trace provider 不可用可返回 availability/partial，不能导致 Kubernetes Configuration 数据消失。

### Events、Replay 与 Overview

| Method | Path | 参数/语义 |
| --- | --- | --- |
| GET | `/events` | `limit`；来自 cache 的 Kubernetes Event/read model。 |
| GET | `/replay` | `limit`；数据库 snapshot timeline。 |
| GET | `/replay/frame` | `at` 和 filters；与历史 Overview 兼容入口。 |
| GET | `/overview` | `at` 和 tenant/model/instance/node 等过滤；聚合资源、指标、Trace。 |
| GET | `/segment` | `start,end` 必填（RFC3339）、窗口 ≤ 24h、tenant/model/instance/node 过滤；返回起点/终点快照 + 区间指标与 Trace。 |
| POST | `/experiments` | 创建实验切面（pending）：`tenant,name` 必填、≤63 字符、无控制字符；写入配置快照。 |
| POST | `/experiments/{id}/start` | 开始实验：pending→running，写入起点全局快照，混合采样器开始跟踪。 |
| POST | `/experiments/{id}/complete` | 结束实验：running→completed，写入终点快照、摘要并关联窗口内 Trace。 |
| POST | `/experiments/{id}/fail` | 标记失败：running→failed，body `reason`，同样写入终点快照并关联 Trace。 |
| GET | `/experiments` | 实验列表；`status` 过滤（pending/running/completed/failed）、`limit` 1..200。 |
| GET | `/experiments/{id}` | 实验详情：segments 一行 + 事件/指标分桶/关联 Trace。 |

`replay` 名称表示历史浏览，不代表事件重新执行。无 `at` 时读 live；旧 `at` 仅使用最后一个不晚于该时间的 snapshot。`/segment` 是时间段切面（起点/终点快照 + 区间数据），与点查询互补；任一端无快照返回 `unavailable` + 告警，不伪造数据。

`/experiments` 是切面的生命周期入口（issue #51）：实验创建后为 `pending`（配置快照已定格），开始后进入 `running` 由后台混合采样器持续写入 `segment_events` / `segment_metrics` / `trace_index.segment_id`，完成后封存为不可变归档。写接口走既有写认证与幂等链路；详情接口在存储不可用时返回 503，不降级为假数据。


### AIOps（M0+M1，#93）

| Method | Path | 参数/语义 |
| --- | --- | --- |
| GET | `/aiops/analyses` | 分析列表；`status=pending|running|aggregating|completed|failed`、`limit` 1..200。 |
| GET | `/aiops/analyses/{id}` | 单条分析（主记录 + L1 实体总结）；`?segmentId=` 按切面查询。 |

`AIOPS_ENABLED=false`（默认）或面板运行时开关关闭时返回 404 `AI_OPS_DISABLED`（例外：`/aiops/settings` 读写始终可用，保证面板能重新打开开关）；持久化存储不可用返回 503。分析由实验 complete/fail 自动入队（开关关闭时不入队），状态机与进度见 Backend 架构第 13 节。

**M2 意图执行（#94）与 M3 时间聚合（#95）：**

| Method | Path | 参数/语义 |
| --- | --- | --- |
| POST | `/aiops/commands` | 一句话意图：LLM 解析 + 模板目录校验，落库 `parsed`；`{"rawInput":"..."}`；`traffic` 支持 steady/tidal/spike/ramp（波形 peakQps 上限 200、倍速上限 100）。 |
| GET | `/aiops/commands/{id}` | 意图命令详情（解析结果 + 执行 steps）。 |
| POST | `/aiops/commands/{id}/confirm` | 确认执行：gate 校验（节点/租户存在）→ 写流量/调倍速 → 创建并启动实验 → `done`/`failed`。 |
| GET | `/aiops/templates` | 只读模板目录（model/node/tenant 各 10 条预置 + orchestrator/traffic，LLM 只能选目录内 id）；model/node/tenant 的 id 与集群 CR 同名，由 `hack/aiops-templates-seed.sh` 预置。 |
| GET | `/aiops/limits` | 意图执行硬限制（峰值 QPS/倍速/波形/潮汐周期），解析校验与前端提示共用。 |
| GET | `/aiops/windows` | 窗口/日总结；`level=L3|L4`、`limit` 1..200。 |
| GET | `/aiops/alerts` | 警戒列表（分数序列规则触发）；`limit` 1..200。 |
| POST | `/aiops/chat` | 同步对话（SSE 流）：`{"message":"...","sessionId":"..."}`；事件 lifecycle/tool/text；限流 6 次/分钟/会话；回答成功后问答对与引用的 window/alert/command ID 落 `aiops_chat_messages`（失败不影响响应）；日配额超限（24h 滚动，默认 300 次/200 万 token，`AIOPS_DAILY_MAX_CALLS`/`AIOPS_DAILY_MAX_TOKENS`）返回 429 `DAILY_QUOTA_EXCEEDED`。 |
| GET | `/aiops/chat/messages` | 某会话问答历史（#112 阶段 D 读侧）：`sessionId` 必填、`limit` 1..200（默认 50），按时间正序；前端打开面板时拉取，失败静默降级。 |
| GET | `/aiops/jobs` | 异步任务列表（`status=pending\|running\|done\|failed`、`limit` 1..200）。 |
| GET | `/aiops/settings` | LLM 配置掩码状态（模型/地址/key 是否已配置 + 运行时开关 `enabled`，不回显明文）。 |
| POST | `/aiops/settings` | 面板写入 LLM 配置：`{"apiKey"?,"model"?,"baseUrl"?,"enabled"?}` 至少一项；apiKey ≥8 字符，enabled 为运行时开关（仅服务端内存，重启恢复部署级启用态）。 |

意图权限边界：AI 只能 create/start/complete/fail 实验、写流量、调倍速、选目录内模板/既有节点；不可改模板/节点/其他 CR。执行只走既有写通道（gateway/store/aggregator）。

### Stream

| Method | Path | 语义 |
| --- | --- | --- |
| GET | `/stream` | SSE `resource.changed` 与 `resync-required`。 |

每个连接的缓冲有限；服务端不保存 durable SSE log。`Last-Event-ID` 不能恢复每一条事件，客户端必须 REST resync。

## 3. Mutation 约定

### Idempotency-Key

- 所有 POST/PATCH/DELETE 命令必须带非空 `Idempotency-Key`。
- 相同 key + 相同命令返回缓存响应，并可带 `X-Idempotent-Replay`。
- 相同 key + 不同 payload 必须拒绝，防止误重用。
- 默认记录保留约 24 小时。
- 命令完成记录写失败时立即释放占位，避免同一 key 被 pending 卡满保留期；重放依赖 Kubernetes apply 幂等语义。

### 批量应用

- 顺序应用一批资源；中途失败时返回 `state=partial` 的成功 envelope（`meta.partial=true`），
  `results` 同时包含已成功项与失败项明细（`convergence=failed` + `error`），不再让客户端误以为整个批次失败。
- dry-run 阶段失败仍整体拒绝，因为此时没有任何资源被写入。

### 乐观并发

- update 使用对象 resourceVersion。
- delete 使用 If-Match 或请求中的版本。
- 冲突不自动覆盖；返回可重试问题，前端 refetch 并让用户重新确认。

### Dry-run

Configuration apply 先对所有资源执行 API Server dry-run，捕获 CRD/CEL/RBAC/引用类错误。之后才顺序真正写入。dry-run 无法保证写入阶段所有对象仍保持不变，因此不等于事务锁。

### 审计

命令记录 kind/name/action、请求、结果、时间和 request ID。写请求经过应用层认证：配置 `ADMIN_TOKEN` 后必须携带 Bearer Token；审计主体取自认证身份（匿名写时记录 `system:anonymous`）。`X-Remote-User` 只在请求通过认证且显式开启 `TRUST_REMOTE_USER_HEADER` 时才被信任，防止伪造上游身份头。生产环境必须配置 token，未配置时写接口返回 503。审计持久化使用独立于请求生命周期的超时上下文，客户端断开不丢审计。

## 4. 字段权限

| 资源 | Read | Create/Update/Delete | Status |
| --- | --- | --- | --- |
| 7 个配置 CR | 是 | 是，Spec allowlist | 否 |
| SimulationClock/default | 是 | 专用 Clock API 仅 create/update rate；不可 delete | 否 |
| SimulatorInstance | 是 | 否 | 否 |
| TenantPerformance | 是 | 否 | 否 |
| TenantRuntime | 是 | 否 | 否 |
| Pod/Node/Service/Event | 是 | 否 | 否 |
| Deployment/ReplicaSet | 是 | 否 | 否 |
| Lease | 是 | 否 | 否 |

Backend ServiceAccount RBAC 与应用 allowlist 是双重边界。两者都要收紧；只靠 UI 隐藏不是授权。

## 5. 参数和安全

- JSON body 上限 1MiB，未知 JSON 字段拒绝。
- CORS 是显式 origin allowlist，开发默认允许 `localhost:5173`。
- 时间必须为 RFC3339；窗口、step、limit 有服务端边界。
- traceID、kind/name、metricId 等需严格校验，不能直接拼 URL/PromQL。
- HTTP 日志不应记录数据库密码、Authorization、Secret data。
- Security headers、panic recovery 和 request timeout 由 middleware 统一处理。

## 6. 错误分类建议

| 类别 | 示例 | retryable |
| --- | --- | --- |
| Validation | 未知字段、非法 QPS、无效时间窗 | false |
| Not Found | 资源或 Trace 不存在 | false/视重建语义 |
| Conflict | resourceVersion、幂等 key payload 冲突 | true 或需用户确认 |
| Dependency Unavailable | cache、required DB、Prom/Jaeger | 通常 true |
| Historical Unavailable | 请求时间前无 snapshot | false，除非等待采集 |
| Forbidden | Kind/字段不在 allowlist、RBAC | false |
| Internal | 未分类错误 | 视 details，默认 false |

不要把所有 provider 错误都返回 500；Overview 可用 partial 时应返回成功 envelope + warnings。

## 7. 版本演进

未来应从 Go DTO/路由生成 OpenAPI，并建立：

- additive 字段兼容策略；删除/重命名前先 deprecate。
- endpoint/API version 与 CRD version 分离。
- enum unknown handling。
- sourceVersions schema。
- contract tests：Backend response 与 Frontend TypeScript schema 同步。
