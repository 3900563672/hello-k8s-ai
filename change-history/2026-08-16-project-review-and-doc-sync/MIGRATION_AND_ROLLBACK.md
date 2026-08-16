# 升级与回滚

## 1. 迁移

- 无代码、CRD、数据库变化，不存在数据迁移。
- `project-review/` 从忽略变为跟踪是一次性变更；提交后 GitHub 上即出现 10 条审查记录。

## 2. 回滚

- 文档回滚：`git revert <本提交>` 恢复 README / AGENTS.md / docs/agents/ 与 change-history。
- project-review 回滚：在 `.gitignore` 恢复 `/project-review/`，并从 Git 索引移除（`git rm -r --cached project-review/`）后提交。

## 3. 风险与注意

- `project-review/` 内容为 2026-08-13 审查基线，进入版本控制后会被 Git 历史保留；如需更新审查结论，通过新增条目或修订提交完成，不删除历史。
- 文档漂移硬约束会略微增加每次交付的改动面，但避免"代码新、文档旧"的长期负债。
