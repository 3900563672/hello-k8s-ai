# WSL #41286 follow-up 评论 v4 正式发送（编辑 v3，不新增）

> 日期：2026-08-19

## 为什么做

- 官方 #41286 评论 v3（issuecomment-5329767813）有两处软肋：DLL 版本锚定是"无法确认"、剂量响应只有现象没有机制；#65（指纹）与 #64（参数实验）闭环后，v3 已过时可升级。
- 用户要求"编辑不新增"，避免刷屏且不改变评论 ID/链接；编辑不重启观察窗口（窗口自定为 09-01）。

## 改成什么

1. **正文整体替换为 v4**（10163 字符）：删掉 "cannot verify the installed wsldevicehost.dll contains exactly this version"，替换为已确认锚定 openvmm commit 0bb5cf75（tcp 27/27、udp 7/7、dns_tcp 2/2）。
2. **定量机制**：10.4ms/bind 固定同步链、延迟≈并发×10.4ms、128 并发 p50=1.32s 且 >1s 占 99.27%、吞吐恒定 ~96 bind/s。
3. **诚实修正**：60s 窗口语义（竞态保护而非保留期）、TIME_WAIT 81~187s（分关闭方向）、WSAEADDRINUSE 是同四元组非端口、撤销"每进程隔离"断言。
4. **补充退化态观察**：持续人工压力后进入"注册不再物化"状态，wsl --shutdown 恢复；明确声明非自然触发。
5. **正文遵守外部编号禁令**：不含任何 GitHub issue/PR 编号，避免自动 cross-reference。

## 关键行为

- 流程：本仓库演练 issue #68 渲染检查（首尾完整、无乱码、与定稿仅差尾部空行）→ `gh issue delete 68` 删除 → 正式 `gh api PATCH`。
- 发送方式：`gh api -X PATCH repos/microsoft/WSL/issues/comments/5329767813 --input payload.json`（jq --rawfile 构造）。
- 验证：回读正文与定稿 diff 仅差尾部空行；无 U+FFFD；无本地路径泄漏。
- 官方响应入口：microsoft/WSL#41286 评论区通知；无响应则按既定 09-01 观察窗口决定下一步。

## 验证

- 演练渲染：issue #68（已删除）body=10163 字符，首尾完整。
- 正式发送：updated_at 2026-08-18T17:13:48Z（UTC），id/url 不变，作者 3900563672。
- 证据：Desktop\WSL\24_comment_v4_sent.md + 24_comment_v4_sent_body.md（仓库 Documents/ 同步副本，gitignore 不入库）。

## 回滚

- 恢复 v3：用 Desktop\WSL\13b_comment_body_v3.md 再次 PATCH 同 ID 评论。
- 观察窗口自定 09-01，编辑不重启窗口；若官方回复要求调整，再走一次"演练 → PATCH → 回读"流程。
