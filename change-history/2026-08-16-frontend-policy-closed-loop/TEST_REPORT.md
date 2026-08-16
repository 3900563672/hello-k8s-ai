# 测试报告：策略闭环

## 自动化验证

- `cd dashboard/frontend/my-app && npm run check`：lint + build + SSR state-check 全部通过。

## 真实集群闭环验证（docker-desktop）

请求序列与前端完全一致（Backend `/api/v1/configuration:apply`）：

| 步骤 | 结果 |
| --- | --- |
| 创建 Model e3-model（gpuUnits=1, absoluteScore=100） | 202 accepted |
| 创建 Tenant e3-tenant（qps=5） | 202 accepted |
| 创建 Orchestrator e3-orch（minReplicas=1, maxReplicas=2） | 202 accepted |
| 创建 TenantModelPolicy Allow | 202 accepted |
| 创建 TenantNodePolicy Allow（desktop-worker） | 202 accepted |
| 创建 ModelNodePolicy Allow（desktop-worker） | 202 accepted |
| SimulatorInstance 状态 | Pending → Running，replicas 0 → 1（约 15s） |
| Simulator Pod | `simulator-e3-tenant-e3-model-*` Running 1/1（desktop-worker） |
| 删除策略/Orchestrator/Tenant/Model | 实例与 Pod 自动清理 |

对照实验（修复前行为）：

- 只有 Model/Tenant/Orchestrator（无策略）：SimulatorInstance 不创建（无 TenantModelPolicy），
  或创建后 replicas 保持 0（无 TenantNodePolicy 时 placement 不可行）。
- Orchestrator Ready 条件停留在 `MetricsNotReady`，不会自行启动工作负载。

## 部署验证

- `docker build -t hello-k8s-ai-dashboard-frontend:dev dashboard/frontend/my-app` 成功。
- `kubectl rollout restart deployment/hello-k8s-ai-dashboard-frontend` 成功。
- 新 bundle 已包含策略页代码（产物中可检索「访问策略」「新建策略」）。

## 未验证项

- 浏览器交互（新建/编辑/删除表单）未做人工点击验证；由类型检查、构建与 API 链路测试覆盖。
