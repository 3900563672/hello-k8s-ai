# 变更总览：仓库健康度治理——安全政策、分支保护与依赖扫描

> 日期：2026-08-18 ｜ 级别：P2 ｜ 对应 Issue：无（用户指定的仓库体检与治理）

## 为什么做

- 用户要求对仓库做一次全面体检，补齐"重要但没搞"的 GitHub 侧设置与仓库内文件。
- 体检发现：无 LICENSE、无 SECURITY.md、main 无分支保护、Dependabot 未启用、merge commit 全开、.gitignore 有重复段落、README 无 CI 徽章。

## 改成什么

1. **GitHub 侧设置（api 完成，已生效）**
   - 关闭 merge commit，只保留 squash + rebase；开启 PR 合并后自动删除分支（delete_branch_on_merge）。
   - 新建 main 分支 ruleset：禁止 force push、禁止删除（enforcement=active）。
   - 开启 Dependabot alerts 与 automated-security-fixes（依赖漏洞扫描）。
   - 开启 Private vulnerability reporting（私密漏洞报告入口）。
2. **仓库内文件**
   - 新增 `SECURITY.md`：漏洞报告渠道、响应承诺、已知安全边界、依赖安全说明。
   - 新增 `.github/dependabot.yml`：Go / npm / GitHub Actions 每周扫描（周一凌晨，Asia/Shanghai）。
   - `hack/check-docs.py` 根目录白名单加入 `SECURITY.md`；TROUBLESHOOTING 第 18 节与 DEPLOYMENT 8.1 同步。
   - README 顶部加 3 枚 CI 状态徽章（代码检查 / 源码与部署验证 / 文档检查）。
   - `.gitignore` 清理重复段落（Archives / Backup / Logs 各两份合一）。
3. **未做（需用户决策）**
   - LICENSE：公开仓库无许可证，别人无法合法复用；待用户选择 MIT / Apache-2.0 后补充。
   - Release 版本化：项目尚无 v0.1.0，建议后续里程碑发布。
   - 分支保护强制 PR 审查：用户当前单人 + AI 直推 main 工作流，未强制。

## 关键行为

- SECURITY.md 是 GitHub 特殊文件，加入根目录白名单后 docs-check 全绿。
- Dependabot 每周一凌晨扫描依赖并开 PR（最多 5 个/生态），PR 合并走 squash。

## 验证

- `make docs-check` / `make docs-sync-check`：全绿。
- GitHub 侧逐项 `gh api` 核对：ruleset id 20962830 active；vulnerability-alerts / automated-security-fixes / private-vulnerability-reporting 均 204。
- README 徽章 URL 指向仓库自身 workflow badge，不依赖第三方服务。

## 回滚

- 仓库内：revert 本提交（SECURITY.md / dependabot.yml 删除，白名单还原，gitignore 恢复）。
- GitHub 侧：ruleset 可删除；merge 方式与 Dependabot 在仓库 Settings 改回。
