# API 示例

> 维护层：human | last-reviewed：2026-08-21 | 事实源：dashboard/backend/internal/api/

示例假设 Backend 在 `http://localhost:8080`。生产环境必须通过认证/TLS；不要复制开发凭据。

```bash
API=http://localhost:8080/api/v1
```

## 1. 健康与引导

```bash
curl -sS "$API/health/live"
curl -sS "$API/health/ready"
curl -sS "$API/capabilities"
curl -sS "$API/bootstrap"
```

`live=200` 不代表 cache/DB ready；以 ready checks 和 capabilities 判断页面/命令可用性。

## 2. 读取配置

```bash
curl -sS "$API/configuration"
curl -sS "$API/configuration?at=2026-08-12T14:00:00Z"
```

历史无 snapshot 可能仍返回 200 envelope，但 `partial=true`、availability/warning 指出不可用。

## 3. Dry-run 创建 Tenant

```bash
curl -sS -X POST "$API/configuration:apply" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: tenant-demo-dryrun-001' \
  --data-binary @- <<'JSON'
{
  "dryRun": true,
  "resources": [
    {
      "kind": "Tenant",
      "name": "tenant-demo",
      "spec": {
        "displayName": "Demo Tenant",
        "priority": "P3",
        "qps": 10,
        "ttftThresholdMs": 500,
        "queueThreshold": 100,
        "ttftScaleDownThresholdMs": 200,
        "queueScaleDownThreshold": 30
      }
    }
  ]
}
JSON
```

将 `dryRun` 改为 false 并使用新的 Idempotency-Key 才会持久化。不要把 dry-run key 复用于不同 payload。

## 4. 创建完整基础配置批次

```bash
curl -sS -X POST "$API/configuration:apply" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-config-001' \
  --data-binary @- <<'JSON'
{
  "dryRun": false,
  "resources": [
    {
      "kind": "Model",
      "name": "model-demo",
      "spec": {
        "displayName": "Demo Model",
        "gpuUnits": 800,
        "maxConcurrency": 16,
        "absoluteScore": 100,
        "coldStartMs": 30000,
        "performance": {
          "prefillBaseMs": 50,
          "prefillPerTokenUs": 500,
          "decodePerTokenMs": 20
        }
      }
    },
    {
      "kind": "Tenant",
      "name": "tenant-demo",
      "spec": {
        "displayName": "Demo Tenant",
        "priority": "P3",
        "qps": 10,
        "ttftThresholdMs": 500,
        "queueThreshold": 100,
        "ttftScaleDownThresholdMs": 200,
        "queueScaleDownThreshold": 30
      }
    },
    {
      "kind": "TenantModelPolicy",
      "name": "tenant-demo-model-demo",
      "spec": {
        "tenantRef": {"name": "tenant-demo"},
        "modelRef": {"name": "model-demo"},
        "effect": "Allow"
      }
    },
    {
      "kind": "Orchestrator",
      "name": "tenant-demo",
      "spec": {
        "tenantRef": {"name": "tenant-demo"},
        "scaleUpCooldownSeconds": 60,
        "scaleDownCooldownSeconds": 120,
        "allowScaleToZero": true,
        "minReplicas": 1,
        "maxReplicas": 0
      }
    }
  ]
}
JSON
```

这只是示例的一部分：要调度 Pod，还需要与真实 Node 同名的 WorkerNode 和 Node Policies。`Model.spec.absoluteScore` 是必填正整数，可由 Dashboard 或该 API 维护。批次先全量 dry-run、再顺序写，仍非跨对象原子事务。

## 5. 更新资源

先从 Configuration 响应取得 `metadata.resourceVersion`，然后提交完整允许 Spec：

```json
{
  "kind": "Tenant",
  "name": "tenant-demo",
  "resourceVersion": "12345",
  "spec": {
    "displayName": "Demo Tenant",
    "priority": "P2",
    "qps": 20,
    "ttftThresholdMs": 500,
    "queueThreshold": 100,
    "ttftScaleDownThresholdMs": 200,
    "queueScaleDownThreshold": 30
  }
}
```

