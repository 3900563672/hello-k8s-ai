# 变更归档

> 维护层：human | last-reviewed：2026-08-18 | 事实源：change-history/ 目录本身

本目录保存需要长期追踪的代码变更记录。它不替代 `docs/` 中的当前架构说明：`docs/` 描述现在如何运行，本目录说明某次修改为什么发生、改了什么、如何测试和回滚。

## 格式规范

- 每个变更一个目录：`YYYY-MM-DD-短横线-slug/`，内含 `README.md`（首行 H1，必带 `> 日期：YYYY-MM-DD` 元信息；`make docs-check` 强制校验）。
- 两代格式并存：
  - 旧格式（2026-08-16 及之前）：四件套——`README.md`（总览）/ `IMPLEMENTATION_DETAILS.md` / `TEST_REPORT.md` / `MIGRATION_AND_ROLLBACK.md`，适合大改动。
  - 新格式（2026-08-17 起）：单文件 `README.md`，内含「为什么做 / 改成什么 / 关键行为 / 验证 / 回滚」等节，适合中小改动。
- 追加或归档条目后运行 `make docs-sync`：README 时间线段与 `docs/status.md` 自动更新；未同步时 `make docs-check` 会失败。
- 新条目按日期倒序写入下方索引表。

## 变更索引

