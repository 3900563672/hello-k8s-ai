# 升级与回滚

- 变更日期：2026-08-16
- 关联问题：Fixes #14

## 升级方式

- 无数据库迁移、无 CRD 变化、无 Schema 变化：直接更新代码并重新运行 `bash setup.sh`（或 `make cluster-up`）即可。
- 部署清单变化：`dashboard/deploy`（backend/frontend `imagePullPolicy: Always`）、`config/observability/grafana.yaml`（sub-path 与嵌入变量）。
- 滚动更新后端口转发会断开，执行 `make cluster-open` 重建。

## 回滚方式

- `git revert` 关联提交即可。
- 回滚后本地恢复 4 个端口转发（`open_ports` 重新包含 grafana/prometheus/jaeger），Grafana 恢复无 sub-path 部署，前端「监控面板」入口与 `/grafana` 反代随之消失。
- 已部署工作负载不受影响；`imagePullPolicy` 回退为 `IfNotPresent` 后，`:dev` 标签的 digest 缓存问题会重现（需删除旧镜像或手工 `docker pull`）。

## 风险与注意事项

- Grafana 匿名 Viewer 可读全部预置面板；如需访问控制，应在 Grafana 侧启用认证并把 `GRAFANA_ENABLED=false` 关闭反代入口。
- 本机 `make cluster-open` 只管理 Dashboard 转发；历史残留的 prometheus/jaeger 转发进程不会被自动清理（`make cluster-down` 会统一停止）。
- 浏览器缓存可能保留旧的 Grafana 面板 JS；`?kiosk` 参数变化时可硬刷新。
