# 变更总览：环境一键自愈 make env-up（重启后一条命令恢复联调）

> 日期：2026-08-21 ｜ 级别：P1

## 为什么做

- Docker Desktop / WSL 重启后，恢复完整联调环境需要手动做一堆易错操作：等 apiserver 起来、处理 Kind 节点 PV 被 tmpfs 遮罩、重建 port-forward、注入密钥启动本地后端、再起前端 vite——每一步失败都难以一眼定位（#109）。
- 目标是「跑一条命令，环境自己回来」：自愈、转发、后端、前端全部幂等，可反复执行。

## 改成什么

1. 新增 hack/dev-env-up.sh（约 290 行，make env-up 入口）：自愈 + port-forward + 本地后端 + 前端 vite 四段式。
2. 自愈：apiserver /healthz 不可达 → docker restart control-plane 并等待恢复；Kind 节点 /var/lib/hello-k8s-ai-pv 被 tmpfs 遮罩（旧 extraMounts bind 失效 fallback）→ umount 后重建 PVC 工作负载（PostgreSQL pod + Prometheus/Jaeger rollout restart）。
3. port-forward 幂等：.runtime/port-forward/KEY.pid + ss 端口连通双检查；端口被占自动向上找空闲端口，实际端口写入 .port 文件。
4. 本地后端：从集群 Secret hello-k8s-ai-dashboard-aiops 读 API key（不落明文到仓库），生成 .runtime/start-backend.sh（chmod 700，.runtime 已 gitignore），go run ./cmd/server 监听 127.0.0.1:18080。
5. 前端 vite：`VITE_API_PROXY_TARGET=http://127.0.0.1:18080` 代理到本地后端，`--host 0.0.0.0`（Windows 浏览器经局域网 IP 访问），端口被占自动 5173→5176 探测。
6. 环境变量开关：DEV_ENV_SKIP_HEAL=1 跳过自愈、DEV_ENV_SKIP_FRONTEND=1 不起前端。

## 关键行为

- 全部幂等：重复执行会识别并复用已运行的 port-forward / 后端 / 前端进程，不重复拉起；孤儿后端进程（无 pid 文件但占用 18080 且是本项目 server）也能识别复用并回写 pid。
- 端口检查用 ss 而非建连探测，避免 WSL 网络退化态下 TCP connect hang。
- 密钥只出现在 .runtime/（chmod 700），不入 git。

## 验证

- ash hack/dev-env-up.sh 连续运行 3+ 次：全部幂等复用，无重复进程。
- 自愈路径在 apiserver 不可达与 PV tmpfs 遮罩两种故障下实测通过。
- make lint-sh（shellcheck）通过。

## 回滚

- 未合并：git reset --hard HEAD~1。
- 运行中进程：按 .runtime/*.pid kill 后删除 .runtime/；删除 Makefile nv-up 目标与 hack/dev-env-up.sh 即可整体移除。

## 未验证/待办

- 在全新环境（未部署过集群）上未验证：脚本要求集群已存在（make cluster-up 先行）。
