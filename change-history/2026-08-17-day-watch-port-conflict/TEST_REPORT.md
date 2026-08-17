# 测试报告

测试日期：2026-08-17（Asia/Shanghai）

| 检查 | 结果 | 说明 |
| --- | --- | --- |
| `node --check` day-watch.mjs | PASS | 语法通过 |
| `bash -n` local-cluster.sh | PASS | 语法通过 |
| WSL 内 18080 连续请求 | PASS | 4/4 全 200 |
| Windows 侧 8080 | PASS | 保持 200（用户入口不受影响） |
| day-watch PATCH 链路 | PASS | 50qps 生效、35qps 调回生效（异步收敛 ~15s） |
| 常驻重启 | PASS | PID 248009 运行中 |
