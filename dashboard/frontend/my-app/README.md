# 调度控制台

React + TypeScript 前端，通过 Dashboard Backend 读取 Kubernetes 当前状态、PostgreSQL 历史切面、Prometheus 指标和 Jaeger Trace。

## 本地运行

要求 Node.js 22+、npm 10+，并先启动 `dashboard/backend`。

```bash
npm ci
npm run dev
```

Vite 默认将 `/api` 代理到 `http://localhost:8080`。如需修改目标地址，设置 `VITE_API_PROXY_TARGET`。

提交前执行：

```bash
npm run check
```

该命令依次执行 oxlint、TypeScript/Vite 构建和状态契约检查。

## 数据边界

- TanStack Query 管理 Backend 数据与历史查询。
- Zustand 仅保存跨页面 UI、时间游标和未提交草稿。
- Config 修改通过 Backend 写入 Kubernetes，不使用 localStorage 保存生产数据。
- 历史模式只读。
- Traffic 模板与 Overlay 是场景设计稿（内存态，不落盘）；点击“确认叠加”会把该叠加的起始增量与租户当前目标 QPS 相加，通过 `PATCH /api/v1/tenants/{name}/traffic` 写入 `Tenant.spec.qps`，成功后 overlay 才加入本地列表。控制面当前只支持常量目标 QPS，曲线仅作场景预览；历史模式只读，不能应用。
- SSE 只负责失效通知；重连或丢事件后由 REST 重新同步。

## 生产入口

`Dockerfile` 构建静态资源，`nginx.conf` 提供 SPA 回退，并将 `/api/` 代理到 Dashboard Backend。Kubernetes 清单位于 `dashboard/deploy/`。

项目结构、API 和运行说明见 [工程文档索引](../../../docs/INDEX.md)。
