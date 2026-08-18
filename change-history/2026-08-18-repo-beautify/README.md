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
4. **Wiki（内容已备好，待初始化）**
   - 已编写 7 个页面（Home/Quick-Start/Architecture/Deployment/Documentation/Roadmap/FAQ），面向人类、链接回仓库 docs/ 避免漂移。
   - 阻塞：GitHub Wiki 仓库必须先在网页创建第一页才会初始化（`git push` 与 API 均不能自动创建），需要用户登录 GitHub 访问 `https://github.com/3900563672/hello-k8s-ai/wiki` 点一次「Create the first page」；初始化后推送页面即可。
   - 待办：profile name/bio 需 `gh auth refresh -h github.com -s user`（当前 token 无 user scope）后设置。

## 关键行为

- 根目录白名单现在允许 4 个 Markdown：README / AGENTS / PROJECT_OVERVIEW_NEW / CONTRIBUTING。
- Wiki 页面内容以仓库 docs/ 为事实源，页面顶部声明"最新以仓库为准"，避免双份维护漂移。

## 验证

- `make docs-check`：全绿（含新增 CONTRIBUTING.md 的链接与白名单检查）。
- 主页 README、仓库 description/topics 已通过 `gh api` 核对。
- Wiki 页面未推送（等待初始化，见上）。

## 回滚

- 仓库内：revert 本提交即可（CONTRIBUTING 删除、白名单还原）。
- GitHub 侧：profile README 可直接编辑；topics/description 可在仓库 Settings 修改；Wiki 页面删除对应 git 提交。
