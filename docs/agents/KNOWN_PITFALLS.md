# 已知坑位清单

> 维护层：agents ｜ 最后同步：2026-08-17 ｜ 对应变更：change-history/2026-08-17-night-run-first-execution/
> 记录格式：现象 → 原因 → 解决 → 验证 → 日期。新坑按日期倒序追加到对应主题。
> 这是"踩坑即记录"的流水账，不替代 `docs/` 的正式说明。
> 领域层面的"已知易误判点"（来自原 AI_CONTEXT 第 8 节）见本文最后一节。

## 夜间长时运行（night-run）
### 2026-08-17 宿主机空闲 15 分钟自动睡眠会冻结 WSL（值守事故根因）
- 现象：2026-08-17 00:50 后值守会话与 keepalive 全部停滞约 7 小时，07:48 恢复；keepalive.log 无新检查记录、sleep 等待命令 7 小时不返回；恢复后系统本身无故障（18 Pod 全 Running）。
- 原因：Windows 电源计划"平衡"下交流空闲 15 分钟自动睡眠（powercfg 实测 STANDBYIDLE AC=900s）、3 小时自动休眠（HIBERNATEIDLE=3h）；睡眠冻结整个 WSL VM。
- 解决：已落地 `hack/night-run/sleep-guard.ps1/.sh`（方案 A，2026-08-17 预授权执行 `on`，当前 guard=on）：`bash hack/night-run/sleep-guard.sh status|on|off`，on/off 走 UAC 提权并写 `%TEMP%\sleep-guard.log`；原值存 `%LOCALAPPDATA%\night-run-sleep-guard.json`。提示词前提从"App 必须保持运行"扩展为"App 运行 + 宿主机不睡眠"；PowerToys Awake 可作免管理员备选（方案 B，未选）。
- 注意：`powercfg /change` 只认 `standby-timeout-ac`/`hibernate-timeout-ac` 专用名，不认 `STANDBYIDLE` 等 SUB_SLEEP GUID 别名（实测报"参数无效"）。
- 验证：07:48 唤醒后系统自动恢复，队列自动回排；未实测禁用睡眠后的整夜值守。
- 备注：合盖行为由"合盖操作"设置单独控制，改空闲睡眠不影响合盖逻辑。

### 2026-08-17 nohup 挡不住 exec 会话进程组回收，keepalive 必须 setsid 启动
- 现象：`nohup node keepalive.mjs --loop ... &` 启动的常驻进程（PID 120061）在命令会话结束后静默消失，日志无第二轮检查、无错误输出。
- 原因：Codex exec 在命令返回后回收该会话的进程组，nohup 只能忽略 SIGHUP、挡不住进程组终止。
- 解决：`setsid nohup node ... < /dev/null >> log 2>&1 &`（setsid 脱离会话）；已在 phase_a_prompt.md 与 hack/night-run/README.md 同步。
- 验证：setsid 重启（PID 122828）跨命令存活，首轮检查全绿。

### 2026-08-17 snapshot.mjs 漏定义 sleep 导致采集崩溃（node --check 查不出）
- 现象：`node hack/night-run/snapshot.mjs --once --summary` 在 port-forward 偶发失败走重试路径时崩溃：`ReferenceError: sleep is not defined`（snapshot.mjs:47），整份快照丢失。
- 原因：从 keepalive.mjs 拷贝 httpGet 时漏拷 `const sleep` 辅助函数；`node --check` 只查语法不查运行时引用，所有 fetch 首试成功时脚本可跑通，掩盖了 bug。
- 解决：snapshot.mjs 补 `const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));`。
- 验证：修复后实跑 `snapshot.mjs --once --summary` 成功（2026-08-16T23:55:54Z 快照 ok:true）。
- 备注：同类脚本改动后除 `node --check` 外，至少实跑一次触发重试路径（或直接跑通一次）。


### 2026-08-16 kubectl port-forward 偶发连接复用失败
- 现象：Node fetch 连续请求 Backend 时偶发 `TypeError: fetch failed`，curl 同时刻正常。
- 原因：port-forward 单监听 127.0.0.1:8080，Node 长连接复用被后端/隧道偶发重置。
- 解决：脚本 httpGet 网络层错误自动重试 3 次（间隔 500ms）；keepalive 启动阶段健康探针失败最多恢复 3 轮（间隔 10s）。
- 验证：重试后快照/健康检查稳定输出全绿。

### 2026-08-16 自动化会话模型：cron 型每次触发都是全新会话
- 现象/原因：project 型 cron 自动化按 rrule 触发新会话，不复用既有会话；thread heartbeat 型才会复用指定线程。
- 解决：提示词必须自包含（先读 AGENTS.md / README / phase prompt / problems.md），信息落仓库与 `.runtime/`；不要假设新会话有对话记忆。
- 验证：`$CODEX_HOME/automations/<id>/automation.toml` 逆向 app.asar 确认 `kind="cron"` + `target={type="project"}` 语义。

