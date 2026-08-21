# 2026-08-21 AIOps 流量波形根因治理（AI 描绘波形 + 限制可见 + 时长自由可停止）

> 日期：2026-08-21 ｜ 关联：docs/aiops/AIOPS_OVERVIEW.md、docs/backend/API_DESIGN.md、issue #134

## 为什么做

第一轮流量形状（PR #132）落地后暴露三个根因：

1. **波形只能靠模板枚举/手画 controlPoints**：预设模板最长 5 分钟导致波形图只有 0~5 分钟；新增的 2h 潮汐模板 controlPoints 全是 y=30，画出来是平线，与描述「峰值 50 正弦」不符。
2. **限制不可见**：用户/AI 说 500 QPS 被钳到 200、未给数字被默认成 20，用户完全不知道；日配额（#129）页面不可见，被拒时懵。
3. **运行中状态不可见、无法主动停止**：波形调度是后台 goroutine，跑完前无任何状态，也没有 stop 入口。

## 改成什么

1. **AI 描绘波形（主路径）**：`GenerateTrafficCurve(shape, peak, period, duration)` 按形状参数生成采样点（自适应粒度：30 分钟内 1 分钟、6 小时内 5 分钟、24 小时内 15 分钟、更长 30 分钟，点数有界）；解析/查询/确认/停止任一入口返回 `applied.curve`，前端直接渲染曲线预览，无需用户手画。模板保留为快捷方式兜底。
2. **请求值 → 生效值可见**：`NormalizeTrafficIntent` 就地钳制（超上限 → 200/100）与补默认（未给峰值 → 20、未给倍速 → 1、非稳态未给时长 → 一个周期），返回 `applied.values`（field/requested/effective/reason=clamped-to-max|defaulted|ok）；`ValidateCommandIntent` 只做结构校验（正数/形状/租户必填），不再拒绝超上限。
3. **时长自由 + 可停止**：不设时长上限（`limits.unlimitedDuration=true`）；新增 `POST /api/v1/aiops/commands/{id}/stop`（停止信号 registry → 调度器 QPS 归零 → 恢复倍速 → 状态 stopped）；命令状态机新增 `stopped`；波形调度期间状态保持 `executing`，结束置 `done`（此前 confirm 后立即 done，与实际运行不符）。
4. **配额可见**：新增 `GET /api/v1/aiops/quota`（24h 调用/token 用量与上限），前端面板展示剩余额度。
5. **修复平线模板**：`preset-traffic-tidal-2h` 改为真实正弦采样点（峰值 50、周期 30 分钟、5 分钟粒度 25 点，与后端 `TrafficShapeQPS` 同源算法）。
6. 前端命令卡片：AI 波形 SVG 预览 + 生效值列表（要求 500 → 生效 200（超上限，已钳制））+ 执行中进度条/当前 QPS（按曲线插值）/墙钟剩余 + 停止按钮 + 2 秒轮询状态。

## 关键行为

- 对 AI 说「2 小时潮汐、峰值 500」→ 解析成功，卡片显示「峰值 QPS 500 → 200（超上限，已钳制）」与正弦曲线；执行中显示进度与当前 QPS，可随时停止。
- 用户说「模拟 48 小时」→ 不拒绝：曲线预览自适应粒度，调度器按墙钟推进，可停止。
- `GET /api/v1/aiops/limits` 新增 `defaultRate / unlimitedDuration / supportsStop`，提示条展示「时长不限 / 随时可停止」。

## 验证

- 后端单测：`TestNormalizeTrafficIntent`（默认/钳制/缺时长补周期/2h 曲线全程覆盖）、`TestGenerateTrafficCurve`（潮汐非平线、72h 粒度有界）；`go test ./...` 全绿。
- 前端：`tsc -b` + `oxlint` 0 警告 + `vite build` 通过。

## 回滚

- git revert 本提交；若回滚后残留流量，手动 `kubectl patch tenant <name> --type=merge -p '{"spec":{"qps":0}}'`；`stopped` 状态与 stop 路由随 revert 移除。
