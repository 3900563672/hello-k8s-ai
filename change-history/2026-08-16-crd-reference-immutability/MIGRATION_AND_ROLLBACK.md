# 升级与回滚

## 1. 迁移

- CRD 升级后，已有 CR 不受影响（引用不可变校验只在更新时生效）。
- 如果存量 CR 的引用曾在旧版本被修改过，新版本下再改会被拒绝，需删除重建。

## 2. 回滚

- `git revert` 本提交，或手动移除 CRD 中 x-kubernetes-validations 段落后重新应用。
- 注意：CRD 降级/回滚后旧行为恢复（引用可改），不会破坏数据。

## 3. 风险与注意

- CEL 规则对 Create 也生效，创建时 oldSelf 为 null，因此规则统一使用 `!has(oldSelf) || self.name == oldSelf.name`：
  - 创建时 `has(oldSelf)` 为 false，规则直接通过；
  - 更新时比较新旧引用，不一致则拒绝。
- 不能简写为 `self.name == oldSelf.name`：创建时 oldSelf 为 null，对 null 取字段会报错，导致创建被 API Server 拒绝。