# WSL 回环中继降级：完整排查案例 + 可复现探针（preflight/selfcheck）

> 日期：2026-08-18

## 为什么做

- 此前把"WSL 回环 TCP 当前整体不可用"作为环境遗留记入变更归档，断言模糊且未经核实。
- 用户要求核实该断言：实际排查后结论被修正——不是"整体不可用"，而是 WSL2 localhost 转发中继（guest 侧 `Relay`）降级，仅影响 IPv4 回环**新监听端口的首连**（0–300ms 间歇窗口）。
- 两个 Grafana 代理测试本地失败、CI 正常，一直缺少一个可复现、可自动化的判断手段；按"坑的终点是自动化"原则沉淀为探针。

## 改成什么

1. **面试案例文档**（人类可读）：`docs/operations/WSL_LOOPBACK_CASE_STUDY.md`——完整复现排查方法论：可复现探针 → 对照组设计（127.0.0.1 / 127.0.0.2 / eth0 / loopback0 / ::1）→ 排除法（raw socket / eBPF / iptables）→ 证据收敛（/proc/net/tcp 已 LISTEN 但 SYN 被 RST + dmesg `UtilAcceptVsock` 时间线）→ 区分环境问题与代码问题。诚实结论：定位 + 规避，修复 = 重置 WSL 网络栈。
2. **探针**：`hack/wsl-loopback-probe/main.go`——每次新建 `127.0.0.1` 随机端口并立即拨号 ×N，统计失败率；读取 dmesg 中 `UtilAcceptVsock` 错误计数；非 WSL 环境输出 `RESULT: SKIP`（CI 安全）。结果分级：PASS / WARN（间歇失败或历史错误）/ FAIL（全失败）。
3. **接入**：`hack/preflight.sh` 新增第 9 节（FAIL→bad、WARN→warn、PASS/SKIP→ok）；`make selfcheck` 新增 WSL 回环探针步骤。
4. **文档入口**：`docs/operations/TROUBLESHOOTING.md` 新增 3.3 节（症状 / 快速判断 / 处置 / 指向案例）；`docs/getting-started/DEPLOYMENT.md` 自动验收门补环境行；`docs/INDEX.md` 登记案例文档。

## 关键行为

- 探针只读、无副作用；`go run ./hack/wsl-loopback-probe` 秒级完成，不阻塞 `make preflight` / `make selfcheck`。
- 探针默认不设置退出码（结果由调用方决定门禁）：preflight 中 FAIL 阻止启动，WARN 仅提示。
- 当前本机实测：10/10 首连失败 + dmesg 计数 1620 → `RESULT: FAIL`，探针有效复现已知降级。

## 验证

- 本机：`go run ./hack/wsl-loopback-probe` 输出 FAIL（与已知降级一致）；`go vet ./hack/wsl-loopback-probe` 通过；`bash -n hack/preflight.sh` 通过；`make -n selfcheck` 解析通过。
- CI：探针在非 WSL 环境自动 SKIP，不影响现有 selfcheck 门禁。
- 未验证：整机重启 / `wsl --shutdown` 后的探针 PASS 恢复（需用户同意后执行）；CI 上 selfcheck 新步骤的实跑（本地无 CI 环境）。

## 回滚

- 移除 `hack/wsl-loopback-probe/`、`hack/preflight.sh` 第 9 节、`make selfcheck` 中对应步骤；文档同步删案例与入口（`TROUBLESHOOTING.md` 3.3、`DEPLOYMENT.md` 环境行、`INDEX.md` 行、`change-history` 本条）。
- 纯文档与工具链改动，无数据迁移、无运行时行为变化，回滚零风险。