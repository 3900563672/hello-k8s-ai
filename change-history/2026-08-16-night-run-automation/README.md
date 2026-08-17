# 夜间长时运行自动化：Phase A 维持采集 + Phase B 分析修复

- 变更日期：2026-08-16
- 关联问题：无（用户拍板"直接走 A 方案"，配置无人值守长时运行）
- 变更级别：P1 运行能力（不改业务代码）
- 变更范围：`hack/night-run/`、`docs/agents/WORKFLOW.md`、Codex 桌面自动化配置（`$CODEX_HOME/automations/`，不入库）
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

- 新增夜间运行骨架 `hack/night-run/`：README、keepalive.mjs（健康检查 + 断线自恢复）、snapshot.mjs（指标快照）、phase_a/b_prompt.md（四槽位提示词模板）、problems.template.md（问题档案模板）。
- 配置两条 Codex 桌面自动化（cron 型，project 关联 hello-k8s-ai）：
  - `night-run-phase-a-keepalive`：每天 00:00（Asia/Shanghai）触发 Phase A，2026-08-17 首次执行；
  - `night-run-phase-b-fix`：每天 04:30 触发 Phase B，读档案后按决策矩阵处理。
- 自动化 TOML 位于 `C:\Users\hh\.codex\automations\<id>\automation.toml`（版本化不适用，属本机配置）。
- `docs/agents/WORKFLOW.md` 新增 4.2 夜间长时运行小节。

## 2. 关键行为

- Phase A：维持运行 + 施压 + 采集（每 15 分钟健康检查、每 30 分钟快照、至少 3 次配置变更），不推任何代码。
- Phase B：读 `.runtime/night-run/<日期>/problems.md`，按决策矩阵直接修 / 建 issue / 存疑，本地验证后**全部走 PR 交付（不推 main）**，创建完 PR 等其 CI 绿，早上由用户审阅合并。
- 非运行日（非 2026-08-17）触发自动空跑退出，避免浪费 Token。
- 快照采集真实链路：controller.errorRate（Reconcile 错误比例）、simulator.ttft/queue/qps/tickLatency、traffic 档位、资源计数、DB 状态。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| hack/night-run | 新增 6 个文件（Agent 本机工具，不进 CI 之外的产物） |
| docs/agents | WORKFLOW.md 新增 4.2 小节 |
| Codex 自动化 | 新增 2 条 cron 自动化（本机配置，不入库） |
| change-history | 新增本条 |

## 4. 未验证 / 风险

- 自动化 TOML 由逆向 app.asar 所得格式手写，桌面 UI 是否立即显示待用户打开 Automations 面板确认；若未显示，重启 Codex 桌面应用后应加载。
- 快照脚本对 port-forward 偶发连接复用失败已加重试；长时间运行稳定性待夜间实测。
- Phase B 的修复质量依赖提示词约束，首次运行后按实际问题复盘调整。