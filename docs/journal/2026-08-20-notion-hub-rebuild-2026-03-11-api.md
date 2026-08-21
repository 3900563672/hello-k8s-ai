# Notion Hub 全量重建：2026-03-11 API 移动块不可行，改用"快照→重建→position 插标题"

> 日期：2026-08-20 ｜ 触发者：本地 Agent ｜ 相关：change-history/2026-08-20-notion-hub-rebuild-data-pipeline/

## 现象

- Hub 页 11 个数据库需要重排（成绩/考试/通知/校历/体测/课程表 归位到学习管理分区），API 无法移动 child_database 块。
- PATCH /v1/blocks/{id} 传 position/after 均 400；官方 OpenAPI（makenotion/notion-mcp-server 的 scripts/notion-openapi.json）确认块无 move 端点，只有 POST /v1/pages/{page_id}/move。
- 建库时若 schema 含 relation 指向已删除的旧库，POST /v1/databases 直接 404（Could not find data_source）。

## 上下文

- 用户在 Notion 搭"学习与开源中枢"：一页看全 12 个库（课表/课程/作业/学习时段/成绩/考试/通知/校历/体测/证明材料/开源项目/开源任务），数据来自东北大学教务系统、体测预约系统。

## 处理

1. 全量快照：11 库 ×（GET /v1/databases/{id} 拿 data_sources[0].id → GET /v1/data_sources/{ds_id} 拿 properties → POST /v1/data_sources/{ds_id}/query 分页拿全部行），落盘 JSON。
2. 全量重建：DELETE 全部块（删 child_database 块会连数据源一起删，无孤儿）→ 按目标顺序 POST /v1/databases（initial_data_source.properties）→ POST /v1/pages（parent database_id）回填行 → position:{type:'start'} 插顶部区块 → 逐个 position:{type:'after_block'} 在库间插标题。
3. relation 处理：建库时剥离 relation/rollup 属性；全部库建完后用 PATCH /v1/data_sources/{ds_id} + relation.data_source_id 补回（PATCH /v1/databases 补的 relation 对新数据源无效，行校验报 property does not exist）。
4. 行级 relation 映射：开源项目行先建，旧 id→新 id 映射后建开源任务行。
5. 教务系统补充抓取：体测预约（12.2-12.5 南湖补测、未预约、需填手机号后选时段）；证明打印 5 种（中/英文学籍证明、中/英文成绩单、均分证明，每学期上限 50）→ 新增"证明材料"库 5 行、体测库补 1 行预约记录。

## 阻塞与限制

- 证明 PDF 预览页（printZm.do）用老旧 embed application/pdf，Edge 不渲染（about:blank），自动化拿不到证书文件；打印会计入每学期限额，未代打。
- 秋季学期（2026-2027-1）课表/成绩尚未发布（系统仍显示 2025-2026 春 第25周）。
- 浏览器自动化：cua 点击只作用于活动标签页；SPA 侧栏需先展开目录再点子项；playwright evaluate 沙箱无 fetch/MouseEvent/NodeFilter（只读 DOM），跨域 iframe 内容要走 frameLocator。

promoted: lessons/process-notion-api-first-automation.md