Gateway 用提交的 `spec` 替换整个 Spec，不是字段级 merge；调用方必须保留所有需要字段。

## 6. 修改 Tenant QPS

```bash
curl -sS -X PATCH "$API/tenants/tenant-demo/traffic" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: tenant-demo-qps-20-001' \
  --data-binary '{"qps":20,"resourceVersion":"12345","dryRun":false}'
```

该命令修改 Tenant.spec.qps；实例分配要等待 Traffic Controller。receipt `convergence=pending` 是正常。

## 7. 修改 Simulator 时间倍速

先读取 Clock，保存响应中的 `resourceVersion`：

```bash
curl -sS "$API/clock"

curl -sS -X PATCH "$API/clock/rate" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: simulator-rate-10-001' \
  --data-binary '{"rate":10,"resourceVersion":"12345","dryRun":false}'
```

rate 必须是 1..20。Clock 尚不存在时省略 resourceVersion，Backend 会创建 `SimulationClock/default`；已有对象必须提交当前版本，冲突时先 refetch。命令 accepted 后继续读取 `/clock`，直到 `converged=true`、desired/applied rate 相同且同步计数一致。

该命令只加速 Simulator 离散事件引擎，不改变 Backend 时间、Controller 冷却、Lease、Prometheus 抓取或历史游标。

## 8. 删除配置

```bash
curl -sS -X DELETE \
  "$API/configuration/Tenant/tenant-demo?dryRun=true" \
  -H 'Idempotency-Key: tenant-demo-delete-check-001' \
  -H 'If-Match: "12345"'
```

确认 dry-run 后使用新 key 和 `dryRun=false`。删除使用 background propagation；派生资源还需 Controller/finalizer 收敛。

## 9. Traffic 与 Overview

```bash
curl -sS "$API/traffic"
curl -sS "$API/traffic?tenant=tenant-demo"
curl -sS "$API/overview"
curl -sS "$API/overview?at=2026-08-12T14:00:00Z&tenant=tenant-demo"
# 时间段切面：起点/终点快照 + [start,end] 区间指标与 Trace（窗口 ≤ 24h）
curl -sS "$API/segment?start=2026-08-12T13:00:00Z&end=2026-08-12T14:00:00Z"

# 实验切面生命周期（issue #51）：创建 → 开始 → 运行中由采样器沉淀 → 完成
curl -sS -X POST "$API/experiments" \
  -H "Content-Type: application/json" -H "Idempotency-Key: experiment-1" \
  -d '{"tenant":"tenant-demo","name":"扩容验证-20x"}'
curl -sS -X POST "$API/experiments/<segmentId>/start" -H "Idempotency-Key: experiment-1-start"
curl -sS "$API/experiments?status=running"
curl -sS -X POST "$API/experiments/<segmentId>/complete" -H "Idempotency-Key: experiment-1-complete"
curl -sS "$API/experiments/<segmentId>"
# 失败路径带原因
curl -sS -X POST "$API/experiments/<segmentId>/fail" \
  -H "Content-Type: application/json" -H "Idempotency-Key: experiment-1-fail" \
  -d '{"reason":"指标异常"}'
```

## 10. Metrics

```bash
curl -sS --get "$API/metrics/query" \
  --data-urlencode 'metricId=simulator.ttft' \
  --data-urlencode 'start=2026-08-12T13:45:00Z' \
  --data-urlencode 'end=2026-08-12T14:00:00Z' \
  --data-urlencode 'step=10s' \
  --data-urlencode 'tenant=tenant-demo'
```

不能提交 PromQL；使用 catalog 中 metricId。

## 11. Traces

```bash
curl -sS --get "$API/traces" \
  --data-urlencode 'start=2026-08-12T13:45:00Z' \
  --data-urlencode 'end=2026-08-12T14:00:00Z' \
  --data-urlencode 'service=hello-k8s-ai-simulator' \
  --data-urlencode 'operation=simulator.tick' \
  --data-urlencode 'tenant=tenant-demo' \
  --data-urlencode 'limit=20'

curl -sS "$API/traces/<trace-id>"
```

