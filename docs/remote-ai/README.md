# 远程 AI 工作手册（docs/remote-ai/）

> 维护层：remote-ai ｜ 最后同步：2026-08-16 ｜ 对应变更：change-history/2026-08-16-prompting-workflows/
> 本目录给**只在自己工作区工作**的 AI（如网页版 ChatGPT / Claude，用 5.6 SOL 等模型）：它只能读取用户发来的打包内容，不能访问用户电脑、仓库或 GitHub。
> 能操作本机仓库的 Agent 见 [docs/agents/](../agents/README.md)；人类入口是根目录 [README.md](../../README.md)。

## 你是谁

- **你能**：读取打包内容里的全部文件（源码、docs、change-history、清单）；在自己工作区分析、规划、写代码、写文档；按约定格式产出交付物。
- **你不能**：访问用户的 GitHub、Kubernetes 集群、数据库；运行用户的构建/测试；修改用户电脑上的任何文件。
- 你的产出必须由用户带回，再由本地 Agent 落地。因此"标注真实状态"比"看起来完整"更重要。

## 开工顺序（每次任务都走）

1. 先读打包根目录的 `CONTEXT_PACK.md`（包的地图与当前状态，含生成日期）。
2. 再读本手册、[PROMPTING.md](PROMPTING.md) 与 [WORKFLOW.md](WORKFLOW.md)。
3. 默认事实源是包内**源码、生成清单与 `change-history/`**；包默认不含人类文档（`docs/` 专题），需要时请用户提供 FULL 包（`make context-pack FULL=1`）。
4. 产出按 WORKFLOW.md 的交付格式，明确标注"推断 / 未验证"。

## 阅读决策

| 任务 | 必读 |
| --- | --- |
| 理解项目 / 架构 | CONTEXT_PACK.md、PROJECT_OVERVIEW_NEW.md（背景一页） |
| 写代码 / 审查代码 | CONTEXT_PACK.md、docs/agents/PRINCIPLES.md、PROMPTING.md、相关源码 |
| 写文档 / 设计方案 | CONTEXT_PACK.md、docs/agents/WORKFLOW.md、docs/agents/SYNC.md |
| 排查行为问题 | CONTEXT_PACK.md、相关源码、change-history/ |

## 包是怎么来的

- 用户（或本地 Agent）执行 `make context-pack` 生成：`CONTEXT_PACK.md` + 关键文件副本 + `hello-k8s-ai-context-pack.tar.gz`，输出在 `.runtime/context-pack/`。
- 生成脚本是 `hack/gen-context-pack.sh`，模板是 `hack/context-pack-template.md`。
- 包不是实时仓库：一切以 `CONTEXT_PACK.md` 的生成日期与最近提交为准，不臆测更新。

## 反馈回路

- 你在文档、工作流、踩坑上的建议写成交付物（见 WORKFLOW.md 交接格式），用户带回后由本地 Agent 更新本目录或 `docs/agents/`。
- 本目录与 `docs/agents/` 都允许远程 AI 提出修订，但落地必须由本地 Agent 执行并提交。