# 变更总览：WSL 概率挂死根因修复——autoMemoryReclaim 配置失效 + doctor 动态阈值（#181）

> 日期：2026-08-22 ｜ 级别：P0

## 为什么做

- 近两天多次复现「有 Docker 就跑着跑着 wsl 命令概率挂死」：新建 `wsl` 会话无限挂起、`wsl --shutdown` 超时、30+ wsl.exe 客户端堆积，重启 wslservice 才能恢复（issue #181 完整证据链）。
- 根因 1（内存）：`.wslconfig` 的 `autoMemoryReclaim=gradual` 误放在 `[wsl2]` 段，WSL 要求放在 `[experimental]` 段；配置被静默忽略（事件日志「配置行6未知」），VM 内存只涨不收，长期顶在 12GB 上限。
- 根因 2（触发）：WSL 2.9 统一 VM 下 Docker 与 Ubuntu 共用 12GB，Docker Auto-Pause 恢复 + 构建/容器起停抖动触发分配路径卡死。
- 根因 3（启动竞态）：Docker Desktop 检测到 WSL 恢复会立即自动重启引擎，与 VM 首次启动赛跑，bootstrap 失败后重试 5 分钟（2026-08-22 21:08-21:14 实测）。

## 改成什么

1. `C:\Users\hh\.wslconfig`（Windows 宿主，不在仓库）：`memory=12GB` → `16GB`；`autoMemoryReclaim=gradual` 移到 `[experimental]` 段。
2. Docker Desktop `settings-store.json`（Windows 宿主）：新增 `autoPauseTimeoutSeconds: 31536000`（等效禁用自动暂停，消除暂停/恢复抖动）；`AutoStart=false` 已确认。
3. `hack/doctor.sh`：WSL VM 内存 WARN 阈值从硬编码 11.5GB/12GB 改为动态读取 `.wslconfig` 上限（当前 16GB → 15.5GB 告警），避免下次调配置再漂移。
4. 新增 `hack/wsl-vm-cap.ps1`：读取 `.wslconfig` 的 `[wsl2] memory=` 输出 MB（解析失败默认 16384），供 doctor.sh 调用。
5. `docs/agents/RESILIENCE.md` 3.5 节内存预算同步为 16GB 口径。

## 关键行为

- `make doctor` 内存检查：上限跟随 `.wslconfig`，当前 `vmmemWSL > 15.5GB` 才 WARN（此前 12GB 体系下 12.8GB 误报 WARN）。
- WSL 事件日志不再出现「wsl2.autoMemoryReclaim: ... 配置行6未知」。
- Docker 冷启动：WSL 稳定后 5 秒引擎就绪（此前 26s+，赛跑时 5 分钟重试）。

## 验证

- 重启 WSL 后事件日志仅剩网络回退提示，配置告警消失。
- `make doctor` 全 PASS，WSL VM 内存行显示 PASS（13112MB < 15500MB 阈值）。
- `shellcheck hack/doctor.sh` 通过；`make lint-ps1`（PSScriptAnalyzer）通过。
- Docker Desktop 重启两次：引擎 5s/5s 就绪，无错误对话框，无重试循环。
- dev 集群恢复：5/5 节点 Ready（docker start 恢复 Exited 节点容器）。

## 回滚

- `.wslconfig` 还原为 12GB + `[wsl2]` 段配置并 `wsl --shutdown`；`settings-store.json` 删除 `autoPauseTimeoutSeconds` 键并重启 Docker Desktop（备份 `settings-store.json.bak-20260822`）。
- 仓库侧：`git revert` 本条目即可还原 doctor.sh / wsl-vm-cap.ps1 / RESILIENCE.md。

## 未验证/待办

- 长跑压测（30 分钟多会话 + Docker 同时操作）未完成，建议后续观察。
- `make doctor` 内存阈值已动态化；`docs/agents/RESILIENCE.md` 中「Docker 内置 K8s 节点数」描述仍按旧口径，属历史遗留，未随本次改动。
