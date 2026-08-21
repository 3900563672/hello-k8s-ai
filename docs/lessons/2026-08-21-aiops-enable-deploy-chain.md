# 过程复盘：AIOps 启用 + 部署修复链路（2026-08-21）

## 一句话结论

一次"启用 AIOps"最终变成 2 个修复 PR（#115/#116）+ 3 轮部署，耗时长的根因是：默认关闭的功能路径是 CI 盲区 + 门禁系统对每次修复都要求完整验证 + 环境脚本/工具链小坑叠加。

## 时间都花在哪（按占比排序）

1. **合并残留 bug（最贵）**：#114 先注册 `/api/v1/aiops/jobs`（handleListAIOpsJobs），#113 后合并时残留旧注册（handleListAIOpsAnalyses 挂同一 pattern）。git 合并不报冲突（两行都是新增），CI 全绿（AIOPS_ENABLED=false 时整块路由不执行），直到启用 AIOps 后端启动 panic。定位 + 修复 + 回归测试 + 等 CI 约 1 轮完整链路。
2. **门禁多轮全量 CI**：main 有分支保护，任何修复必须 PR + 全量 status check（含 E2E 6-7 分钟，push 与 PR 各触发一套）。#115 因 change-history 引用缺失/MAP 文档同步/脚本 bug 经历多轮 amend-push，每轮 ~10 分钟。
3. **环境脚本断言过严**：`kubectl get simulationclock --no-headers` 无资源时 exit 0 误入等待分支（干净环境部署必现超时）；干净环境断言把"保留 PVC 的历史快照"判为失败（复用环境必现）。
4. **工具链小坑**：`kind get nodes` 不支持 `--context`（命令静默失败导致镜像导入循环空转，新 pod 一直用旧镜像）；GraphQL archive mutation 的 payload 字段是 `item` 不是 `projectV2Item`；PowerShell 多行引号被拆。
5. **误提交清理**：#113 把 node_modules 符号链接（gitlink）提交进 main，需单独清理 PR。

## 已验证的事实（AIOps 历史数据不会刷爆 API）

- L1/L2 逐切面分析**只由实验 complete/fail 入队**触发（EnqueueAnalysis），worker 启动只回收崩溃遗留的 pending/running（RequeueStale，当前 0 条），**没有历史全量回填逻辑**。
- M3 窗口聚合只扫最近 6 个窗口槽位（WindowGranularity×6），且窗口内必须有 analyses 记录才调 LLM；历史数据无 analyses 记录 → 跳过。
- 已有保护：对话限流 6 次/分钟/会话、worker 单轮批量上限、单分析 MaxAttempts=3、OpenAI 调用预算（budget.go）。
- 结论：启用后不会对存量历史数据发起批量 LLM 调用。未来若做"历史回填分析"，必须设计：显式开关 + 分批窗口 + 限流 + 预算上限，否则 API 会被刷爆。

## 必须动作（guarded 于 FR-014）

- 默认关闭的功能块必须有一条"启用路径最小测试"（路由注册不 panic + 关键端点 200）。
- 环境断言要区分"全新环境"与"复用 PVC 的幂等部署"。
- kind 类命令失败要检查退出码，不能静默吞错。
