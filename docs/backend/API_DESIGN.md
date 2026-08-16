# API 设计

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

`replay` 名称表示历史浏览，不代表事件重新执行。无 `at` 时读 live；旧 `at` 仅使用最后一个不晚于该时间的 snapshot。

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

### 乐观并发

- update 使用对象 resourceVersion。
- delete 使用 If-Match 或请求中的版本。
- 冲突不自动覆盖；返回可重试问题，前端 refetch 并让用户重新确认。

### Dry-run

Configuration apply 先对所有资源执行 API Server dry-run，捕获 CRD/CEL/RBAC/引用类错误。之后才顺序真正写入。dry-run 无法保证写入阶段所有对象仍保持不变，因此不等于事务锁。

### 审计

命令记录 kind/name/action、请求、结果、时间和 request ID。写请求经过应用层认证：配置 `ADMIN_TOKEN` 后必须携带 Bearer Token；审计主体取自认证身份（匿名写时记录 `system:anonymous`）。`X-Remote-User` 只在请求通过认证且显式开启 `TRUST_REMOTE_USER_HEADER` 时才被信任，防止伪造上游身份头。生产环境必须配置 token，未配置时写接口返回 503。

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
