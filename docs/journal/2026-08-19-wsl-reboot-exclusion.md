> 日期：2026-08-19
# WSL 重启后排除测试与探针修复（32 号，2026-08-19）

## 背景

用户整机重启（09:27），要求先验证环境符合预期再恢复工作；同时闭环 #62（重启后 Docker 排除测试 + 健康态 200 轮直方图）。

## 执行流水

- 11:30 环境检查：kind 5 节点 Ready、系统 Pod 8/8；端口转发重建（8080 dashboard / 18080 grafana）。
- 11:35 A 轮（Docker ON，旧探针）：probe 4/8 快 + 4 STALL；mix 40/40 OK；Grafana 3/3 FAIL。
- 11:42 B 轮（Docker OFF）：probe 3/8 成功（win=ok 43-64ms）+ 5 STALL；mix 40/40 OK；Grafana 3/3 FAIL。
- 11:49 探针修复：补 Accept goroutine（读请求 + 响应 200）+ `curl.exe -o NUL`（原 `-o /dev/null` 在 Windows 侧 rc=23 写错误）。
- 11:55 C 轮（Docker ON 重启后）：probe 1/8 成功 + 7 STALL；zero 18/20 FAIL、explicit 20/20 OK；Grafana 3/3 FAIL。
- 11:56 启动 200 轮直方图（`-rounds 200 -timeout 30s -wincheck`），预计 13:00-13:30 完成。
- tmpfs SOP 复跑：Docker 重启后 5 节点 tmpfs 覆盖回来 → umount/chown/删 Pod 重建 → 数据完好（PostgreSQL resource_events 560,684 且持续增长）。

## 关键发现

1. **Docker 排除**：A/B/C 三轮行为完全一致（Docker ON/OFF 无差异）→ Docker 不参与本故障。
2. **探针 wincheck 从未真正工作**（缺陷已修）：只 Listen 不 Accept + `-o /dev/null` 退出码 23 → 历史 `win=UNREACHABLE` 读数全部作废；修复后首见 `win=ok:45ms`。
3. **Grafana 测试失败机制精确化**：注册时序竞态（bind 返回先于 Windows 侧 listener 就绪 ~200ms，t+0 拨号必 refused），"网络栈残留"解释作废。
4. **整机重启不消除健康态端口 0 注册失败窗口**：重启后 2h 内探针仍 1/8~3/8 成功；修复方向 = 升级 WSL 2.9.5+（#41051/#41125，31 号锚定）。

## 遗留

- 200 轮直方图归档（跑完后更新 32 号文档与本文）。
- 退化态黑洞复现 + 同步抓包（需用户睡前授权，夜间执行）。
- #63 WSL Preview 复现、2.9.5+ 升级验证（需用户拍板）。
- 26 号缓存预热对照实验（会产生 churn，必须等直方图完成后执行）。

## 证据

- Desktop\WSL\logs\reboot-exclusion/（roundA/B/C 全部日志 + histogram200_dockerON.txt）
- Desktop\WSL\research\probe_repro.go（修复后探针源码）
- change-history/2026-08-19-wsl-reboot-exclusion-test/README.md（32 号归档）
