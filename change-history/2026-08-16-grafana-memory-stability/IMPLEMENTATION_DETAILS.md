# 实现细节：Grafana 资源限额调整

## 修改文件

- `config/observability/grafana.yaml`：deployment 容器 resources 段。

## 决策依据

- Grafana 13 首次加载 Dashboards/Provisioning 后工作集约 500MiB；384MiB 无余量。
- Docker Desktop VM 内存 15.3GiB，1024Mi 上限不会挤占其他组件。
- 只放宽 Grafana，其余组件按实测水位维持现状（见 TEST_REPORT）。
- 探针参数（period/timeout）不变：根因是内存而非探针配置；放宽内存后探针稳定。

## 为什么不改探针

探针超时的根因是内存打满导致请求排队；单纯调大 `timeoutSeconds` 会掩盖问题
且延长故障发现时间，因此保持 `liveness periodSeconds=10 / readiness periodSeconds=5`。
