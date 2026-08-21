# 变更总览：Notion Hub 数据库内嵌化与沉淀库清理

> 日期：2026-08-20 ｜ 级别：P3

## 为什么做

- Hub 页 7 个数据库只显示标题链接，用户要求"一页看全、不二段跳转"，与学生课表模板的内嵌表格行为一致。
- 用户确认 Notion 沉淀库与仓库 docs/lessons/ 不是同一套，要求删除 Notion 侧沉淀库。

## 改成什么

1. 7 个完整页面型数据库全部 `PATCH /v1/databases/{id}` + `{"is_inline": true}` 翻转为内嵌型，Hub 页原位渲染表格，数据与顺序不变。
2. 清理实验产生的 7 个链接视图冗余容器（`in_trash: true`，回收站可恢复）。
3. 删除 Notion 沉淀库（22 条记录全部为 docs/lessons/ 镜像，仓库有底）。

## 关键行为

- 以后 Notion 内嵌展示用 is_inline 翻转，不用链接视图（避免重复容器）；删除数据库对象走 `in_trash` 而非 block `archived`（400）。
- 新版 API 查库内容路径：views → view queries → 逐页 pages。

## 验证

- Edge DOM 断言：`.notion-collection_view_page-block` 从 7 降到 0；6 张内嵌表（沉淀库删除后）全部渲染数据。
- API 断言：7 个库 `is_inline == true`；删除对象 `in_trash == true`。

## 回滚

- Notion 侧：回收站恢复被删对象，is_inline 翻回 false 即恢复原样；仓库侧：revert 本条目相关文件即可。
