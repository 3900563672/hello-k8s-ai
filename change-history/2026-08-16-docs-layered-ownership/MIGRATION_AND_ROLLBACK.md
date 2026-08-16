# 升级与回滚

## 升级

- 无运行组件升级：本次只改文档、AGENTS.md 与脚本。
- 用户侧动作：拉取 main 后，`make context-pack` 生成默认包（不含人类专题）；需要人类专题时 `make context-pack FULL=1`。
- 已生成的 `.runtime/context-pack/` 是本地产物，删除或重新生成即可。

## 回滚

| 内容 | 回滚方式 |
| --- | --- |
| SYNC.md 与各层元数据 | 删除 `docs/agents/SYNC.md`，移除各文档头部元数据行 |
| 维护边界声明 | 还原 `AGENTS.md`、`docs/README.md`、`docs/agents/README.md`、`docs/remote-ai/` 相关段落（git 历史可查） |
| 打包脚本 | 还原 `hack/gen-context-pack.sh` 与 `hack/context-pack-template.md` |

## 风险

- 默认包不含人类专题：需要背景阅读的远程 AI 任务需改用 FULL 包，否则背景信息不足。
- Agent 默认不读 `docs/`：对依赖人类文档背景的任务，Agent 仍需按需阅读；工作流已写明。