package controller

// 这些常量对应 CRD 中的枚举值，集中定义以避免各 Controller 重复硬编码。
const (
	// Ready condition 类型，K8s 标准
	conditionTypeReady = "Ready"

	// 策略效果：Allow 允许，Deny 拒绝
	policyEffectAllow = "Allow"
	policyEffectDeny  = "Deny"

	// 扩缩容方向：ScaleUp 扩容，ScaleDown 缩容
	scalingActionUp   = "ScaleUp"
	scalingActionDown = "ScaleDown"

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

	// tracing 属性名，跟 Span 里用的 key 保持一致
	traceAttributeTenantName            = "platform.tenant.name"
	traceAttributeModelName             = "platform.model.name"
	traceAttributeSimulatorInstanceName = "platform.simulator_instance.name"
)
