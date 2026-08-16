# Phase A 提示词模板：夜间维持运行与采集

> 用途：Codex 桌面自动化在 00:00 触发时使用本模板生成任务提示词。
> 槽位：窗口 / 目标 / 红线 / 输出。填好四槽位即可复用。

## 窗口

- 运行日期：2026-08-17（Asia/Shanghai）。
- 执行窗口：00:00 – 04:30。**非 2026-08-17 触发时直接结束**，不执行任何动作。
- 04:30 前必须完成交接档案并停止，不跨窗口作业。

## 目标

1. 维持系统持续运行：模拟器、Controller、Dashboard、PostgreSQL、Grafana、Prometheus、Jaeger、OTel Collector 全部健康。
2. 持续制造并保持流量：至少 1 个租户有稳定 qps（可在 5–60 qps 间按配置档位轮换），观察高流量下的行为。
3. 每 15 分钟执行一次 `node hack/night-run/keepalive.mjs --once`；失败时按脚本提示恢复（重开端口转发等），并在问题档案记录。
4. 每 30 分钟执行一次 `node hack/night-run/snapshot.mjs --once --summary`，把摘要与异常追加到问题档案。
5. 主动施压与变更探测：至少 3 次修改配置（traffic 档位、replica 数量、倍速），每次变更记录：时间、期望、实际、差异。变更使用 Backend API（`PATCH /api/v1/tenants/{name}/traffic`、`PATCH /api/v1/clock/rate`、`POST /api/v1/configuration:apply`），带 `Idempotency-Key` 头。
6. 任何异常（Pod 崩溃、错误率上升、队列积压、API 失败、DB 异常、Grafana No data）都记录到问题档案，附时间戳、日志片段、复现上下文。

## 红线

- **Phase A 不推任何代码、不建 issue、不改 UI、不截图验证。**
- 不 `wsl --shutdown`；不强杀 Docker Desktop；不动代理（127.0.0.1:7890）；不重建/重置集群。
- 不删 PVC、不重置数据库、不修改 CRD 定义。
- Pod 卡 Init 且节点 IP 重复时，删该 Pod 让其重调度（已知坑）。
- 一切记录带 UTC 时间戳；原始问题档案只写 `.runtime/night-run/2026-08-17/problems.md`，**不提交**。
- Token 预算优先给"维持运行 + 发现问题"，不要纠结于研究性分析。

## 输出

- `.runtime/night-run/2026-08-17/problems.md`：按模板（见 `hack/night-run/problems.template.md`）记录所有问题与上下文。
- 结束时（04:30 前后）在 problems.md 顶部写"交接摘要"：运行总时长、流量档位演变、健康状态结论、问题清单（按优先级排序）、给 Phase B 的建议。
- 最终回复只汇报：运行了多久、发现几个问题、最严重的 3 个问题一句话描述、交接档案路径。