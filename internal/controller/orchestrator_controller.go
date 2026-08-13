package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	"github.com/3900563672/hello-k8s-ai/internal/observability"

	"go.opentelemetry.io/otel/attribute"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const orchestratorSyncPeriod = 10 * time.Second

// OrchestratorReconciler 根据租户级状态做出一次扩缩容决策。
// Now 是一个可替换的时间函数，在测试中可以注入假时间。
type OrchestratorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Now    func() time.Time
}

// +kubebuilder:rbac:groups=platform.study.com,resources=tenantperformances,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=tenants,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=tenantmodelpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=tenantnodepolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=modelnodepolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=models,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=workernodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=simulatorinstances,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=platform.study.com,resources=simulatorinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.study.com,resources=orchestrators,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=orchestrators/status,verbs=get;update;patch

func (r *OrchestratorReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (result ctrl.Result, reconcileErr error) {
	// 开启一次 reconcile 的监控和追踪
	ctx, observation := beginReconcile(ctx, componentOrchestrator, req)
	defer func() { observation.finish(result, reconcileErr) }()

	logger := log.FromContext(ctx).WithValues(componentOrchestrator, req.Name)

	var config platformv1.Orchestrator
	if err := r.Get(ctx, req.NamespacedName, &config); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get orchestrator %q: %w", req.Name, err)
	}
	// 资源正在删除，不处理
	if !config.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	tenantName := config.Spec.TenantRef.Name
	observation.span.SetAttributes(
		attribute.String(traceAttributeTenantName, tenantName),
		attribute.Int64("k8s.resource.generation", config.Generation),
		attribute.String("k8s.resource.uid", string(config.UID)),
	)

	// 租户名不能为空
	if tenantName == "" {
		err := fmt.Errorf("orchestrator %q has an empty tenant reference", config.Name)
		_ = r.setOrchestratorReadyCondition(ctx, config.Name, metav1.ConditionFalse, "InputInvalid", err.Error())
		return ctrl.Result{}, err
	}

	// 记录决策耗时
	decisionStarted := time.Now()
	defer func() { orchestratorDecisionDuration.Observe(durationSeconds(decisionStarted)) }()

	// 获取租户的性能数据，如果还没准备好就延迟重试
	performanceCtx, performanceSpan := startOperation(
		ctx,
		componentOrchestrator,
		"load-performance",
		attribute.String(traceAttributeTenantName, tenantName),
	)
	performance, err := r.performanceForTenant(performanceCtx, tenantName)
	observability.EndSpan(performanceSpan, err)
	if err != nil {
		// 依赖未就绪（比如还没有 TenantPerformance），需要等待
		_, dependencyPending := errors.AsType[*dependencyNotReadyError](err)
		if dependencyPending {
			_ = r.setOrchestratorReadyCondition(ctx, config.Name, metav1.ConditionFalse, "MetricsNotReady", err.Error())
			return ctrl.Result{RequeueAfter: orchestratorSyncPeriod}, nil
		}
		_ = r.setOrchestratorReadyCondition(ctx, config.Name, metav1.ConditionFalse, "InputInvalid", err.Error())
		return ctrl.Result{}, err
	}

	// 收集决策所需的所有输入（阈值、模型、节点、实例等）
	inputCtx, inputSpan := startOperation(
		ctx,
		componentOrchestrator,
		"gather-decision-input",
		attribute.String(traceAttributeTenantName, tenantName),
	)
	input, err := r.gatherDecisionInput(inputCtx, tenantName, performance)
	observability.EndSpan(inputSpan, err)
	if err != nil {
		// 如果某个依赖资源还没创建，可以等一等
		_, dependencyPending := errors.AsType[*dependencyNotReadyError](err)
		if dependencyPending || apierrors.IsNotFound(err) {
			logger.V(1).Info("waiting for orchestration dependency", "error", err)
			_ = r.setOrchestratorReadyCondition(ctx, config.Name, metav1.ConditionFalse, "DependencyNotReady", err.Error())
			return ctrl.Result{RequeueAfter: orchestratorSyncPeriod}, nil
		}
		_ = r.setOrchestratorReadyCondition(ctx, config.Name, metav1.ConditionFalse, "InputInvalid", err.Error())
		return ctrl.Result{}, err
	}

	// 统计有多少实例还带着未完成的扩缩计划
	pendingPlans := 0
	for _, instance := range input.ExistingInstances {
		if instance.PendingScalePlan != "" {
			pendingPlans++
		}
	}
	orchestratorPendingPlans.Observe(float64(pendingPlans))

	// 先尝试恢复上次没执行完的扩缩操作
	recoveryCtx, recoverySpan := startOperation(
		ctx,
		componentOrchestrator,
		"resume-pending-scaling",
		attribute.Int("scaling.pending_plan_count", pendingPlans),
	)
	resumed, err := r.resumePendingScaling(recoveryCtx, input)
	observability.EndSpan(recoverySpan, err, attribute.Bool("scaling.plan_resumed", resumed))
	if err != nil {
		_ = r.setOrchestratorReadyCondition(ctx, config.Name, metav1.ConditionFalse, "ScalingRecoveryFailed", err.Error())
		return ctrl.Result{}, err
	}
	// 如果有恢复动作，立刻 requeue 检查结果
	if resumed {
		return ctrl.Result{RequeueAfter: immediateRequeueAfter}, nil
	}

	// 给还没 EffectiveScore 的实例算一个初始分数
	scoreCtx, scoreSpan := startOperation(ctx, componentOrchestrator, "initialize-effective-scores")
	err = r.initializeEffectiveScores(scoreCtx, input)
	observability.EndSpan(scoreSpan, err)
	if err != nil {
		_ = r.setOrchestratorReadyCondition(ctx, config.Name, metav1.ConditionFalse, "ScoreInitializationFailed", err.Error())
		return ctrl.Result{}, err
	}

	// 真正的扩缩容决策
	decision := DecideAt(input, r.currentTime())
	action := decisionActionLabel(decision.Action)
	reason := valueOrDefault(decision.Reason, "unspecified")
	orchestratorDecisions.WithLabelValues(action, reason).Inc()

	// 把决策结果挂到 span 上
	observation.span.SetAttributes(
		attribute.String("scaling.action", action),
		attribute.String("scaling.reason", reason),
		attribute.String("scaling.trigger_id", input.TriggerID),
		attribute.Int("scaling.target_replicas", decision.TargetReplicas),
		attribute.Int("performance.ttft_ms", input.AvgTTFT),
		attribute.Int("performance.queue_depth", input.AvgQueue),
	)

	// 如果连一个有效的性能指标都没有，不能做决策，等下一轮
	if !input.HasTTFT && !input.HasQueue && decision.Action == NoOp {
		if err := r.setOrchestratorReadyCondition(
			ctx,
			config.Name,
			metav1.ConditionFalse,
			"MetricsNotReady",
			"waiting for a fresh TTFT or queue metric",
		); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: nextOrchestratorRequeue(decision.RequeueAfter)}, nil
	}

	// 执行决策（更新副本数、记录等）
	executeCtx, executeSpan := startOperation(
		ctx,
		componentOrchestrator,
		"apply-decision",
		attribute.String("scaling.action", action),
		attribute.String("scaling.reason", reason),
		attribute.String(traceAttributeSimulatorInstanceName, decision.InstanceName),
	)
	err = r.applyDecision(executeCtx, decision, input)
	observability.EndSpan(executeSpan, err)
	if err != nil {
		// 如果是决策过时（实例状态已变），立刻 requeue
		if errors.Is(err, errStaleDecision) {
			return ctrl.Result{RequeueAfter: immediateRequeueAfter}, nil
		}
		_ = r.setOrchestratorReadyCondition(ctx, config.Name, metav1.ConditionFalse, "ScalingFailed", err.Error())
		return ctrl.Result{}, err
	}

	// 本次 reconcile 成功，设置 Ready condition
	if err := r.setOrchestratorReadyCondition(
		ctx,
		config.Name,
		metav1.ConditionTrue,
		"Reconciled",
		"the latest metrics and policy inputs have been reconciled",
	); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: nextOrchestratorRequeue(decision.RequeueAfter)}, nil
}

