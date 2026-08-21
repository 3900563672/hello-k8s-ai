# WSL/Docker 安全重启 SOP + doctor 宿主残留检查（固化排查链路）

> 日期：2026-08-21 ｜ 关联：docs/operations/WSL_DOCKER_RESTART_SOP.md、docs/lessons/deploy-wsl-zombie-vhdx-lock.md

## 为什么做

- 2026-08-21 发生「WSL 彻底重启 → 僵尸锁链 → 强杀 vmcompute → 系统崩溃重启」事故后，教训只沉淀在 lesson，没有可执行的操作流程：下次遇到同样症状，仍要靠现场判断。
- 用户要求固化 SOP 并沉淀得非常详细，同时把「Docker 残留检查」做进 `make doctor`，让故障识别从人工回忆变成机器自动判定。

## 改成什么

1. `hack/doctor.sh` 新增第 2.1 节「宿主 VM 残留」：检查 vmwp / vmmemWSL 进程数量与 wsl.exe 响应性（timeout 兜底），孤儿进程且引擎不可达时直接 FAIL 并指向 SOP。
2. 新增 `docs/operations/WSL_DOCKER_RESTART_SOP.md`：现状快照 / 故障识别速查表 / 标准重启顺序（先停 Docker 再停 WSL，先启 Docker 再启 Ubuntu，每步含验证点）/ 四个异常分支 / 可杀与不可杀边界 / 外部已知 issue 对应。
3. `docs/operations/TROUBLESHOOTING.md` 增加「2.1 宿主层（WSL/Docker）故障速查」并更新 doctor 检查类目（8 类 → 9 类）。
4. `docs/getting-started/DEPLOYMENT.md` 增加 5.2 节「宿主层重启与自检」，指向 SOP。
5. 查重结论：与 microsoft/WSL#11082、docker/for-win#14024 / #14669 / #14827 / #14656 同源，不新开 issue。

## 关键行为

- doctor.sh 只新增只读检查（Get-Process / wsl -l --running），不改任何环境状态；FAIL 时只提示处置路径。
- SOP 为纯文档；环境恢复已在事故后完成（WSL / Docker / kind 集群健康）。

## 验证

- `bash hack/doctor.sh`：新增第 2.1 节 PASS（当前 vmwp=0、vmmemWSL 存在、distro 运行中、wsl.exe 正常响应）。
- `make docs-sync` / `make docs-check` 通过。
- `make preflight` 集群健康（5 节点 / 8 工作负载 / 3 PVC）。

## 回滚

- 删除 SOP 与 change-history 条目；doctor.sh 回退第 2.1 节（git revert 本提交）。
