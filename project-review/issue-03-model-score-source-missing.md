# 1. 问题标题

Model 绝对分没有系统内生产者

> 处理状态（2026-08-14）：已修复。`spec.absoluteScore` 现为必填配置，Backend/Frontend 已补齐入口，Orchestrator 保留旧 Status 回退并能明确报告缺失原因。原审查内容保留用于说明问题背景；实施记录见 `change-history/2026-08-14-model-absolute-score-production-path/`。

## 2. 当前状态描述

`api/v1/model_types.go` 把 `Model.status.absoluteScore` 定义为单个预热副本的能力基准分数。该字段位于 Status，注释说明由 Backend 或运维维护。

`internal/controller/orchestrator_data.go` 读取该字段并转换成调度输入。`internal/controller/orchestrator_scoring.go` 会跳过绝对分小于等于零的 Model，因此一个没有 absoluteScore 的新模型无法成为扩容候选，初始化 `SimulatorInstance.status.effectiveScore` 也无法完成。

全仓库对该字段的写入检索只发现 `hack/local-cluster.sh`：演示部署在应用样例资源后，使用 `kubectl patch --subresource=status` 写入固定的 `DEMO_MODEL_ABSOLUTE_SCORE`。Controller 的 manager RBAC 只读取 Model，Simulator RBAC 也只读取 Model；Backend 的可写字段清单只允许修改 Model Spec，Dashboard RBAC 没有 `models/status` 写权限。也就是说，注释中的“Backend 维护”目前没有对应实现。

归档中的完整部署之所以成功，是因为本地脚本显式执行了这一步。脱离该脚本，通过 Backend 或常规 YAML 新建 Model 时，主链路不会自动生成绝对分。

## 3. 问题定位

这是核心输入数据缺少所有者。absoluteScore 不是可选的展示字段，而是扩容候选的硬门槛；字段在 CRD 中标记 optional，却在调度逻辑中具有 required 语义。

系统不会明确告诉使用者“模型等待基准分”，而是由评分阶段把该模型过滤掉，最终可能表现为 `no_feasible_placement`。这会把配置缺失伪装成容量不足或策略问题，增加排障成本。

固定演示分数也不能代表真实模型能力。若未来不同模型、硬件或性能参数需要比较，分数来源、版本、测量环境和更新时间都需要可追溯，否则调度结果无法解释。

## 4. 影响范围

- CRD：Status 字段的 optional 定义与运行时硬依赖不一致，也没有 Ready Condition 表达等待原因。
- Orchestrator：新模型无法扩容，且错误原因不够直接。
- Simulator：只有生成 Simulator 副本后才能产生运行数据，但副本启动又依赖初始分，形成启动依赖环。
- Backend/Frontend：可以创建 Model，却不能完成接入所需的 Status 初始化，也未提示必要步骤。
- 部署：只有本地演示脚本包含隐式补偿步骤，其他环境容易遗漏。
- 测试：评分测试提供了预置分数，没有覆盖“通过公开入口新建模型并完成首次扩容”。

该问题已在代码路径上确定；本地样例通过脚本规避，因此不是当前演示失败。

## 5. 根本原因分析

模型能力分同时具有“配置基线”和“运行状态”两种属性，但当前没有明确权威来源。字段被放入 Status 后，普通 API 写入自然受限；负责调度的 Controller又只消费不生产，最终由部署脚本承担了领域职责。

从依赖关系看，系统先完成了基于分数的调度和基于运行数据的 Simulator，但没有定义模型首次进入系统时的 bootstrap 流程。这不是评分公式本身的问题，而是生命周期缺了一步。

## 6. 修改方向建议

- 明确 absoluteScore 的唯一所有者和生成时机：可以来自受控运维输入、基准测试服务或确定性计算，但必须只有一个权威路径。
- 如果它本质上是用户配置，重新评估 Spec/Status 归属并提供完整校验、版本和审计；如果它是观测结果，则由专门 Controller/服务维护 Status。
- 在 Model 或 Orchestrator Condition 中区分“缺少基准分”“分数过期”“容量不足”，避免统一落为无可行候选。
- 设计新模型从创建、评分、可调度到首次 Simulator 启动的完整状态机，并让 Backend/Frontend 能展示当前阶段。
- 让部署脚本只创建样例，不再承担不可见的核心业务初始化职责。
- 增加从公开写入口创建 Model 到首次扩容的端到端测试。

## 7. 优先级

优先级：P0

只要系统需要接入样例之外的新模型，这就是主流程阻断点，应优先明确数据所有权。
