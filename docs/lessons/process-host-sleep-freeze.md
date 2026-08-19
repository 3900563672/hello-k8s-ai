# 宿主机空闲睡眠会冻结 WSL，值守任务必须先开睡眠守卫

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-16-night-run.md ｜ 适用对象：本地 Agent
> 触发条件（Use when）：无人值守 / 夜间长跑 / 任务可能跨宿主机睡眠时

## 现象

夜间/长时值守期间 WSL 与集群全部冻结；唤醒后自动化会话中断、keepalive 失效。

## 根因

Windows 交流空闲约 15 分钟自动睡眠，冻结整个 WSL2 VM；`nohup` 挡不住 exec 会话进程组回收。

## 可复用规则

- 无人值守前先 `bash hack/night-run/sleep-guard.sh status`，必须 `guard=on`；否则 `... on`（弹 UAC 需人在场）。
- 长驻进程用 `setsid` 启动（`nohup` 不够），并写 PID 文件供清理。
- 禁止 `wsl --shutdown`：会关闭所有发行版，包括用户正在用的 Ubuntu。

## 验证方法

`hack/night-run/sleep-guard.sh status` 返回 `guard=on`；会话内 `date` 与宿主时间差 < 1 分钟。
