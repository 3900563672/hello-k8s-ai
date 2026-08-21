# WSL 访问 GitHub 慢/断连：优先走 Windows localhost 代理（FlClash 7890）

> 提升日期：2026-08-19 ｜ 来源：journal/2026-08-19-wsl-github-proxy.md ｜ 适用对象：本地 Agent / 远程 AI
> 触发条件（Use when）：WSL 内 git clone / push / 建 PR 慢或失败（Connection reset / fetch-pack disconnect / 几十 KB/s）；任何 GitHub 网络操作前

## 现象
- WSL 直连 GitHub 三个通道全慢/断：https clone `Connection reset by peer`；SSH clone 5 分钟 11MB 后 `fetch-pack: unexpected disconnect`；codeload tar 包约 60KB/s。
- 但 `api.github.com`（gh CLI 走 REST API）一直秒回——**API 快不等于 git 通道快**。
- Windows 侧代理（FlClash，混合端口 7890）正常；Windows 直连 GitHub 不慢。

## 根因
- WSL 流量默认不经过 Windows 代理软件；FlClash 只监听 Windows localhost（`::` 双栈 + 127.0.0.1:9090 控制口）。
- 本机 WSL2.7 实测：WSL 内 `127.0.0.1:<端口>` 可直接访问 Windows 上的代理（localhost 共享生效），无需 Allow LAN、无需网关 IP。
- SSH（22 端口）不经过 http 代理，所以 SSH clone 仍是直连慢速。

## 可复用规则（一条规则一句话，禁止复述现象）
- WSL 里访问 Windows 代理优先试 `127.0.0.1:<端口>`，不要先测网关 IP（`ip route` 的 gateway）；FlClash 常见混合端口 7890 / 7897 / 10809 / 1080。
- GitHub 操作统一走 https + 代理：`git config --global http.https://github.com.proxy http://127.0.0.1:7890` + `gh auth setup-git`（https 凭据）；SSH 不走 http 代理，禁用 SSH clone/push。
- 代理只配 github.com 域名（`http.https://github.com.proxy`），不影响 goproxy 等其他源。
- 遇到"git 慢/断但 gh api 快"先查代理，不要反复重试直连。

## 验证方法（命令 / 断言 / E2E；能自动化的给脚本路径）

```bash
bash hack/wsl-github-proxy.sh            # 检测代理端口 + 自动配置 git + 测速
timeout 10 curl -s -o /dev/null -w "%{http_code} %{time_total}s\n" -x http://127.0.0.1:7890 https://github.com
git clone --depth 1 https://github.com/<owner>/<repo>.git   # 走代理后应秒级完成
```
