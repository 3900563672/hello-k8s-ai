# Orchestrator maxReplicas 支持“无限制”（0 = 不限制副本数）

- 变更日期：2026-08-17（Asia/Shanghai 08:35；UTC 2026-08-17 00:35）
- 关联问题：无（用户直接指令；CRD 契约变化，按 WORKFLOW 本应建 design issue，本次为最小指令变更，未建）
- 变更级别：P2 策略配置能力
- 变更范围：CRD/API、Controller、Frontend、样例与人类文档
- CRD 变化：`Orchestrator.spec.maxReplicas` 由“必填正整数”改为“必填非负整数，0 = 不限制”
- 数据库变化：无

## 1. 完成结果

- `maxReplicas: 0` 表示不限制副本数：Orchestrator 决策在 0 时不触发 `maximum_replicas` 封顶，扩容持续到节点/模型容量上限为止；决策与副本地板逻辑原本就带 `> 0` 判断，语义天然兼容，改动集中在契约与校验。
- 前端默认值 `DEFAULT_ORCHESTRATOR` 与“核心租户编排策略”预置模板改为 0；表单加说明、校验允许 0、预览与表格显示 `∞`，“配置详解”页同步写明 0 = 无限制。
- 背景：模拟器不是网关，没有流量拦截层，理论上应接受任意 QPS；此前“停在 10 副本”是 `maxReplicas: 10` 的策略配置，不是引擎能力限制。

## 2. 关键行为

- 决策：`orchestrator_decision.go` 的 `maximum_replicas` 检查仅在 `MaxReplicas > 0` 时生效（原有逻辑，无需改）。
- 校验：CRD `minimum` 1 → 0，CEL 放宽为 `self.maxReplicas == 0 || self.minReplicas <= self.maxReplicas`；Controller 组装输入仅拒绝负数。
- 前端：`orchestrator.schema.ts` 允许 0 且跨字段校验跳过 0 场景；表格与预览把 0 显示为 `∞`；GuidePage“配置详解”明确说明。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| `api/v1/orchestrator_types.go` | MaxReplicas 注释、Minimum=0、CEL 规则 |
| `config/crd/bases/platform.study.com_orchestrators.yaml` | 生成物同步 |
| `internal/controller/orchestrator_data.go` | 仅拒绝负数，0 合法 |
| `internal/controller/refactor_test.go` | 新增 maxReplicas=0 继续扩容用例 |
| `dashboard/frontend/my-app` | 校验、表单说明、表格/预览 ∞、默认值与模板、配置详解 |
| `config/samples`、`docs/kubernetes/CRD_DESIGN.md`、`docs/reference/API_EXAMPLES.md` | 同步 0 = 无限制 |

## 3.5 部署事故记录（已修复）

- `make deploy`（config/default）覆盖 controller 时丢掉 dev 栈的 `SIMULATOR_IMAGE` env，controller 回落默认 `simulator:latest`（本地过期镜像，无 9090 端点），导致模拟器新副本 CrashLoop（探针 refused + 29s 优雅退出循环）。
- 已用 `kubectl kustomize config/dev | kubectl apply -f -` 恢复 env 并重启 controller；模拟器 Deployment 模板被纠正回 `hello-k8s-ai-simulator:dev`，CrashLoop RS 缩到 0，实例恢复 Running 并继续扩缩（实测 REPLICAS 10→12）。
- 坑位已沉淀：`docs/agents/KNOWN_PITFALLS.md`「集群操作与部署」新增条目。

## 4. 未验证 / 风险

- 未对运行集群做“扩容超过 10 副本”的真实压测；真实上限由节点并发容量决定（model-lite 每副本 16 并发，2 个 desktop-worker 节点）。
- 集群内已有 `orch-core` 仍是 `maxReplicas: 10`，需手动改为 0 才生效（见 MIGRATION_AND_ROLLBACK.md）。
- 无背压设计保留：队列只积压不拒绝，属已知设计缺口，本次不处理。
