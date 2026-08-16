# 夜间长时运行（night-run）

> 维护层：agents ｜ 位置：hack/night-run/ ｜ 运行产物：.runtime/night-run/（不入库）
> 用途：在无人值守时段长时间维持系统运行、持续施压与采集，把问题沉淀成档案，再交给下一阶段分析修复。
> 首次运行：2026-08-17 00:00–09:00（Asia/Shanghai），由 Codex 桌面自动化触发。

## 为什么需要它

潮汐问题（流量峰值、长时运行、内存/DB 累积）只能靠真实长时运行暴露：
短时验证看不到 Reconcile 错误比例漂移、队列积压、PVC 增长与 Grafana 丢数据。
夜间无人值守正好承担这个角色，且不打扰用户。

## 两段式结构

| 阶段 | 窗口（Asia/Shanghai） | 职责 | 是否提交代码 |
| --- | --- | --- | --- |
| Phase A | 00:00–04:30 | 维持运行 + 施压 + 全量采集 | 否 |
| Phase B | 04:30–09:00 | 依据档案分析、决策、修复、提交 | 是（按决策矩阵） |

切换点：Phase B 从 `problems.md` 开始，不知道看什么就先看交接档案。

## 文件说明

- `keepalive.mjs`：健康检查 + 断线自恢复（端口转发、traffic 档位、模拟器 Leader、Pod 状态）。
- `snapshot.mjs`：每 30 分钟抓一次指标快照，追加到 `.runtime/night-run/<日期>/snapshots/`。
- `phase_a_prompt.md`：Phase A 的 Agent 提示词模板（可复用，四槽位：窗口/目标/红线/输出）。
- `phase_b_prompt.md`：Phase B 的 Agent 提示词模板（可复用，先读档案再动手）。
- `problems.template.md`：问题档案模板；Phase A 实例化为 `.runtime/night-run/<日期>/problems.md`。

## 运行约定

- 一切动作带 UTC 时间戳；问题档案原始文件不入库，摘要进 `change-history/`。
- Phase A 不推任何代码；Phase B 按决策矩阵处理（小改直接改、契约变化建 issue）。
- 不改 UI；不截图验证；不 `wsl --shutdown`；不强杀 Docker Desktop；不动代理（127.0.0.1:7890）；不重建集群。
- Pod 卡 Init 且节点 IP 重复时，删 Pod 让其重调度（已知坑，见 docs/agents/KNOWN_PITFALLS.md）。
- API 写操作需要 `Idempotency-Key` 头（≤200 安全字符）；当前部署 `ADMIN_TOKEN` 未配置，非生产环境匿名写可用。

## 手动运行

```bash
# 单次健康检查
node hack/night-run/keepalive.mjs --once

# 常驻循环（每 15 分钟一次）
node hack/night-run/keepalive.mjs --loop --interval 900

# 抓一次快照
node hack/night-run/snapshot.mjs --once

# 抓全部指标并输出摘要
node hack/night-run/snapshot.mjs --once --summary
```

## 自动化入口

Codex 桌面自动化（`$CODEX_HOME/automations/`）在 00:00 与 04:30 各触发一次：
- Phase A：读 `hack/night-run/phase_a_prompt.md` 并严格执行；
- Phase B：读 `hack/night-run/phase_b_prompt.md` 并严格执行。

非运行日（非 2026-08-17）的触发会自动空跑退出，避免浪费 Token。