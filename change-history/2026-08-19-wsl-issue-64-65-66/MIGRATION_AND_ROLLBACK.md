# MIGRATION_AND_ROLLBACK

## 迁移

无。纯研究沉淀：新增 research/exp/ 实验源码与 change-history 条目；未改任何运行组件。

## 回滚

1. 删除 `research/exp/` 与 `change-history/2026-08-19-wsl-issue-64-65-66/`。
2. 清理桌面/临时文件（可选）：%TEMP% 下 wpr-*、tw-*、t1-*、churn*、wsl-tcp-loopback.etl。
3. 实验进程已全部退出（tw_passive_listener / tw_rebind / churn_heavy）；WPR 会话已停止。

## 风险

- 无运行时影响。抓包仅 3 分钟最小 profile，无系统配置变更。
- 退化态复现实验（未来）需注意：数小时压力可能触及 C 盘/内存占用，参考 19 号回滚文档的 C 盘防护。
