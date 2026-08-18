# MIGRATION_AND_ROLLBACK：WSL 回环中继研究链

## 迁移

- 无数据迁移：纯探针（Go 工具）与文档改动。
- 探针接入点：`hack/preflight.sh` 第 9 节 + `make selfcheck`（此前已接入，本条目仅更新语义说明，不改接入代码）。

## 回滚

1. 删除 `hack/wsl-loopback-probe/` 目录（git revert 即可，工具无状态）。
2. `hack/preflight.sh` 第 9 节与 `make selfcheck` 对应步骤随 git revert 一并撤销。
3. 文档回滚：journal / CASE_STUDY / lessons / TROUBLESHOOTING §3.3 恢复旧版（git revert 同一提交）。
4. 风险：回滚后"立即拨号 ×10"的误报问题会回来——若再次使用旧探针，健康态会被误判 FAIL 并阻断 preflight。建议保留新探针语义。

## 注意事项

- 多轮模式（`-attempts >1`）结果不能判定环境故障（会命中健康态瞬态）。
- dmesg 计数判级：仅严重形态下持续增长；健康态为 0/恒定。
