# TEST_REPORT：Docker bind 失效恢复

## 验证环境

- Docker Desktop 引擎 29.7.2（杀进程 + wsl --shutdown + 重启后恢复）；kind 集群 hello-k8s-ai-dev（5 节点，K8s v1.36.1）。

## 恢复验证

| 检查项 | 结果 |
| --- | --- |
| 5 节点 `mount | grep hello-k8s-ai-pv` | 全部 NO_MOUNT（tmpfs 已移除） |
| 目录所有权 postgres（worker3） | 70:70, drwx------ |
| 目录所有权 prometheus（worker） | 65534:65534, drwx------ |
| 目录所有权 jaeger（worker4） | 777, drwxrwxrwx |
| 系统 Pod | 8/8 Ready（controller/backend/frontend/postgres/grafana/jaeger/otel/prometheus） |
| postgres 日志 | `database system is ready to accept connections` |
| 数据恢复脚本 | 后台运行中（/var/tmp/restore-data.log），SQL COPY 阶段 |

## 数据备份完整性（勿动）

| 文件 | 大小 | 内容 |
| --- | --- | --- |
| dashboard.sql | 2.5G | resource_events ~52 万行 + snapshots 3670 |
| prometheus.tar.gz | 264M | Prometheus TSDB |
| jaeger.tar.gz | 395M | Jaeger badger 数据 |

## 未验证

- 数据恢复完成后的行数核对（待 restore 完成）。
- 节点容器重启后 umount 是否需重做（预期需要，已记入 lesson）。
- 长期修复（删 extraMounts）尚未实施（需重建集群，待用户确认）。
