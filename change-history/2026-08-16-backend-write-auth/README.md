# Dashboard Backend 写接口可信认证与授权边界

- 变更日期：2026-08-16
- 关联问题：Fixes #15（Project Review issue-02）
- 变更级别：P0 安全边界
- 变更范围：`dashboard/backend/internal/api`（认证中间件与审计主体）、`internal/config`、部署清单与文档
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

Backend 写接口（配置批量应用、配置删除、租户 QPS、模拟倍速）此前无认证，审计 actor 直接信任客户端提供的 `X-Remote-User`，任意调用方都可伪造。本次建立最小可行认证边界：

- 配置 `ADMIN_TOKEN` 后，写请求必须携带匹配的 `Authorization: Bearer <token>`，否则 401；
- 未配置 token 时，非生产环境保持匿名写（本地演示不受影响），生产环境写接口返回 503（fail-closed）；
- 审计主体改取自认证身份；`X-Remote-User` 只在请求通过 Bearer 认证且显式开启 `TRUST_REMOTE_USER_HEADER` 时才被信任；
- 认证中间件位于幂等中间件之外，未认证的写请求不会触碰幂等存储。

## 2. 关键行为

- 只读接口不要求认证，身份记录为 `system:anonymous`。
- CORS preflight（OPTIONS）在认证前返回，不阻塞跨域预检。
- 开发/演示环境未配置 token 时，每次匿名写会输出 Warning 日志提示。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| Backend API | 写请求要求 Bearer Token（配置后）；审计 actor 可信 |
| 配置 | 新增 `ADMIN_TOKEN`、`TRUST_REMOTE_USER_HEADER` 环境变量 |
| 部署 | `dashboard/deploy/backend.yaml` 增加可选 Secret 配置注释 |
| 测试 | 新增 `auth_test.go`，覆盖认证/降级/伪造头场景 |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 实测结论与停止线

- `go test ./... -skip Grafana` 通过（Grafana proxy 两个用例为环境既有失败，见测试报告）。
- 停止线：不做 OIDC/会话、不做按租户授权模型、不改 REST API 与 CRD 业务模型；这些属于后续安全迭代。
