# 2026-08-21 kind PV tmpfs 遮罩误判与根治

> 日期：2026-08-21 ｜ 触发者：本地 Agent ｜ 相关：docs/lessons/kind-pv-tmpfs-umount-sop.md

## 背景

- 前一日（08-20）已发生同类故障并沉淀 SOP（`kind-pv-tmpfs-umount-sop.md`）。
- 08-21 上午准备联调：节点容器重启后 PG / Prometheus / Jaeger 全部 CrashLoopBackOff，
  postgres 报 `mkdir pgdata: Permission denied`。

## 误判过程（教训）

1. 直接看目录：`ls` 显示 PV 目录空、属主 root、时间戳 08-20 04:09 → 下了"数据丢失"结论。
2. 走了错误路径：`chown` 空目录 + 删除 pod 让其重新初始化 → 在 tmpfs 遮罩上写入了
   约 2800 条假事件 + 空 aiops 表（假数据在后续 umount 时丢弃，真数据无损）。
3. 反转点：用户追问"为什么总出问题" → 读 08-20 SOP + `mount | grep hello-k8s-ai-pv`
   → 发现 `none on /var/lib/hello-k8s-ai-pv type tmpfs`（5.9G tmpfs 遮罩）。

**SOP 第一条教训再次应验："数据目录变空" ≠ 数据丢失，先查 mount 再下结论。**

## 真实机制（08-21 实测补全）

- 旧配置集群（#88 之前）节点容器挂载：`/var/lib/hello-k8s-ai-pv` 是 bind 挂载，
  Source 指向 Docker Desktop 的 WSL bind-mounts（`/run/desktop/mnt/host/wsl/docker-desktop-bind-mounts/Ubuntu/<hash>`），
  即 WSL Ubuntu 发行版的 `/var/lib/hello-k8s-ai-pv`。
- Docker Desktop / WSL 重启后：VM 内 bind-mounts 注册整体丢失（实测该目录不存在）→ bind 失效。
- Docker Desktop 不报错，静默用 5.9G tmpfs 覆盖挂载点 → 表现 = 空目录 + root 属主 + 新时间戳。
- 数据实际两份：
  - WSL Ubuntu `/var/lib/hello-k8s-ai-pv/`（bind 有效期 08-18~08-20 写入，旧）
  - 节点容器 `/var` named volume（08-20 SOP 恢复后写入，更新；PG 1.9G / prom 329M / jaeger 780M）
- 恢复验证：`resource_events` 604,362 条、`resource_snapshots` 3,670、`resource_states` 930 全部在位。

## 恢复（08-21 上午，非根治）

1. 5 节点 `umount /var/lib/hello-k8s-ai-pv`（摘掉 tmpfs 遮罩，露出 /var named volume 真数据）。
2. 删除 PG(StatefulSet) / prometheus / jaeger pod，kubelet 重新挂载。
3. 本地后端重启时 migration 005_aiops.sql 在真数据上补建（migrationsApplied=5）。

## 根治（08-21）

1. `hack/kind/backup-data.sh`：pg_dump（dashboard.sql）+ prometheus/jaeger PVC tar 打包 → /var/tmp/hello-k8s-ai-backup-rootfix/。
2. `make kind-down`：删除旧集群（旧 /var named volume 保留，含全量数据）。
3. `make cluster-up`：新配置 kind-5node.yaml（无 extraMounts）重建 5 节点 + local-path + 部署全套。
4. `hack/kind/restore-data.sh`：psql 导入 + tar 解包回新 PVC。
5. 验证：PG 行数、Prometheus `count(up)`、Jaeger 服务、Grafana 面板。

## 教训追加

- chown / 重建 pod 只能让"当前遮罩"可用，**下一次 Docker Desktop 重启会再次出现 tmpfs 遮罩**（容器挂载是创建时定死的）。
- 根治必须重建集群（无 extraMounts 新配置），临时恢复一律走 SOP umount。
- 双份数据（Ubuntu 侧 + /var named volume）中，**以 /var named volume 为准**（SOP 恢复后写入，更新）。
- 每次 Docker Desktop / WSL 重启后，若出现 CrashLoop + Permission denied：先 `docker exec <node> mount | grep hello-k8s-ai-pv`，
  是 tmpfs 就按 SOP umount，不要 chown、不要重建 pod 初始化。

## 追加：kind 重建后 apiserver 端口映射不可达（08-21 实测）

- 现象：kind create 后 kubeconfig 指向 `127.0.0.1:<port>`，但 Windows/WSL 侧全部连接失败
  （HTTP 000 / connection refused），容器内 `curl localhost:6443` 正常。
- 定位：docker inspect 显示 `127.0.0.1:<port>->6443/tcp` 映射存在且与正常容器绑定一致；
  新起容器（nginx -p）映射 Windows 侧可达 → 不是网络/绑定问题，是 kind 创建时 Docker Desktop
  端口转发注册丢失（kind 网络上的新容器映射也正常，排除网络差异）。
- 修复：`docker restart hello-k8s-ai-dev-control-plane` 触发 Docker Desktop 重新注册转发，立即可达。
- 教训：kind create 后 kubectl 连不上先试 `docker restart control-plane`，不要急着删集群重建
  （重建可能再次遇到同样问题）。
