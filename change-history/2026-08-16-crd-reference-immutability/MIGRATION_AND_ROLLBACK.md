# 升级与回滚

## 1. 迁移

- CRD 升级后，已有 CR 不受影响（oldSelf 校验只在更新时生效）。
- 如果存量 CR 的引用曾在旧版本被修改过，新版本下再改会被拒绝，需删除重建。

## 2. 回滚

- `git revert` 本提交，或手动移除 CRD 中 x-kubernetes-validations 段落后重新应用。
- 注意：CRD 降级/回滚后旧行为恢复（引用可改），不会破坏数据。

## 3. 风险与注意

- CEL 规则对 Create 也生效（oldSelf 在 Create 时为 null，`self.name == oldSelf.name` 在 oldSelf 为 null 时……注意：CEL oldSelf 在创建对象时为 null，规则 `self.name == oldSelf.name` 会怎样？Kubernetes CEL 中，创建时 oldSelf 是 untyped null，比较 `self.name == oldSelf.name` → null 比较 false？这会导致创建被拒！
  - 关键：Kubernetes 对 XValidation 的 oldSelf：在创建时，oldSelf 绑定到 null。规则 `self.name == oldSelf.name`：`oldSelf.name` 在 null 上取字段会报错（null 没有字段）？CEL 中 null.name 是错误。这可能导致创建也被拒——危险！
  - 正确写法：`self.name == oldSelf.name` 需要处理创建场景。常见模式：`self == oldSelf` 用于整个对象；对创建时的 oldSelf null，官方文档说："When the object is created, oldSelf is set to null"——规则必须处理 null。例如 `!has(oldSelf) || self.name == oldSelf.name`。
  
  我需要立即修正规则为 `!has(oldSelf) || self.name == oldSelf.name`！否则创建会被拒绝，破坏所有功能！

让我验证：Kubernetes CEL XValidation 语义（官方文档）：oldSelf 在创建时为 null；若规则对 null oldSelf 求值失败（如 oldSelf.name），API Server 会拒绝创建？文档说 validation rule 中引用 oldSelf 时，创建请求中 oldSelf 为 null，"rules that rely on oldSelf fields will fail on create"。所以必须用 has(oldSelf) 保护。

修正规则：`!has(oldSelf) || self.name == oldSelf.name`
- has(oldSelf)：oldSelf 为 null 时 false → !has(oldSelf) = true → 规则通过（创建 OK）
- 更新时 oldSelf 是对象 → has(oldSelf) true → 比较 self.name == oldSelf.name

立即修正两个文件 + 重新生成。幸好发现得早！