### 2026-08-17 自动化 TOML 必须显式写 model，否则用默认模型（自定义 Provider 会失败）
- 现象：`night-run-phase-a-keepalive` 00:00 触发后只创建了线程（`state_5.sqlite` threads 表可见 `01a00b4d`，`model` 列为 None），turn 未启动，任务"没跑起来"；UI 显示模型为 SOL。
- 原因：用户全局 `config.toml` 是自定义 Provider（`model_provider="deepseek"`、`model="deepseek-chat"`），但手写 automation.toml 未带 `model` 字段，自动化会话回落到 App 默认模型（SOL，用户实际不可用），线程启动失败。
- 解决：automation.toml 显式加 `model = "deepseek-chat"`、`reasoning_effort = "max"`（与全局配置一致）。
- 验证：tomllib 解析通过；最早实测点为 04:30 Phase B 空跑（若线程 `model` 列为 deepseek-chat 即生效），其次次日 00:00 Phase A。
- 备注：用户"先创建会话再指定会话"的 heartbeat 方案可作备选（复用线程模型、不涉及默认模型），但会累积线程上下文。

### 2026-08-16 自动化触发前提：Codex 桌面 App 必须保持运行
- 现象：App 退出/合盖时到点不触发自动化。
- 解决：夜间运行前确认 App 保持运行；Phase A 开工先拉起 nohup 常驻 keepalive，会话中断脚本仍继续。
- 验证：首次夜间运行（2026-08-17 00:00）实测。

### 2026-08-16 Windows 宿主没有 gh，GitHub 操作必须在 WSL 内执行
- 现象：Codex 自动化/会话的工作目录是 `\\wsl.localhost\...`（UNC），Windows 侧 `gh` 不存在（`C:\Program Files\GitHub CLI\gh.exe` 也没有）。
- 解决：一切 git/gh 操作走 `wsl -d Ubuntu -- bash -lc "..."`；WSL 内 `gh` 已认证（account 3900563672，scopes 含 repo）。
- 验证：PR #24 创建成功。

### 2026-08-16 gh 偶发 TLS handshake timeout（代理链路）
- 现象：`gh pr create` / GraphQL 请求偶发 `TLS handshake timeout`，重试即成功；与 WSL http_proxy=127.0.0.1:7890（Clash）链路有关。
- 解决：失败后等 5–8 秒重试，最多 5 次；不要怀疑认证或重复创建 PR（会报 "a pull request already exists"）。
- 验证：PR #24 第 2 次尝试成功。

## 命令与终端（WSL / Windows 宿主）

### 2026-08-16 apply_patch 无法写 UNC 路径
- 现象：在 `\\wsl.localhost\Ubuntu\...` 工作目录用 `apply_patch` 写文件，报 "UNC paths are not supported. Defaulting to Windows directory." 后 "Access is denied"。
- 原因：apply_patch 底层走 cmd.exe，不支持 UNC 当前目录。
- 解决：写新文件用 PowerShell here-string + `[System.IO.File]::WriteAllText`（UTF-8 无 BOM）；复杂脚本内容多时 base64 传 WSL。
- 验证：本次 UI 验证链路文档与脚本全程使用该方式。

### 2026-08-16 PowerShell 直传 wsl.exe 引号被拆
- 现象：`wsl.exe -d Ubuntu -- bash -lc "..."` 内含引号或括号时 bash 报 `syntax error`。
- 原因：Windows 到 wsl.exe 的原生参数传递会重排引号，内部双引号被拆开。
- 解决：脚本内容 base64 编码后 `echo <b64> | base64 -d > /tmp/x.sh && bash /tmp/x.sh`；含中文或引号的脚本一律走 base64。
- 验证：本次文档体系交付全程使用该方法无失败。
- 备注：wsl 启动时的 "localhost 代理未镜像" 是环境噪音，可忽略。

### 2026-08-16 git commit 中文提交信息丢失
- 现象：PowerShell 传参提交后，GitHub 上提交标题只剩英文前缀。
- 原因：Windows → WSL 参数编码吞掉非 ASCII 字符。
- 解决：`echo "提交信息" | base64 -w0 > /tmp/msg.b64 && git commit -F <(base64 -d /tmp/msg.b64)`。
- 验证：`dc5308e`、`89916cc` 中文完整。

### 2026-08-16 Docker Desktop WSL2 数据盘迁移：DataFolder 配置无效，必须用 Junction
- 现象：C 盘 vhdx 占 112GB；在 `settings-store.json` 设置 `DataFolder=D:\DockerData` 并重启后，`docker ps` / `docker volume ls` 全部为空。数据没丢，只是引擎挂到了默认位置的新空盘。
- 原因：Docker Desktop 的 WSL2 引擎硬编码在 `%LOCALAPPDATA%\Docker\wsl\disk` 找 `docker_data.vhdx`，忽略 DataFolder 配置。
- 解决：把 vhdx 移动到目标盘后，在 `%LOCALAPPDATA%\Docker\wsl\disk` 建立指向新位置的目录 Junction。
- 验证：`dir C:\Users\hh\AppData\Local\Docker\wsl\disk` 可见 vhdx 且 LinkType=Junction；`docker volume ls` 恢复 15 个卷。
- 备注：以后容器/卷显示为 0，先查 Junction 与目标 vhdx 是否存在，不要贸然重置 Docker。

