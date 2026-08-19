# 实现修改明细

## 1. API 与 CRD

### `api/v1/model_types.go`

`ModelSpec` 新增：

```text
absoluteScore: int，必填，Minimum=1
```

该字段与 GPU、最大并发、冷启动时间一起构成调度前的静态输入。Model 的 kubectl printer columns 新增 `AbsoluteScore`。

原 `ModelStatus.AbsoluteScore` 没有直接删除，而是标记为 Deprecated。它只为已存储的旧对象提供读取兼容，不再定义任何新 writer。

### `config/crd/bases/platform.study.com_models.yaml`

该文件由 `make manifests generate` 生成，变化包括：

- OpenAPI schema 新增 `spec.absoluteScore`；
- `minimum: 1`；
- 加入 Spec required 列表；
- additionalPrinterColumns 新增分数列；
- Status 字段说明改为旧版本兼容。

没有手工编辑生成清单，也没有增加新的 CRD version 或 conversion webhook。

## 2. Orchestrator 数据读取

### `internal/controller/orchestrator_data.go`

新增 `modelAbsoluteScore`：

1. `spec.absoluteScore > 0` 时直接返回 Spec；
2. Spec 为零仅表示对象来自旧 schema，此时尝试旧 `status.absoluteScore`；
3. 两者都没有有效正数时返回 0。

`collectAvailableModels` 改为调用该函数，后续的 `ModelInfo.AbsoluteScore`、decision trigger hash、effectiveScore 初始化和评分算法继续使用原有数据结构。因此没有改写评分公式，也没有改变 trigger 的组成方式。

### `internal/controller/orchestrator_controller.go` 的分数同步

`initializeEffectiveScores` 除了给新实例写初值，也会在 Model Spec 分数变化后刷新已有实例的 `status.effectiveScore`。已有副本即使已经占满节点剩余容量也能同步配置；休眠实例的首次初始化仍要求存在可行节点。这样 Dashboard 修改分数和本地演示变量覆盖不会等到下一次扩容才生效。

## 3. Orchestrator 决策与状态

### `internal/controller/orchestrator_decision.go`

新增内部 reason 常量：

```text
model_absolute_score_missing
```

`placementModelScoreState` 只分析已有 SimulatorInstance 实际引用的可用 Model：

- 全部引用模型都缺分数时，返回缺失 Model 的稳定排序列表；
- 任一实际候选模型有正分数时，不把后续失败归因于分数。

`placementUnavailableDecision` 在 `findBestPlacement` 没找到候选时区分两类结果：

| 条件 | Decision reason |
| --- | --- |
| 所有实例模型都缺有效分数 | `model_absolute_score_missing` |
| 存在有效分数，仍没有合法节点/容量 | `no_feasible_placement` |

该逻辑被拆成独立函数，保持 `DecideAt` 的复杂度在项目 lint 阈值内。

### `internal/controller/orchestrator_controller.go`

Reconcile 识别上述 reason，并把 Orchestrator Ready Condition 更新为：

```text
status=False
reason=ModelScoreMissing
message=实例引用的允许模型缺少 spec.absoluteScore: <模型列表>
```

随后按现有 10 秒周期重试，不执行扩缩容，也不把本轮标记为正常 Reconciled。正常容量不足和其他 NoOp 路径保持原行为。

## 4. Dashboard Backend

### `dashboard/backend/internal/kubernetes/commands.go`

Model 可写 Spec 白名单加入 `absoluteScore`。`validateIntent` 对 Model 增加字段存在性检查，使 Backend 在访问 Kubernetes 前就能返回明确错误：

```text
Model 缺少必填字段 spec.absoluteScore
```

数值类型与 `>=1` 仍由 CRD OpenAPI schema 作最终权威校验。Backend 没有获得 `models/status` 写权限，也没有新增绕过 Spec 的接口。

### `dashboard/backend/internal/kubernetes/commands_test.go`（新增）

覆盖：

- 带 absoluteScore 的 Model 命令可通过 Gateway 校验；
- 缺失字段被明确拒绝；
- 把 Status 冒充 Spec 字段写入仍被白名单拒绝。

## 5. Frontend

### 类型、校验和默认值

| 文件 | 修改 |
| --- | --- |
| `src/types/config.types.ts` | `ModelSpec` 新增必填 `absoluteScore: number`。 |
| `src/lib/validations/model.schema.ts` | 要求有限的正整数；模板预览增加能力基准分。 |
| `src/lib/constants/defaultValues.ts` | 新建 Model 的可见初始值为 100。 |

Frontend 的 100 只是当前 Config 创建流程使用的表单初始值，不是 CRD 默认值。API/YAML 调用方仍必须显式提交字段。

### API 映射

`src/api/endpoints/configApi.ts`：

