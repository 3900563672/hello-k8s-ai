# 沉淀「事故 vs bug」复盘——系统级故障先存档证据、后追源码路径

> 日期：2026-08-21 ｜ 关联：docs/lessons/incident-vs-bug-wsl-service-layer.md

## 为什么做

- 2026-08-21 我们亲身经历 wslservice 卡死（StopPending / 无限挂起 / 强杀崩溃），处理方式是修环境 + 固化 SOP；同一周社区用反编译源码级分析在相同区域（microsoft/WSL #41384/#41386）产出 bug 报告，当天被贴 bug。
- 对照暴露出四个偏差：问题定义（事故 vs bug）、研究侧（Linux 内核 vs Windows 服务层）、门槛误判（以为"轮不到我们"）、精力形态（顺手 vs 专项）。需要固化，避免下次事故再犯同样的"修完就收工"。

## 改成什么

1. 新增 `docs/lessons/incident-vs-bug-wsl-service-layer.md`：现象对照、四个根因偏差、下次遇到系统级故障的正确处置（先存档证据 → 修环境 → 追问"哪行代码写出来的" → 按社区模板产出）、性价比边界（只追功能正确性，不追性能权衡，不专职反编译）。

## 关键行为

- 纯文档沉淀，无代码改动；不与其他 issue 交叉引用。
- 处置流程核心：证据存档（1 分钟）在修环境之前，事后无法补。

## 验证

- `make docs-sync` / `make docs-check` 通过（文档门禁）。

## 回滚

- 删除 lesson 与 change-history 条目（git revert 本提交）。