### 2026-08-16 禁止 wsl --shutdown：会关闭所有发行版
- 现象：为"清理空间"执行 `wsl --shutdown`，用户的 Ubuntu（跑着 Agent 与项目）被一起关掉。
- 解决：除非用户明确同意，只对单个发行版操作（`wsl -d Ubuntu -- ...`），不执行 `wsl --shutdown`。
- 备注：用户环境约束：不要重启电脑（有 Agent 项目在跑）；确需重启必须先征得同意。

### 2026-08-16 强杀 Docker Desktop 后内置 Kubernetes 不会自动恢复
- 现象：Stop-Process 强杀 Docker Desktop/com.docker.backend 后，desktop-control-plane、desktop-worker* 等内置 K8s 容器全部消失，kubectl 连不上。
- 解决：不要强杀 Docker Desktop；重启走正常流程。恢复需在 Settings → Kubernetes 重新启用（本次随 Docker Desktop 正常重启自愈）。
- 验证：恢复后 10 个节点 Ready，kubectl cluster-info 正常。

### 2026-08-16 非提权进程写不了 D:\ 根目录
- 现象：普通进程在 D:\ 创建目录失败。
- 原因：ACL 只有 Administrators 可写、Everyone 只读。
- 解决：UAC 提权创建目录并 icacls 授权；提权操作写成 .ps1 脚本，用 `Start-Process -Verb RunAs` 执行，脚本写日志到 %TEMP% 后轮询确认。

### 2026-08-16 PowerShell 复杂内联命令被安全策略拦截
- 现象：含 Remove-Item 组合、多层引号嵌套的内联 PowerShell 被安全策略拦截。
- 解决：删除/移动文件改用 Node.js fs 模块；复杂逻辑写成 .ps1 脚本文件再执行。

### 2026-08-16 系统代理（127.0.0.1:7890）不能动
- 现象：代理配置被改后网络全断。
- 解决：保持 ProxyEnable=1 不变；Docker Hub 拉取失败多为瞬时故障（auth.docker.io 超时），预拉 + 重试即可，不要改代理。

## Go 构建与 CRD 生成

### 2026-08-16 gofmt 对齐问题被 lint 拦下
- 现象：`internal/controller/simulationclock_controller_test.go` 等 3 个文件报 "File is not properly formatted (gofmt)"。
- 原因：手工编辑导致对齐与 gofmt 不一致；lint 使用自定义 golangci-lint（`.custom-gcl.yml`）。
- 解决：`make fmt`；lint 前先 `make golangci-lint` 编译带自定义插件的二进制。
- 验证：修复后 `make lint` 通过。

### CRD 修改后必须重新生成
- 现象：只改 `api/v1/*_types.go` 会导致清单与生成结果不一致。
- 解决：`make manifests generate YEAR=2026`；`config/crd/bases/*.yaml`、`config/rbac/role.yaml`、`zz_generated.deepcopy.go` 只由生成器维护，不手改。
- 验证：CI "源码与部署验证" 会核对生成一致性。

## GitHub API 与 gh

### 2026-08-16 gh 偶发 TLS handshake timeout / EOF
- 现象：批量创建 label、编辑 issue 时部分请求失败。
- 解决：失败项重试即可；批量脚本要容忍失败，最后统一核对清单。
- 验证：重试后 17 个 label、5 个 issue 标签全部就位。

### gh 权限不足不等于无数据
- 现象：Projects v2 字段查询报权限错误；SSH keys 列表 404。
- 原因：token scopes 缺 `read:project`、`admin:public_key`。
- 解决：先查 `totalCount` 判断"有没有"；缺 scope 需 `gh auth refresh` 或重新授权，不要根据报错断定无数据。
- 验证：`projectsV2 totalCount=0` 确认真无 Projects。

## GitHub Actions 与 CI

### 2026-08-16 golangci-lint 预置普通二进制会跳过自定义插件编译
- 现象：CI 直接把官方 golangci-lint 二进制放到 `bin/` 后，`make lint` 不再编译 `.custom-gcl.yml` 里的 logcheck 插件，lint 语义悄悄变化。
- 原因：Makefile 的 `golangci-lint` 目标以文件存在性判断是否安装；v2.12.2 官方 release 没有 with-plugins 预编译资产。
- 解决：CI 中先用官方二进制执行 `golangci-lint custom --destination bin --name golangci-lint-custom` 再 `mv` 覆盖 `bin/golangci-lint`，与本地 `make lint` 一致；`bin/` 用 actions/cache 缓存（key 含 `.custom-gcl.yml` 哈希）。
- 验证：golang 容器内完整复现 CI 步骤，`golangci-lint run` 0 issues。

