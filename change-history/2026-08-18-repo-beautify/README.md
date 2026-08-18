# 变更总览：仓库美化——CONTRIBUTING、主页 README、Wiki 与仓库元数据

> 日期：2026-08-18 ｜ 级别：P3 ｜ 对应 Issue：无（用户指定的仓库维护任务）

## 为什么做

- 用户要求维护并美化 GitHub 主页与仓库：修正名词拼写（Grafana）、优化主页、完善 Wiki 与仓库展示。
- 仓库此前缺少贡献指南（CONTRIBUTING.md），根目录 Markdown 白名单也未覆盖它。
- GitHub 主页（profile）没有 README，仓库 description/topics 与 Wiki 均未整理。

## 改成什么

1. **仓库内（本提交）**
   - 新增根目录 `CONTRIBUTING.md`：Issue 模板、PR 流程、验证命令、协作约定（一个逻辑闭环一个 commit、中文提交、文档同步门禁）。
   - `hack/check-docs.py` 根目录 Markdown 白名单加入 `CONTRIBUTING.md`（GitHub 约定贡献指南放根目录）。
   - `README.md` 增加 CONTRIBUTING 引用。
2. **GitHub 主页（不入库）**
   - 创建 profile 仓库 `3900563672/3900563672` 与 README：项目介绍、主力项目 hello-k8s-ai 亮点、项目一览表、技术栈、AI 协作理念。
3. **仓库元数据（GitHub 侧）**
   - hello-k8s-ai：description 更新为「11 个 CRD、7 个 Controller、Simulator 与全链路可观测，Docker Desktop 一键部署」；topics 从 8 个补充到 18 个（go/typescript/kubebuilder/prometheus/jaeger/grafana/opentelemetry/postgresql/docker/dashboard）。
   - AI-JSON-Repair-Tool：补充 description 与 5 个 topics（此前为空）。
4. **Wiki（已完成初始化）**
   - 已编写 7 个页面（Home/Quick-Start/Architecture/Deployment/Documentation/Roadmap/FAQ），面向人类、链接回仓库 docs/ 避免漂移。
   - 初始化：GitHub Wiki 必须先由用户在网页创建第一页（git push 与 API 均不能自动创建）；用户创建占位 Home 后已 rebase 并推送全部页面，Home 替换为完整版。
5. **主页身份（已完成）**
   - 用户完成 `gh auth refresh -h github.com -s user` 授权后，已设置 profile name=hh、bio=「Kubernetes / AI 推理调度仿真平台开发者；hello-k8s-ai 作者」。

## 关键行为

- 根目录白名单现在允许 4 个 Markdown：README / AGENTS / PROJECT_OVERVIEW_NEW / CONTRIBUTING。
- Wiki 页面内容以仓库 docs/ 为事实源，页面顶部声明"最新以仓库为准"，避免双份维护漂移。

## 验证

- `make docs-check`：全绿（含新增 CONTRIBUTING.md 的链接与白名单检查）。
- 主页 README、仓库 description/topics 与 profile name/bio 已通过 `gh api` 核对。
- Wiki 7 页已推送并核对（远端 git ls-tree 确认 Home/Quick-Start/Architecture/Deployment/Documentation/Roadmap/FAQ）。

## 踩坑记录

- GitHub Wiki 必须先由用户在网页创建第一页，git push 与 API 均不能自动初始化；初始化后远端会带一个占位 Home 提交，本地 rebase 时与该提交冲突属正常现象。
- 本机环境经 PowerShell→WSL 嵌套传命令时，heredoc 与嵌套引号会被损坏（反引号丢失、语句截断）；写脚本文件后执行（UTF-8 无 BOM）可避免。
- 本次曾出现 git push origin master 被拒（non-fast-forward）但 git push origin HEAD:master 成功的情况；commit 前先核对本地 ref 与远端对象关系。

## 回滚

- 仓库内：revert 本提交即可（CONTRIBUTING 删除、白名单还原）。
- GitHub 侧：profile README 可直接编辑；topics/description 可在仓库 Settings 修改；Wiki 页面删除对应 git 提交。
