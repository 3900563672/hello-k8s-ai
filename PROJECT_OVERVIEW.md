# hello-k8s-ai 项目概览

## 1. 项目背景

hello-k8s-ai 最初的目标是开发一个基于 Kubernetes 的 AI 模拟调度控制器，用于探索 AI 推理任务在 Kubernetes 环境中的调度策略。

为了验证控制器行为是否正确，同时方便调试和观察调度过程，项目额外开发了对应的 Simulator（模拟器）以及 Dashboard Backend（控制面）。

因此当前项目不仅包含调度控制器本身，还包含：

- Kubernetes Controller；
- AI 调度模拟器；
- 后端控制面；
- 前端 Dashboard；
- 可观测性相关组件。

整体目标是构建一个可以观察、验证和调试 AI 推理调度策略的 Kubernetes 原生实验平台。

> 注意：当前项目仍处于开发阶段，尚未达到真正智能化调度的目标。目前主要用于调度逻辑验证、系统模拟以及工程化探索。


---

## 2. 项目技术基础

本项目基于 Kubebuilder 开发。

为了共享同一个 Go Module，同时方便 Controller、Simulator 和 Backend 之间复用代码，目前项目采用单仓库结构：


hello-k8s-ai
├── Kubebuilder Controller
├── Simulator
├── Backend
└── Dashboard


其中 Simulator 和 Backend 与 Kubebuilder 脚手架结构处于同一级目录，而不是独立 Go Module。

项目整体结构与典型 Kubebuilder 项目保持一致。

主要技术栈：

- Go
- Kubebuilder
- Kubernetes Controller Runtime
- Kubernetes client-go
- React
- TypeScript
- Vite


---

## 3. 当前运行环境

当前项目主要依赖项目作者本机已有的 Docker Desktop Kubernetes 集群。

默认运行环境：

- Kubernetes Context：`docker-desktop`
- 节点数量：10 个
    - 1 个控制节点
    - 9 个工作节点

目前该部署方式仍存在限制：

- 暂未完全适配不同名称的 Kubernetes 集群；
- 暂未适配不同数量的 Node；
- 部分配置仍依赖当前实验环境。

因此，如果在其他环境运行，可能需要额外调整。


---

## 4. 部署注意事项

README 提供了最简单的部署方式，但需要明确：

当前项目不是一个开箱即用的生产系统。

运行前必须确保：

- Docker Desktop 已启动；
- Kubernetes 已启用；
- Kubernetes API Server 正常运行；
- 所需组件均可访问。

如果基础环境未准备完成，可能出现非预期错误，而不是优雅提示。

推荐第一次运行前：

1. 确认 Kubernetes 状态；
2. 确认当前 Context；
3. 确认节点资源；
4. 再执行部署脚本。


---

## 5. 项目目录说明

由于项目主要基于 Kubebuilder，因此整体结构与标准 Kubebuilder 项目接近。

第一次接手项目时，建议优先阅读：


docs/


目录中的文档。

推荐阅读顺序：

1. `docs/INDEX.md`
2. `docs/AI_CONTEXT.md`
3. README
4. 具体模块文档


如果希望快速进入源码，可以重点关注：


api/
dashboard/
internal/
simulator/


这些目录包含主要业务逻辑。

其他目录可以在需要时进一步了解。


---

## 6. 核心模块说明

### API

负责 Kubernetes Custom Resource Definition（CRD）相关定义。

主要包括：

- 自定义资源结构；
- Kubernetes API 类型定义；
- Controller 使用的数据模型。


### Internal

主要包含项目内部核心逻辑。

包括：

- Controller 实现；
- Backend 逻辑；
- 公共业务模块。


### Simulator

负责模拟 Kubernetes 中的 AI 推理工作负载运行过程。

用于：

- 验证调度策略；
- 模拟状态变化；
- 辅助 Controller 调试。


### Dashboard

前端管理界面。

技术栈：

- React
- TypeScript
- Vite

当前 Dashboard 已具备基础功能，但仍存在部分未完善功能。


---

## 7. 后端与 Kubernetes 交互

项目后端和模拟器主要使用：


client-go


与 Kubernetes API Server 进行交互。

Controller 则基于：


Kubebuilder
Controller Runtime


实现 Kubernetes 原生控制循环。


---

## 8. AI 辅助开发

如果使用 AI 工具辅助开发，建议优先阅读：


docs/AI_CONTEXT.md


其中包含项目背景、架构信息以及开发上下文。

第一次接手项目时，也推荐查看：


docs/INDEX.md


通过索引了解已有文档。


---

## 9. 当前开发状态

当前项目主要目标：

- 验证 AI 调度控制器设计；
- 提供可调试的模拟环境；
- 建立完整的 Kubernetes 调度实验闭环。


目前仍存在：

- Frontend 功能未完全完善；
- 部分部署流程依赖固定实验环境；
- 尚未实现真正智能化调度。


因此当前项目定位为：

> Kubernetes AI 调度控制器的实验验证与工程化探索平台。

而不是生产级 AI 调度系统。


---

## 10. 推荐阅读路线

第一次接手项目：


README
↓
docs/INDEX.md
↓
docs/AI_CONTEXT.md
↓
api/
↓
internal/
↓
simulator/
↓
dashboard/


如果主要关注调度逻辑：


api/
internal/
simulator/


如果主要关注用户交互：


dashboard/
backend/


如果主要关注部署：


setup.sh
config/
docs/getting-started/