### 2026-08-16 CI 里 go install kind 每次都要编译
- 现象：E2E workflow 用 `go install sigs.k8s.io/kind@v0.32.0`，每次冷编译浪费约 1 分钟。
- 解决：curl 官方 release 预编译二进制（`kind-linux-amd64`）到 `$HOME/.local/bin` 并加入 `GITHUB_PATH`。
- 验证：`kind version` 通过，E2E 不再有 go install 编译阶段。

### 2026-08-16 BuildKit gha 缓存要先 docker buildx create
- 现象：`--cache-from/--cache-to=type=gha` 在 builder 未初始化时不生效或报错。
- 解决：工作流先 `docker buildx create --use --name ci-builder`（`|| true` 容忍已存在）；Makefile 通过 `DOCKER_BUILD_CACHE` 变量注入缓存参数，本地默认空不受影响。
- 验证：镜像构建 job 启用 gha 缓存；注意 `|| true` 会掩盖创建失败，出错时先查 builder。

### 2026-08-16 等待 CI 不要长 sleep
- 现象：Agent 等 CI 用固定长 sleep，用户看到"全部跑完了但还在等"。
- 解决：每 30 秒轮询一次（`gh run list` / `gh run view --json jobs`），预期 3-6 分钟，超过 10 分钟无结论再停下排查；失败取 `gh run view <run-id> --log-failed`。
- 验证：本次交付全程 30 秒轮询，无空等。

### 2026-08-16 墙钟由最慢 job 决定，优化墙钟以下的 job 不改变总耗时
- 现象：lint / controller / verify-deploy 各自提速明显，但整次 push 墙钟仍约 5 分 20 秒。
- 原因：三个 workflow 并行，墙钟 = 最慢 job = E2E（5m22s）；其余 job 本来就在墙钟内跑完。
- 解决：先量各 job 耗时找出瓶颈（`gh run view --json jobs`），再决定优化对象；本次结论是 E2E 的 Go 编译（约 3 分钟）为硬成本。
- 验证：lint 从 2m13s 降到 24s，墙钟不变；E2E 内部并行化后墙钟仍不变。

### 2026-08-16 E2E 并行构建 Go 镜像无墙钟收益（CPU 密集）
- 现象：BeforeSuite 并行构建 manager/simulator 镜像（两个 goroutine），构建阶段仍 2m55s，与串行相同。
- 原因：Go 编译是 CPU 密集，4 vCPU runner 上并行两个编译 = 总 CPU 时间不变。
- 解决：并行仍保留（收益在"与 CertManager 安装重叠"），但不要指望并行编译本身省时间；要省只能复用镜像产物（架构级改动，暂不做）。
- 验证：f111704 实测构建阶段 2m55s（串行基线 1m46s + 1m09s）。

### 2026-08-16 gha 镜像缓存对 Go 编译层无效
- 现象：`--cache-from/--cache-to=type=gha` 参数正确传入、builder 用 docker-container，但层输出几乎没有 `CACHED`（4 个镜像仅 3 层）。
- 原因：Dockerfile 里 `COPY . .` 之后是 `go build`；源码每次提交都变，编译层必然重跑。gha 能缓存的只有 base 镜像与 `go mod download` 层，收益约 1 分钟内。
- 解决：保留 cache 参数（对依赖层有少量收益）；不要把镜像构建提速押在 gha 缓存上。
- 备注：actions/cache 的命中匹配除 key 外还包含由 path 派生的 version；修改 cache path 会导致旧缓存 miss（属正常失效，不要误判为 bug）。

### 2026-08-16 E2E 测试阶段约 1 分钟是就绪轮询
- 现象：E2E 测试执行约 1m17s，其中约 24 秒的 spec 里有 11 次每秒一次的轮询。
- 原因：spec 等待资源就绪（deployment/pod/端点），每次轮询间隔 1 秒，属于稳定逻辑。
- 解决：不要为省时间调小轮询间隔（有偶发失败风险）；如确需优化，应改为"按事件等待"（需要较大重构）。
- 验证：多次运行该 spec 稳定约 24 秒。

### 2026-08-16 E2E 偶发 make undeploy 静默挂起直到套件超时（runner 环境 flake）
- 现象：4ab7ec9 的 E2E 全部用例通过后，`make undeploy` 静默挂起约 6 分钟直到 ginkgo 10 分钟超时；`gh run rerun --failed` 重跑同一 commit 直接全绿。
- 原因：`kubectl delete -k config/default` 在 GitHub runner 上偶发不返回（疑似 kind API 或 docker 环境瞬时故障）；同一代码在其他多次运行中 undeploy 均 15 秒内完成。
- 解决：先重跑确认（`gh run rerun <run-id> --failed`）；若同一 commit 复现再排查，看 `gh run view <run-id> --log` 超时前最后一步与挂起命令。
- 验证：31942621277 重跑 success；369c158 等历史运行 undeploy 正常。

## 可观测性与 Grafana 嵌入

