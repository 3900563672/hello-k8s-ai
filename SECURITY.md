# 安全政策

## 报告漏洞

- **公开问题**：非敏感问题请在 [Issues](https://github.com/3900563672/hello-k8s-ai/issues) 提出。
- **私密报告**：使用 GitHub 的 Private vulnerability reporting（仓库 → Security → Report a vulnerability）。
- 请勿在公开渠道（Issue / PR / 讨论）发布未修复漏洞的完整利用细节。

## 响应承诺

- 确认收到：3 个工作日内回复。
- 修复时间：P0/P1 高危漏洞目标 7 天内给出修复计划；低危漏洞随版本迭代修复。

## 安全边界（本仓库已知）

- 本项目面向受控本机开发环境（Docker Desktop），默认不暴露公网。
- 未配置 OIDC、用户级授权、TLS、完整 NetworkPolicy、备份或 HA；不要把本地一键成功写成生产就绪。
- Dashboard Backend 写接口只修改白名单 CR 的 Spec；遥测失败不会阻止控制面启动。

## 依赖安全

- Dependabot alerts 与自动安全修复已启用；Go / npm / GitHub Actions 依赖每周扫描，更新以 PR 形式提出。
