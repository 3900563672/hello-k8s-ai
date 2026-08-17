# 迁移与回滚

## 迁移

- 代码更新后执行 `make cluster-open` 会自动补齐 18080 转发（幂等；已运行则跳过）。
- day-watch 常驻进程需重启以使用新 BASE（kill 旧 PID 后重新 setsid 启动）。

## 回滚

- 恢复旧端口方案：删除 `local-cluster.sh` 的 dashboard-internal 转发行、day-watch 恢复 8080 默认；但 WSL 内 8080 冲突问题会复现（脚本需依赖重试与假阳性恢复）。
- Windows 侧 dllhost 8080 占用是系统行为，不在仓库控制内，不提供回滚。
