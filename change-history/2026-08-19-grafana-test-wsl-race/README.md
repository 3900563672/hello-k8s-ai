# Grafana 代理测试 WSL 注册竞态修复（Fixes #73）

> 日期：2026-08-19 ｜ 关联：docs/operations/WSL_LOOPBACK_CASE_STUDY.md、32 号文档

## 为什么做

- 本机 WSL 环境下 `TestGrafanaProxyPreservesSubPathAndForwards` / `TestGrafanaProxyRootPath` 偶发失败：新端口存在注册时序竞态（bind 返回先于 Windows listener 就绪 ~200ms，t+0 拨号必 refused），直接请求被误报成代理 502；退化窗口内新端口数分钟不可达（200 轮直方图 STALL 91.5%）。

## 改成什么

1. 新增测试 helper `waitBackendReady`：轮询拨号直到后端可达（8s 上限），再进入断言。
2. WSL 退化窗口内超时 → `t.Skipf`（带案例文档与升级指引），不再误报；CI 原生 Linux 无 WSL 竞态，仍严格失败，不弱化回归防护。
3. 新增 `onWSL()`：`/proc/version` 含 microsoft 内核标识即判定为 WSL。

## 关键行为

- 仅测试代码改动（`dashboard/backend/internal/api/grafana_proxy_test.go`），生产代码未动。
- 本地验证：健康窗口 PASS（~0.1s）；退化窗口 SKIP（8s 后带原因）；5 连跑不再出现 502 误报。

## 验证

- `go vet ./internal/api/` 无告警；`go test ./...` 全绿（本地，退化窗口内以 SKIP 呈现）。
- CI（ubuntu-latest 原生 Linux）：两个测试应直接 PASS。

## 回滚

- 删除 `waitBackendReady` / `onWSL` 与两处调用即可，零运行时影响。