### 2026-08-16 Grafana 13 面板无 data-panelid，屏外懒渲染读不全
- 现象：`querySelectorAll('[data-panelid]')` 返回 0；未滚动 iframe 时 `innerText` 缺下半屏面板（如 Leader 面板）。
- 原因：Grafana 13 面板容器是 `[class*="panel-container"]` 而非 data-panelid；屏外面板懒渲染不产出文本。
- 解决：先 `iframe.contentWindow.scrollTo(0, iframe.contentDocument.body.scrollHeight)` 再读；选择器用 `[class*="panel-container"]`。已内置到 `hack/ui-check/grafana-panels.mjs`。
- 验证：滚动后 12 个面板文本全部可读，Leader 面板 10 行 0/1 完整。

### 2026-08-16 本环境 view_image 不可用，视觉验证以 DOM 读取为主
- 现象：`view_image` 连最小 PNG 都返回 Unsupported Image。
- 解决：截图 + DOM 读取双通道；Agent 判定以 DOM 渲染文本为准，截图复制给用户核实。
- 验证：Leader 面板问题通过 DOM 文本确认（10 行 0/1，hjf2g=1）。

### 2026-08-16 Chrome --screenshot 多 target 报错，统一走 CDP 脚本
- 现象：`chrome --headless=new --screenshot` 报 "Multiple targets are not supported in headless mode"；in-app browser 的 node_repl 报 "failed to write kernel assets: 系统找不到指定的路径 (os error 3)"。
- 解决：统一用 `hack/ui-check/grafana-panels.mjs`（WSL Node + Windows Chrome CDP）：`Page.captureScreenshot` 截图 + `Runtime.evaluate` 读 DOM。
- 验证：脚本输出 12 面板文本 + 1578×902 截图。

### 2026-08-16 Grafana Live WebSocket 经反代握手 400
- 现象：控制台反复 `WebSocket connection to 'ws://localhost:8080/grafana/api/live/ws' failed: ... 400`。
- 原因：Backend 反代未对 `/grafana/api/live` 做 WS 升级，Grafana 自动退回轮询刷新。
- 解决：无需处理（面板 10s 刷新正常）；若要修，反代对该路径放行 Upgrade 头。
- 验证：仅控制台噪音，面板数据正常。

### 2026-08-16 Grafana sub-path 反代必须保留 /grafana 前缀
- 现象：iframe 白屏或显示控制台首页；`/grafana/d/...` 返回 301 到 `http://localhost:8080/grafana/...`，静态资源 404。
- 原因：Grafana 以 `GF_SERVER_SERVE_FROM_SUB_PATH=true` 部署时，页面与静态资源只认 `/grafana/...` 路径；反代剥前缀后 Grafana 把面板页 301 回外部入口，经 nginx SPA fallback 变成控制台首页。
- 解决：Backend 反代保留 `/grafana` 前缀原样转发；`GF_SERVER_ROOT_URL=http://localhost:8080/grafana/` 与前端 iframe 路径必须一致。
- 验证：`curl localhost:8080/grafana/d/hello-k8s-ai-overview` 200 且 HTML 含 `<base href="/grafana/" />`。

### 2026-08-16 Grafana 嵌入开关变量名
- 现象：设置了 `GF_SERVER_ALLOW_EMBEDDING=true`，面板响应仍带 `X-Frame-Options: deny`。
- 原因：`allow_embedding` 属于 `[security]` 段，环境变量是 `GF_SECURITY_ALLOW_EMBEDDING`；`GF_SERVER_*` 前缀对应 `[server]` 段，不生效。
- 解决：改用 `GF_SECURITY_ALLOW_EMBEDDING=true`。
- 验证：改后直接请求 Grafana 无 X-Frame-Options 头。

### 2026-08-16 Backend 安全中间件会覆盖 Grafana 放行
- 现象：Grafana 已放行嵌入，但经 Dashboard 8080 访问面板仍带 `X-Frame-Options: DENY`。
- 原因：Backend `securityHeadersMiddleware` 对所有响应强制加 DENY，包括 `/grafana/*` 反代响应。
- 解决：中间件对 `/grafana/` 前缀跳过 X-Frame-Options；API 路径保持 DENY。
- 验证：`security_headers_test.go` 覆盖两条路径；8080 全链路无 X-Frame-Options。
- 补充（同日）：引入幂等与写认证中间件后，Grafana 前端查询（`POST /api/ds/query`）被 `MISSING_IDEMPOTENCY_KEY` 400 拦截，面板全部 No data。修复：`idempotencyMiddleware` 与 `authMiddleware` 对 `/grafana/` 前缀直接放行（上游 UI 流量不是 Dashboard 命令），API 命令路径保持原有约束。
- 验证：`idempotency_test.go` / `auth_test.go` 覆盖；真实集群 `POST /grafana/api/ds/query` 返回 10 个 frame 时间序列，控制器/编排器/模拟器面板查询均返回 1000+ 数据点。

