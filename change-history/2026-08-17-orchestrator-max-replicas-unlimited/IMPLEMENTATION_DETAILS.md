# 实现修改明细

## 1. API 与 CRD

### `api/v1/orchestrator_types.go`

- `MaxReplicas` 注释改为“必填；0 表示不限制（模拟器无网关，接受任意 QPS，扩到容量上限为止）”。
- `+kubebuilder:validation:Minimum` 1 → 0。
- CEL 规则从 `self.minReplicas <= self.maxReplicas` 放宽为 `self.maxReplicas == 0 || self.minReplicas <= self.maxReplicas`，错误消息同步说明 0 = unlimited。
- 仍保持 `+required`：字段必填，0 是合法值。

### 生成物

`make manifests` 重新生成 `config/crd/bases/platform.study.com_orchestrators.yaml`（description、minimum、CEL 规则）。`make generate` 无 DeepCopy 变化（字段类型未变）。

## 2. Controller

### `internal/controller/orchestrator_data.go`

- 组装 `DecisionInput` 时，`MaxReplicas <= 0` 报错改为 `MaxReplicas < 0` 才报错；0 表示不限制，通过。
- 错误消息改为 `maxReplicas must be non-negative (0 means unlimited)`。

### 决策层（未改动，语义天然兼容）

- `orchestrator_decision.go` `maximum_replicas` 检查：`if input.MaxReplicas > 0 && totalReplicas >= input.MaxReplicas`。
- `desiredReplicaFloor`：`if input.MaxReplicas > 0 { floor = min(floor, input.MaxReplicas) }`。
- 0 时扩容持续到 `findBestPlacement` 找不到可行 (模型, 节点) 组合为止（返回 `no_feasible_placement`），即“扩到容量上限”。

## 3. 测试

### `internal/controller/refactor_test.go`

`TestDecideAtSupportsScaleToZeroAndMaximum` 追加用例：`MaxReplicas: 0`、已有 10 副本、队列压力 200 > 阈值 100、模型/节点容量充足 → 期望 `ScaleUp`（而不是 `NoOp maximum_replicas`）。

## 4. Frontend

- `orchestrator.schema.ts`：`maxReplicas` 校验 `positiveInt` → `nonNegativeInt`；跨字段比较加 `maxReplicas !== 0` 例外；预览“副本范围”在 0 时显示 `∞`。
- `OrchestratorForm.tsx`：`maxReplicas` 输入框 `min="0"`，加 description“填 0 表示不限制副本数（模拟器无网关，接受任意 QPS，扩到容量上限为止）”。
- `OrchestratorTable.tsx`：副本范围在 0 时显示 `∞`。
- `defaultValues.ts`：`DEFAULT_ORCHESTRATOR.maxReplicas` 10 → 0。
- `presetTemplates.ts`：“核心租户编排策略” `maxReplicas` 10 → 0（注释说明）；“弹性扩缩策略”20、“保守稳定策略”4 保留。
- `GuidePage.tsx`：“配置详解”`minReplicas / maxReplicas` 行默认值改为 `1..∞`，说明 0 = 不限制；预置模板摘要同样把 0 显示为 `∞`。

## 5. 样例与人类文档

- `config/samples/platform_v1_orchestrator.yaml`：`maxReplicas: 0` + 注释。
- `docs/kubernetes/CRD_DESIGN.md`：`maxReplicas` 行改为“0 或正整数，0 = 不限制（扩到容量上限为止），必填”。
- `docs/reference/API_EXAMPLES.md`：示例 `maxReplicas` 10 → 0。
- `docs/agents/PRINCIPLES.md`：字段所有权速查补充 `Orchestrator.spec.maxReplicas` 语义行。
