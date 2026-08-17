# kubectl 输出超过 1MB 会 ENOBUFS，脚本读取要分页或限长

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-16-cluster-and-deploy.md ｜ 适用对象：本地 Agent

## 现象

Pod 数多（100+）时 `kubectl get pods -o json` 在 Node.js `spawnSync/execFileSync` 中报 ENOBUFS。

## 根因

管道缓冲区溢出（约 1MB），同步子进程输出未被消费。

## 可复用规则

- 大输出用 `--no-headers` + 字段裁剪，或分页（`--chunk-size`），或写文件再读。
- 脚本里对 kubectl 输出量做上限假设（按 Pod 数估算），超过则换采集方式。

## 验证方法

构造大列表场景跑采集脚本，确认无 ENOBUFS；错误日志里不再出现该报错。
