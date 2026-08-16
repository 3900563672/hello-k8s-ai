# 1. 问题标题

Backend 写接口缺少可信的认证与授权边界

## 2. 当前状态描述

`dashboard/backend/internal/api/server.go` 注册了配置批量应用、配置删除和租户 QPS 修改等写接口。HTTP 中间件包含请求 ID、恢复、日志、安全响应头、CORS、超时和幂等处理，但没有认证中间件，也没有基于用户、租户或动作的授权判断。

`dashboard/deploy/rbac.yaml` 把 Backend ServiceAccount 绑定到集群级 ClusterRole。该角色可读取全部平台 CR，并可创建、更新、Patch、删除 Model、WorkerNode、Tenant、三类 Policy 和 Orchestrator。所有业务 CRD 又都是 cluster-scoped，因此 Backend 的权限覆盖整个集群业务平面。

命令审计位于 `dashboard/backend/internal/api/handlers_command.go`。审计 actor 直接取客户端提供的 `X-Remote-User`；没有上游可信代理校验，任意调用方都可以伪造该请求头。没有该头时，写操作仍会执行，只把 actor 记录为 `system:anonymous`。

当前本地部署通过 `kubectl port-forward` 暴露 Frontend，确实降低了默认网络暴露范围，但这只是部署方式，不是应用安全边界。一旦后续增加 Ingress、共享开发环境或远程访问，所有可达 Backend 的主体都会继承 Backend ServiceAccount 的集群级写能力。

## 3. 问题定位

这是控制平面的权限代理问题。Backend 并不是普通只读 Dashboard；它代表用户向 Kubernetes 发起高权限变更。缺少认证意味着系统不知道“是谁”，缺少授权意味着系统无法判断“该主体能修改哪个租户或哪类资源”。

CORS 只限制浏览器跨域行为，不能阻止 curl、同源脚本或集群内 Pod 直接调用。`Idempotency-Key` 只解决重复请求，不是访问凭证。安全响应头也不提供身份校验。因此现有中间件不能替代认证授权。

审计日志当前保存的是未验证的自报身份，无法作为合规或事故追踪依据。更严重的是，所有租户共享 cluster-scoped CR，一次越权操作可能影响整个系统，而非单个 Namespace。

## 4. 影响范围

- Backend：所有写接口均受影响，部分敏感只读接口也可能泄露全局拓扑和租户数据。
- Kubernetes：攻击者可借用 Backend ServiceAccount 执行其 ClusterRole 允许的集群级操作。
- CRD：cluster-scoped 设计扩大了单个身份边界失效后的影响面。
- Frontend：当前没有登录态、用户上下文或权限驱动的功能控制。
- PostgreSQL：audit_log 中 actor 的可信度不足。
- 部署：本地端口转发下风险受限，但增加 Ingress 或共享访问后会成为直接安全缺口。

归档中没有发现已发生的越权证据；该问题属于明确存在的发布阻断风险，而非已确认安全事件。

## 5. 根本原因分析

当前 Backend 最初按本地演示控制面建设，网络可达性被隐含当作信任条件。随后增加了写 API、幂等和审计，但身份体系没有同步进入同一设计，使“可以安全重试”先于“谁有权执行”得到实现。

另一个根因是 Backend 以单一 ServiceAccount 代理所有用户操作。Kubernetes 只看见 Backend 身份，应用层又没有建立用户到租户、资源和动作的授权模型，导致权限只能是全有或全无。

## 6. 修改方向建议

- 在任何对外暴露前确定唯一可信身份入口，并让 Backend 只接受经过验证的身份信息。
- 建立动作级与租户级授权模型，覆盖读取、配置应用、删除和流量修改，不依赖 Frontend 隐藏按钮实现安全控制。
- 只有在受保护的代理链路中才信任上游身份头，并把经过验证的主体写入审计；直接客户端提供的同名头应被忽略或拒绝。
- 重新核对 Backend RBAC，让应用授权和 Kubernetes 最小权限共同生效；必要时把只读聚合与写命令的运行身份分开。
- 为匿名访问、跨租户访问、伪造身份头和权限降级增加安全测试。
- 不必改变现有 REST API 或 CRD 业务模型，重点是补上控制面入口的身份与授权契约。

## 7. 优先级

优先级：P0

本地单人环境可继续使用，但在共享网络、多人协作或生产部署前必须处理。
