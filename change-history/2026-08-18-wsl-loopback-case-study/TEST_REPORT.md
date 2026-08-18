# TEST_REPORT：WSL 回环中继研究链

## 环境

- Windows 26200.9168 / WSL 2.7.8.0 / Ubuntu 26.04 / 内核 6.18.33.1-microsoft-standard-WSL2 / NAT 模式。
- 阶段一：故障态（dmesg UtilAcceptVsock 持续增长）。
- 阶段二：`wsl --shutdown` 恢复后的健康态。

## 探针验证（健康态，恢复后）

| 命令 | 结果 |
| --- | --- |
| `go run ./hack/wsl-loopback-probe`（默认单轮）×4 | PASS 4/4 |
| `go run ./hack/wsl-loopback-probe -attempts 3 -delay 0` | WARN 1/3（稳定复现健康态瞬态） |
| `go vet ./hack/wsl-loopback-probe` | 通过 |
| `gofmt -l hack/wsl-loopback-probe/` | 无输出（已格式化） |
| `bash -n hack/preflight.sh` | 通过 |

## 恢复验证（缺口②）

| 检查 | 结果 |
| --- | --- |
| Windows→WSL 新端口 echo 18530 | HTTP 200，~2ms |
| Windows netstat 新端口监听 | dllhost 37696 承载 |
| vsock host 50000/50001/50002/50005 | CONNECTED |
| dmesg UtilAcceptVsock 计数（健康态全程） | 0 |

## 旧探针推翻证据

- H1 健康对照：10 轮 t=0 立即拨号 10/10 必失败（健康态注册延迟 50–100ms）→ 旧探针设计缺陷，E1/E3a/E3b 客侧读数作废。
- H2/H2b：未监听端口 vsock 超时正常；已知存活端口 CONNECTED → vsock 通道活着，旧"vsock 失效"撤销。

## 未验证

- CI（GitHub Actions）上 selfcheck 新步骤实跑——探针在非 WSL 环境自动 SKIP，逻辑上不影响门禁，但未在 CI 实跑。
- 微软 issue 投稿：稿件已完成（仓库外 Documents/），未被官方回复。
