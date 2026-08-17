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
| Phase B | 04:30–09:00 | 依据档案分析、决策、修复 | 是（全部走 PR，不推 main） |

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
- 实测边界（2026-08-17 首次执行）：rate 有效范围 1–20；租户名为 `tenant-core`；50qps@10 副本触发队列积压（35qps 健康）；副本由 Orchestrator 控制，`kubectl scale` 不可用。

## 白天无人值守 / 定时长跑（day-watch，0 Token）

潮汐/高峰时段不想耗 Token 时，用纯脚本按时间表跑流量并采集，事后一次性分析：

```bash
# 单轮试跑（不动流量：--baseline-qps 35 --peak-qps 35 恒为当前档）
node hack/night-run/day-watch.mjs --once

# 常驻：每小时前 45 分钟 35qps（基线）+ 后 15 分钟 50qps（压测），每 15 分钟一轮
mkdir -p .runtime/day-run/$(date +%F)
setsid nohup node hack/night-run/day-watch.mjs --loop --interval 900 \
  < /dev/null >> .runtime/day-run/$(date +%F)/day-watch.log 2>&1 &

# 定时长跑：14:00 跑到本地 18:00 自动停止、恢复 35qps 并生成 summary.md
setsid nohup node hack/night-run/day-watch.mjs --loop --interval 900 --until 18:00 \
  --baseline-qps 200 --peak-qps 350 --peak-minutes 15 --cycle-minutes 60 --final-qps 35 \
  < /dev/null >> .runtime/longrun/$(date +%F)/day-watch.log 2>&1 &

# 自定义剧本：--baseline-qps / --peak-qps / --peak-minutes / --cycle-minutes / --tenant
# 时长控制：--until HH:MM（本地时区，到点停止）或 --hours N（跑 N 小时）；--final-qps 结束时恢复的档位
```

- 每轮：按剧本判定目标 qps（周期相位从进程启动起算，前 `cycle-peak` 分钟基线、最后 `peak` 分钟压测）→ GET `/api/v1/traffic` 对比 → 偏差时 `PATCH /api/v1/tenants/{name}/traffic`（带 `Idempotency-Key`）→ 跑 `keepalive.mjs --once` 健康检查 → 每 2 轮跑 `snapshot.mjs --once --summary` 快照 → `kubectl` 采集节点用量 / 最近扩缩 / 实例副本。
- 产物统一落 `.runtime/longrun/<日期>/`：`rounds/` 每轮完整记录（JSON，含 keepalive/snapshot 全量与 kubectl 采集）、`snapshots/` 指标快照、结束时 `summary.md`（轮次统计 / 扩缩容事件 / 指标范围 / 趋势）。日志与快照不再分家。
- 启动时自动 preflight：18080 可达性（3 次探测）、`sleep-guard.sh status`；不满足只警告不阻塞（keepalive 会尝试恢复端口转发）。
- 轮次间隔按"上一轮实际耗时"补足，长跑不漂移；异常只记录不折腾（维持模式），事后由 Agent 读 rounds/快照一次性分析。
- 停止：`kill <PID>`（`ps aux | grep day-watch` 查 PID）。运行前确认 `sleep-guard.sh status` 为 `guard=on`、Backend 18080 可达（WSL 内脚本专用端口；8080 是 Windows 浏览器入口，见 KNOWN_PITFALLS）。

## 手动运行


```bash
# 单次健康检查
node hack/night-run/keepalive.mjs --once

# 常驻循环（每 15 分钟一次；必须 setsid，nohup 挡不住 exec 进程组回收）
setsid nohup node hack/night-run/keepalive.mjs --loop --interval 900 < /dev/null >> .runtime/night-run/<日期>/keepalive.log 2>&1 &

# 抓一次快照
node hack/night-run/snapshot.mjs --once

# 抓全部指标并输出摘要
node hack/night-run/snapshot.mjs --once --summary
```

## 睡眠守卫（sleep-guard，方案 A）

宿主机交流空闲 15 分钟自动睡眠会冻结整个 WSL（2026-08-17 值守事故根因），值守前必须启用睡眠守卫：

```bash
bash hack/night-run/sleep-guard.sh status   # 查询：standby_ac=15 hibernate_ac=180 guard=off
bash hack/night-run/sleep-guard.sh on       # 关闭交流空闲睡眠/休眠（弹 UAC，需人点"是"）
bash hack/night-run/sleep-guard.sh off      # 恢复原值（15 分钟睡眠 / 3 小时休眠，弹 UAC）
```

- `on` 会把原值保存到 `%LOCALAPPDATA%\night-run-sleep-guard.json`，`off` 按保存值恢复，缺省 15/180。
- 值守流程：开工确认 `guard=on`；收尾（Phase B 09:00 前后）尝试 `off`，弹窗无人点则失败可接受，事后手动恢复。
- 预授权：白天先执行一次 `on`（本次已做，2026-08-17 08:14 CST），值守期间零弹窗；结束后记得 `off` 恢复日常睡眠习惯。

## 自动化入口

Codex 桌面自动化（`$CODEX_HOME/automations/`）在 00:00 与 04:30 各触发一次：
- Phase A：读 `hack/night-run/phase_a_prompt.md` 并严格执行；
- Phase B：读 `hack/night-run/phase_b_prompt.md` 并严格执行。

非运行日（非 2026-08-17）的触发会自动空跑退出，避免浪费 Token。

## 会话模型（重要）

- 两条自动化是 project 型 cron：**每次触发都是全新会话**，不复用任何已有会话，不存在上下文积压问题。
- 新会话没有对话上下文，所以提示词要求"先读文件再干活"（AGENTS.md、README、phase prompt、problems.md），信息全部落在仓库与 `.runtime/`。
- 前提：Codex 桌面 App 必须保持运行，到点才会触发；合盖/退出 App 会导致自动化不执行。
- Phase A 开工后先拉起常驻 keepalive（`setsid nohup`，见上），即使自动化会话提前结束，健康检查与采集仍继续。