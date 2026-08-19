# 实现修改明细

## 文件清单

### 新增

| 路径 | 说明 |
| --- | --- |
| `docs/agents/SYNC.md` | 同步协议：维护边界、触发条件、同步步骤、时间戳规则、远程 AI 义务、可复用提示词 |

### 修改

| 路径 | 说明 |
| --- | --- |
| `docs/agents/README.md` | 头部元数据；新增"维护边界"节；阅读决策改为"默认读源码，人类文档仅按需背景" |
| `docs/agents/WORKFLOW.md` | 头部元数据；第 3 步默认读源码；第 6 步改为"归档与同步"并链接 SYNC.md |
| `docs/agents/KNOWN_PITFALLS.md` | 头部元数据 |
| `docs/agents/PRINCIPLES.md` | 头部元数据 |
| `docs/remote-ai/README.md` | 头部元数据；开工顺序明确"包默认不含人类文档"；阅读决策改为源码与 change-history |
| `docs/remote-ai/WORKFLOW.md` | 头部元数据；新增"版本与同步义务"节 |
| `docs/README.md` | 头部元数据；声明为纯人类文档，AI 默认不读 |
| `AGENTS.md` | 新增"文档维护边界"小节 |
| `hack/context-pack-template.md` | 新增"包内容与阅读策略"节（含 __MODE__ 占位符），章节重排 |
| `hack/gen-context-pack.sh` | 支持 `FULL=1`；默认只复制 docs/agents 与 docs/remote-ai；改为从 PKG 目录整体打包（源码 + 文档 + 时间线），tar 天然排除未复制的构建产物 |
| `change-history/README.md` | 索引表追加本次条目 |

## 设计要点

- 阅读路径隔离：人类读 `docs/`，Agent 读 `docs/agents/` + 源码，远程 AI 读 `CONTEXT_PACK.md` + `docs/remote-ai/` + 包内源码；三者默认不交叉。
- 事实源不变：源码、生成清单、可执行测试；`docs/` 人类专题不再是任何 AI 的默认输入。
- 唯一共享时间线：`change-history/`；所有层通过 SYNC 协议挂到时间线上。
- Token 优化：默认包不再携带约 5800 行人类专题，体积从约 13MB 降至约 1.4MB。
- 人类文档保护：Agent 不擅自改 `docs/`，只通过"待同步清单"提醒用户。
