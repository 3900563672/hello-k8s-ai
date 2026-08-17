# hello-k8s-ai 项目概览

## 1. 项目简介

hello-k8s-ai 是一个基于 Kubernetes 的 AI 调度策略实验验证平台。

项目最初目标是开发一个面向 AI 推理任务的 Kubernetes 调度控制器，用于探索不同调度策略在 Kubernetes 环境中的行为表现。

为了验证 Controller 的调度逻辑是否正确，同时方便调试、观察和分析调度过程，项目进一步扩展了：

- Kubernetes Controller；
- AI 工作负载 Simulator；
- Dashboard Frontend；
- Dashboard Backend；
- 数据存储系统；
- 可观测性系统。

因此当前项目并不是单一 Kubernetes Controller，而是一套完整的 Kubernetes 原生实验平台。

整体目标：

> 构建一个可以配置、模拟、观察和分析 AI 推理任务调度过程的 Kubernetes 实验环境。

当前阶段主要用于：

- 验证调度策略；
- 模拟 AI 推理行为；
- 分析调度结果；
- 建立 Kubernetes Controller 调试闭环。

需要注意：

当前项目仍处于开发阶段，尚未实现真正意义上的 AI 自动优化调度。

目前重点是：

- 调度流程验证；
- Kubernetes 控制器开发；
- 模拟环境构建；
- 工程化架构探索。

---

## 2. 整体架构

### 2.1 架构总览

hello-k8s-ai 采用 Kubernetes Controller Pattern 设计。

系统整体由：

- 控制面（Control Plane）
- 数据面（Data Plane）
- 可观测链路（Observability Pipeline）
- 数据存储层（Storage Layer）

组成。

整体架构如下：

```text
             用户
              |
              v
      Dashboard Frontend
              |
              v
      Dashboard Backend
              |
              v
    Kubernetes API Server
              |
              v
          CR / CRD
              |
              v
         Controller
              |
              v
         Simulator
              |
              v
      模拟性能 / 状态数据
              |
    +---------+-----------+
    |                     |
    v                     v

   Metrics Pipeline Event Storage
    |                     |
    v                     v
   Prometheus / OTel PostgreSQL
    |                     |
    v                     v
 Grafana               Jaeger
    |                     |
    v                     v    
    +---------+-----------+
              |
              v
         Dashboard 展示
```

---

## 3. 系统组件说明

### 3.1 Frontend Dashboard

Frontend 是用户操作入口。

主要职责：

- 提供 Web 操作界面；
- 创建和修改 AI 调度任务配置；
- 查看集群运行状态；
- 展示调度结果；
- 展示历史运行数据。

技术栈：

- React
- TypeScript
- Vite

用户通过 Frontend 完成：

1. 配置任务参数；
2. 点击应用；
3. 创建 Kubernetes Resource；
4. 查看运行结果。

### 3.2 Dashboard Backend

Backend 是 Dashboard 与 Kubernetes 集群之间的控制层。

主要职责：

- 接收 Frontend 请求；
- 转换用户配置；
- 操作 Kubernetes API；
- 创建和修改 CR 实例；
- 查询历史运行数据；
- 聚合监控信息。

Backend 主要连接：

```text
       Frontend
          |
          |
       Backend
          |
          |
  +-------+-------+
  |               ^
  v               |
Kubernetes    PostgreSQL
API Server        

```

Backend 不直接负责调度。

它主要负责：

- 管理资源；
- 提供 API；
- 提供数据查询。

### 3.3 Kubernetes API / CRD

项目通过 Kubernetes CRD（Custom Resource Definition）扩展 Kubernetes API。

API 目录主要负责：

- CRD 定义；
- Kubernetes Resource 类型；
- 数据结构定义；
- Controller 使用的数据模型。

目录：

```text
api/
```

负责：

- Custom Resource Definition

例如：

```text
tenant
model
workerNode
```

等 Kubernetes 自定义资源。

用户在 Dashboard 中配置的信息最终会转换为 CR 实例。

例如：

```text
用户配置
   |
   v

Backend
   |
   v

Kubernetes API
   |
   v  

CR Instance
```

CR 表示系统期望状态。

Controller 根据 CR 当前状态进行调节。

