# 实现细节：首次执行事故修复与坑位沉淀

## 1. 改动前状态（问题来源）

- `hack/night-run/snapshot.mjs`：从 keepalive.mjs 拷贝 `httpGet` 时漏拷 `const sleep` 辅助函数；第 47 行 `await sleep(500)` 运行时 ReferenceError，任何 fetch 首次失败即整份快照丢失（问题 #2）。
- `hack/night-run/phase_a_prompt.md`：
  - 目标 2 示例"5–60 qps""replica +2"：replica 不可 kubectl scale（SimulatorInstance 无 scale 子资源，副本由 Orchestrator 写 spec.replicas）（问题 #6）；
  - 目标 4 未注明 rate 有效范围 1–20（示例档位 20→40→80 超出上限，实测 400 COMMAND_REJECTED）（问题 #5）；
  - API 路径示例用租户名 `core`，实际资源名为 `tenant-core`（404）（问题 #4）。
- `hack/night-run/README.md`：手动运行部分 keepalive 常驻命令为裸 `nohup ... &`，在 Codex exec 环境下进程组被回收（问题 #8，PID 120061 死亡；setsid 重启 PID 122828 存活）。
- 值守前提缺失：提示词只写"App 必须保持运行"，未覆盖宿主机电源睡眠（问题 #9，交流空闲 15 分钟自动睡眠，powercfg 实测）。

## 2. 实现

| 文件 | 改动 | 对应问题 |
| --- | --- | --- |
| hack/night-run/snapshot.mjs | `utcNow` 后补 `const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));` | #2 |
| hack/night-run/phase_a_prompt.md | 启动命令改 `setsid nohup ... < /dev/null >> log 2>&1 &` 并注明原因；qps 档位改 5–50 并注明 50 实测积压；小步档位改"qps +10 或 rate ±1 档"；目标 4 注明 rate 范围与"不做 kubectl scale" | #4/#5/#6/#8 |
| hack/night-run/README.md | 常驻命令加 setsid 与说明；运行约定补充实测边界（rate 1–20、tenant-core、50qps 积压、副本归 Orchestrator） | #4/#5/#6/#8 |
| docs/agents/KNOWN_PITFALLS.md | 新增 3 条坑位（睡眠冻结 WSL / nohup 进程组回收 / snapshot sleep 漏拷），更新头部时间戳 | #9/#8/#2 |

## 3. 未修复问题（留档）

- #7 容量边界：50qps@10 副本队列爆炸、无背压——需补 45qps、rate10 隔离测试后决策（可能涉及 Orchestrator/队列策略）。
- #1 port-forward 瞬时 000 高频：重试可恢复，评估直连 backend service 或换转发方式。
- #3 simulator.errorRate 空序列：catalog PromQL 与模拟器指标名可能不一致，待核对。
- #9 休眠：本条目只沉淀坑位；方案 A/B 待用户选择后实施。
