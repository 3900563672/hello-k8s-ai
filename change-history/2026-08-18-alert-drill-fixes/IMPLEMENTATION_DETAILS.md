# 实现细节：降级演练缺陷修复

## 改动前状态

- `config/observability/prometheus.yaml`（8/17 提交 1129d9c 引入）：
  - 内存告警：`working_set / clamp_min(limit, 1) > 0.85`——limit=0 的容器被 clamp 成 1，比例爆表假阳性（模拟器容器当时无 limit，全部误报）；
  - 重启告警：`changes(container_start_time_seconds[10m]) > 0`——该指标带 container_id label，重启后是新序列，changes 数不出。
- 模拟器容器无 resources（`internal/controller/simulatorinstance_controller.go` 的 upsert 函数只设 image/env/probe/securityContext）。

## 实施步骤

1. **内存告警**：分子 `working_set{...} and on (namespace, pod, container) (container_spec_memory_limit_bytes{...} > 0)`，分母不变——无 limit 容器被过滤。
2. **重启告警迭代**（实触发验证驱动）：
   - 第一版（提交 5fa4da6）：`max by (namespace, pod, container)(...) - max by (...)(... offset 10m) > 60`——验证不触发：Pod 名随重启变化，两侧 label 不匹配；
   - 第二版（提交 771962d）：聚合键改为 (namespace, container)（重启前后稳定），再排除 simulator（模拟器扩缩容是新 Pod 启动，非异常重启）→ 实触发成功。
3. **模拟器 resources**：在 `SecurityContext` 前注入 `container.Resources`，import `k8s.io/apimachinery/pkg/api/resource`；requests 50m/64Mi、limits 500m/256Mi（实测均值 17MB，256Mi 安全）。
4. **部署**：`make docker-build`（hello-k8s-ai-controller:dev）→ `rollout restart` controller → 新建 TenantModelPolicy 让 Orchestrator 拉起模拟器实例验证 resources。

## 涉及文件

- `config/observability/prometheus.yaml`（两条告警表达式）
- `internal/controller/simulatorinstance_controller.go`（模拟器容器 resources）