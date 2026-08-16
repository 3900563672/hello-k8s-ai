# 夜间长时运行首次执行：值守中断事故 + 工具修复与坑位沉淀

- 变更日期：2026-08-17（Asia/Shanghai；UTC 跨 2026-08-16/17）
- 关联：hack/night-run 首次真实执行；自动化 00:00 因模型问题未启动（见 KNOWN_PITFALLS），改由手动会话值守
- 变更级别：P1 运行能力（不改业务代码，只改值守工具与 Agent 文档）
- 变更范围：`hack/night-run/snapshot.mjs`、`hack/night-run/phase_a_prompt.md`、`hack/night-run/README.md`、`docs/agents/KNOWN_PITFALLS.md`
- CRD 变化：无 ｜ 数据库变化：无

## 1. 完成结果

- 首次真实值守：00:17–00:50 主动执行（基线、keepalive、9 次施压变更），00:50 宿主机空闲睡眠冻结 WSL 约 7 小时，07:48 恢复后补写交接档案并按纪律停止；共归档 9 个问题（P1×2、P2×3、P3×4）。
- 修复问题：#2 snapshot.mjs 漏定义 `sleep`（采集崩溃）；#4 提示词租户示例 `core`→实际 `tenant-core`；#5 提示词 rate 示例 40/80 超出系统上限 20；#6 移除 `kubectl scale` 提示（副本归 Orchestrator）；#8 keepalive 启动命令加 `setsid`（nohup 挡不住 exec 进程组回收）。
- 沉淀 3 条坑位：宿主机空闲 15 分钟自动睡眠冻结 WSL、nohup 进程组回收、snapshot sleep 漏拷（见 `docs/agents/KNOWN_PITFALLS.md`）。
- 实测容量结论：50qps@10 副本触发队列积压（queue≈5000+、TTFT 分钟级），35qps@rate20 健康——队列无背压，待后续分析。

## 2. 关键行为

- 快照脚本修复后实跑通过（`--once --summary` 输出 ok:true）；系统 07:48 恢复后 18 Pod 全 Running、50qps 积压队列自动回排中。
- 值守红线保持一致：Phase A 不推代码、不建 issue；问题档案在 `.runtime/`（不入库）。
- 休眠问题待用户选择方案（A：夜间自动禁用空闲睡眠/休眠；B：PowerToys Awake），方案落地后提示词与 README 同步。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| hack/night-run | snapshot.mjs 补 sleep；phase_a_prompt/README 修正示例与启动命令 |
| docs/agents | KNOWN_PITFALLS 新增 3 条坑位（含本次事故） |
| change-history | 新增本条 |
| .runtime（不入库） | 首次执行档案 problems.md（问题 #1–#9）、快照 3 份、snapshot-fixed.mjs |

## 4. 未验证 / 风险

- 整夜值守未完成：休眠中断导致快照缺失约 7 份；禁用睡眠方案未实测。
- Phase B 自动化（04:30）本次未触发（系统休眠）；其 model 修复效果仍未实测。
- 队列积压回排期间 TTFT 仍高（实跑快照 ttft≈991s），属 50qps 事件积压的残余影响，恢复时间未测量。