### 2026-08-16 Grafana 384MiB 内存上限运行中打满
- 现象：集群运行一段时间后 Grafana 探针间歇失败（`context deadline exceeded` / HTTP 503），日志大量 `http: Handler timeout` 与 8-10s 请求超时；RESTARTS 可能仍为 0。
- 排查：`kubectl exec <grafana-pod> -- cat /sys/fs/cgroup/memory.current /sys/fs/cgroup/memory.max`，实测 383MiB/384MiB（99.7%）。
- 解决：`config/observability/grafana.yaml` 限额提到 memory 1024Mi / requests 256Mi / cpu 1000m；其余组件按实测水位判断，不超限不动。
- 验证：滚动后 0 重启、无探针告警，水位稳定约 547MiB/1GiB。
- 教训：不要只看 RESTARTS 与就绪状态判断“意外停止”；先看 cgroup 水位与最近事件。

### 2026-08-16 Docker Desktop kubelet 缓存 :dev 标签 digest
- 现象：重建镜像并 `rollout restart` 后，Pod 仍在跑旧镜像（`imageID` 与本地 `docker image inspect` 不一致）。
- 原因：Docker Desktop 内嵌 kubelet 的 containerd 按标签缓存 digest，`imagePullPolicy: IfNotPresent` 不重新解析。
- 解决：dev 部署清单（backend/frontend）改 `imagePullPolicy: Always`；排查时对比 Pod `imageID` 与本地镜像 ID。
- 验证：改后重启 Pod 的 imageID 与本地一致。

### 2026-08-16 滚动更新会杀死端口转发
- 现象：`kubectl rollout restart` 后 `localhost:8080` 连接拒绝，转发日志报 "network namespace ... is closed"。
- 原因：`port-forward svc/` 在首次连接时 pin 到具体 Pod，Pod 被滚动更新删除后转发即断。
- 解决：部署/重启后重新执行 `make cluster-open`（脚本的存活检查只覆盖"检查时刻"，检查后 Pod 再被删仍会断）。
- 验证：重启后重开转发，8080 恢复。
## YAML 与模板

### 2026-08-16 issue form description 中 `bug: ` 冒号被 YAML 解析
- 现象：`yaml.safe_load` 报 "mapping values are not allowed here"。
- 原因：plain scalar 里 `bug: ` 被当成嵌套 mapping。
- 解决：description 整行加双引号包裹。
- 验证：3 个 issue 模板 YAML 校验通过。

## 数据库与容器验证

### 2026-08-16 WSL 无 Go：用 golang 容器编译与测试
- 现象：WSL 里没有 go 命令，无法本地编译/跑测试。
- 解决：`docker run --rm -v $PWD:/app -w /app golang:1.26 go test ./...`；版本与 Dockerfile 保持一致（当前 golang:1.26）。
- 验证：Backend 全量测试在容器内通过。

### 2026-08-16 PostgreSQL 集成测试
- 方法：`docker run -d --name hk8s-pg-test -e POSTGRES_USER=dashboard -e POSTGRES_PASSWORD=dashboard -e POSTGRES_DB=dashboard -p 55432:5432 postgres:17-alpine`，然后 `TEST_DATABASE_URL=postgres://dashboard:dashboard@localhost:55432/dashboard?sslmode=disable go test ./internal/store/ -run TestPostgresLifecycle -v`。
- 注意：测试容器访问宿主端口用 `--network host`（WSL 原生 docker 没有 host.docker.internal）。
- 测试用专用数据库，不要指向真实集群的库。

### 2026-08-16 当前态读路径的降级边界
- `/configuration`、`/overview`、`/traffic` 无 `at` 参数时优先从数据库 `resource_states` 重建当前态；数据库可用但表为空（快照循环未跑过）时同样回退实时聚合，不要误判为故障。
- `asOf` 是数据库最新 `captured_at`（数据时间），可能比墙钟晚最多一个快照周期（默认 30s），不是 bug。
- 修改读路径时保持响应结构与实时路径一致（空集合输出 `[]` 而非 `null`）。

### 2026-08-16 resource_states 只增不改导致已删除资源变幽灵数据
- 现象：删除 Model/WorkerNode/策略等资源后，`/configuration` 仍显示已删除对象；集群 `kubectl get` 已无该对象，数据库 `resource_states` 残留旧行。
- 原因：`UpsertResourceStates` 只有 INSERT ... ON CONFLICT DO UPDATE，没有删除路径；快照循环每 30s 把当前态 upsert 进 `resource_states`，读路径又优先从该表重建当前态，已删除资源永远不会消失。
- 解决：`persistSnapshot` 每次写当前态后调用新增的 `PruneResourceStates`，按本次快照的活跃业务资源集合删除库中不存在的业务行（Model/WorkerNode/Tenant/三种 Policy/Orchestrator/SimulationClock/SimulatorInstance/Performance/Runtime/Traffic）；Node/Deployment/Pod 系统遥测保留。无业务资源时同样清理，保证删除全部资源后读路径为空。
- 验证：真实集群创建 model-prune-test → 快照写入 → API 删除 → 一个快照周期后库中与 `/configuration` 均消失；`TestPostgresLifecycle` 覆盖清理行为。
- 备注：已部署环境的存量幽灵行需要手动 DELETE 一次，新代码只负责增量清理。

