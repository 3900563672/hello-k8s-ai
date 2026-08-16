# 测试报告：Grafana 内存修复

## 修复前（复现证据）

| 指标 | 值 |
| --- | --- |
| cgroup memory.current | 374,996,992 B（≈383MiB） |
| cgroup memory.max | 402,653,184 B（384MiB） |
| 占用率 | 99.7% |
| 探针事件 | Liveness/Readiness failed：context deadline exceeded、HTTP 503 |
| Grafana 日志 | http Handler timeout、/api/datasources 8.3s、/api/annotations 8.7s、/api/prometheus 10.7s |

## 修复后

| 指标 | 值 |
| --- | --- |
| cgroup memory.current | 573,702,144 B（≈547MiB） |
| cgroup memory.max | 1,073,741,824 B（1024MiB） |
| 占用率 | 53% |
| restartCount | 0 |
| 探针事件 | 无（滚动完成 15 分钟后采样） |

## 其他组件水位（本次未改动）

| 组件 | current / max | 占用率 |
| --- | --- | --- |
| PostgreSQL | 604MiB / 1024MiB | 59% |
| Prometheus | 145MiB / 512MiB | 28% |
| Jaeger | 118MiB / 512MiB | 23% |
| frontend | 32MiB / 128MiB | 25% |

## 未验证项

- 长时间（>24h）稳定性观察未覆盖，由运行态持续验证。
