# 变更总览：宿主工具链恢复——Codex 配置自修复、整机重启恢复顺序与圈复杂度处理

> 日期：2026-08-18 ｜ 级别：P2 ｜ 对应 Issue：无（环境与工具链坑位沉淀，无仓库代码变更）

## 为什么做

- 8/17 深夜整机重启后，Codex 桌面应用自动更新运行时，`config.toml` 中 notify 路径过期，exec 工具一度不可用（`helper_unknown_error: setup refresh had errors`），用户经其他 AI 修复后仍有残留问题；
- 8/18 凌晨恢复开发环境时，发现整套恢复顺序（Docker Desktop 引擎 → 内置 K8s 节点 → controller-manager → 端口转发）无沉淀，每次重启都要重新摸索；
- 用户询问 CI 报的 `gocyclo 圈复杂度 31 > 30` 是否代表代码改差，需要给出可核对的结论并沉淀"接近阈值函数先评估"的纪律。

## 改成什么

1. **机器侧（不入库）**：`C:\Users\hh\.codex\config.toml` 的 `[model_providers.deepseek] notify` 路径从已删除的运行时 `cua_node/2f053e67fec2d258` 修正为现有 `cua_node/1cb4becc994cbb02`（改前备份 `config.toml.bak-20260817-2359`）。
2. **知识库**：`docs/agents/KNOWN_PITFALLS.md` 新增"宿主与工具链（2026-08-18）"主题 3 条：Codex notify 路径过期、整机重启恢复顺序、gocyclo 31>30 处理。
3. **环境恢复（本次实测执行）**：Docker Desktop 引擎启动、内置 K8s 10 节点自动恢复、controller-manager `rollout restart`、`make cluster-open` 恢复 8080/18080 端口转发，全链路验收通过。

## 关键行为

- Codex 沙箱初始化窗口（约 30-60 秒）内 exec 工具报错属正常现象，等初始化完成即自愈，不要反复重启应用；
- 重启后 controller-manager 可能因 API server 未就绪启动失败（i/o timeout），用 `rollout restart` 恢复，不需要重建；
- 端口转发（port-forward）是前台进程，整机重启后必须重新 `make cluster-open`；
- 圈复杂度超阈值优先按职责提取 helper，不用 `//nolint` 掩盖。