### 迁移规则（本次确立）
- 新表/结构变更只追加 `migrations/NNN_*.sql`，不修改已应用的迁移（`schema_migrations` 已记录）。
- 迁移必须幂等（`IF NOT EXISTS` / `ON CONFLICT`），Backend 启动自动应用。
- 验证：`TestPostgresLifecycle` 覆盖"迁移幂等 + 重启后历史仍在"。

### 2026-08-16 一键启动脚本的两个坑
- `hack/local-cluster.sh` 可能丢失执行位（Windows 侧操作后 100644）：`setup.sh` 报 `Permission denied` 时先 `chmod +x hack/*.sh` 并提交 mode 变化。
- 端口转发存活检查只看 ps 会误判：进程死亡但 PID 文件残留时 `cluster-open` 不会重建转发（8080 无监听但日志说"已在运行"）。修复后检查包含 `/dev/tcp` 端口探测；遇到 8080 无响应先看 `.runtime/port-forward-*.pid` 与 `ps aux | grep port-forward`。
- 并行构建四个镜像后，构建日志会交错输出；判断失败以退出码与最终镜像存在为准，不要按日志顺序读。
### 2026-08-16 rollout restart 会杀死 kubectl port-forward 进程
- 现象：`rollout restart` backend/frontend 后，8080 变 000；`ps` 里 port-forward 进程消失，日志结尾是 `lost connection to pod`。
- 原因：kubectl port-forward 在 pod 重建后不会自动恢复连接，进程直接退出；脚本的 pid 文件仍残留旧 PID。
- 解决：部署后检查 `/dev/tcp/127.0.0.1/8080` 与 `ps aux | grep port-forward`；进程不存在就重建（`bash setup.sh open` 或手动 nohup + 更新 pid 文件）。
- 验证：本次部署后手动重建转发，8080/guide 恢复 200。

## 集群操作与部署
### 2026-08-17 更新 Controller 必须用 config/dev 部署，make deploy(config/default) 会丢掉 SIMULATOR_IMAGE env
- 现象：`make deploy`（kustomize config/default）更新 controller 后，SimulatorInstance Controller 重建模拟器 Deployment，新 Pod 用 `simulator:latest`（本地很老、无 9090 端点的镜像）→ readiness/liveness 探针 connection refused → 29s 优雅退出循环（Exit 0 + Completed，易误判为正常退出）。
- 原因：dev 栈的 `SIMULATOR_IMAGE=hello-k8s-ai-simulator:dev` env 在 `config/dev/manager-observability-patch.yaml` 里，`make deploy` 用 `config/default` 不含该 patch，apply 覆盖 Deployment 后 env 丢失，controller 回落默认 `simulator:latest`；本地 `simulator:latest` 是过期镜像。
- 解决：dev 集群更新 controller 一律 `kubectl kustomize config/dev | kubectl apply -f -`（幂等），之后 rollout restart；不要用 `make deploy`。判断依据：`kubectl get deploy hello-k8s-ai-controller-manager -n hello-k8s-ai-system -o yaml | grep SIMULATOR_IMAGE` 应有值。
- 验证：config/dev 重部署后 controller 重建模拟器 Deployment 模板为 `hello-k8s-ai-simulator:dev`，CrashLoop 的 RS 缩到 0，实例恢复 Running 并继续扩缩（本次实测 REPLICAS 10→12）。
- 备注：`simulator:latest` 默认值仅用于 CI/隔离环境；本机 dev 栈镜像 tag 是 `hello-k8s-ai-simulator:dev`（Makefile SIMULATOR_IMG）。



### 2026-08-16 Simulator Pod 调度绑定 WorkerNode 名：虚拟节点名无法调度
- 现象：用虚拟节点名（如 node-gpu-1）创建 WorkerNode 并建 TenantNodePolicy 后，SimulatorInstance 副本一直 Pending，`describe` 显示 node selector 匹配不到真实节点。
- 原因：Simulator 物化的 Pod 通过 affinity/nodeSelector 绑定 WorkerNode 名称，虚拟名在真实集群里不存在。
- 解决：WorkerNode 的 name 必须使用集群真实节点名（docker-desktop 为 desktop-worker、desktop-worker2 ...）；建 WorkerNode 前先 `kubectl get nodes` 核对。
- 验证：改为真实节点名后 SimulatorInstance 副本正常调度并 Running。
- 备注：Docker Desktop 内置 K8s 的节点名是 desktop-worker*，与 Kind 测试集群（hello-k8s-ai-test-e2e 的 kind-control-plane/worker）不同，切换环境要重查。

