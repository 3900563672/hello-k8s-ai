> 日期：2026-08-19 ｜ 触发者：本地 Agent ｜ 相关：change-history/2026-08-19-wsl-upgrade-validation/、Desktop/WSL 34 号文档

## 现象

- #71 执行中先做源码级验证：GitHub compare 证明 #41051/#41125 不在最新 stable 2.7.12、在 2.9.5 → 跳过 stable 直接走 Preview。
- 预览通道实际装到 2.9.4.0（2.9.5 tag 存在但无 release 资产）→ 探针 40 轮 ok=5/40（STALL 87.5%，与 2.7.8.0 基线 91.5% 同区间）→ **窗口仍在**，版本范围扩展至 2.9.4。
- 回滚时踩坑：`Add-AppxPackage` 只换 Store 身份不换 `C:\Program Files\WSL\` 二进制（0x80073D06 → 0x80073D28 → 补装 x64 MSI 才恢复），已实测并修正 19 号 3.2。

## 处理

- 环境已回滚至 2.7.8.0（wsl.exe/DLL/内核指纹与升级前完全一致）；集群恢复后台执行中（cluster-restore.log）。
- 2.9.5 可用性每日只读监控已挂入 CodexWatch41286 计划任务（watch-wsl295.sh）。
- #71/#63 保持 open，等 2.9.5 推送后闭环。