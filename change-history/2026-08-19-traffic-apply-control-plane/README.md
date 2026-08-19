# Traffic 叠加应用写入租户目标 QPS（Fixes #89）

> 日期：2026-08-19 ｜ 关联：dashboard/frontend/my-app/README.md（数据边界）

## 为什么做

- Traffic 页面"确认叠加"此前只写前端 Zustand 内存 overlay，页面提示"已将模板叠加到租户"，但集群里 `Tenant.spec.qps` 没有任何变化——误导性反馈。
- 后端 `PATCH /api/v1/tenants/{name}/traffic` 写接口早已存在，缺口完全在前端未接入。

## 改成什么

1. `trafficApi` 新增 `setTenantTraffic`：PATCH 写入租户目标 QPS，带幂等键（沿用 `updateSimulationRate` 模式），返回写入回执。
2. `TrafficPage` 应用流程改为 mutation：目标 QPS = 租户当前 `requestedQPS` + 模板起始增量（`getTemplateValueAtTime(template, 0)`），写入成功后才把 overlay 加入本地列表并提示；失败显示具体错误（红色通知），不再假装成功。
3. `ApplyOverlayDialog` 展示"应用后租户目标 QPS"，提交进行中禁用按钮；历史模式只读，禁止应用并提示。
4. 类型注释与前端 README 数据边界描述同步更新（控制面为常量目标 QPS，曲线仅作场景预览）。

## 关键行为

- 叠加语义与页面文案一致：原流量 + 模板起始增量 = 新目标 QPS（纯 QPS 加法）。
- 控制面只支持常量 QPS，曲线为场景预览；如需时段曲线需 CRD/API 级改造（另行评估）。
- 历史模式不可应用，维持"历史只读"边界。

## 验证

- `npm run check` 通过（oxlint + tsc build + 状态契约检查）。
- 未在真实集群点击验证（当前未起前端环境）；接口契约与既有写接口一致。

## 回滚

- git revert 本提交即可恢复"应用仅本地草稿"行为；不影响控制面数据。
