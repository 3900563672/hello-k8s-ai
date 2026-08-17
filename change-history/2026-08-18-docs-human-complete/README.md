# 变更总览：人类文档补全与远程 AI 手册完善——文档漂移全面对齐

> 日期：2026-08-18 ｜ 级别：P2 ｜ 对应 Issue：无（用户指定的文档专项任务）

## 为什么做

- 用户要求"文档漂移全部做好、远程 AI 手册完善、人类文档补全，以人为本"，并授权一路执行到无法再明确修改为止。
- 上一轮文档体系重构（2026-08-18-docs-architecture-rework）建立了门禁与分层，但存在遗留漂移：前端 `/monitor`、`/guide` 页面已实现却未写入页面文档；ROADMAP 标题编号错乱；DEPLOYMENT 并入段落编号重复；白皮书基线停留在 2026-08-14；远程 AI 手册只有单一入口、缺少任务决策、踩坑速查与交付模板。
- `make docs-check` 发现 MAP 门禁失败：`hack/gen-context-pack.sh` 的 `FULL=1` 默认变更未同步到映射文档。

## 改成什么

1. **漂移修复**
   - `docs/overview/ROADMAP.md`：标题从 `## 2.` 起改回 `## 1./2./3.` 连续编号。
   - `docs/getting-started/DEPLOYMENT.md`：并入段（原 operations/CLUSTER_INFORMATION 第 5/6 节）重编号为 `## 8.` + `### 8.1/8.2`，消除与前面 5/6 的编号冲突。
   - `docs/operations/TROUBLESHOOTING.md`：新增第 17 节"上下文包生成失败"，同步 `FULL=1` 默认全量包行为（解决 MAP 门禁）。
   - `docs/frontend/PAGE_STRUCTURE.md`：路由表补 Monitor/Guide 两行，新增第 7 节 Monitor 页面、第 8 节 Guide 页面（原迁移矩阵顺延为第 9 节）。
   - `docs/frontend/FRONTEND_ARCHITECTURE.md`：组件分层图与目录职责补 Monitor/Guide；状态所有权表补 Monitor 健康状态与 Guide 常量行。
   - `docs/reference/SOURCE_MAP.md`：路由行补全 5 条路由；features 目录补 monitor/guide 组件。
   - `docs/MAP.yaml`：新增 `dashboard/frontend/my-app/src/components/features/monitor/` 映射（→ PAGE_STRUCTURE、FRONTEND_ARCHITECTURE）。
   - `docs/INDEX.md`：文档状态说明更新为 2026-08-18 门禁现状；PAGE_STRUCTURE 行补 Monitor/Guide；全部文档表新增 change-history 入口。
   - `PROJECT_OVERVIEW_NEW.md`：修复 `Granafa` 错字；目录结构更新为当前真实结构（dashboard/backend、dashboard/frontend/my-app、hack、test、change-history）；阅读路线去重并补"最快体验完整链路"指引。
   - `docs/whitepaper/COMPLETE_OVERVIEW.md`：基线更新为 2026-08-18；3.2 页面结构表补 Monitor/Guide；`## 5.9` 重复编号改为 `## 5.10`；7.4 Grafana 节补 `/monitor` 内嵌与 `/grafana/` 代理说明。
   - `docs/overview/ROADMAP.md`：从 GitHub Open Issues 同步 #31（告警规则实测触发验收，P1）与 #32（扩容节奏与超大副本行为评估，P2）。
   - `docs/README.md`：修正"远程 AI 默认不接收本目录"过时表述（context-pack 默认 `FULL=1` 全量包，docs/ 只作背景）。
   - 根 `README.md`：当前能力表补 Monitor（Grafana 内嵌）与 Guide（填写指南）行。
2. **远程 AI 手册完善**（`docs/remote-ai/`）
   - `README.md` 重写为入口：新增"本目录文件"表与开工顺序（决策矩阵 → 专题 → 踩坑 → 交付模板），保持 41 行（门禁上限 60）。
   - 新增 `DECISION_MATRIX.md`：8 类任务（理解/审查/设计/写代码/写文档/排障/issue 草稿/评审）的输入、输出与注意点。
   - 新增 `PITFALLS.md`：环境身份、架构语义、方案代码、既有 lessons 引用、时间与提交五类踩坑速查。
   - 新增 `DELIVERY.md`：标准交接格式、四类交付物要求（报告/diff/文档片段/issue 草稿）与交出前自检清单。
   - `hack/context-pack-template.md` 第 7 节文档分层引用三份协议；`docs/MAP.yaml` 新增 `hack/context-pack-template.md` → remote-ai/README.md 精确映射（以后改模板必须同步远程 AI 手册）。
   - `docs/agents/README.md` 与 `docs/getting-started/AI_COLLABORATION.md` 的远程 AI 引用更新为新协议结构（决策矩阵/踩坑/交付模板）。
   - 落实白皮书 PDF 移出版本控制：`git rm --cached hello-k8s-ai-complete-overview.pdf`（`.gitignore` 早已忽略但未生效），与 `BUILD_PDF.md` 的"PDF 输出 `.runtime/` 不提交仓库"声明一致。
   - 同步 2026-08-17 可观测持久化变更（Prom/Jaeger PVC + 168h）到 6 个人类文档：白皮书（2.3 事实源表、7.1/7.3、第八章、第十章）、ARCHITECTURE_OVERVIEW、ROADMAP、IMPLEMENTATION_RETROSPECTIVE、DEPLOYMENT、DATABASE_DESIGN——此前仍写"24h / emptyDir / 易失"。

## 关键行为

- 以后修改前端 features 目录时，`docs/MAP.yaml` 已有 monitor 映射，改 monitor 组件必须同提交更新 PAGE_STRUCTURE 与 FRONTEND_ARCHITECTURE。
- 远程 AI 现在有"先定位任务类型 → 避开已知坑 → 按模板交付"的完整协议，不再只靠一段 README。
- 人类文档与源码一致性由 `make docs-check` 持续拦截；本次修复后全绿。

## 验证

- `make docs-sync`：README 时间线段、`docs/status.md`、`docs/remote-ai/llms.txt`、docs/README 所有权表重新生成。
- `make docs-check`：全绿（链接、白名单、行数、MAP 门禁、新鲜度、front-matter、change-history、journal、孤儿文档）。
- `make docs-sync-check`：提交后工作区无派生差异。

## 回滚

- 纯文档改动，无业务代码：revert 本提交即可；派生文件重跑 `make docs-sync` 恢复。
- 远程 AI 新文件（DECISION_MATRIX/PITFALLS/DELIVERY）删除不影响任何代码路径。
