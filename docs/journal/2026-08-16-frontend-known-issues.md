# 前端已知问题清单（待用户确认，未修）

> 日期：2026-08-16 ｜ 触发者：本地 Agent ｜ 相关：change-history/2026-08-16-ui-visual-verification/（2026-08-18 从 UI_VERIFICATION.md 移入 journal）

### 已知前端问题清单（待用户确认，未修）

| # | 问题 | 状态 |
| --- | --- | --- |
| 1 | Grafana「Reconcile 错误比例」面板（用户报的第一个 Grafana 错） | 待核对：当前显示 0%，但 Reconcile 速率面板有 `simulator-instance / error` 计数 |
| 2 | 流量页始终显示"还没配置流量"（实际已配置 qps=25） | 待查 |
| 3 | 时间飞速时间轴每次自动刷新闪一遍（其他面板不闪） | 待查 |
| 4 | 「模拟器 Leader」面板：10 行 0/1 平铺、9 红 1 绿，一眼找不到"当前 Leader" | 已确认为展示设计问题；最小修法：expr 加 `== 1` 只留 Leader 行，再去掉红色阈值 |
| 5 | Grafana Live WebSocket `/grafana/api/live/ws` 握手 400（反代不支持 WS 升级） | 无害，面板走轮询；可后置修反代 |
