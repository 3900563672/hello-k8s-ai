# 远程 AI 工作手册（docs/remote-ai/）

> 维护层：remote ｜ last-reviewed：2026-08-18 ｜ 事实源：docs/MAP.yaml、源码、change-history/

> 本目录给**只在自己工作区工作**的 AI（如网页版 ChatGPT / Claude）：它只能读取用户发来的打包内容，不能访问用户电脑、仓库或 GitHub。
> 能操作本机仓库的 Agent 见 [docs/agents/README.md](../agents/README.md)；人类入口是根目录 [README.md](../../README.md)。

## 你是谁

- **你能**：读取打包内容里的全部文件（源码、docs、change-history、清单）；在自己工作区分析、规划、写代码、写文档；按约定格式产出交付物。
- **你不能**：访问用户的 GitHub、Kubernetes 集群、数据库；运行用户的构建/测试；修改用户电脑上的任何文件。
- 你的产出必须由用户带回，再由本地 Agent 落地。因此"标注真实状态"比"看起来完整"更重要。

## 开工顺序（每次任务都走）

1. 先读打包根目录的 `CONTEXT_PACK.md`（包的地图与生成时间）。
2. 再读 `llms.txt`（本文档索引），按任务定位专题文档。
3. 默认事实源是包内**源码、生成清单与 `change-history/`**；`docs/` 人类文档只作背景，不据此写代码。

## 产出规则

- **结论先行**：第一段给出结论与推荐做法，再给依据与细节；依据引用包内文件路径（`internal/controller/...`），不写"文档说"。
- **来源分色**：源码与 `change-history/` 是事实；`docs/` 只是背景；推断必须显式标注。
- **未验证写"未验证（原因）"**：你无法运行任何东西，不写"已测试 / 已部署 / 已验证"。
- CRD/API 结论必须核对 `docs/kubernetes/FIELD_OWNERSHIP.md`；时间、倍速、历史语义先读 `docs/data-flow/TIME_AND_REPLAY.md`。
- 发现文档与源码不一致时，作为交付物列出差异，不静默按文档写代码。

## 交接格式

```text
标题：<任务名>（<日期>）
结论：<一两句话>
依据：<包内文件路径列表>
交付物：<报告 / diff / 文档片段>
未验证：<逐项列出与原因>
给 Agent 的落地建议：<本地 Agent 执行时的注意点>
```

## 包是怎么来的

- 用户（或本地 Agent）执行 `make context-pack` 生成：`CONTEXT_PACK.md` + 全量 `docs/` + 源码副本 + `hello-k8s-ai-context-pack.tar.gz`，输出在 `.runtime/context-pack/`；`FULL=0` 生成精简包（仅 `docs/agents/` 与 `docs/remote-ai/`）。
- 包不是实时仓库：一切以 `CONTEXT_PACK.md` 的生成日期与最近提交为准，不臆测更新。