### 2026-08-16 Docker Desktop 重建节点后可能出现重复主机 IP（双节点同 172.18.0.4）
- 现象：rollout 新 Pod 卡 Init:0/1，init 容器 `wait-for-postgresql` 报 `no response`；Pod IP 属于其他节点网段（worker6 上的 Pod 拿到 10.244.2.x，而 worker6 的 CIDR 是 10.244.8.0/24）。
- 原因：此前强杀 Docker Desktop 后内置 K8s 节点容器重建，desktop-worker6 与 desktop-worker9 主机 IP 都是 172.18.0.4（`kubectl get pods -A -o wide` 可见两个节点同 IP），CNI 分配混乱。
- 解决：删除卡住的 Pod 让它重新调度到健康节点；必要时 `kubectl cordon desktop-worker6 desktop-worker9`。不要为此重建集群或重启 Docker。
- 验证：新 Pod 落到 worker9（10.244.2.5）后 rollout 成功；演练数据（模拟器/数据库）全程不受影响。
- 备注：彻底修复需 Docker Desktop → Settings → Kubernetes 重新启用节点容器（会中断全部工作负载），本次未做。

### 2026-08-16 清理演示 CR 必须在 Controller 在线时进行
- 现象：`cluster-down` 后删除 Tenant/Model/Policy 卡在 DeletionTimestamp，对象不消失。
- 原因：tenant-model-policy、simulator-instance-controller、performance-collector、traffic-distribution 四个 finalizer 依赖 Controller 处理。
- 解决：先恢复 Controller（`make cluster-up`）再按顺序删除：orchestrator → tenantmodelpolicy → 等 instance 消失 → tenant/model/派生 CR → 动态策略与 WorkerNode。
- 验证：本次清理 10 类业务 CR 全部归零。
- 备注：`simulationclock/default` 是系统默认对象，删除后控制器会自动重建，不需要也不建议清理。

### 2026-08-16 空配置不再写历史快照（干净环境预期）
- 现象：干净环境（无业务 CR）下 `/replay` 没有 `snapshot-*`。
- 原因：`persistSnapshot` 新增 `snapshotHasBusinessData` 判定，无模型/租户/节点/策略/编排器/实例时跳过写快照。
- 解决：这是预期行为；`resource_events` 仍会记录真实系统 Lease/Node 心跳事件，不属于假数据。
- 验证：`bash setup.sh` 干净模式验收通过。
- 备注：旧版本后端写入的残留快照不会自动消失；发现 `resource_snapshots` 有历史行时可 TRUNCATE 后再验收（脚本断言只检查 `/replay` 响应，不检查库表行数）。`resource_states` 的业务行已由 `PruneResourceStates` 自动清理，无需手工处理。

### 2026-08-16 本机 Go 测试 httptest 回环有约 300ms accept 延迟
- 现象：`TestGrafanaProxyPreservesSubPathAndForwards`、`TestGrafanaProxyRootPath` 在本机 WSL 报 502，CI 正常。
- 原因：本机 WSL/Docker Desktop 环境下 `httptest.NewServer` 的 127.0.0.1 监听刚建立时连接被拒（独立复现：延迟 300ms 后 dial 与反向代理均成功）。
- 解决：本地判断与本次改动无关时，以 CI 结果为准；不要为此改测试。
- 验证：stash 本次改动后同样失败；GitHub Actions 上历史提交均通过。

## 领域已知易误判点（原 AI_CONTEXT 第 8 节）

- Traffic Overlay 是本地草稿；页面有真实数据不等于场景已写回控制平面。
- `TenantRuntime.status.instanceCount` 的实现含义是可用 Replica 总数。
- `Model.spec.absoluteScore` 是用户/Backend 提供的必填能力基准；旧 `status.absoluteScore` 仅用于滚动升级兼容，不应再写入。
- TenantNodePolicy、ModelNodePolicy 的 Status 当前没有 writer；空 Conditions 不等于失败。
- 前端只创建 Model/Tenant/Orchestrator 不会启动工作负载：SimulatorInstance 由 TenantModelPolicy(Allow) 物化，节点可调度性由 TenantNodePolicy(Allow) 决定（无显式 Allow 不可调度），模型-节点范围由 ModelNodePolicy 过滤；三类策略缺一不可，Orchestrator 在无可行 placement 时副本保持 0。
- 新租户没有 Simulator 时 Orchestrator 停在 MetricsNotReady 属正常引导态：性能指标来自运行中的 Simulator Pod，只有策略齐全后 bootstrap 扩容（到 minReplicas 地板）才会创建 Pod。
- Backend watch ReplicaSet 并记录事件，但 Workloads DTO 当前未直接展示 ReplicaSet。
- 数据库 `clock_state` 仍未驱动运行时。`SimulationClock/default` 只控制 Simulator 引擎倍速；Backend server/actual/logical time、Controller cooldown/freshness、Lease 和采集周期继续使用真实 UTC。
- 配置批次会先 dry-run 全部对象，再顺序写入；跨对象写入并非数据库式原子事务。
- SSE 是非持久通知流，慢客户端可能丢事件；30 秒轮询是安全网。
