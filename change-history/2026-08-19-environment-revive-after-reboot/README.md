# 重启后环境恢复 + Codex 卡死排查 + WSL 协作模式评估

> 日期：2026-08-19

## 为什么做

- 整机重启后 Codex 反复"卡一会好一会"；用户询问能否切换到 WSL 模式提升协作稳定性，并要求把完整开发环境重新拉起。

## 排查结论

1. **卡死根因（候选已定位）**：项目以 UNC 路径打开（`\\wsl.localhost\Ubuntu\root\hello-k8s-ai`）+ 桌面端配置 `integratedTerminalShell = "wsl"`，命中 codex#18506（UNC 项目 + wsl 终端导致终端初始化阻塞）。次要因素：本会话上下文大 + stream idle 超时 600s。
2. **WSL 模式评估（不建议切换）**：WSL 内没有 codex CLI / config.toml / 认证（`/root/.codex` 仅 sqlite）；codex#34782 记录 WSL 模式回归风险；用户对 config.toml 高度敏感。结论：保持 Windows Agent + 命令经 `wsl -e bash -lc` 执行。
3. **最小配置改动**：`integratedTerminalShell = "wsl"` → `"powershell"`（备份 `config.toml.bak-20260819-094011`），需完全退出并重启 Codex 后生效。

## 环境恢复步骤（本次实际执行）

1. 启动 Docker Desktop（settings-store 显示 `KubernetesEnabled=false` 为预期——底座已是 Kind，不再启用 Docker 内置 K8S）。
2. **WSL docker CLI 指向 Desktop 引擎**：WSL 内 systemd dockerd（29.1.3）抢占 `/var/run/docker.sock`，导致 kind 看不到 Desktop 引擎（29.7.2）上的节点容器。修复：`DOCKER_HOST=unix:///mnt/wsl/docker-desktop/shared-sockets/guest-services/docker.proxy.sock` 写入 `~/.profile` + `~/.bashrc`。
3. **恢复 kubeconfig**：`kind get kubeconfig --name hello-k8s-ai-dev > ~/.kube/config`，context `kind-hello-k8s-ai-dev`。
4. **tmpfs 覆盖恢复（复用 SOP）**：5 节点 `/var/lib/hello-k8s-ai-pv` 挂成 tmpfs → 逐个 `umount` 露出 `/var` volume 底层 → chown（postgres 70:70/700、prometheus 65534:65534/700、jaeger 777）→ 删除 CrashLoopBackOff Pod 重建。
5. 数据未丢：底层 `/var` volume 数据完好，无需全量 restore-data.sh。

## 验证

- 5 节点全部 Ready（1 control-plane + 4 worker，v1.36.1）。
- 8 个系统 Pod 全部 1/1 Running（controller/backend/frontend/postgres/grafana/jaeger/otel/prometheus）。
- 数据：PostgreSQL resource_events=545735 / snapshots=3670；Prometheus 8608 series；Jaeger 3 服务。
- 端口 8080/18080 已开：Dashboard 200、Grafana 200、backend `/api/v1/health/live` 与 `/ready` 均 200。

## 待办 / 风险

- 重启 Codex 后 `integratedTerminalShell=powershell` 才生效（本次会话内不重启可继续工作）。
- kind-5node.yaml 的 extraMounts 非持久问题仍为已知长期项（lesson 已记录，需重建集群根治）。
- WSL 内部 systemd dockerd 与 Desktop 引擎并存：已用 DOCKER_HOST 绕过；是否停用 systemd docker 服务待用户决定。
- 8-18 13:57 之后产生的增量运行数据在 Desktop VM 根文件系统上，整机重启后不可恢复（本次恢复到的数据截止 8-18 13:57）。

## 回滚

- 配置：`Copy-Item C:\Users\hh\.codex\config.toml.bak-20260819-094011 C:\Users\hh\.codex\config.toml`。
- DOCKER_HOST：删除 `~/.profile` / `~/.bashrc` 中的 export 行。
