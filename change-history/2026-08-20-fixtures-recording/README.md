# 真实 API fixtures 录制：前端免后端迭代的素材底座

> 日期：2026-08-20 ｜ 关联：dashboard/frontend/my-app/scripts/record-fixtures.mjs、dashboard/frontend/my-app/src/lib/mocks/fixtures/

## 为什么做

- 前端视觉与信息架构重构需要"改完直接看效果"的循环，不能每次依赖完整后端与集群。
- 现有部署后端镜像早于 #51（实验生命周期 API）合入 3 小时，/api/v1/experiments 404，实验接口无法访问。
- 数据库（PostgreSQL，PVC 持久化）有真实历史：3670 条资源快照、61.9 万条资源事件、7275 条 Trace 索引，可支撑"真实数据 + 免后端"的浏览体验。

## 改成什么

1. 新增 dashboard/frontend/my-app/scripts/record-fixtures.mjs：只读遍历 Dashboard Backend 的 GET 端点，把响应原样保存为 JSON 快照，并生成 manifest.json（录制时间、来源、每项状态）；动态详情（Trace）由列表自动发现；/stream（SSE）不录制。
2. 录制 71 个真实快照到 src/lib/mocks/fixtures/：当前与历史窗口的 bootstrap/overview/replay/segment（3 个时间窗口）/traces（2 个窗口 + 40 条 Trace 详情）/metrics（6 个指标）/experiments（空列表，见下）等，全部 HTTP 200。
3. 重建 hello-k8s-ai-dashboard-backend:dev 镜像并滚动更新，补齐 #51 实验接口（常规开发部署，未动 WSL/Docker/集群生命周期）。

## 关键行为

- fixtures 是真实响应快照，只读复用；需要新数据时重录，不手工改内容。
- experiments 为空列表是真实状态：切面功能 08-18 合入前从未部署过带该接口的后端，数据库无历史实验记录；空态设计可先基于此进行。
- 录制脚本幂等：重跑会清空 fixtures 目录后重录。

## 验证

- 全部 71 项 HTTP 200；node --check 与 oxlint 通过。
- manifest.json 记录每项来源路径与大小，可审计。

## 回滚

- git revert 本提交即移除脚本与快照；后端镜像更新属常规部署，不影响数据。
