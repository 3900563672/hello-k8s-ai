# Notion 中枢搭建：浏览器自动化反复失败，转人工即成功

> 日期：2026-08-20 ｜ 触发者：本地 Agent ｜ 相关：change-history/2026-08-20-notion-agent-collab-lesson/

## 现象
- Notion 市集「Get template / 获取模板」自动化点击无任何反馈（实际已触发导入），真人一点即成功。
- Edge 走 FlClash 代理间歇 `ERR_PROXY_CONNECTION_FAILED` / `ERR_CONNECTION_RESET`；PowerShell 走同一代理全部 200。
- `dom_cua.click(node_id)` 坐标偏移：点「集成」实际命中上一行「通知我」，反复重试 20+ 分钟。
- Notion API（内部集成）无法创建 workspace 根级页面，报 `parent.page_id` 限制。

## 上下文
- 搭建 Notion 个人中枢（课表 + 学习管理 + 开源流水线 + 沉淀库）：导入 3 个模板、共享集成、建 7 个中文数据库。

## 处理
- 模板添加、创建中枢页、页面共享集成 → 转人工（每次 15 秒内完成）；数据库结构全部走 Notion API（一次成功）。
- 已解决：3 个模板导入完成，中枢页已建并共享给集成，待 API 建库与排版。
- promoted: lessons/process-human-agent-division-of-labor.md
