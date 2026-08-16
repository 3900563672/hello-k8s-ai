# 升级与回滚

## 升级

- 无运行组件升级：本次只改文档、脚本与 Makefile targets。
- 用户侧动作：重新拉取 main 后，`make context-pack` 即可生成上下文包；`make docs-check` 可随时校验链接。
- 已生成的 `.runtime/context-pack/` 是本地产物，删除或重新生成即可，不影响仓库。

## 回滚

| 内容 | 回滚方式 |
| --- | --- |
| 文档分层 | 删除 `docs/agents/`、`docs/remote-ai/`，恢复 `docs/AI_CONTEXT.md` 旧内容（git 历史可查），还原 `AGENTS.md`/`docs/README.md`/`docs/INDEX.md` |
| Makefile targets | 移除 `context-pack`、`docs-check` 两个 target |
| hack 脚本 | 删除 `hack/gen-context-pack.sh`、`hack/check-docs.py`、`hack/context-pack-template.md` |
| change-history 纳入 git | 保留无害；如要恢复，可 `git rm --cached` 未跟踪过的三个旧目录 |

## 风险

- `docs/AI_CONTEXT.md` 被外部引用改为薄入口后，依赖旧章节编号的读者需按新入口导航；所有旧链接仍指向该文件，不会 404。
- 上下文包包含源码与文档快照，体积约 13MB（不含 .git、构建产物、大文件），发送前请确认接收方容量。