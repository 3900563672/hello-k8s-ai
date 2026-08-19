# 实现修改明细

## 1. 改动前状态

- `dashboard/backend/internal/api/server.go` 的中间件链只有请求 ID、恢复、日志、安全头、CORS、超时和幂等，没有认证。
- `handlers_command.go` 的 `recordAudit` 直接读取客户端 `X-Remote-User` 作为 actor，缺失时记为 `system:anonymous`；任意调用方可伪造该头。
- Backend ServiceAccount 拥有集群级业务 CR 写权限，写接口一旦被共享网络可达即继承该权限。

## 2. 修改

- `dashboard/backend/internal/api/auth.go`（新增）：
  - `authMiddleware(httpConfig, environment, logger, next)`：对 POST/PATCH/DELETE 请求建立身份边界；
    - 配置 token 时用 `crypto/subtle.ConstantTimeCompare` 校验 Bearer Token，失败返回 401 `UNAUTHORIZED`；
    - 未配置 token 且环境为 `production` 时返回 503 `AUTH_NOT_CONFIGURED`（fail-closed）；
    - 未配置 token 且非生产环境时放行匿名写并输出 Warning 日志（保持本地演示可用）。
  - `actorName(request, trustRemoteUser)`：审计主体解析。`X-Remote-User` 仅在 `authenticated == true` 且 `TRUST_REMOTE_USER_HEADER=true` 时被采用，否则使用认证身份。
- `dashboard/backend/internal/config/config.go`：`HTTPConfig` 新增 `AdminToken`（env `ADMIN_TOKEN`，默认空）与 `TrustRemoteUser`（env `TRUST_REMOTE_USER_HEADER`，默认 false）。
- `dashboard/backend/internal/api/server.go`：中间件链在幂等中间件之外注册 `authMiddleware`，未认证写请求不进入幂等存储。
- `dashboard/backend/internal/api/handlers_command.go`：`recordAudit` 改用 `actorName`。
- `dashboard/deploy/backend.yaml`：env 区增加注释形式的 `ADMIN_TOKEN` Secret 配置示例。
- 文档：`docs/operations/SECURITY_AND_RBAC.md`、`docs/backend/API_DESIGN.md` 同步认证契约。

## 3. 未做

- 未引入 OIDC/session/JWT、CSRF 策略与按用户/租户授权模型。
- 未拆分只读/写命令 ServiceAccount（保留既有 RBAC 结构）。
- 未增加 rate limit 与 mutation 配额（单独迭代）。
- 生产启用 token 后 Frontend 的登录态与凭据传递方案未实现（文档已列为后续项）。
