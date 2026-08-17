# 蒸馏知识（lessons）

> 维护层：agent ｜ 适用读者：本地 Agent 与远程 AI ｜ 与 [docs/journal/](../journal/README.md) 配套

## 这是什么

从 journal 流水账中**蒸馏出的可复用规则**：现象 → 根因 → 规则 → 验证。与 journal 的区别：

| | journal | lessons |
| --- | --- | --- |
| 门槛 | 踩坑即记，3-5 行 | 定期提升，结构完整 |
| 内容 | 一次性的上下文与处理 | 可复用的规则与验证方法 |
| 终点 | 可能过时、可重复 | 规则应沉淀为脚本 / E2E 断言（坑的终点是自动化） |

## 提升流程（每攒约 20 条 journal 或每周一次）

1. 扫描 `docs/journal/`，归类、合并重复条目。
2. 能提炼出"一句话规则 + 可验证方法"的，写成 `<主题>-<slug>.md`（模板见下）；主题前缀：`api` / `controller` / `simulator` / `observability` / `dashboard` / `deploy` / `process`。
3. 验证方法能自动化的，补到 `make selfcheck`、preflight 或 E2E（优先于文档）。
4. 在原 journal 条目标注 `promoted: lessons/<文件>`。
5. 无法提炼的保持 journal 原样，不强行提升。

## 模板

```markdown
# <一句话规则或主题>

> 提升日期：YYYY-MM-DD ｜ 来源：journal/<原条目> ｜ 适用对象：本地 Agent / 远程 AI

## 现象
## 根因
## 可复用规则（一条规则一句话，禁止复述现象）
## 验证方法（命令 / 断言 / E2E；能自动化的给脚本路径）
```
