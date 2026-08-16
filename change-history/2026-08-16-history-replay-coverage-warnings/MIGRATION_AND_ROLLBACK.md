# 升级与回滚

## 1. 迁移

- Backend 环境变量新增 `PROMETHEUS_RETENTION`（默认 24h）与 `JAEGER_RETENTION`（默认 0）。
- 不设置时行为与部署清单一致：Prometheus 24h、Jaeger 内存模式。无需修改清单即可升级。

## 2. 回滚

- `git revert` 本提交，或移除环境变量与代码逻辑后重新部署。
- 回滚后恢复旧行为：历史查询不再提示覆盖缺口（不破坏数据，只少提示）。

## 3. 风险与注意

- 告警是保守提示：即使目标时间早于保留窗口，Provider 仍可能返回部分数据；告警只说明"可能不完整"。
- 如果部署方调整了 Prometheus retention 或 Jaeger 存储，应同步更新对应环境变量，避免告警与实际保留窗口不一致。