| 日期 | 主题 | 级别 | 入口 |
| --- | --- | --- | --- |
| 2026-08-21 | AIOps 模板预置：10 模型 + 10 租户 + 10 节点（空环境待命） | P1 | [查看记录](2026-08-21-aiops-template-seed/README.md) |
| 2026-08-21 | 环境一键自愈 make env-up（重启后一条命令恢复联调） | P1 | [查看记录](2026-08-21-env-up/README.md) |
| 2026-08-21 | E2E 双触发并行 flake 修复：push 限 main + go test 超时放宽 | P1 | [查看记录](2026-08-21-e2e-trigger-fix/README.md) |
| 2026-08-21 | AIOps 运行时开关：面板启用/停用分析入队 + 历史数据清理 | P1 | [查看记录](2026-08-21-aiops-panel-toggle/README.md) |
| 2026-08-19 | WSL GitHub 代理配置沉淀：检测脚本 + 蒸馏规则 | P1 | [查看记录](2026-08-19-wsl-github-proxy/README.md) |
| 2026-08-20 | 前端重构第四轮：设计打磨 + 配置样例数据 + 模板加载真实 bug（#101） | P2 | [查看记录](2026-08-20-frontend-redesign-round4/README.md) |
| 2026-08-20 | 浏览器自动化与人工分工经验沉淀（Notion 中枢搭建复盘） | P3 | [查看记录](2026-08-20-notion-agent-collab-lesson/README.md) |
| 2026-08-20 | 真实 API fixtures 录制：前端免后端迭代的素材底座 | P2 | [查看记录](2026-08-20-fixtures-recording/README.md) |
| 2026-08-19 | Traffic 叠加应用写入租户目标 QPS（Fixes #89） | P2 | [查看记录](2026-08-19-traffic-apply-control-plane/README.md) |
| 2026-08-19 | Kind 集群移除 extraMounts：PVC 数据改存节点 /var 数据卷（Fixes #54） | P0 | [查看记录](2026-08-19-kind-extra-mounts-removed/README.md) |
| 2026-08-19 | Agent 对外协作规则落地：AI 产出自查 + 披露 trailer + lessons 触发式升级 | P2 | [查看记录](2026-08-19-ai-collab-rulebook/README.md) |
| 2026-08-19 | WSL 源码级验证：2.9.5+ 修复已确认存在（#71 最后证明项） | P1 | [查看记录](2026-08-19-wsl-source-fix-verified/README.md) |
| 2026-08-19 | Agent 进化：静态检查三件套 + 失败模式注册表 + make doctor（文本沉淀→机器强制） | P0 | [查看记录](2026-08-19-agent-evolution-force-layer/README.md) |
| 2026-08-19 | 一切皆异步：工作流硬规则沉淀（WORKFLOW 4.3 + AGENTS 第 7 条） | P0 | [查看记录](2026-08-19-async-workflow-principle/README.md) |
| 2026-08-19 | WSL 升级验证实验：2.9.4 预览线仍复现，环境已回滚（#71/#63） | P1 | [查看记录](2026-08-19-wsl-upgrade-validation/README.md) |
| 2026-08-19 | WSL 缓存预热对照实验：瓶颈定位到 seccomp 通知链（#72） | P2 | [查看记录](2026-08-19-cache-warm-research/README.md) |
| 2026-08-19 | Grafana 代理测试 WSL 注册竞态修复（Fixes #73） | P2 | [查看记录](2026-08-19-grafana-test-wsl-race/README.md) |
| 2026-08-19 | 跨平台文件卫生沉淀：UTF-8 BOM / Git 执行位噪音 规则与修复 | P3 | [查看记录](2026-08-19-cross-platform-file-hygiene/README.md) |
| 2026-08-19 | WSL 重启后排除测试 + 探针工具缺陷修复 | P1 | [查看记录](2026-08-19-wsl-reboot-exclusion-test/README.md) |
| 2026-08-19 | WSL 回环研究 openvmm/WSL 溯源 + 版本缺陷区间勘误 | P1 | [查看记录](2026-08-19-wsl-history-trace/README.md) |
| 2026-08-19 | 重启后环境恢复 + Codex 卡死排查 + WSL 协作模式评估 | P2 | [查看记录](2026-08-19-environment-revive-after-reboot/README.md) |
| 2026-08-19 | WSL 回环研究三 issue 闭环（#66 查重 / #65 指纹 / #64 参数实验） | P1 | [查看记录](2026-08-19-wsl-issue-64-65-66/README.md) |
| 2026-08-18 | 仓库健康度治理：安全政策、分支保护与依赖扫描 | P2 | [查看记录](2026-08-18-repo-hygiene/README.md) |
| 2026-08-18 | 仓库美化：CONTRIBUTING、主页 README、Wiki 与仓库元数据 | P3 | [查看记录](2026-08-18-repo-beautify/README.md) |
| 2026-08-18 | 白天长时运行收尾 + Kind 迁移数据备份（#50 前置） | P1 | [查看记录](2026-08-18-longrun-and-kind-migration-prep/README.md) |
| 2026-08-18 | 项目采用 Apache-2.0 许可证 | P2 | [查看记录](2026-08-18-license/README.md) |
| 2026-08-18 | Kind 底座迁移完成：部署闭环 + 数据恢复 + 工具链修复（#50） | P0 | [查看记录](2026-08-18-kind-migration-complete/README.md) |
| 2026-08-18 | 切面为第一公民：生命周期 API + 混合采样器 + 分层存储（Fixes #51） | P0 | [查看记录](2026-08-18-issue51-segment-lifecycle/README.md) |
| 2026-08-18 | 业务层优雅降级闭环：物理水位联动 + 资源受限标记（Fixes #49） | P0 | [查看记录](2026-08-18-issue49-graceful-degradation/README.md) |
| 2026-08-18 | 变更总览：#31 告警规则实测触发验证（含 LeaderMissing 规则修复）+ #32 扩容节奏参数化 | P1 | [查看记录](2026-08-18-issue31-issue32/README.md) |
| 2026-08-18 | 宿主工具链恢复：Codex 配置自修复 + 整机重启恢复顺序 + gocyclo 处理 | P2 | [查看记录](2026-08-18-host-toolchain-recovery/README.md) |
| 2026-08-18 | 人类文档补全与远程 AI 手册完善：文档漂移全面对齐 | P2 | [查看记录](2026-08-18-docs-human-complete/README.md) |
| 2026-08-18 | 文档体系重构：机械门禁、内容迁移与入口重写（PR #33/#34） | P1 | [查看记录](2026-08-18-docs-architecture-rework/README.md) |
| 2026-08-18 | Docker bind 失效 → tmpfs 覆盖 → PVC "丢数据"：恢复 + 根因修正 | P0 | [查看记录](2026-08-18-docker-bind-failure-recovery/README.md) |
| 2026-08-18 | 提交节奏规则：逻辑闭环 ≤2 commit 与 PR squash merge | P3 | [查看记录](2026-08-18-commit-rhythm/README.md) |
| 2026-08-18 | 降级演练缺陷修复：告警表达式三修 + 模拟器容器资源限制（Fixes #30） | P1 | [查看记录](2026-08-18-alert-drill-fixes/README.md) |
| 2026-08-17 | 稳定性恢复顺序：运行前体检（preflight）+ 工具链自检（selfcheck）+ Prometheus 内存/重启告警 | P1 | [查看记录](2026-08-17-stability-recovery/README.md) |
| 2026-08-17 | 扩容加速（批量扩容）+ 稳定性矩阵 + 容量指南 | P2 | [查看记录](2026-08-17-scaleup-acceleration/README.md) |
| 2026-08-17 | 时间段切面（Run Segment）：起点/终点全局状态 + 区间指标与 Trace | P1 | [查看记录](2026-08-17-run-segment/README.md) |
| 2026-08-17 | Orchestrator maxReplicas 支持“无限制”（0 = 不限制副本数） | P2 | [查看记录](2026-08-17-orchestrator-max-replicas-unlimited/README.md) |
| 2026-08-17 | 可观测组件持久化（Prometheus/Jaeger PVC）+ 事件丢弃可观测化（指标 + TimelineGap） | P0 | [查看记录](2026-08-17-observability-persistence/README.md) |
| 2026-08-17 | 夜间长时运行首次执行：值守中断事故 + 工具修复与坑位沉淀 | P1 | [查看记录](2026-08-17-night-run-first-execution/README.md) |
| 2026-08-17 | 长跑工具修复（--until 精确停止 / 每轮指标 / summary 口径）+ 长跑验收结论 | P1 | [查看记录](2026-08-17-longrun-tooling-fixes/README.md) |
| 2026-08-17 | 容量校准公式确立 + 长跑验证（14:00-18:00 执行中） | P1 | [查看记录](2026-08-17-longrun-capacity-calibration/README.md) |
| 2026-08-17 | 宿主内存治理：WSL2 内存爆满根因修复（.wslconfig + 负载清零 + Jaeger 校准） | P0 | [查看记录](2026-08-17-host-memory-governance/README.md) |
| 2026-08-17 | day-watch 首次白天值守：流量调整失效根因（8080 端口冲突）与修复 | P2 | [查看记录](2026-08-17-day-watch-port-conflict/README.md) |
| 2026-08-16 | UI 视觉验证链路沉淀：CDP 截图 + DOM 读取 + 监控面板现状 | P2 | [查看记录](2026-08-16-ui-visual-verification/README.md) |
| 2026-08-16 | 流量分配零副本残留 QPS 清理 | P1 | [查看记录](2026-08-16-traffic-stale-qps-cleanup/README.md) |
| 2026-08-16 | Simulator 冷启动进度与 reporter 生命周期解耦 | P1 | [查看记录](2026-08-16-simulator-coldstart-persistence/README.md) |
| 2026-08-16 | 提示词工作流体系：人类 / 本地 Agent / 远程 AI 三份协议 | P1 | [查看记录](2026-08-16-prompting-workflows/README.md) |
| 2026-08-16 | Project Review 看板迁移到仓库级 | P1 | [查看记录](2026-08-16-project-review-repo-level/README.md) |
| 2026-08-16 | Project Review 看板与任务闭环 | P1 | [查看记录](2026-08-16-project-review-board/README.md) |
| 2026-08-16 | Project Review 纳入版本控制与文档同步机制强化 | P1 | [查看记录](2026-08-16-project-review-and-doc-sync/README.md) |
| 2026-08-16 | 可观测性收敛到 Dashboard 单入口（Prometheus / Jaeger / Grafana） | P1 | [查看记录](2026-08-16-observability-single-entry/README.md) |
| 2026-08-16 | 夜间长时运行自动化：Phase A 维持采集 + Phase B 分析修复 | P1 | [查看记录](2026-08-16-night-run-automation/README.md) |
| 2026-08-16 | 本地完整栈本机验证与启动速度优化 | P2 | [查看记录](2026-08-16-local-startup-optimization/README.md) |
| 2026-08-16 | 历史回放覆盖告警：明确 Provider 保留边界 | P1 | [查看记录](2026-08-16-history-replay-coverage-warnings/README.md) |
| 2026-08-16 | 修复 Grafana 运行中内存打满导致探针失败与组件意外停止 | P1 | [查看记录](2026-08-16-grafana-memory-stability/README.md) |
| 2026-08-16 | 前端策略管理打通配置到真实工作负载的完整闭环 | P1 | [查看记录](2026-08-16-frontend-policy-closed-loop/README.md) |
| 2026-08-16 | 分层文档维护边界与同步协议 | P1 | [查看记录](2026-08-16-docs-layered-ownership/README.md) |
| 2026-08-16 | 文档体系分层重构 | P1 | [查看记录](2026-08-16-docs-hierarchy/README.md) |
| 2026-08-16 | 数据库生命周期自动化与当前态持久化读路径（Phase 1-3） | P1 | [查看记录](2026-08-16-database-lifecycle/README.md) |
| 2026-08-16 | 策略 CRD 引用不可变校验 | P1 | [查看记录](2026-08-16-crd-reference-immutability/README.md) |
| 2026-08-16 | 收窄 WorkerNodeUsage 事件映射与用量统计范围 | P1 | [查看记录](2026-08-16-controller-event-mapping-scope/README.md) |
| 2026-08-16 | 统一 Backend 命令幂等、批量应用与审计的一致性边界 | P1 | [查看记录](2026-08-16-command-consistency-boundary/README.md) |
| 2026-08-16 | 全库重复代码抽取重构（不改变业务行为） | P2 | [查看记录](2026-08-16-code-dedup-refactor/README.md) |
| 2026-08-16 | 一键启动默认干净环境：预置配置模板与参数填写指南 | P1 | [查看记录](2026-08-16-clean-startup-templates-guide/README.md) |
| 2026-08-16 | CI 加速与工作流细化（轮询节奏 / 归档详略） | P1 | [查看记录](2026-08-16-ci-acceleration-and-workflow/README.md) |
| 2026-08-16 | Dashboard Backend 写接口可信认证与授权边界 | P0 | [查看记录](2026-08-16-backend-write-auth/README.md) |
| 2026-08-14 | Simulator 时间倍速控制链路 | P1 | [查看记录](2026-08-14-simulator-time-scale/README.md) |
| 2026-08-14 | Orchestrator 放置修复的 CI 收敛 | P0 follow-up | [查看记录](2026-08-14-orchestrator-placement-ci-follow-up/README.md) |
| 2026-08-14 | Orchestrator 选点执行契约修复 | P0 | [查看记录](2026-08-14-orchestrator-placement/README.md) |
| 2026-08-14 | Model 能力基准分生产路径修复 | P0 | [查看记录](2026-08-14-model-absolute-score-production-path/README.md) |
| 2026-08-13 | 2026-08-13 部署整理与一键启动交付（原根目录 CHANGELOG.md） | P1 | [查看记录](2026-08-13-initial-deployment/README.md) |