// 将内部决策动作常量转为指标标签
func decisionActionLabel(action DecisionAction) string {
	switch action {
	case ScaleUp:
		return "scale_up"
	case ScaleDown:
		return "scale_down"
	default:
		return "no_op"
	}
}

// performanceForTenant 获取租户的唯一 TenantPerformance 资源，不存在或多于一个都报错。
func (r *OrchestratorReconciler) performanceForTenant(
	ctx context.Context,
	tenantName string,
) (*platformv1.TenantPerformance, error) {
	var performances platformv1.TenantPerformanceList
	// 用索引快速查到该租户的性能数据
	if err := r.List(ctx, &performances, client.MatchingFields{orchPerformanceIndex: tenantName}); err != nil {
		return nil, fmt.Errorf("list tenant performances for tenant %q: %w", tenantName, err)
	}
	if len(performances.Items) == 0 {
		return nil, &dependencyNotReadyError{resource: "tenant performance", name: tenantName}
	}
	if len(performances.Items) > 1 {
		return nil, fmt.Errorf("tenant %q has %d tenant performance resources; exactly one is required", tenantName, len(performances.Items))
	}
	return &performances.Items[0], nil
}

// currentTime 返回当前时间，测试时可用注入的 Now 函数替代。
func (r *OrchestratorReconciler) currentTime() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// nextOrchestratorRequeue 计算下一次 reconcile 的延迟，不超过 syncPeriod。
func nextOrchestratorRequeue(requested time.Duration) time.Duration {
	if requested <= 0 {
		return orchestratorSyncPeriod
	}
	return min(requested, orchestratorSyncPeriod)
}