- 读取时优先使用 `resource.spec.absoluteScore`；
- 遇到旧对象时回显旧 `status.absoluteScore`，用户下一次保存会自然迁入 Spec；
- 两处都缺失时显示 0，表单正整数校验会阻止保存，避免静默猜值；
- 创建/更新 payload 始终写 `spec.absoluteScore`。

### UI

| 文件 | 修改 |
| --- | --- |
| `src/components/features/config/forms/ModelForm.tsx` | 基础配置增加能力基准分输入和调度语义说明。 |
| `src/components/features/config/tables/ModelTable.tsx` | 列表增加“基准分”；旧缺失对象显示“待配置”。 |
| `src/components/features/config/ConfigPage.tsx` | Model 到表单值的映射保留能力基准分。 |

没有改动页面路由、状态管理架构或 Config 创建交互结构。

## 6. 部署与样例

### `config/samples/platform_v1_model.yaml`

样例 Model 直接声明 `absoluteScore: 100`，所以 `kubectl apply -k config/demo` 不再依赖后续 Status patch 才能创建有效对象。

### `hack/local-cluster.sh`

新增 `migrate_legacy_model_scores`：在新 CRD 建立后遍历 Model；若 Spec 缺失且旧 Status 是正整数，则用 merge patch 复制到 Spec。它不会覆盖已经存在的 Spec，也不会为完全缺失的模型生成默认值。

`deploy_demo` 保留 `DEMO_MODEL_ABSOLUTE_SCORE` 可配置能力，但目标改为 `spec.absoluteScore`，不再使用 Status subresource。

### `Makefile`（本次改动）

只修正变量注释以匹配当前字段所有权；变量名保持兼容。

## 7. 测试修改

| 文件 | 覆盖点 |
| --- | --- |
| `internal/controller/refactor_test.go` | Spec 优先、旧 Status 回退、完全缺失；缺分数 reason 与容量 reason 不混淆。 |
| `internal/controller/controller_integration_test.go` | TenantModelPolicy 收集路径确实读取 Spec、保留旧对象回退，并验证已有副本同步修改后的分数。 |
| `dashboard/backend/internal/kubernetes/commands_test.go` | Backend 写白名单和必要字段。 |
| `test/e2e/e2e_test.go` | CRD 拒绝缺失字段；公开 CR 创建后，Orchestrator 首次扩容、effectiveScore 和 Pod Ready 闭环。 |

## 8. 文档同步

同步修改 AI 上下文、CRD 设计、字段所有权、资源生命周期、Controller 架构、API 示例、配置参考、排障、生产检查、路线图、复盘和 whitepaper。`project-review` 保留原问题分析，并增加已处理标记和 change-history 链接。

## 9. 明确未修改

- 未新增评分 Controller、基准测试服务或后台任务；
- 未修改 `scoreModel` 公式、冷启动权重、placement 算法或 Kubernetes Scheduler 约束；
- 未修改 Simulator 引擎、Prometheus 指标、OpenTelemetry Trace 或 PostgreSQL schema；
- 未给 Backend 增加 Status 权限；
- 未删除旧 Status 字段，避免破坏滚动升级；
- 未引入新依赖、框架或数据库。

## 10. 构建与自动化校正

### `internal/controller/constants.go`

该文件在首次提交 Model 能力基准修复时被误删。`observability.go` 等文件仍引用其中的 `metricsNamespace`、`componentController`、`componentOrchestrator` 和 `metricLabelOutcome`，所以所有根模块编译入口都会报 `undefined`。

本次从删除前版本逐字节恢复原文件。恢复内容仅包含项目原有常量，没有新增业务分支、CRD 字段或运行时行为。

### Makefile（提交前验证入口）

- 新增不改写文件的 Go 格式检查；
- 增加 Backend、Frontend、E2E 编译和部署渲染的独立 target；
- 提供 `make verify` 作为提交前统一入口；
- 固定 E2E Kind 节点镜像；
- E2E 测试失败时仍执行集群清理，并优先返回原始测试错误码。

### `.github/workflows/`

在原有三个 workflow 内扩展检查，没有增加新的 workflow 文件：

| 文件 | 校正内容 |
| --- | --- |
| `test.yml` | 分离 Controller、Backend、Frontend 和部署镜像 job；检查 tidy、race、生成文件、Kustomize 和四类镜像。 |
| `lint.yml` | 增加任务超时，避免工具异常时无限占用 Runner。 |
| `test-e2e.yml` | Kind 固定为 v0.32.0，节点镜像固定为 Kubernetes v1.36.1，并增加 `always()` 清理。 |

这部分只提高故障发现和部署可重复性，没有调整 Controller、Simulator、Backend 或 Frontend 的业务契约。
