# WSL 反复使用过的端口残留失效状态：新监听注册被旧状态遮蔽

> 提升日期：2026-08-21 ｜ 来源：/root/frontend-redesign DEV-NOTES.md（前端重构工作副本，2026-08-20 实测）｜ 适用对象：本地 Agent / 远程 AI
> 触发条件（Use when）：本机 WSL 内开发服务端口间歇性拒绝连接 / vite dev（HMR）不可靠 / 换端口后恢复 / 前端预览链路搭建时

## 现象

- `127.0.0.1:4173`（vite）间歇性拒绝连接；Windows 侧 `localhost`/eth0 也随之中断。
- 排除：与进程无关（python 监听 4173 同样坏）、与 keep-alive/HMR 无关（关闭后仍坏）。
- **规律：端口特定**——被反复使用过的端口（4173）残留失效状态，新监听器在该端口约 1 分钟后失效；**全新端口（4180）持续稳定**（20+ 分钟采样全绿）。

## 根因（假设，与 WSL issue 调查同源）

- 与 `tcp.rs TimeWait 不启动 timer` 假设吻合（见 `process-wsl-loopback-fresh-listen-refused.md` 的严重形态）：残留连接状态不回收，新监听注册被旧状态遮蔽。
- 重启监听进程只能恢复约 1 分钟（与新端口注册停滞 2–5s 自愈的瞬态形态不同：这里是**持久**失效，换端口才能根治）。

## 对策（必须动作）

1. **换全新端口**（如 4180/18080），不要重试旧端口；
2. vite dev（HMR 端口固定）在本机不可靠 → 改用构建产物 + 静态服务（`python3 mock-server.py <全新端口>`）；
3. 服务重启命令固化到脚本（`bash /tmp/start-mock4180.sh`），避免每次手敲；
4. 长存活端口（8080/18080）不受影响，生产链路不受此坑影响。

## 证据链

- 前端重构工作副本 `/root/frontend-redesign/DEV-NOTES.md`（2026-08-20）；`docs/lessons/process-wsl-loopback-fresh-listen-refused.md`（同源严重形态）。
- 状态：guarded（预览链路已固化换端口对策；本机 vite dev 不用）。