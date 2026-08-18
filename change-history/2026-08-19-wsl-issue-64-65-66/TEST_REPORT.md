# TEST_REPORT：命令与真实结果

## 环境确认

```text
2026-08-19 00:23:40 CST, up 5:52, load 0.01（健康态）
```

## 实验 1/2（既有，复核）

- `cd /root/research/exp && go run bind_latency.go seq 200` → p50=10.4ms
- `go run bind_latency.go churn 200`（8 goroutine）→ p50=93.3ms
- `go run sixty_second_window.go` / `sixty_no_rebind.go` → listener <2s 释放

## 实验 3b（TIME_WAIT）

```text
tw_rebind 45681：L0_listen 00:28:23.307 → L1_accepted 00:28:26.635 → REBIND_OK #1 elapsed=1.326s
netstat：00:28:28 TIME_WAIT 可见 → 00:29:47 仍在 → 00:30:27 消失（约 81~121s）
tw2：55552 close 00:31:58.878 → +145s REBIND_FAIL(10048) → 00:35:0x 消失（约 145~187s）
```

## 实验 4（极端 churn）

```text
./churn_heavy 64 100：p50=666.2ms max=734.6ms stalls=0% throughput=96/s
./churn_heavy 128 100：p50=1.322s max=1.420s stalls=12707/12800=99.27% throughput=96/s
```

## 实验 5（WPR 抓包）

- wpr -start（提权）→ 27MB ETL → Get-WinEvent 44598 事件
- T1：R55554/R55556 两轮 `Duplicate TCB` 拒绝，12ms WSAEADDRINUSE，无 SYN
- 基线 20 连全部 0ms
- 8080 对照：RexmitCount=4，RTO 300ms→6s

## 未验证

- 退化态黑洞（>30s 无响应）抓包：健康态不可复现，需退化态（数小时压力/故障态）
- BindEndpoint 事件数与 bind 数差异（744/6400）根因
- TIME_WAIT 时长分布多轮采样（每轮约 3 分钟）
