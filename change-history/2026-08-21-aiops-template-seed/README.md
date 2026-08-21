# AIOps 模板预置：10 模型 + 10 租户 + 10 节点（空环境待命）

> 日期：2026-08-21 ｜ 关联：docs/aiops/AIOPS_OVERVIEW.md、docs/backend/API_DESIGN.md

## 为什么做

- 演示链路要求环境空载（无预置流量/实验），但 AIOps 可选模板要够用：打开页面后由内置 AI 一句话起实验，实验与调度数据随之产生，而不是死数据。
- 此前模板目录每类仅 3 条，且节点模板无法通过执行 gate（只认真实 Node 名），AI 选了模板节点也会被拒。

## 改成什么

1. `dashboard/backend/internal/aiops/command.go`：`TemplateCatalog` 扩为 model/node/tenant 各 10 条，id 与集群 CR 名一一对应（`preset-model-001..010` / `preset-node-001..010` / `preset-tenant-001..010`），orchestrator/traffic 保持 3 条。
2. `dashboard/backend/internal/api/handlers_aiops_commands.go`：gate 节点校验同时接受真实 Node 与 WorkerNode CR，模板节点可被 AI 直接选中。
3. `hack/aiops-templates-seed.sh`（新增）：幂等预置 10 Model + 10 Tenant（qps=0 空环境）+ 10 WorkerNode + 70 条关系策略（ModelNodePolicy/TenantNodePolicy/TenantModelPolicy）。
4. 文档同步：AIOPS_OVERVIEW、ARCHITECTURE_OVERVIEW、API_DESIGN、BACKEND_ARCHITECTURE、API_EXAMPLES、DATA_FLOW。

## 关键行为

- 模板 id = CR 名 = seed 脚本生成名，三者强制对齐；AI 选中的节点/租户可直接过 gate 并执行。
- 租户 `qps` 预置 0：环境无预置流量，起实验后才产生数据。

## 验证

- 后端单测（模板目录 id 引用同步更新）全绿；shellcheck 通过。
- `bash hack/aiops-templates-seed.sh` 后 `GET /api/v1/aiops/templates` 返回 10/10/10，集群 CR 计数 10/10/10 + 策略 70。
- 演示：AI 面板一句话起实验，选中模板节点/租户可过 gate 并产生数据。

## 回滚

- git revert 本提交；删除预置 CR（`kubectl delete model,tenant,workernode,modelnodepolicy,tenantnodepolicy,tenantmodelpolicy -l app.kubernetes.io/managed-by=aiops-templates-seed`）；如需还原 gate 只认真实 Node，撤销 handlers_aiops_commands.go 改动。
