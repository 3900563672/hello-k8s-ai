package controller

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// 各种字段索引名，按租户查询用
	orchSimIndex          = "orchestrator.simulatorInstance.tenant"
	orchPolicyIndex       = "orchestrator.tenantModelPolicy.tenant"
	orchNodePolIndex      = "orchestrator.tenantNodePolicy.tenant"
	orchConfigTenantIndex = "orchestrator.config.tenant"
	orchPerformanceIndex  = "orchestrator.performance.tenant"

	// 阈值和副本数的默认值，租户没配就用这些
	defaultTTFTUp      = 500
	defaultQueueUp     = 100
	defaultTTFTDown    = 200
	defaultQueueDown   = 30
	defaultMaxReplicas = 100

	// 实例注解的 key
	lastScaleTriggerKey = "platform.study.com/last-scale-trigger"
	pendingScalePlanKey = "platform.study.com/pending-scale-plan"
)

// DecisionInput 是一次决策的完整输入快照，所有字段在 gather 阶段填好后不再修改。
type DecisionInput struct {
	TenantName       string
	OrchestratorName string
	TriggerID        string // 本次输入的哈希，变更检测用
	TenantQPS        int

	// 性能平均值
	AvgTTFT  int
	AvgQueue int
	HasTTFT  bool // TTFT 是否有效（数据过旧则为 false）
	HasQueue bool

	// 扩缩阈值
	TTFTThresholdUp    int
	TTFTThresholdDown  int
	QueueThresholdUp   int
	QueueThresholdDown int

	// 资源快照
	AvailableModels   []ModelInfo
	AvailableNodes    []NodeInfo
	ExistingInstances []InstanceInfo

	// 冷却
	ScaleUpCooldown   int
	ScaleDownCooldown int
	LastScaleUpTime   metav1.Time
	LastScaleDownTime metav1.Time

	// 副本限制
	MinReplicas      int
	MaxReplicas      int
	AllowScaleToZero bool
}

type ModelInfo struct {
	Name              string
	GPUUnits          int
	AbsoluteScore     int
	ColdStartMs       int
	MaxConcurrency    int
	EligibleNodeNames map[string]bool // 该模型能跑在哪些节点上（策略过滤后的）
}

type NodeInfo struct {
	Name                 string
	RemainingGPU         int // 还剩多少 GPU
	RemainingConcurrency int // 还剩多少并发
}

type InstanceInfo struct {
	Name              string
	ModelName         string
	CurrentReplicas   int
	EffectiveScore    int
	HasEffectiveScore bool   // 是否已经有有效分数（没被初始化过就是 false）
	LastScaleTrigger  string // 上次扩缩的 triggerID，对比用
	PendingScalePlan  string // 上一次没执行完的扩缩计划
}

// dependencyNotReadyError 表示某个依赖资源还没就绪，外部可以据此判断是重试还是报错。
type dependencyNotReadyError struct {
	resource string
	name     string
}

func (e *dependencyNotReadyError) Error() string {
	return fmt.Sprintf("%s %q is not ready", e.resource, e.name)
}