### 3.4 Controller

Controller 是系统核心调度组件。

基于：

- Kubebuilder
- Controller Runtime

开发。

主要职责：

- Watch Kubernetes Resource；
- 执行 Reconcile 流程；
- 根据状态执行调度逻辑；
- 修改 CR Status；
- 根据 Simulator 数据调整策略。

Controller 工作模式：

```text
Observe

读取当前状态

    ↓

Analyze

分析资源和性能数据

    ↓

Act

执行调度动作

    ↓

Update

更新 Kubernetes Resource
```

Controller 不直接模拟任务。

它依赖 Simulator 提供环境反馈。

### 3.5 Simulator

Simulator 用于模拟真实 Kubernetes AI 推理任务运行过程。

由于当前项目主要目标是验证调度策略，因此不直接运行真实 AI 模型，而通过 Simulator 产生模拟数据。

Simulator 负责：

- 模拟 AI workload；
- 模拟节点资源变化；
- 模拟推理性能；
- 生成调度反馈。

运行关系：

```text
Controller
    |
    v

Scheduler Decision
    |
    v

Simulator
    |
    v

Performance Data
    |
    v

Controller Next Decision
```

Simulator 是调度闭环中的数据产生模块。

### 3.6 PostgreSQL 数据存储

系统不会直接依赖 Kubernetes 实时状态展示所有信息。

为了支持历史分析，引入 PostgreSQL。

数据库保存：

- 调度事件；
- 状态变化；
- Simulator 输出；
- Controller 决策结果；
- 时间切面的运行数据。

数据模型：

```text
Event

Timestamp

Resource State

Scheduling Result

=
Historical Record
```

Dashboard 展示流程：

```text
Database
   |
   v

Backend Query
   |
   v

Frontend Display
```

当前 Dashboard 不是简单实时刷新模式。

而是：

> 数据产生 → 数据保存 → 数据查询 → 页面展示

### 3.7 可观测性链路

系统加入：

- Prometheus
- OpenTelemetry
- Jaeger

用于观察：

- Controller 行为；
- Simulator 性能；
- 调度过程；
- 请求链路。

链路：

```text
           Controller
               |
               |
           Simulator
               |
               |
         Metrics / Trace
               |
               |
      +--------+---------+
      |                  |
      v                  v
      
  Prometheus       OpenTelemetry
      
      |                  |
      v                  v
      +--------+---------+
               |
               v
      
             Jaeger
```

---

## 4. 核心运行流程

### 4.1 用户配置阶段

用户进入 Dashboard：

```text
    Frontend
       
       ↓
       
 AI 调度任务配置
       
       ↓
       
    点击应用
       
       ↓
       
 发送 Backend 请求
```

### 4.2 CR 创建阶段

Backend 根据用户配置生成 Kubernetes Resource。

流程：

```text
   Backend
      
      ↓
      
  Kubernetes API
      
      ↓
      
创建 CR Instance
      
      ↓
      
Controller Watch
```

### 4.3 调度执行阶段

Controller 检测到 CR 变化。

执行：

1. 获取 Resource 状态；
2. 获取 Simulator 数据；
3. 执行调度逻辑；
4. 更新状态。

流程：

```text
     CR Change
        
        ↓
        
Controller Reconcile
        
        ↓
        
Scheduling Decision
        
        ↓
        
     Simulator
```

### 4.4 模拟运行阶段

Simulator 根据 Controller 决策产生：

- 性能数据；
- 资源状态；
- 工作负载变化。

这些数据反馈给 Controller。

形成闭环：

```text
      Controller
          
          ↓
          
      Simulator
          
          ↓
          
 Performance Feedback
          
          ↓
          
     Controller
```

### 4.5 数据回流阶段

系统运行过程中产生：

- Metrics；
- Trace；
- Event。

分别进入：

```text
    Metrics
       
       ↓
       
   Prometheus
       
     Trace
       
       ↓
       
   OpenTelemetry
       
       ↓
       
     Jaeger
       
     Event
       
       ↓
       
   PostgreSQL
```

Frontend 最终通过 Backend 查询数据库展示。

---

## 5. Kubernetes 资源关系

整体资源关系：

