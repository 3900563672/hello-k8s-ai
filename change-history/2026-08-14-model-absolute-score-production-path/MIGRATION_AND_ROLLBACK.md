# 升级、迁移与回滚

## 1. 升级前备份

本次包含 CRD required 字段变化。覆盖源码并部署前，建议保存 Model 与 CRD：

```bash
kubectl get models.platform.study.com -o yaml > /tmp/models-before-absolute-score-fix.yaml
kubectl get crd models.platform.study.com -o yaml > /tmp/models-crd-before-absolute-score-fix.yaml
```

## 2. 推荐升级顺序

使用项目的一键本地部署时，继续执行现有命令即可：

```bash
make cluster-up
```

脚本按以下顺序处理：

1. 应用新 CRD 和 Controller 清单；
2. 等待 CRD Established；
3. 将旧 `status.absoluteScore` 正数复制到缺失的 Spec；
4. 重启并等待 Controller；
5. 应用含 `spec.absoluteScore` 的演示 Model；
6. 执行现有完整链路验收。

自定义部署流程也必须在新对象创建时提交 `spec.absoluteScore`。旧对象没有 Status 分数时，系统无法可靠猜测真实能力，需要人工给值。

## 3. 查看迁移状态

```bash
kubectl get models.platform.study.com \
  -o custom-columns='NAME:.metadata.name,SPEC_SCORE:.spec.absoluteScore,LEGACY_STATUS_SCORE:.status.absoluteScore'
```

正常状态：每个正在被 TenantModelPolicy Allow 的 Model 都有正数 `SPEC_SCORE`。`LEGACY_STATUS_SCORE` 可以暂时保留，它不再是新权威。

如需手工补一个模型：

```bash
MODEL='<Model 名称>'
SCORE='<正整数能力基准分>'
kubectl patch model.platform.study.com "$MODEL" --type=merge \
  --patch "{\"spec\":{\"absoluteScore\":$SCORE}}"
```

## 4. 升级期间的兼容行为

- 已有 Spec 分数：直接使用，不被脚本覆盖。
- 只有旧 Status 分数：新 Controller 可立即回退读取，一键部署随后复制到 Spec。
- Spec 与旧 Status 都有且不同：Spec 胜出；这是明确的权威规则。
- 两处都没有：Orchestrator 不扩容，Condition reason 为 `ModelScoreMissing` 并列出 Model。
- 新建 Model 缺少字段：Kubernetes CRD 拒绝创建；Backend 会更早返回缺字段错误。

## 5. 观察与验收

```bash
kubectl get models.platform.study.com
kubectl get orchestrators.platform.study.com -o yaml
kubectl get simulatorinstances.platform.study.com \
  -o custom-columns='NAME:.metadata.name,REPLICAS:.spec.replicas,EFFECTIVE_SCORE:.status.effectiveScore'
```

验收要点：

1. Model 列表的 AbsoluteScore 有正数；
2. Orchestrator 没有 `ModelScoreMissing`；
3. 首次 Allow 的 Model 能把对应 SimulatorInstance 从 0 扩到 floor；
4. effectiveScore 根据 Spec 分数与原冷启动权重计算；
5. Backend Config API 与前端表单读写同一 Spec 字段。

## 6. 安全回滚

旧 Controller 只读取 `status.absoluteScore`。若必须回滚到旧代码，先把当前 Spec 分数复制回旧 Status，否则在本次修复后新建的 Model 会再次无法调度。

对每个 Model 执行：

```bash
MODEL='<Model 名称>'
SCORE="$(kubectl get model.platform.study.com "$MODEL" -o jsonpath='{.spec.absoluteScore}')"
kubectl patch model.platform.study.com "$MODEL" \
  --subresource=status --type=merge \
  --patch "{\"status\":{\"absoluteScore\":$SCORE}}"
```

确认旧 Status 已写入后，再回滚 Controller、Backend、Frontend 和 CRD 清单。不要先应用旧 CRD：旧 schema 不认识 `spec.absoluteScore`，后续对象更新可能裁剪该字段。

回滚前后的 YAML 备份应保留，直到所有 Model 在旧 Controller 下恢复调度。数据库无需回滚。

## 7. 删除旧 Status 字段的时机

本次不删除旧字段。只有满足以下条件后，才应在后续 API 版本中移除：

- 所有环境的 Model 都已有 Spec 分数；
- 没有旧 Controller 仍在运行或可能回滚；
- 已完成 storage version / conversion 方案；
- 迁移验证和备份恢复演练通过。
