# 实现修改明细

## 1. 改动前状态

- 人类侧没有任何"如何指挥 AI"的文档：任务怎么下、怎么审核交付全凭口头经验，换人即丢。
- `docs/agents/SYNC.md` 第 6 节只有一段"可复用提示词"，没有任务解析、默认假设、转交模板与检查清单。
- 远程 AI 侧有交接格式（WORKFLOW.md 第 5 节），但没有"收到任务先做什么"的协议与可复制模板。

## 2. 修改

### 新增文件

- `docs/getting-started/AI_COLLABORATION.md`（human 层）：
  - 两类 AI 协作者能力边界表（本地 Agent vs 远程 AI）。
  - 任务五要素（目标 / 边界 / 约束 / 验收 / 交付）。
  - 五个可复制提示词模板：A 先探索先给方案、B 最小改动改代码、C 分批执行、D GitHub/看板操作、E 交给远程 AI。
  - 好例子 / 坏例子表（取自仓库真实协作历史）。
  - 交付审核清单（git log / change-history / CI / 四段式汇报 / 抽查 diff）。
  - 协作节奏建议（单 commit、先小后大、存档兜底、文档跟代码走、保留否决权）。
- `docs/agents/PROMPTING.md`（agents 层）：
  - 任务五要素解析表 + 缺项默认假设。
  - 默认假设清单七条（最小改动 / 保风格 / 先方案后动手 / 验证再交付 / 四段式汇报 / 同步义务 / 单 commit）。
  - 澄清协议（能查证的不问；三类必须问）。
  - 开工陈述模板、任务转交模板（给 Agent / 给远程 AI）、交付检查清单（8 项勾选）。
- `docs/remote-ai/PROMPTING.md`（remote-ai 层）：
  - 收到任务三步（读 CONTEXT_PACK / 读协议 / 提取五要素并标注假设）。
  - 提示词模板（人类或 Agent 转交时附带）。
  - 产出组织规则（结论先行 / 依据可回溯 / 来源分色 / 核对 FIELD_OWNERSHIP / 时间语义先读 TIME_AND_REPLAY）。
  - 交接格式与五条红线。

### 更新文件

- `docs/agents/WORKFLOW.md`：第 1 节开工加 PROMPTING 解析步骤；第 7 节汇报引用模板；时间戳更新。
- `docs/remote-ai/WORKFLOW.md`：第 1 节加 PROMPTING；第 5 节交接引用协议；时间戳更新。
- `docs/agents/README.md`：开工顺序加 PROMPTING；阅读决策表加"提示词 / 任务转交"行；时间戳更新。
- `docs/remote-ai/README.md`：开工顺序与阅读决策表加 PROMPTING；时间戳更新。
- `docs/agents/SYNC.md`：第 6 节升级为"提示词工作流"按对象取用的协议入口；时间戳更新。
- `docs/README.md`：入口加 AI_COLLABORATION；时间戳更新。
- `docs/INDEX.md`：全部文档表加 AI_COLLABORATION 行；新人路径末尾加协作入口。
- 根 `README.md`：首段加 AI 协作入口一句。

## 3. 未做

- 不重写 WORKFLOW.md 主链（7 步流程已稳定，本次只补提示词段）。
- 不修改 `hack/context-pack-template.md` 的包结构（`cp -r docs/agents docs/remote-ai` 已自动覆盖新文件）。
- 不把人类手册内容复制进 agents 层（分层原则：每层只维护自己的内容）。
