package controller

// 这些常量对应 CRD 中的枚举值，集中定义以避免各 Controller 重复硬编码。
const (
	// Ready condition 类型，K8s 标准
	conditionTypeReady = "Ready"

	// WorkerNode 物理水位压力条件；Orchestrator 据此停止扩容进入降级
	conditionTypePhysicalPressure = "PhysicalPressure"

	// Orchestrator 资源受限降级条件：任一可调度节点物理水位超阈值时置 True
	conditionTypeResourceLimited = "ResourceLimited"

	// 物理水位压力阈值（百分比）：真实 Node 已分配 requests 占 allocatable 超过该值
	// 视为接近物理上限。系统常量而非用户配置：它是宿主机安全边界，不是业务策略；
	// 如后续需要按环境调整，再参数化到 WorkerNode spec。
	physicalPressureThresholdPercent = 90

	// 策略效果：Allow 允许，Deny 拒绝
	policyEffectAllow = "Allow"
	policyEffectDeny  = "Deny"

	// 扩缩与放置动作
	scalingActionUp        = "ScaleUp"
	scalingActionDown      = "ScaleDown"
	scalingActionRebalance = "Rebalance"

	// 资源运行阶段
	phaseRunning = "Running" // 正常运行
	phasePending = "Pending" // 等待调度
	phaseFailed  = "Failed"  // 失败

	// 组件标识，用于指标和日志区分来源
	componentController   = "controller"
	componentOrchestrator = "orchestrator"
	componentTraffic      = "traffic"
	componentPerformance  = "performance"

	// 操作结果分类
	operationOutcomeSuccess = "success"
	operationOutcomeError   = "error"

	// 指标命名空间和通用标签
	metricsNamespace   = "hello_k8s_ai"
	metricLabelOutcome = "outcome"
	// 决策原因：资源受限降级（任一可调度节点物理水位超阈值，扩容挂起）
	decisionReasonResourceLimited = "resource_limited"

	// 指标子系统与节点标签
	metricsSubsystemWorkerNode = "worker_node"
	metricLabelNode            = "node"

	// tracing 属性名，跟 Span 里用的 key 保持一致
	traceAttributeTenantName            = "platform.tenant.name"
	traceAttributeModelName             = "platform.model.name"
	traceAttributeSimulatorInstanceName = "platform.simulator_instance.name"
)
