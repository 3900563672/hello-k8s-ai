# 升级与回滚

## 1. 迁移

- 无 CRD、数据库变化。新中间件随 Backend 镜像升级自动生效。
- 未配置 `ADMIN_TOKEN` 时，现有非生产部署行为不变（匿名写继续可用），无需额外操作。
- 生产/共享环境：创建含 `admin-token` 的 Secret（如 `kubectl create secret generic hello-k8s-ai-dashboard-auth --from-literal=admin-token=$(openssl rand -hex 32)`），取消 `backend.yaml` 中 `ADMIN_TOKEN` 注释并重新部署。

## 2. 回滚

- `git revert` 本提交即可恢复旧行为（写接口不认证、actor 直接取客户端头）；无数据迁移负担。

## 3. 风险与注意

- 配置 token 后，未携带 Bearer 的客户端（含旧版 Frontend 写操作）会被拒绝；本地开发如需恢复，清除 `ADMIN_TOKEN` 环境变量即可。
- `TRUST_REMOTE_USER_HEADER` 只应在受信代理链路（代理持有 token）后开启；直接暴露的 Backend 开启该开关不会引入额外信任。
- token 为静态凭据，建议定期轮换并通过 Secret 注入，不要写入 Git。