```text
       User
        
        |
        
     Frontend
        
        |
        
     Backend
        
        |
        
       CR
        
        |
        
  Kubernetes API
        
        |
        
    Controller
        
        |
        
    Simulator
        
        |
        
  Status Update
        
        |
        
    CR Status
```

核心关系：

- CRD 定义资源类型；
- CR 保存用户期望状态；
- Controller 调整实际状态；
- Simulator 提供运行反馈。

符合 Kubernetes：

```text
   Desired State
   
         ↓
   
     Controller
   
         ↓
   
     Actual State
```

模型。

---

## 6. 项目目录结构

```text
hello-k8s-ai
     ├── api/v1/                  CRD Go 类型与 Kubebuilder 标记
     ├── cmd/                     Controller Manager 入口
     ├── internal/                7 个 Controller、observability
     ├── simulator/               AI workload 模拟、Lease 选主、Metrics/Trace
     ├── dashboard/
     │   ├── backend/             Backend API、Kubernetes cache、PostgreSQL
     │   └── frontend/my-app/     React 控制台（5 个页面）
     ├── config/                  CRD、RBAC、部署与可观测性清单
     ├── docs/                    分层文档（人类 / Agent / 远程 AI）
     ├── change-history/          每次变更的归档与时间线
     ├── hack/                    部署、文档检查、长跑与验证脚本
     ├── test/                    E2E 与测试工具
     ├── Makefile / setup.sh      构建与一键部署入口
     └── AGENTS.md                本地 Agent 工作准则
```

---

## 7. 开发修改入口

### 修改 Kubernetes Resource

查看：

```text
api/
```

包括：

- CRD；
- Resource 类型。

### 修改 Controller 调度逻辑

查看：

```text
internal/
```

包括：

- Reconcile；
- Scheduler；
- 状态处理。

### 修改模拟行为

查看：

```text
simulator/
```

包括：

- 模拟数据；
- 性能模型。

### 修改用户界面

查看：

```text
dashboard/
```

包括：

- React 页面；
- Backend API。

### 修改部署方式

查看：

```text
config/
```

包括：

- Kubernetes YAML；
- Deployment；
- RBAC。

---

## 8. 新开发者阅读路线

第一次接触项目：

```text
      README.md（1 分钟：是什么、怎么跑）
     
        ↓
     
 PROJECT_OVERVIEW_NEW.md（本文：10 分钟总览）
     
        ↓
     
  docs/INDEX.md（专题索引，按需进入）
     
        ↓
     
 docs/getting-started/LOCAL_RUN.md（动手跑起来）
     
        ↓
     
 docs/overview/ARCHITECTURE_OVERVIEW.md（理解架构）
     
        ↓
     
       api/ → internal/ → simulator/ → dashboard/
```

最快体验完整链路：

```bash
bash setup.sh
# 打开 http://localhost:8080，进入「填写指南」（/guide）
# 用预置模板创建模型、节点、租户与流量，再提交运行
```

一键启动得到的是干净环境：没有预置租户、模型与历史数据，这是预期行为；配置模板只预填表单，提交与运行由你决定。

关注调度逻辑：

```text
     api/
      
      ↓
      
   internal/
      
      ↓
      
  simulator/
```

关注用户功能：

```text
dashboard/frontend/my-app/

        ↓

 dashboard/backend/
```

关注部署：

```text
       setup.sh（一键入口）

         ↓

       config/（Kubernetes 清单）

         ↓

docs/getting-started/DEPLOYMENT.md
```

---

## 9. 当前项目定位

hello-k8s-ai 当前定位：

> Kubernetes AI 调度控制器实验验证与工程化探索平台。

不是：

- 生产级 AI 调度系统；
- 自动机器学习优化平台。

当前主要价值：

- 提供 Kubernetes Controller 实验环境；
- 验证 AI 调度策略；
- 模拟真实运行环境；
- 提供完整观测和分析能力。

---

## 10. 开发目标验收

完成本文档后，新开发者应该能够：

- 理解项目整体架构；
- 理解 Frontend、Backend、Controller、Simulator 的关系；
- 理解 CRD 与 Kubernetes Resource 的关系；
- 知道不同功能修改的位置；
- 能够完成基础环境运行；
- 能够定位问题所在模块。