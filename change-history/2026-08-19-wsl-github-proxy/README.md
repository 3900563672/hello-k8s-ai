# WSL GitHub 代理配置沉淀：检测脚本 + 蒸馏规则

> 日期：2026-08-19

## 为什么做
- 实测 WSL 直连 GitHub 全通道慢/断（https reset、SSH 5 分钟 11MB、codeload 60KB/s），但 Windows 侧 FlClash 代理正常；反复重试直连浪费大量时间。
- 排查发现 WSL2.7 可用 `127.0.0.1:<端口>` 直接访问 Windows localhost 代理，配置 git 后 186MB 秒下——规则值得沉淀为脚本 + lesson，避免下次重复排查。

## 改成什么
- 新增 `hack/wsl-github-proxy.sh`：检测代理端口（默认 7890/7897/10809/1080 @ 127.0.0.1）→ 配置 git 全局代理（仅 github.com）→ 测速输出；`--check` 只检测不改配置。
- 新增 `docs/lessons/process-wsl-github-proxy.md`：触发条件 / 现象 / 根因 / 可复用规则 / 验证方法。
- 新增 journal 条目 `docs/journal/2026-08-19-wsl-github-proxy.md`；lessons 速查表补一行。

## 关键行为
- git 代理只配 `http.https://github.com.proxy`（不影响 goproxy 等其他源）；GitHub 操作统一 https（SSH 不走 http 代理）。
- 本机 git 已配置生效：`http://127.0.0.1:7890`；`gh auth setup-git` 已配 https 凭据。

## 验证
- `bash hack/wsl-github-proxy.sh --check`：检测到 7890，github.com HTTP 200、1.3s。
- `git clone --depth 1 https://github.com/3900563672/kueue.git`：186MB 秒级完成。
- `make lint-sh lint-md` 通过（本条目交付检查）。

## 回滚
- `git config --global --unset http.https://github.com.proxy`；删除 `hack/wsl-github-proxy.sh` 与 lesson 文件即可。
