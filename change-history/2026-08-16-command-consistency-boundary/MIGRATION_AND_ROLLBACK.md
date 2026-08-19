# 升级与回滚

## 1. 迁移

- 无 CRD、数据库变化；API 响应新增可选字段 `results[].error`，旧客户端忽略即可。
- 幂等释放与审计超时随 Backend 镜像升级自动生效。

## 2. 回滚

- `git revert` 本提交即可恢复旧行为（批量失败返回整体错误、完成写失败保留 pending）；无数据迁移负担。

## 3. 风险与注意

- 完成写失败后释放占位：若 Kubernetes 已写入成功，客户端重试可能再次执行幂等 apply（对 create/update 无副作用；delete 重试可能得到 NotFound，客户端需按已删除处理）。
- 批量 partial 返回 HTTP 202 + `state=partial`，不是错误响应；依赖 `meta.partial` 与 `results[].error` 呈现明细。
