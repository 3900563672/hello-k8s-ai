# WSL 回环研究：openvmm/WSL “为什么这么写”git 溯源 + 版本缺陷区间勘误（31 号文档）

> 日期：2026-08-19 ｜ 关联：docs/journal/2026-08-19-wsl-param-experiments.md、Desktop\WSL\31_openvmm_history_trace.md（仓库 Documents/ 同步副本，gitignore 不入库）

## 为什么做

- 用户要求对 openvmm 历史提交做“为什么这么写”溯源：回答 TIME_WAIT 无 timer、禁 SYN 重传、无 SO_REUSEADDR、排他锁等设计是历史包袱还是刻意选择。
- 溯源过程中发现既有研究文档（27/28/29 号）的版本缺陷区间归因有误：#40187 被错误归为“2.7.8.0 不含”，且 #41125/#41051 的首发版本被误写为“2.7.11 起”。勘误直接改变投稿叙事锚点，需立即沉淀并同步面试详案。

## 改成什么

1. **git 溯源（只读，两仓库全史）**：
   - openvmm：TIME_WAIT timer TODO（tcp.rs:2006/2022）、禁 SYN 重传（windows.rs:28-50 SIO_TCP_INITIAL_RTO）、无 SO_REUSEADDR + listen(10)（TcpListener::new）全部始于初始导入 304657e9（2024-10-03）；公开历史 0 条 TIME_WAIT/SO_REUSEADDR 相关提交。net_consomme TCP 状态机为自研（smoltcp 仅 wire/checksum）。
   - microsoft/WSL：GnsPortTracker 5 个提交逐一定位（#14333/#40187/#40287/#41125/#41051），每个对应一个真实报告场景；排他锁自初始开源提交 697572d6 起存在。
2. **版本缺陷区间勘误**：
   - #40187（inline 端口 0 解析）在 2.7.8.0 guest 侧 tracker 中**已含**（guest 侧文件，不在 host DLL）——修正 27/28/29 号文档。
   - 2.7.8.0 真正缺失：#41051（PIDFD_THREAD，线程 bind 追踪）+ #41125（listen 隐式绑定）；首发均为 2.9.5（2026-08-11），2.7.x 全系与 2.8.x 不含——修正 27 号“2.7.11 起”表述。
   - **ETW 证据（host 侧 0 个 port request 事件）的直接机制对应 = #41051**：非主线程 bind(0) → pidfd_open(tid,0) 失败 → 注册请求从未发出；Go 探针跑在 runtime 非主线程，链条自洽。
3. **文档同步**：新增 31 号文档（Desktop/WSL + 仓库 Documents 镜像）；27/28/29 号追加勘误节；面试详案 E11/7.1/8.3 三处版本表述修正（Documents/ 本地副本）。

## 关键行为

- 全程只读：git log/blame/log -L/tag --contains/merge-base 验证，未改任何源码、未动环境、未重启。
- 版本判定方法：`git merge-base --is-ancestor <commit> <tag>` + `git tag --contains`；本机 WSL 2.7.8.0（tag 2026-06-05）、wsldevicehost.dll v1.2.14.0（构建 04-20，net_consomme 锚定 0bb5cf75）。
- 勘误原则：历史文档正文不改写，追加“勘误（31 号）”节保留可追溯链。

## 验证

- 31 号文档已写 Desktop/WSL 与仓库 Documents/（gitignore 不入库），27/28/29 号两处同步完成。
- 面试详案三处替换全部命中，“缺 #40187”残留 0 处。
- 未验证：2.9.5+ 升级后探针成功率恢复（闭环动作，需用户拍板装新版 WSL）；guest 侧 pidfd 日志（超出本机可观测范围）。

## 回滚

- 纯研究沉淀：删除 31 号文档 + 各勘误节即可回滚，零运行时风险；仓库侧条目删除同目录即可。
