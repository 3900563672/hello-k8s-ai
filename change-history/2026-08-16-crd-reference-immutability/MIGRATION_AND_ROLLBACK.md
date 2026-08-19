# 升级与回滚

## 1. 迁移

- CRD 升级后，已有 CR 不受影响（引用不可变校验只在更新时生效）。
- 如果存量 CR 的引用曾在旧版本被修改过，新版本下再改会被拒绝，需删除重建。

## 2. 回滚

- `git revert` 本提交，或手动移除 CRD 中 x-kubernetes-validations 段落后重新应用。
- 注意：CRD 降级/回滚后旧行为恢复（引用可改），不会破坏数据。

## 3. 风险与注意

- CEL 规则为 transition rule，只在 UPDATE 时求值：更新时比较新旧引用（`oldSelf.name`），不一致则拒绝。
- Create 天然不校验（transition rule 不在创建时求值），新增策略可以正常创建。
- 不能使用 `!has(oldSelf) || ...`：字段级规则的 `oldSelf` 根变量不是 message，`has()` 无法编译（API Server 报 `invalid argument to has() macro`）。
