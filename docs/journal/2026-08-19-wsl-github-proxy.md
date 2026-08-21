# 2026-08-19 WSL 访问 GitHub 慢/断连：走 Windows FlClash 代理解决

> 日期：2026-08-19 ｜ 触发者：本地 Agent ｜ 相关：open-source-contributions（kueue fork）

## 现象（1-2 行）
- WSL 直连 GitHub：https clone `Connection reset by peer`、SSH clone 5 分钟仅 11MB 后 `fetch-pack disconnect`、codeload 约 60KB/s；但 `api.github.com` 一直很快。
- Windows 侧 FlClash 代理正常（混合端口 7890），Windows 直连 GitHub 不慢。

## 上下文（当时在做什么）
- 为 kueue #14548 认领后的 fork + clone 做准备；WSL2.7 实测 WSL 内 `127.0.0.1:7890` 可直接访问 Windows 上的 FlClash（localhost 共享生效）；之前测网关 IP 192.168.10.1 全不通是因为代理只监听 Windows localhost。

## 处理（结果 + 是否已解决）
- 已解决：`git config --global http.https://github.com.proxy http://127.0.0.1:7890` + `gh auth setup-git`，https 浅克隆 186MB 秒下。
- 规则已提升：lessons/process-wsl-github-proxy.md；一键脚本：hack/wsl-github-proxy.sh。
