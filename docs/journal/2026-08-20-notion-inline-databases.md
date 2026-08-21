> 日期：2026-08-20 ｜ 触发者：本地 Agent ｜ 相关：Notion 学习与开源中枢

## 现象

Hub 页 7 个数据库块只渲染标题链接，要点开全屏页才能看数据；导入的学生课表模板却能内嵌渲染整张表。

## 上下文

用户在 Notion 搭"学习与开源中枢"，要求一页看全、不二段跳转。此前模型建的是完整页面型数据库（`is_inline=false`），嵌在父页面就是链接；模板的 Class Schedule 是内嵌型（`is_inline=true`）。

## 处理

1. 试建链接视图（`POST /v1/views` + `create_database`）验证内嵌可行，但会产生独立容器块，与源库形成重复数据视图。
2. 找到更优解：`PATCH /v1/databases/{id}` + `{"is_inline": true}` 一次翻转 7 个库，原位内嵌、数据不动、顺序不变。
3. 清理冗余容器：`PATCH /v1/databases/{id}` + `{"in_trash": true}`（对 `child_database` 块用 `archived` 会 400）。
4. 应要求删除 Notion 沉淀库：22 条记录全部是 docs/lessons/ 的镜像（每条带来源路径），仓库有底，删后回收站可恢复。
5. 新版 API（Notion-Version 2026-03-11）查库内容走 `POST /v1/views/{view_id}/queries`，返回体只含 page id，属性需逐页 `GET /v1/pages/{id}`。

promoted: lessons/process-notion-api-first-automation.md
