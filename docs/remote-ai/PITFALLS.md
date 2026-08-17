# 远程 AI 踩坑速查

> 维护层：remote ｜ last-reviewed：2026-08-18 ｜ 适用：产出前扫一遍；包内 `docs/lessons/` 含全量蒸馏条目

## 一、环境与身份（最容易犯）

| 坑 | 正确姿势 |
| --- | --- |
| 以为能访问 GitHub/集群/数据库 | 你只能读包；一切外部状态写"未验证（无访问）" |
| 把包当实时仓库 | 以 `CONTEXT_PACK.md` 生成时间与最近提交为准；文档/源码不一致时列差异 |
| 写"已测试/已验证" | 你没运行过任何东西；只能写"建议验证命令" |
| 假设包里有最新 issue | 包内 Open Issues 是生成时刻快照；缺失时说明无法读取 |

## 二、架构语义（写结论前必读）

| 坑 | 正确姿势 |
| --- | --- |
| 把 PostgreSQL 当配置源 | Kubernetes API Server 是唯一事实源；PostgreSQL 只存历史快照、事件、审计、幂等记录 |
| 认为 Frontend 直连 Prometheus/Jaeger | Frontend 只调用 Dashboard Backend；外部系统只被 Backend 访问 |
| 混淆当前态/历史快照/指标/Trace | 四类来源与时间语义不同；先读 `docs/data-flow/TIME_AND_REPLAY.md` |
| 把倍速当成全局时间加速 | `SimulationClock` 只加速 Simulator 离散事件引擎；Controller 冷却、数据新鲜度、历史时间不变 |
| 忽略字段所有权 | 谁写 Spec、谁写 Status 有硬约束（`docs/kubernetes/FIELD_OWNERSHIP.md`）；方案不能越权 |
| 默认"没有 Allow 就是允许" | 策略语义是显式 Allow、Deny wins；没有 Allow 默认不是允许 |

## 三、方案与代码（交付前自检）

| 坑 | 正确姿势 |
| --- | --- |
| 大改重写 | 仓库风格是分层、幂等、最小改动；先复用既有 helper/模式 |
| 动生成文件 | `config/crd/bases/*.yaml`、`config/rbac/role.yaml`、`zz_generated` 由工具生成，方案不得手改 |
| 默认值自行发明 | 默认值/常量以包内源码为准；归属（用户可配置/系统常量/开发测试）见前端 `/guide` 常量 |
| 文档写"已实现" | 只有源码路径存在才算已实现；"清单声明" ≠ 集群 Ready |
| 建议绕过校验/权限 | 遇到 forbidden/校验失败应修正配置或改 allowlist，不绕过 |

## 四、仓库既有 lessons（重点引用）

包内含 `docs/lessons/` 全量条目；按任务类型优先引用：

- `deploy-docker-data-junction` / `deploy-docker-desktop-k8s-recovery`：部署类方案先读，避免建议重置集群
- `observability-prom-memory-alert` / `observability-prom-restart-alert`：写告警/指标方案必读
- `simulator-scale-node-capacity`：副本/容量类方案必读（`maxReplicas=0` 不设上限语义）
- `api-domain-misjudgments`：涉及 API/CRD 语义必读
- `process-wsl-powershell-quoting` / `process-ci-poll-30s` / `process-host-sleep-freeze`：涉及落地执行建议必读（本地 Agent 会执行你的建议）

## 五、时间与提交

- 时间戳一律用 UTC 或带时区；仓库变更归档目录格式 `YYYY-MM-DD-*`。
- 提交信息风格：`feat:` / `fix:` / `docs:` / `refactor:` + 中文 + 可选 `Fixes #N`；远程 AI 不提交，只给建议。
