# 生产就绪度

## 1. 结论

当前项目是功能完整度较高的开发/演示控制平面，但**不是生产就绪部署**。核心领域架构可以演进，基础设施和运营保障仍需专门阶段。

## 2. 就绪矩阵

| 领域 | 当前 | 生产门 | 状态 |
| --- | --- | --- | --- |
| 功能控制环 | 7 Controllers + Simulator | 完整 E2E、故障恢复、规模测试 | 部分 |
| API 兼容 | 内部 `/api/v1` | OpenAPI、版本策略、contract test | 未完成 |
| AuthN/AuthZ | ServiceAccount | OIDC、用户/租户授权、审计 actor | 未完成 |
| Secrets/TLS | 本地随机 Secret/集群内明文 | external secrets、rotation、TLS/mTLS | 未完成 |
| PostgreSQL | 单实例 PVC | HA、PITR、备份恢复、监控 | 未完成 |
| Metrics/Trace | 单副本 PVC（Prom 168h / Jaeger badger 168h） | 持久化/远端存储、HA、retention | 部分 |
| Alerting | Prom rules | Alertmanager、routing、on-call/runbook | 未完成 |
| 网络 | 基础 | default deny、最小 allowlist、Ingress TLS | 未完成 |
| 可用性 | 多数 app 单副本 | replicas、PDB、anti-affinity、capacity | 未完成 |
| 供应链 | dev images/tags | digest、registry、SBOM、scan、sign | 未完成 |
| DR | 无 | RPO/RTO、演练 | 未完成 |
| 可重复仿真 | 随机/墙钟 | seed/clock/checkpoint/version | 未完成 |

## 3. 数据持久化

### PostgreSQL

迁移到托管 HA 或成熟 Operator；启用 TLS、Secret rotation、WAL/PITR；设监控和容量；做恢复演练。明确 snapshot/audit/idempotency retention 和隐私删除。

### Prometheus

本地已 PVC + 168h（可支撑历史复盘，但单副本无 HA）。生产仍用持久 PVC + HA/remote_write/Thanos/Mimir 等组织标准方案；避免双副本重复告警；配置 Alertmanager。

### Jaeger

本地已 badger + PVC（单副本、TTL 168h）；生产配置受支持的持久 storage、retention、query/ingest HA；验证 Jaeger v2 API 兼容。Trace 采样和敏感属性政策先于扩容。

### Grafana

禁用匿名 Viewer，接入 SSO/RBAC，持久化/声明式 dashboard，保护 datasource credentials。

## 4. 高可用

- Controller Manager 可多副本 + leader election；验证 Lease 切换和 pending plan 恢复。
- Simulator Deployment 多副本已有 per-instance leader，但 leader 切换会丢引擎状态；评估业务影响。
- Backend 可多副本，但每副本都运行全量 informer/可能 snapshot/recorder；需要明确单 writer/leader 或幂等策略，避免重复 snapshot/event。
- Frontend 可无状态多副本。
- PostgreSQL/Prom/Jaeger/Collector/Grafana 需各自 HA 设计。
- 加 PDB、topology spread、anti-affinity 和容量预留。

## 5. 性能与规模

必须基准：

- 10k+ CR/Pods 时 informer memory、initial sync、event rate。
- Controller queue/reconcile latency、API QPS/Burst。
- Snapshot JSON 大小、30s 写入、prune/vacuum、历史查询。
- Overview fan-out 对 Prometheus/Jaeger 的并发和高基数。
- SSE 客户端数、慢连接、代理超时。
- Simulator 高 QPS virtual queue、Tick duration、Status write contention。

达到阈值后再决定 informer namespace/shard、多 Backend read replica、snapshot leader、缓存和 provider query budget。

## 6. 可靠性测试

| 故障 | 必测结果 |
| --- | --- |
| Manager kill | 新 leader接管，无重复/丢失关键副作用，pending plan 恢复 |
| Simulator leader kill | Lease 切换，单 reporter，无双写；承认 engine reset |
| API Server 断连 | client重连，Reconcile重试，不删除未知资源 |
| PostgreSQL down | read/ready/commands按配置退化，恢复后 recorder/snapshot 可继续 |
| Prometheus/Jaeger down | Overview partial，配置/控制面正常 |
| Backend restart | idempotent command 可重放，SSE 客户端 resync |
| Node drain | Pods 重调度，Policy/affinity仍遵守，容量重算 |
| Storage full | 告警，DB/Prom/Trace 不静默损坏 |

## 7. SLO 建议

先定义产品目标，再配置告警：

- Backend read API availability/latency；mutation accepted/error/conflict。
- informer cache freshness/initial sync。
- Controller reconcile error rate/queue latency。
- Simulator observedAt age、每 Instance leader count=1、Tick/status write success。
- Traffic total conservation、TenantPerformance freshness。
- snapshot success/age、Recorder drop=0。
- Prom targets/Collector export/Jaeger query health。

不要仅以 Pod Ready 作为业务 SLO。

## 8. 发布流程

1. 版本化 Controller/Simulator/Backend/Frontend 镜像，使用 immutable digest。
2. 生成 SBOM、扫描、签名。
3. API/CRD migration compatibility test。
4. 在临时 Kind/预发布环境执行全栈 E2E。
5. Kustomize/Helm 渲染和策略验证。
6. 灰度/滚动部署；观察 SLO。
7. 明确 rollback：新 CRD storage version/DB migration 可能不可直接回滚。
8. 发布后采集 Cluster Information 快照和验证证据。

## 9. 生产前不可妥协项

- 将本地随机 Secret 迁移到组织 Secret 管理，并禁用明文数据库连接。
- 完成认证授权和 tenant isolation。
- 数据持久化、备份和恢复演练。
- 在目标机保存完整一键验收证据，并在 CI 中建立等价全栈 E2E。
- Alertmanager/on-call/runbook。
- NetworkPolicy/TLS/Secret 管理。
- 修正 Traffic/UI 误导语义。
- 为 `absoluteScore` 校准值建立版本与审计策略；当前只有 Simulator 引擎倍速，完整逻辑时间、pause/Seek 和确定性运行仍未实现。
