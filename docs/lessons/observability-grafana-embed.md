# Grafana 嵌入 Dashboard：sub-path 前缀、Live WS、安全中间件三处易错

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-16-grafana-embed.md ｜ 适用对象：本地 Agent

## 现象

Grafana 经 Dashboard 反代嵌入：Live WebSocket 握手 400、资源 404（丢 /grafana 前缀）、或整体被安全中间件拦截。

## 根因

1. 反代必须保留 `/grafana` 前缀（sub-path），否则静态资源 404；
2. WebSocket 升级路径也要走 sub-path；
3. Backend 安全中间件会覆盖 Grafana 放行规则。

## 可复用规则

- 反代规则与安全中间件放行都按 `/grafana` 前缀处理；面板 iframe 用相对路径。
- 改嵌入配置后先验证 Grafana 页面资源（非 API），再验证面板数据。

## 验证方法

浏览器打开 Dashboard 监控面板：无 404、无 WS 400；Grafana 面板加载数据。