// initializeEffectiveScores 为还没有有效分数的实例计算初始分数。
// 已有分数的实例保持稳定，直到下一次扩容重新计算。
func (r *OrchestratorReconciler) initializeEffectiveScores(ctx context.Context, input DecisionInput) error {
	// 把模型信息放到 map 里方便查找
	models := make(map[string]ModelInfo, len(input.AvailableModels))
	for _, model := range input.AvailableModels {
		models[model.Name] = model
	}
	for _, instance := range input.ExistingInstances {
		// 已经有分数就跳过
		if instance.HasEffectiveScore {
			continue
		}
		model, ok := models[instance.ModelName]
		if !ok {
			continue
		}
		// 在可用节点里找一个最好的分数
		score, found := bestEffectiveScoreForModel(model, input.AvailableNodes)
		if !found {
			continue
		}
		if err := r.updateInstanceEffectiveScore(ctx, instance.Name, score); err != nil {
			return fmt.Errorf("initialize effective score for %q: %w", instance.Name, err)
		}
	}
	return nil
}

// setOrchestratorReadyCondition 更新 Orchestrator 资源的 Ready condition。
// 用了 Merge Patch，避免覆盖其他 controller 改的字段。
func (r *OrchestratorReconciler) setOrchestratorReadyCondition(
	ctx context.Context,
	name string,
	status metav1.ConditionStatus,
	reason string,
	message string,
) error {
	return retryOnConflict(func() error {
		var config platformv1.Orchestrator
		if err := r.Get(ctx, client.ObjectKey{Name: name}, &config); err != nil {
			return err
		}
		before := config.DeepCopy()
		meta.SetStatusCondition(&config.Status.Conditions, metav1.Condition{
			Type:               conditionTypeReady,
			Status:             status,
			ObservedGeneration: config.Generation,
			Reason:             reason,
			Message:            message,
		})
		// 如果 condition 没变化，不用发起 API 请求
		if conditionsEqual(before.Status.Conditions, config.Status.Conditions) {
			return nil
		}
		return r.Status().Patch(ctx, &config, client.MergeFrom(before))
	})
}

// orchestratorRequestsForTenant 根据租户名找到对应的 Orchestrator 资源，生成 reconcile 请求。
func (r *OrchestratorReconciler) orchestratorRequestsForTenant(ctx context.Context, tenantName string) []reconcile.Request {
	if tenantName == "" {
		return nil
	}
	var configs platformv1.OrchestratorList
	// 用索引查
	if err := r.List(ctx, &configs, client.MatchingFields{orchConfigTenantIndex: tenantName}); err != nil {
		log.FromContext(ctx).Error(err, "list Orchestrators while mapping event", "tenant", tenantName)
		return nil
	}
	requests := make([]reconcile.Request, 0, len(configs.Items))
	for i := range configs.Items {
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Name: configs.Items[i].Name}})
	}
	return requests
}

// mapNamedTenant 将对象的名字当作租户名去查 Orchestrator。
func (r *OrchestratorReconciler) mapNamedTenant(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.orchestratorRequestsForTenant(ctx, obj.GetName())
}

