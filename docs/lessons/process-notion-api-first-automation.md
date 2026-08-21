# Notion 自动化：API 优先，内嵌展示用 is_inline 翻转而非链接视图

> 提升日期：2026-08-20 ｜ 来源：journal/2026-08-20-notion-inline-databases.md ｜ 适用对象：本地 Agent / 远程 AI
> 触发条件（Use when）：涉及 Notion 页面/数据库自动化、Hub 页内嵌展示、Views API、is_inline、冗余容器清理时

## 现象

- Hub 页上的完整页面型数据库块只显示标题链接，要点开全屏页才能看到数据；导入模板里的数据库却能内嵌渲染整张表。
- 用链接视图（`POST /v1/views` + `create_database`）能内嵌，但会产生独立容器块，与源库形成重复数据视图；且该容器块按普通 block 归档（`archived: true`）报 400。

## 根因

- 数据库对象有 `is_inline` 字段：`false`（完整页面型）嵌在父页面只渲染链接；`true`（内嵌型）直接渲染表格。模板中的库是内嵌型，所以行为不同。
- 冗余容器是独立的 database 对象，删除要走数据库端点而不是 block 端点。

## 可复用规则（一条规则一句话，禁止复述现象）

1. Notion 结构化操作一律 API 优先（版本头 `Notion-Version: 2026-03-11`），浏览器只用于登录会话与最终 DOM 验证。
2. 完整页面型库要内嵌展示：`PATCH /v1/databases/{id}` + `{"is_inline": true}`，一次生效、原位展示、数据不动。
3. 删除数据库对象（含链接视图容器）：`PATCH /v1/databases/{id}` + `{"in_trash": true}`；对 `child_database` 块用 `archived` 会 400。
4. 建链接视图：`POST /v1/views`，必带 `create_database.parent.page_id` + `data_source_id`（取自 `GET /v1/databases/{id}` 的 `data_sources[0].id`）+ `type: "table"`；视图名可 `PATCH /v1/views/{id}` 改。
5. 查库内容：先 `GET /v1/views?database_id=` 找 view，再 `POST /v1/views/{view_id}/queries`；返回体只有 page id，属性要逐页 `GET /v1/pages/{id}`。
6. PowerShell 执行带中文的 .ps1 必须 UTF-8 带 BOM（0xEF 0xBB 0xBF），否则 PS 5.1 按 ANSI 解析中文键名报解析错误；复杂命令一律写脚本文件再跑。

## 验证方法（命令 / 断言 / E2E；能自动化的给脚本路径）

- DOM 断言（Edge/Playwright evaluate）：`.notion-collection_view_page-block` 计数应为 0，`.notion-collection_view-block` 内嵌表格存在（同一视图渲染多层 DOM，先按 innerText 去重再数）。
- API 断言：`GET /v1/databases/{id}` 返回 `is_inline == true`；删除后返回 `in_trash == true`，且可在 Notion 回收站恢复。
- 数据完整性：翻转/删除前后对同一视图跑 `POST /v1/views/{view_id}/queries`，`total_count` 不变。

## 2026-03-11 API 补充规则（2026-08-20 全量重建实测，来源 journal/2026-08-20-notion-hub-rebuild-2026-03-11-api.md）

7. 块不可移动：PATCH /v1/blocks/{id} 只接受内容更新或 in_trash；数据库块要重排只能"快照→删块→按目标顺序重建→position 插标题"。
8. 追加块用 position 对象：PATCH /v1/blocks/{parent}/children + position:{type:'start'|'end'|'after_block', after_block:{id}}，替代已废弃的 after。
9. 建新库用 POST /v1/databases + initial_data_source.properties（POST /v1/data_sources 不能建库，报 400 要求用 Create Database API）；响应 data_sources[0].id 是数据源 id。
10. 行查询走 POST /v1/data_sources/{ds_id}/query（/v1/databases/{id}/query 在新版报 400）；行创建走 POST /v1/pages + parent.database_id（data_source_id 报 404）。
11. 删除数据库：DELETE /v1/blocks/{child_database_block_id} 会连数据源一起删（无孤儿），删前必须快照。
12. 新数据源补 relation 属性必须 PATCH /v1/data_sources/{ds_id} + relation.data_source_id；PATCH /v1/databases 补的 relation 不生效，行创建会报 property does not exist。建库时若 schema 含指向已删库的 relation 会 404，应先剥离、建完再补。
13. 浏览器自动化（Edge + cua/dom_cua）：点击只作用于活动标签页（先 tabs.new()+goto 确保目标页在前）；SPA 菜单先展开再点；playwright evaluate 沙箱只读 DOM（无 fetch/MouseEvent），跨域 iframe 用 frameLocator，需要页面内发请求时靠注入 script 标签（evaluate 内无法赋值 window/dataset）。
14. 网络：node 直连 api.notion.com 间歇 ECONNRESET，设 HTTPS_PROXY=<http://127.0.0.1:7890> 后稳定；长流水线脚本统一带 12-14 次指数退避重试，且设计成可断点续跑（先校验现状再补差）。

## 2026-08-20 我的日程置顶补充规则（来源 journal/2026-08-20-notion-schedule-top.md）

15. 删除数据库行：DELETE /v1/pages/{id} 报 invalid_request_url，必须 DELETE /v1/blocks/{row_id}（行也是块）。
16. 新库默认表格视图按"创建倒序"显示：想让默认视图按目标顺序升序展示，就按目标顺序的逆序创建行；视图排序 API 不可控。
17. 视图列顺序/列可见性 API 不可控：用浏览器 UI 调（表格工具栏「设置 → 属性是否可见」面板，拖把手重排，Esc 关闭）；拖拽落点策略：拖到"目标位置下方那行"的中心点，落点即插到该行之前。
