# YAML 与模板

> 日期：2026-08-16 ｜ 触发者：本地 Agent ｜ 相关：原 docs/agents/KNOWN_PITFALLS.md 归档（2026-08-18 文档体系重构拆分为 journal/lessons）

### 2026-08-16 issue form description 中 `bug:` 冒号被 YAML 解析
- 现象：`yaml.safe_load` 报 "mapping values are not allowed here"。
- 原因：plain scalar 里 `bug:` 被当成嵌套 mapping。
- 解决：description 整行加双引号包裹。
- 验证：3 个 issue 模板 YAML 校验通过。
