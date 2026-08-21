# 蒸馏知识（lessons）

> 维护层：agent ｜ 适用读者：本地 Agent 与远程 AI ｜ 与 [docs/journal/](../journal/README.md) 配套

## 这是什么

从 journal 流水账中**蒸馏出的可复用规则库**：现象 → 根因 → 规则 → 验证。采用 **Gutenberg `.agents/skills/` 触发式设计**：每条 lesson 带 `Use when` 触发条件，开工时按任务类型匹配，命中才读正文——不靠"全部扫一遍"，靠"按需命中"。

| | journal | lessons |
| --- | --- | --- |
| 门槛 | 踩坑即记，3-5 行 | 定期提升，结构完整 |
| 内容 | 一次性的上下文与处理 | 可复用的规则与验证方法 |
| 触发 | 按时间翻 | **Use when 触发条件匹配（按任务类型 / 涉及路径）** |
| 终点 | 可能过时、可重复 | 规则应沉淀为脚本 / E2E 断言（坑的终点是自动化） |

## 与失败模式注册表的分工

- [docs/agents/FAILURE_REGISTRY.md](../agents/FAILURE_REGISTRY.md)：**已登记失败模式**（现象 / 触发条件 / 根因 / 必须动作 / 证据链 / 状态），开工**强制扫末尾 3 条**。
- 本目录：**可复用规则库**，按任务类型匹配触发条件，命中先读。
- 合流方式：两者都要求写"触发条件"；从 journal 提升 lesson 时，若现象已是"第二次出现"，同步登记进 FAILURE_REGISTRY。

## Use when 触发条件速查（开工按任务类型匹配）

| 任务类型 / 现象 | 命中先读 |
| --- | --- |
| 涉及 CR 语义判断（策略 Status / replicas / 倍速 / SSE） | [api-domain-misjudgments.md](api-domain-misjudgments.md) |
| 长时运行结束 / 停止负载 / cluster-down | [deploy-cluster-down-revive.md](deploy-cluster-down-revive.md) |
| Docker 数据盘迁移 / docker 显示为空 | [deploy-docker-data-junction.md](deploy-docker-data-junction.md) |
| Docker Desktop / 整机重启后恢复内置 K8s | [deploy-docker-desktop-k8s-recovery.md](deploy-docker-desktop-k8s-recovery.md) |
| 任何涉及 WSL 整体重启 / 网络重置 | [deploy-no-wsl-shutdown.md](deploy-no-wsl-shutdown.md) |
| kind PVC 异常 / Permission denied / tmpfs 覆盖 | [kind-hostpath-docker-desktop-rootfs.md](kind-hostpath-docker-desktop-rootfs.md) |
| Grafana 嵌入 / 反代 / WS 400 / 面板 404 | [observability-grafana-embed.md](observability-grafana-embed.md) |
| Prometheus 内存告警 / 新增容器 limit | [observability-prom-memory-alert.md](observability-prom-memory-alert.md) |
| 重启 / 存活告警 / 扩容误报 | [observability-prom-restart-alert.md](observability-prom-restart-alert.md) |
| 升级单副本 + RWO PVC 组件（Prometheus / Jaeger） | [observability-pvc-single-replica.md](observability-pvc-single-replica.md) |
| 中文 commit / issue 正文 | [process-chinese-commit-file.md](process-chinese-commit-file.md) |
| 推送后等待 CI | [process-ci-poll-30s.md](process-ci-poll-30s.md) |
| Windows 侧写仓库文件 / mode change / docs-sync-check 失败 | [process-cross-platform-file-hygiene.md](process-cross-platform-file-hygiene.md) |
| gh api 传文件正文 / GitHub 写操作回读 | [process-gh-api-file-body.md](process-gh-api-file-body.md) |
| 无人值守 / 夜间长跑 / 跨宿主机睡眠 | [process-host-sleep-freeze.md](process-host-sleep-freeze.md) |
| 脚本读 kubectl 大输出（100+ Pod） | [process-kubectl-enobufs.md](process-kubectl-enobufs.md) |
| 任何 >30s 阻塞等待 | [process-no-idle-wait.md](process-no-idle-wait.md) |
| 本地连 127.0.0.1 新端口被拒 / 回环排查 | [process-wsl-loopback-fresh-listen-refused.md](process-wsl-loopback-fresh-listen-refused.md) |
| WSL 内 GitHub 慢/断连（clone/push/PR） | [process-wsl-github-proxy.md](process-wsl-github-proxy.md) |
| PowerShell 直传 wsl 复杂命令 | [process-wsl-powershell-quoting.md](process-wsl-powershell-quoting.md) |
| 扩容停在节点容量上限 | [simulator-scale-node-capacity.md](simulator-scale-node-capacity.md) |
| 对外提交 / 开源贡献（AI 产出自查与披露） | [process-ai-collaboration-disclosure.md](process-ai-collaboration-disclosure.md) |
| 浏览器 UI 自动化 / 同一 UI 动作连续失败 / 全自动 vs 人工决策 | [process-human-agent-division-of-labor.md](process-human-agent-division-of-labor.md) |

## 提升流程（每攒约 20 条 journal 或每周一次）

1. 扫描 `docs/journal/`，归类、合并重复条目。
2. 能提炼出"一句话规则 + 可验证方法"的，写成 `<主题>-<slug>.md`（模板见下）；主题前缀：`api` / `controller` / `simulator` / `observability` / `dashboard` / `deploy` / `process`。
3. **每条必须有"触发条件（Use when）"**：写清"什么任务类型 / 涉及路径 / 现象关键词命中才读"；写不出触发条件的说明规则不可复用，回到 journal 不提升。
4. 验证方法能自动化的，补到 `make selfcheck`、preflight 或 E2E（优先于文档）。
5. 在原 journal 条目标注 `promoted: lessons/<文件>`。
6. 无法提炼的保持 journal 原样，不强行提升。

## 模板

```markdown
# <一句话规则或主题>

> 提升日期：YYYY-MM-DD ｜ 来源：journal/<原条目> ｜ 适用对象：本地 Agent / 远程 AI
> 触发条件（Use when）：<任务类型 / 涉及路径 / 现象关键词；命中才读>

## 现象
## 根因
## 可复用规则（一条规则一句话，禁止复述现象）
## 验证方法（命令 / 断言 / E2E；能自动化的给脚本路径）
```
