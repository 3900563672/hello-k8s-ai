# WSL 重启后排除测试 + 探针工具缺陷修复（32 号文档）

> 日期：2026-08-19 ｜ 关联：issue #62、31 号文档（版本缺陷区间）、15/19 号恢复 SOP、Desktop\WSL\32_重启后排除测试与探针修复.md（仓库 Documents/ 同步副本，gitignore 不入库）

## 为什么做

- 用户整机重启（09:27）后，要求先验证环境符合预期再恢复工作；同时按 #62 闭环“重启后 Docker 排除测试 + 健康态 200 轮直方图”。
- 排除测试过程中发现 wsl-localhost-probe 的 Windows 侧校验自 08-18 起从未真正工作，历史 `win=UNREACHABLE` 读数全部作废——必须先修工具，再谈结论。
- Grafana 两个测试本地失败的旧解释（“网络栈残留 / 回环整体不可用”）被 race 测试推翻，机制精确化为注册时序竞态。

## 改成什么

1. **A/B/C 三轮排除测试（#62）**：Docker ON（旧探针）→ OFF → ON，probe 成功率、mix 窗口、Grafana 3/3 FAIL、固定端口地面真值行为完全一致 → Docker 排除；整机重启不消除健康态端口 0 注册失败窗口（探针 1/8~3/8 成功）。
2. **探针修复（probe_repro.go）**：①补 Accept goroutine（读请求 + 响应 200）；②`curl.exe -o NUL` 替代 `-o /dev/null`（Windows 侧 rc=23 写错误）；修复后首见 `win=ok:45ms`。
3. **Grafana 测试失败机制精确化**：注册时序竞态（bind 返回先于 Windows listener 就绪 ~200ms，t+0 拨号必 refused），非网络栈残留；CI 无此竞态。
4. **集群恢复**：Docker 重启后 5 节点 tmpfs 覆盖回来 → umount + chown + 删 Pod 重建 → 数据完好（PostgreSQL resource_events 560,684 增长中）；端口 8080/18080 转发重建。
5. **文档修正**：32 号文档（Desktop/WSL + 仓库 Documents/ 镜像）；面试详案 E13/E14 + 13.6；lessons 与 operations 案例文档同步修正“修复 = wsl --shutdown”与“网络栈残留”表述。

## 关键行为

- 全程未改业务代码、未动 WSL 版本；Docker 启停为 #62 明确要求的对照条件，启停前后各完成一轮探针 / 地面真值验证。
- 探针源码与三轮日志归档：Desktop\WSL\logs\reboot-exclusion/、Desktop\WSL\research\probe_repro.go。
- 历史结论作废原则：凡是工具缺陷导致的读数（win=UNREACHABLE）一律不当作环境证据；地面真值 = 真实服务 + Windows netstat/curl。

## 验证

- A/B/C 三轮日志完整（roundA/B/C_*.txt）；race 测试 t+0 refused / t+200ms OK。
- 修复后 wincheck 首见 `win=ok:45ms`；200 轮直方图（-rounds 200 -timeout 30s -wincheck）结果见 logs/reboot-exclusion/histogram200_dockerON.txt。
- 集群 5 节点 Ready、8 系统 Pod Running、Dashboard/Grafana 200。
- 未验证：WSL 2.9.5+ 升级后端口 0 窗口消失（闭环动作，需用户拍板）；Grafana 测试健壮性修复（需用户拍板是否改仓库代码）。

## 回滚

- 纯文档 + 只读实验：删除 32 号文档与 change-history 条目即可回滚；探针修复在 Desktop 研究目录，与仓库代码无关。
- 集群恢复动作（umount/chown/删 Pod）已有 15/19 号 SOP 可重做；数据在 PVC 底层 /var volume，未破坏。
