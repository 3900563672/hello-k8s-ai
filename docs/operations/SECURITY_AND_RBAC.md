# 安全与 RBAC

## 1. 当前信任边界

```mermaid
flowchart LR
  U["浏览器/用户"] --> N["Frontend Nginx"] --> B["Backend ServiceAccount"]
  B --> K["Kubernetes API"]
  B --> D["PostgreSQL"]
  B --> P["Prometheus/Jaeger"]
  C["Controller SA"] --> K
  S["Simulator SA"] --> K
```

当前开发实现主要依赖 Kubernetes ServiceAccount/RBAC；没有完整最终用户认证和按用户/租户授权。任何把 Backend 暴露给不可信用户的部署都必须先补 IAM。

## 2. Backend RBAC

ServiceAccount：`hello-k8s-ai-dashboard-backend`。

| 权限 | 资源 |
| --- | --- |
| get/list/watch | 全部 11 个 platform CRD |
| create/update/patch/delete | Model、WorkerNode、Tenant、TenantModelPolicy、TenantNodePolicy、ModelNodePolicy、Orchestrator |
| create/update/patch | `SimulationClock/default`；应用层只开放 rate，RBAC 不授予 delete/status |
| get/list/watch | Pods、Nodes、Services、Events |
| get/list/watch | Deployments、ReplicaSets |
| get/list/watch | Leases |

没有 Status 子资源写权限，没有原生资源写权限。应用 Gateway 再做 Kind/Spec field allowlist。这是 defense in depth。

生产建议把 read-only 与 command Deployment/ServiceAccount 分离，或按身份做 SubjectAccessReview/impersonation；当前单 SA 意味着所有通过 Backend 的调用共享一套集群权限。

## 3. Controller 与 Simulator

- Controller Manager RBAC 由 kubebuilder markers 生成，负责 CR Status、Deployment 等控制资源和 leader election。
- Simulator 独立 ServiceAccount/ClusterRole，只需读取相关 Instance/Model、更新 SimulatorInstance Status、管理本 namespace Lease 等。
- Prometheus ServiceAccount 可发现 Pods，并访问受保护 metrics nonResourceURL。

任何 RBAC 改动从源码 markers/独立清单生成；`config/rbac/role.yaml` 不手改。

## 4. 容器安全基础

当前多数工作负载：runAsNonRoot、drop ALL capabilities、seccomp RuntimeDefault；Backend/Frontend/Collector/Jaeger 使用 readOnlyRootFilesystem（PostgreSQL 因数据目录例外）。有 requests/limits 与 probes。

仍需：

- 镜像 pin digest、SBOM、漏洞扫描、签名/验证。
- Pod Security Admission namespace policy。
- PDB、anti-affinity、topology spread。
- egress/ingress NetworkPolicy。
- RuntimeClass/沙箱按威胁模型评估。

## 5. Secret 与传输

本地部署首次执行时生成随机 PostgreSQL 密码并写入 Kubernetes Secret，不在 Git 中保存密码；DATABASE_URL 仍为本机集群内 `sslmode=disable`。生产禁止：

- 在 Git/Kustomize 直接放真实密码。
- 用默认密码或长期静态 token。
- 跨节点/网络明文连接数据库、OTLP、API。

应使用外部 Secret manager/CSI/Sealed Secret（按组织标准）、自动 rotation、TLS/mTLS、最小读取权限。日志/Trace/Audit 必须过滤 Authorization、Cookie、DATABASE_URL、Secret data。

## 6. 网络

当前只有 controller metrics 相关基础 NetworkPolicy 目录，完整组件间隔离不足。生产建议 allowlist：

- Frontend -> Backend 8080。
- Backend -> Kubernetes API、PostgreSQL 5432、Prometheus 9090、Jaeger 16686、DNS。
- Controller/Simulator -> Kubernetes API、OTel Collector；Prometheus -> metrics endpoints。
- OTel Collector -> Jaeger；Grafana -> Prometheus/Jaeger。
- 默认 deny 其他 ingress/egress。

Simulator 没有业务 Service；不要无意公开 9090。

## 7. API 安全

已有：严格 JSON、1MiB 限制、显式 CORS、请求 ID、安全 headers、panic recovery、命名 PromQL、Trace 参数边界、幂等/resourceVersion、dry-run、audit。

未有/需加强：

- OIDC/session/JWT 验证和 CSRF 策略。
- 按用户/租户的 authorization。
- mutation/query rate limit、body/query complexity quota。
- actor/IP/user-agent 的可信审计。
- sensitive DTO/attribute 脱敏。
- 防止 SSRF 的 provider URL 管理（只由受信配置设定）。

CORS 不是认证，Idempotency-Key 也不是授权。

## 8. 多租户隔离

CRD 都是 Cluster-scoped，Backend SA 也可读全部对象；当前 Tenant 是业务标签，不是 Kubernetes 安全租户。若要真实多租户：

- 明确用户 -> Tenant membership。
- API 每个 read/write 强制 tenant scope。
- Audit 与 Trace 查询同样过滤。
- 防止通过 metadata.name/ref 枚举其他租户。
- 考虑 namespace-scoped API 或隔离集群的版本设计；不能只在 UI 隐藏。

## 9. 审计与隐私

Audit 应记录 actor、action、resource UID/version、requestId、idempotency key、result，不记录 Secret/完整敏感 payload。Trace attributes 中 Tenant/Model 名可能是业务敏感元数据，需要 retention/access policy。

PostgreSQL/Prometheus/Jaeger/Grafana 访问都应认证、加密、备份并有删除策略。

## 10. 安全发布门

- 无默认凭据；Secret 不在 Git。
- OIDC/授权/tenant scope 测试通过。
- RBAC `can-i` 正负测试；Backend 不能写 Status/Pod/Deployment。
- 全部网络路径最小 allowlist + TLS。
- 镜像扫描/signature/SBOM。
- 审计主体可信、敏感字段脱敏。
- 备份加密/恢复演练。
- 依赖/provider 超时、rate limit、DoS 测试。
- Pod Security/NetworkPolicy 由策略测试验证。
