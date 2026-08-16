# 实现修改明细

## 1. 改动前状态

- 控制器 7 个 Reconcile 文件里散落 9 段几乎相同的「RetryOnConflict + Get + DeepCopy + mutate + Patch/Status().Patch」。
- Simulator 的 `updateOwnedStatus` 是同一模式的第三份副本（使用 `retry.RetryOnConflict` 手写）。
- Prometheus 与 Jaeger 两个 Provider 各自实现 `New`（http.Client 构造）、`resolve`（URL 拼接）和 `getJSON`（GET + 状态码检查 + JSON 解码），错误文案一致。
- 前端 5 个配置表单各自重复「submitError 状态 + 两个 useEffect + submitForm/saveTemplate/loadTemplate + getErrorMessage」约 40 行；18 个数字输入重复「NaN 空值处理 + 单位后缀 span」；PolicyForm 本地 RefSelect 与 OrchestratorForm 内联租户下拉重复；5 个配置表重复「displayName + name 双行单元格」和 `Intl.NumberFormat`。

## 2. 修改

- 新增 `internal/k8sutil/patch.go`：`RetryOnConflict`、泛型 `PatchWithRetry[T client.Object]`、`PatchStatusWithRetry`（无变化不写 API）；`patch_test.go` 承接原 `TestRetryOnConflict`。
- `internal/controller/helpers.go` 删除本地 retryOnConflict / patchWithRetry / patchStatusWithRetry；`orchestrator_executor.go`、`simulationclock_controller.go`、`tenant_controller.go`、`workernodeusage_controller.go`、`performancecollector_controller.go`、`orchestrator_controller.go`、`simulatorinstance_controller.go`、`traffic_controller.go` 改用 `k8sutil.*`；finalizer 帮助函数改用 `k8sutil.RetryOnConflict`。
- `simulator/simulator.go` 的 `updateOwnedStatus` 改为 `k8sutil.PatchStatusWithRetry`，mutate 内仍只写 Score / Performance / ObservedAt / SimulationElapsedMs / ReporterID。
- 新增 `dashboard/backend/internal/providers/httputil/httputil.go`：`NewClient(timeout)`、`ParseBaseURL(rawURL, providerName)`、`Resolve(base, path)`、`GetJSON(...)`（providerName 只进错误文案，maxBodyBytes 保持各 Provider 原值：Prometheus 16<<20、Jaeger 32<<20）。
- `prometheus/client.go`、`jaeger/client.go` 改用 httputil，删除本地 `getJSON` / `resolve`；Jaeger 保留 `net/url`（baseURL 字段与 `url.PathEscape`）。
- 前端：
  - `ConfigFormParts.tsx` 新增 `useConfigForm`、`ConfigTextField`、`ConfigNumberField`、`ConfigRefSelect`。
  - `ModelForm` / `NodeForm` / `OrchestratorForm` / `PolicyForm` / `TenantForm` 改用 hook 与共享字段组件，删除本地脚手架与重复 JSX。
  - `ConfigTable.tsx` 新增 `NameCell` 与共享 `formatNumber`；5 个表格改用。
  - 各处删除不再使用的 import（react-hook-form 类型、lucide 图标、UI 组件按需保留）。

## 3. 未做

- DrawCanvas / PreviewCanvas / PreviewCurve：编辑器与预览的坐标系、交互与状态差异大，强行合并风险高。
- `trafficMath.ts` 与 `timelineMath.ts`：分属流量曲线与时间线回放两个领域，不构成重复。
- Controller 与 Simulator 的 Prometheus 指标注册样板：分别位于 controller 包与 main 包，注册项与标签各异，抽取需引入指标框架，超出本轮。
- `grafana_proxy.go` 的 `url.Parse`：反向代理场景，与 JSON GET 客户端工具不匹配，未纳入 httputil。