// gatherDecisionInput 拼出一次决策需要的所有输入数据。
func (r *OrchestratorReconciler) gatherDecisionInput(
	ctx context.Context,
	tenantName string,
	performance *platformv1.TenantPerformance,
) (DecisionInput, error) {
	input := DecisionInput{TenantName: tenantName}

	// 确认性能数据确实属于这个租户，防止串数据
	if performance.Spec.TenantRef.Name != tenantName {
		return input, fmt.Errorf(
			"tenant performance %q references tenant %q, expected %q",
			performance.Name,
			performance.Spec.TenantRef.Name,
			tenantName,
		)
	}

	// 只有 Running 状态的性能数据才有效，Stale 的不能用于决策
	if performance.Status.Phase == phaseRunning {
		if performance.Status.Performance.AvgTTFT != nil {
			input.AvgTTFT = nonNegative(performance.Status.Performance.AvgTTFT.Value)
			input.HasTTFT = true
		}
		if performance.Status.Performance.AvgQueue != nil {
			input.AvgQueue = nonNegative(performance.Status.Performance.AvgQueue.Value)
			input.HasQueue = true
		}
	}

	// 读租户配置，填阈值和 QPS
	var tenant platformv1.Tenant
	if err := r.Get(ctx, client.ObjectKey{Name: tenantName}, &tenant); err != nil {
		return input, fmt.Errorf("get tenant %q: %w", tenantName, err)
	}
	input.TTFTThresholdUp = positiveOrDefault(tenant.Spec.TTFTThresholdMs, defaultTTFTUp)
	input.TTFTThresholdDown = positiveOrDefault(tenant.Spec.TTFTScaleDownThresholdMs, defaultTTFTDown)
	input.QueueThresholdUp = positiveOrDefault(tenant.Spec.QueueThreshold, defaultQueueUp)
	input.QueueThresholdDown = positiveOrDefault(tenant.Spec.QueueScaleDownThreshold, defaultQueueDown)
	input.TenantQPS = nonNegative(tenant.Spec.QPS)

	// 缩容阈值不能大于等于扩容阈值，否则就一直在那来回弹
	if input.TTFTThresholdDown >= input.TTFTThresholdUp || input.QueueThresholdDown >= input.QueueThresholdUp {
		return input, fmt.Errorf(
			"tenant %q has invalid hysteresis: down thresholds must be lower than up thresholds",
			tenantName,
		)
	}

	// 读 Orchestrator 配置，一个租户只能有一个
	var configs platformv1.OrchestratorList
	if err := r.List(ctx, &configs, client.MatchingFields{orchConfigTenantIndex: tenantName}); err != nil {
		return input, fmt.Errorf("list orchestrators for tenant %q: %w", tenantName, err)
	}
	if len(configs.Items) == 0 {
		return input, &dependencyNotReadyError{resource: componentOrchestrator, name: tenantName}
	}
	if len(configs.Items) > 1 {
		return input, fmt.Errorf("tenant %q has %d orchestrator resources; exactly one is required", tenantName, len(configs.Items))
	}
	config := &configs.Items[0]
	input.OrchestratorName = config.Name

	// 冷却和副本限制，全部取非负数
	input.ScaleUpCooldown = nonNegative(config.Spec.ScaleUpCooldownSeconds)
	input.ScaleDownCooldown = nonNegative(config.Spec.ScaleDownCooldownSeconds)
	input.MinReplicas = nonNegative(config.Spec.MinReplicas)
	input.MaxReplicas = positiveOrDefault(config.Spec.MaxReplicas, defaultMaxReplicas)
	input.AllowScaleToZero = config.Spec.AllowScaleToZero

	if input.MinReplicas > input.MaxReplicas {
		return input, fmt.Errorf(
			"orchestrator %q has minReplicas %d greater than maxReplicas %d",
			config.Name,
			input.MinReplicas,
			input.MaxReplicas,
		)
	}

	// 拿到上次扩缩的时间戳，用于判断冷却
	if config.Status.LastScaleUpTime != nil {
		input.LastScaleUpTime = *config.Status.LastScaleUpTime.DeepCopy()
	}
	if config.Status.LastScaleDownTime != nil {
		input.LastScaleDownTime = *config.Status.LastScaleDownTime.DeepCopy()
	}

	// 兼容旧版本的 LastScaling 字段，直到两个方向的时间戳都迁移到新字段为止
	if config.Status.LastScaling != nil {
		switch config.Status.LastScaling.Action {
		case scalingActionUp:
			if input.LastScaleUpTime.IsZero() {
				input.LastScaleUpTime = config.Status.LastScaling.Time
			}
		case scalingActionDown:
			if input.LastScaleDownTime.IsZero() {
				input.LastScaleDownTime = config.Status.LastScaling.Time
			}
		}
	}

	// 收集模型列表，顺便算出每个模型能在哪些节点上跑
	models, err := r.collectAvailableModels(ctx, tenantName)
	if err != nil {
		return input, err
	}
	if err := r.attachEligibleNodes(ctx, tenantName, models); err != nil {
		return input, err
	}
	input.AvailableModels = models

	// 收集可用节点
	nodes, err := r.collectAvailableNodes(ctx, tenantName)
	if err != nil {
		return input, err
	}
	input.AvailableNodes = nodes

	// 收集现有实例
	instances, err := r.collectExistingInstances(ctx, tenantName)
	if err != nil {
		return input, err
	}
	input.ExistingInstances = instances

	// 把所有输入拼一起算个哈希，作为本次决策的 triggerID
	input.TriggerID = decisionTriggerID(performance, &tenant, config, models, nodes, instances)
	return input, nil
}

func positiveOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// decisionTriggerID 基于所有影响决策的输入计算一个哈希，输入不变则 ID 不变。
// 用于检测决策输入是否已变化，以及实例上的扩缩计划是否还匹配当前状态。
func decisionTriggerID(
	performance *platformv1.TenantPerformance,
	tenant *platformv1.Tenant,
	config *platformv1.Orchestrator,
	models []ModelInfo,
	nodes []NodeInfo,
	instances []InstanceInfo,
) string {
	parts := make([]string, 0, 4+len(models)+len(nodes)+len(instances))

	// 基础资源版本信息
	parts = append(parts,
		string(performance.UID),
		performance.ResourceVersion,
		fmt.Sprint(tenant.Generation),
		fmt.Sprint(config.Generation),
	)

	// 每个模型的属性及其合法节点列表（排序后拼接，保证稳定）
	for _, model := range models {
		eligible := make([]string, 0, len(model.EligibleNodeNames))
		for name, allowed := range model.EligibleNodeNames {
			if allowed {
				eligible = append(eligible, name)
			}
		}
		slices.Sort(eligible)
		parts = append(parts, fmt.Sprintf(
			"model:%s:%d:%d:%d:%d:%s",
			model.Name,
			model.GPUUnits,
			model.AbsoluteScore,
			model.ColdStartMs,
			model.MaxConcurrency,
			strings.Join(eligible, ","),
		))
	}

	// 每个节点的剩余资源
	for _, node := range nodes {
		parts = append(parts, fmt.Sprintf("node:%s:%d:%d", node.Name, node.RemainingGPU, node.RemainingConcurrency))
	}

	// 每个实例的当前状态
	for _, instance := range instances {
		parts = append(parts, fmt.Sprintf(
			"instance:%s:%s:%d:%d:%t",
			instance.Name,
			instance.ModelName,
			instance.CurrentReplicas,
			instance.EffectiveScore,
			instance.HasEffectiveScore,
		))
	}

	// 用 \x00 分割避免字段拼接时产生二义性，取 SHA256 前 12 字节
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:12])
}

// collectAvailableModels 收集租户允许使用的模型列表。
// 先通过 TenantModelPolicy 拿到允许的模型名，再去拿模型详情。
func (r *OrchestratorReconciler) collectAvailableModels(ctx context.Context, tenantName string) ([]ModelInfo, error) {
	var policies platformv1.TenantModelPolicyList
	if err := r.List(ctx, &policies, client.MatchingFields{orchPolicyIndex: tenantName}); err != nil {
		return nil, fmt.Errorf("list tenant model policies for tenant %q: %w", tenantName, err)
	}

	// 先过一遍策略，只有 Effect=Allow 并且 Deny 没拦截的才算
	modelNames := make(map[string]struct{})
	for i := range policies.Items {
		policy := &policies.Items[i]
		if policy.Spec.Effect == policyEffectAllow &&
			tenantModelAllowed(policies.Items, tenantName, policy.Spec.ModelRef.Name) {
			modelNames[policy.Spec.ModelRef.Name] = struct{}{}
		}
	}

	// 排序保证每次顺序一致
	sortedNames := make([]string, 0, len(modelNames))
	for name := range modelNames {
		sortedNames = append(sortedNames, name)
	}
	slices.Sort(sortedNames)

	models := make([]ModelInfo, 0, len(sortedNames))
	for _, name := range sortedNames {
		var model platformv1.Model
		if err := r.Get(ctx, client.ObjectKey{Name: name}, &model); err != nil {
			return nil, fmt.Errorf("get allowed model %q: %w", name, err)
		}
		score := 0
		if model.Status.AbsoluteScore != nil {
			score = nonNegative(*model.Status.AbsoluteScore)
		}
		models = append(models, ModelInfo{
			Name:           model.Name,
			GPUUnits:       model.Spec.GPUUnits,
			AbsoluteScore:  score,
			ColdStartMs:    model.Spec.ColdStartMs,
			MaxConcurrency: model.Spec.MaxConcurrency,
		})
	}
	return models, nil
}