## 12. SSE

```bash
curl -N -H 'Accept: text/event-stream' "$API/stream"
```

收到事件后调用 REST refetch。不要把 SSE 当历史日志，也不要依赖 Last-Event-ID 精确重放。


## 12.1 AIOps 分析（M0+M1，#93）

```bash
# 列表（AIOPS_ENABLED=false 时返回 404 AI_OPS_DISABLED）
curl -sS "$API/aiops/analyses?limit=10"
curl -sS "$API/aiops/analyses?status=completed&limit=10"
# 任务队列语义别名（含 attempts/kind）
curl -sS "$API/aiops/jobs?limit=10"
# 详情：主记录 + L1 实体总结；支持按切面查询
curl -sS "$API/aiops/analyses/<analysisId>"
curl -sS "$API/aiops/analyses?segmentId=<segmentId>"
```

分析由实验 complete/fail 自动入队，状态机 pending→running→aggregating→completed/failed；`l1Done/l1Total` 为进度。`scores` 含 goal/stability/efficiency/anomaly/overall/verdict/reason。

**M2 意图执行（#94）：**

```bash
# 模板目录（只读；LLM 只能选目录内 id）
curl -sS "$API/aiops/templates"
# 一句话 → 解析（落库 parsed，返回 commandId）
curl -sS -X POST "$API/aiops/commands" -H 'Content-Type: application/json'   -d '{"rawInput":"美国时间 9 点开始，持续 2 小时，突发流量高峰"}'
# 确认执行：gate 校验 → 写流量/调倍速 → 创建并启动实验 → done
curl -sS -X POST "$API/aiops/commands/<commandId>/confirm"
# 查询执行结果（含 steps）
curl -sS "$API/aiops/commands/<commandId>"
```

**M3 时间聚合与警戒（#95）：**

```bash
curl -sS "$API/aiops/windows?level=L3&limit=10"
curl -sS "$API/aiops/windows?level=L4"
curl -sS "$API/aiops/alerts?limit=10"
```

**同步对话（#110 阶段二，SSE 流式）：**

```bash
# 流式回答：事件为 data: {json}（lifecycle/tool/text），curl -N 关闭缓冲；回答成功后服务端落库 aiops_chat_messages
curl -sS -N -X POST "$API/aiops/chat" -H 'Content-Type: application/json' \
  -d '{"message":"当前集群什么情况？","sessionId":"demo-session"}'

# 问答历史（#112 阶段 D 读侧）：sessionId 必填、limit 1..200（默认 50），按时间正序；打开面板时前端用于回填
curl -sS "$API/aiops/chat/messages?sessionId=demo-session&limit=50"
```

**面板配置（#110 阶段四，key 不回显、仅存服务端内存）：**

```bash
# 读取掩码状态（key 只显示是否已配置）
curl -sS "$API/aiops/settings"

# 运行时写入：至少一项；apiKey ≥8 字符，留空保持不变
curl -sS -X POST "$API/aiops/settings" -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4o-mini","apiKey":"sk-..."}'
```

**异步任务（#110 阶段一，状态/重试/失败原因可见）：**

```bash
# 最近任务（status 可过滤 pending/running/done/failed）
curl -sS "$API/aiops/jobs?limit=20"
```

窗口/日总结由定时器自动产出（粒度 `AIOPS_WINDOW_GRANULARITY` 可配）；警戒为分数序列规则触发（连续低分/趋势下滑），alert_id 幂等。

## 13. 错误处理脚本规则

- 同时检查 HTTP status 和 envelope 的 `error.code`/`meta.partial`。
- 503 provider/DB/cache 错误按 retryable/backoff；validation 不盲目重试。
- 409 resourceVersion conflict 必须 refetch，不用新 Idempotency-Key 强行覆盖旧对象。
- 日志保存 `meta.requestId`，但不输出 Authorization、Secret 或 DATABASE_URL。
