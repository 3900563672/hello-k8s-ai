# docker cp 从 WSL 推送文件到 kind 节点容器会静默丢失（Docker Desktop）

> 提升日期：2026-08-21 ｜ 来源：PR #118 镜像部署（hello-k8s-ai-dev）｜ 适用对象：本地 Agent / 远程 AI
> 触发条件（Use when）：WSL 内 `docker cp` 向容器推送文件（如镜像 tar）；kind 节点导入镜像；部署脚本里使用 docker cp 时

## 现象

- `docker cp /tmp/backend-img.tar <node>:/tmp/backend-img.tar` 返回 exit 0，但容器内 `ls /tmp/` 为空，`ctr images import` 报 no such file；
- 反向拉取（容器 → WSL）正常，内容正确；`C:/...`、`//wsl.localhost/...` 等路径变体分别被解析为容器路径或 WSL 根路径，均不可用。

## 根因

- Docker Desktop（Windows daemon）对 WSL 路径的推送语义异常：CLI 侧读取成功、daemon 侧写入静默失败，不返回错误码；非 `docker cp` 的常规 exec/stdin 路径不受影响。

## 对策（必须动作）

1. 绕开 docker cp 推送：`docker exec -i <node> sh -c "cat > /tmp/backend-img.tar" < /tmp/backend-img.tar`（stdin 管道写入），再 `ctr --namespace k8s.io images import`；
2. 导入后立即在容器内确认文件存在，不要信任 docker cp 的 exit code；
3. 固化到部署脚本，避免每次手敲。

## 证据链

- PR #118 部署过程：`docker exec -i` 管道导入 5 节点成功；此前 docker cp 静默失败日志。