// mapReferencedTenant 从不同资源里抽出租户名，再去找对应的 Orchestrator。
func (r *OrchestratorReconciler) mapReferencedTenant(ctx context.Context, obj client.Object) []reconcile.Request {
	var tenantName string
	switch value := obj.(type) {
	case *platformv1.TenantPerformance:
		tenantName = value.Spec.TenantRef.Name
	case *platformv1.TenantModelPolicy:
		tenantName = value.Spec.TenantRef.Name
	case *platformv1.TenantNodePolicy:
		tenantName = value.Spec.TenantRef.Name
	case *platformv1.SimulatorInstance:
		tenantName = value.Spec.TenantRef.Name
	}
	return r.orchestratorRequestsForTenant(ctx, tenantName)
}

// mapModelToOrchestrators 模型变化时，通过 TenantModelPolicy 找到所有引用它的租户，触发 Orchestrator reconcile。
func (r *OrchestratorReconciler) mapModelToOrchestrators(ctx context.Context, obj client.Object) []reconcile.Request {
	modelName := obj.GetName()
	// 如果来源是 ModelNodePolicy，取的是里面的 modelRef
	if policy, ok := obj.(*platformv1.ModelNodePolicy); ok {
		modelName = policy.Spec.ModelRef.Name
	}
	var policies platformv1.TenantModelPolicyList
	if err := r.List(ctx, &policies); err != nil {
		log.FromContext(ctx).Error(err, "list TenantModelPolicies while mapping model event", "model", modelName)
		return nil
	}
	requests := make([]reconcile.Request, 0)
	seen := make(map[client.ObjectKey]struct{})
	for i := range policies.Items {
		policy := &policies.Items[i]
		if policy.Spec.ModelRef.Name != modelName {
			continue
		}
		// 同一个 Orchestrator 只触发一次
		for _, request := range r.orchestratorRequestsForTenant(ctx, policy.Spec.TenantRef.Name) {
			if _, exists := seen[request.NamespacedName]; exists {
				continue
			}
			seen[request.NamespacedName] = struct{}{}
			requests = append(requests, request)
		}
	}
	return requests
}

// mapWorkerNodeToOrchestrators 节点变化时，通过 TenantNodePolicy 找到受影响的租户，触发 Orchestrator。
func (r *OrchestratorReconciler) mapWorkerNodeToOrchestrators(ctx context.Context, obj client.Object) []reconcile.Request {
	var policies platformv1.TenantNodePolicyList
	if err := r.List(ctx, &policies); err != nil {
		log.FromContext(ctx).Error(err, "list TenantNodePolicies while mapping WorkerNode event", "node", obj.GetName())
		return nil
	}
	requests := make([]reconcile.Request, 0)
	seen := make(map[client.ObjectKey]struct{})
	for i := range policies.Items {
		policy := &policies.Items[i]
		if policy.Spec.NodeRef.Name != obj.GetName() {
			continue
		}
		for _, request := range r.orchestratorRequestsForTenant(ctx, policy.Spec.TenantRef.Name) {
			if _, exists := seen[request.NamespacedName]; exists {
				continue
			}
			seen[request.NamespacedName] = struct{}{}
			requests = append(requests, request)
		}
	}
	return requests
}

