# 前端新增策略管理，打通“配置 → 真实工作负载”完整闭环

- 变更日期：2026-08-16
- 关联问题：Fixes #2
- 变更级别：P1 功能闭环
- 变更范围：Frontend（配置中心新增「策略」页签）、无 Backend/Controller 改动
- CRD 变化：无
- 数据库变化：无

## 1. 背景与根因

前端已能创建 Model / Tenant / Orchestrator，但创建后的资源不会形成完整运行流程：
集群中的实际工作负载不会启动。

排查（真实集群 + Backend API 验证）发现两条断点：

1. SimulatorInstance 由 TenantModelPolicy（Allow）物化；只有 TenantModelPolicy 而没有
   TenantNodePolicy（Allow）时，节点不可调度，Orchestrator 的 placement 失败，
   实例副本保持 0，Pod 不会创建。
2. 前端没有任何策略管理入口，用户无法补齐这三类策略（TenantModelPolicy /
   TenantNodePolicy / ModelNodePolicy）。

## 2. 修复内容

配置中心新增「策略」页签：

- 列表：三类策略（租户-模型 / 租户-节点 / 模型-节点），显示引用关系与 Allow/Deny 效果。
- 新建：策略类型 + 引用对象选择 + 效果选择，自动生成系统标识（`tenant-model` 等）。
- 编辑：切换引用对象与效果，保存走 Backend `/configuration:apply`。
- 删除：单条与批量删除。

复用既有调用链：Frontend → Backend（`TenantModelPolicy` 等已在白名单）→ K8s API →
TenantModelPolicy Controller 物化 SimulatorInstance → Orchestrator 引导副本 →
Simulator Pod 启动。

## 3. 验证结果

真实 docker-desktop 集群，通过 Backend API 执行与前端完全相同的请求序列：

1. 创建 Model + Tenant + Orchestrator（qps=5、minReplicas=1）。
2. 创建 TenantModelPolicy(Allow) + TenantNodePolicy(Allow) + ModelNodePolicy(Allow)。
3. SimulatorInstance 出现：Pending → Running，replicas 0 → 1。
4. Simulator Pod `simulator-<tenant>-<model>` Running 1/1（约 15 秒内）。
5. 删除策略与资源后实例与 Pod 自动清理。

前端 `npm run check`（lint + build + state-check）通过；新构建镜像已部署到集群，
产物包含策略页代码。