// attachEligibleNodes 根据 TenantNodePolicy 和 ModelNodePolicy，给每个模型算出可以部署的节点列表。
func (r *OrchestratorReconciler) attachEligibleNodes(ctx context.Context, tenantName string, models []ModelInfo) error {
	var tenantPolicies platformv1.TenantNodePolicyList
	if err := r.List(ctx, &tenantPolicies, client.MatchingFields{orchNodePolIndex: tenantName}); err != nil {
		return fmt.Errorf("list tenant node policies for tenant %q: %w", tenantName, err)
	}
	var modelPolicies platformv1.ModelNodePolicyList
	if err := r.List(ctx, &modelPolicies); err != nil {
		return fmt.Errorf("list model node policies: %w", err)
	}

	// 为每个模型分别计算合法节点集合
	for i := range models {
		nodes := eligibleNodeNames(tenantName, models[i].Name, tenantPolicies.Items, modelPolicies.Items)
		models[i].EligibleNodeNames = make(map[string]bool, len(nodes))
		for _, node := range nodes {
			models[i].EligibleNodeNames[node] = true
		}
	}
	return nil
}

// collectAvailableNodes 收集租户可用的节点，并计算每个节点的剩余 GPU 和并发。
// 只有 Effect=Allow 且没有 Deny 的节点才会被纳入。
func (r *OrchestratorReconciler) collectAvailableNodes(ctx context.Context, tenantName string) ([]NodeInfo, error) {
	var policies platformv1.TenantNodePolicyList
	if err := r.List(ctx, &policies, client.MatchingFields{orchNodePolIndex: tenantName}); err != nil {
		return nil, fmt.Errorf("list tenant node policies for tenant %q: %w", tenantName, err)
	}

	// 统计每个节点的 Allow 和 Deny 标记，Deny 优先
	effectsByNode := make(map[string]map[string]bool)
	for i := range policies.Items {
		policy := &policies.Items[i]
		if !policy.DeletionTimestamp.IsZero() {
			continue
		}
		effects := effectsByNode[policy.Spec.NodeRef.Name]
		if effects == nil {
			effects = make(map[string]bool)
			effectsByNode[policy.Spec.NodeRef.Name] = effects
		}
		effects[policy.Spec.Effect] = true
	}

	// 只要明确 Allow 且没有 Deny 的节点
	names := make([]string, 0, len(effectsByNode))
	for name, effects := range effectsByNode {
		if effects[policyEffectAllow] && !effects[policyEffectDeny] {
			names = append(names, name)
		}
	}
	slices.Sort(names)

	nodes := make([]NodeInfo, 0, len(names))
	for _, name := range names {
		var node platformv1.WorkerNode
		if err := r.Get(ctx, client.ObjectKey{Name: name}, &node); err != nil {
			return nil, fmt.Errorf("get allowed worker node %q: %w", name, err)
		}
		// 剩余资源 = 总容量 - 已用，负数归零
		nodes = append(nodes, NodeInfo{
			Name:                 node.Name,
			RemainingGPU:         nonNegative(node.Spec.GPU - node.Status.UsedGPU),
			RemainingConcurrency: nonNegative(node.Spec.MaxConcurrency - node.Status.UsedConcurrency),
		})
	}
	return nodes, nil
}

// collectExistingInstances 收集该租户下所有未删除的实例，并按名称排序保证稳定。
func (r *OrchestratorReconciler) collectExistingInstances(ctx context.Context, tenantName string) ([]InstanceInfo, error) {
	var instances platformv1.SimulatorInstanceList
	if err := r.List(ctx, &instances, client.MatchingFields{orchSimIndex: tenantName}); err != nil {
		return nil, fmt.Errorf("list simulator instances for tenant %q: %w", tenantName, err)
	}

	result := make([]InstanceInfo, 0, len(instances.Items))
	for i := range instances.Items {
		instance := &instances.Items[i]
		if !instance.DeletionTimestamp.IsZero() {
			continue
		}
		info := InstanceInfo{
			Name:             instance.Name,
			ModelName:        instance.Spec.ModelRef.Name,
			CurrentReplicas:  instance.Spec.Replicas,
			LastScaleTrigger: instance.Annotations[lastScaleTriggerKey],
			PendingScalePlan: instance.Annotations[pendingScalePlanKey],
		}
		if instance.Status.EffectiveScore != nil {
			info.EffectiveScore = *instance.Status.EffectiveScore
			info.HasEffectiveScore = true
		}
		result = append(result, info)
	}

	// 排序保证稳定
	slices.SortFunc(result, func(left, right InstanceInfo) int {
		return cmp.Compare(left.Name, right.Name)
	})
	return result, nil
}