// SetupWithManager 注册控制器，建索引，配置 Watch 和 predicate。
func (r *OrchestratorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// 批量建字段索引，后面 List 的时候能直接按租户名查
	indexes := []fieldIndex{
		{&platformv1.SimulatorInstance{}, orchSimIndex, func(obj client.Object) []string {
			return nonEmptyIndexValue(obj.(*platformv1.SimulatorInstance).Spec.TenantRef.Name)
		}},
		{&platformv1.TenantModelPolicy{}, orchPolicyIndex, func(obj client.Object) []string {
			return nonEmptyIndexValue(obj.(*platformv1.TenantModelPolicy).Spec.TenantRef.Name)
		}},
		{&platformv1.TenantNodePolicy{}, orchNodePolIndex, func(obj client.Object) []string {
			return nonEmptyIndexValue(obj.(*platformv1.TenantNodePolicy).Spec.TenantRef.Name)
		}},
		{&platformv1.Orchestrator{}, orchConfigTenantIndex, func(obj client.Object) []string {
			return nonEmptyIndexValue(obj.(*platformv1.Orchestrator).Spec.TenantRef.Name)
		}},
		{&platformv1.TenantPerformance{}, orchPerformanceIndex, func(obj client.Object) []string {
			return nonEmptyIndexValue(obj.(*platformv1.TenantPerformance).Spec.TenantRef.Name)
		}},
	}
	if err := registerFieldIndexes(context.Background(), mgr, componentOrchestrator, indexes...); err != nil {
		return err
	}

	// 各资源变化的过滤条件
	performanceChanged := lifecyclePredicate(func(e event.UpdateEvent) bool {
		oldPerformance, oldOK := e.ObjectOld.(*platformv1.TenantPerformance)
		newPerformance, newOK := e.ObjectNew.(*platformv1.TenantPerformance)
		return oldOK && newOK &&
			(oldPerformance.Spec.TenantRef.Name != newPerformance.Spec.TenantRef.Name ||
				oldPerformance.Status.Phase != newPerformance.Status.Phase ||
				!optionalEqual(oldPerformance.Status.ObservedAt, newPerformance.Status.ObservedAt) ||
				!optionalEqual(oldPerformance.Status.Performance.AvgTTFT, newPerformance.Status.Performance.AvgTTFT) ||
				!optionalEqual(oldPerformance.Status.Performance.AvgQueue, newPerformance.Status.Performance.AvgQueue))
	})
	instanceChanged := lifecyclePredicate(func(e event.UpdateEvent) bool {
		oldInstance, oldOK := e.ObjectOld.(*platformv1.SimulatorInstance)
		newInstance, newOK := e.ObjectNew.(*platformv1.SimulatorInstance)
		return oldOK && newOK &&
			(oldInstance.Spec.Replicas != newInstance.Spec.Replicas ||
				oldInstance.Spec.TenantRef.Name != newInstance.Spec.TenantRef.Name ||
				oldInstance.Spec.ModelRef.Name != newInstance.Spec.ModelRef.Name ||
				oldInstance.Annotations[pendingScalePlanKey] != newInstance.Annotations[pendingScalePlanKey] ||
				!oldInstance.DeletionTimestamp.Equal(newInstance.DeletionTimestamp))
	})
	workerNodeChanged := lifecyclePredicate(func(e event.UpdateEvent) bool {
		oldNode, oldOK := e.ObjectOld.(*platformv1.WorkerNode)
		newNode, newOK := e.ObjectNew.(*platformv1.WorkerNode)
		return oldOK && newOK &&
			(oldNode.Generation != newNode.Generation ||
				oldNode.Status.UsedGPU != newNode.Status.UsedGPU ||
				oldNode.Status.UsedConcurrency != newNode.Status.UsedConcurrency)
	})

	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		Named("orchestrator").
		For(&platformv1.Orchestrator{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&platformv1.TenantPerformance{}, handler.EnqueueRequestsFromMapFunc(r.mapReferencedTenant), builder.WithPredicates(performanceChanged)).
		Watches(&platformv1.SimulatorInstance{}, handler.EnqueueRequestsFromMapFunc(r.mapReferencedTenant), builder.WithPredicates(instanceChanged)).
		Watches(&platformv1.WorkerNode{}, handler.EnqueueRequestsFromMapFunc(r.mapWorkerNodeToOrchestrators), builder.WithPredicates(workerNodeChanged))
	// 以下资源在 generation 变化时才触发
	watchGenerationChanges(controllerBuilder, &platformv1.Tenant{}, r.mapNamedTenant)
	watchGenerationChanges(controllerBuilder, &platformv1.TenantModelPolicy{}, r.mapReferencedTenant)
	watchGenerationChanges(controllerBuilder, &platformv1.TenantNodePolicy{}, r.mapReferencedTenant)
	watchGenerationChanges(controllerBuilder, &platformv1.Model{}, r.mapModelToOrchestrators)
	watchGenerationChanges(controllerBuilder, &platformv1.ModelNodePolicy{}, r.mapModelToOrchestrators)
	return controllerBuilder.WithOptions(controller.Options{MaxConcurrentReconciles: 1}).Complete(r)
}

// nonEmptyIndexValue 空字符串不建索引，返回 nil。
func nonEmptyIndexValue(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}
