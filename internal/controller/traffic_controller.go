package controller

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	"github.com/3900563672/hello-k8s-ai/internal/observability"

	"go.opentelemetry.io/otel/attribute"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	trafficTenantIndex   = "traffic.spec.tenantRef.name"
	trafficRefreshPeriod = 10 * time.Second
	instanceMetricMaxAge = 30 * time.Second // 实例指标最大有效期，过期视为无效
	trafficFinalizer     = "platform.study.com/traffic-distribution"
)

// TrafficReconciler 只负责给 SimulatorInstance 分配 QPS，基于租户的总 QPS 和各实例的运行时分数。
// 分数是实例整体容量的体现，分配结果保证总 QPS 不变，包括零流量场景。
type TrafficReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Now    func() time.Time
}

// +kubebuilder:rbac:groups=platform.study.com,resources=tenants,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=simulatorinstances,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=platform.study.com,resources=simulatorinstances/status,verbs=get

func (r *TrafficReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (result ctrl.Result, reconcileErr error) {
	ctx, observation := beginReconcile(ctx, componentTraffic, req)
	defer func() { observation.finish(result, reconcileErr) }()

	var tenant platformv1.Tenant
	err := r.Get(ctx, req.NamespacedName, &tenant)
	if apierrors.IsNotFound(err) {
		// 租户被删了，清理遗留的 finalizer
		return ctrl.Result{}, r.removeLegacyTrafficFinalizers(ctx, req.Name)
	}
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("get tenant %q: %w", req.Name, err)
	}
	if !tenant.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.removeLegacyTrafficFinalizers(ctx, tenant.Name)
	}
	observation.span.SetAttributes(
		attribute.String(traceAttributeTenantName, tenant.Name),
		attribute.Int("traffic.requested_qps", nonNegative(tenant.Spec.QPS)),
		attribute.Int64("k8s.resource.generation", tenant.Generation),
	)

	if err := r.distributeTrafficForTenant(ctx, &tenant); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.removeLegacyTrafficFinalizers(ctx, tenant.Name); err != nil {
		return ctrl.Result{}, err
	}
	// 定期触发，因为指标过期不会产生 K8s 事件，需要主动重算
	return ctrl.Result{RequeueAfter: trafficRefreshPeriod}, nil
}

// distributeTrafficForTenant 为租户的所有实例分配 QPS，更新指标并记录分配模式。
func (r *TrafficReconciler) distributeTrafficForTenant(
	ctx context.Context,
	tenant *platformv1.Tenant,
) (operationErr error) {
	started := time.Now()
	requestedQPS := nonNegative(tenant.Spec.QPS)
	allocatedQPS := 0
	mode := "no_instances"

	ctx, span := startOperation(
		ctx,
		componentTraffic,
		"allocate",
		attribute.String(traceAttributeTenantName, tenant.Name),
		attribute.Int("traffic.requested_qps", requestedQPS),
	)
	defer func() {
		outcome := operationOutcomeSuccess
		if operationErr != nil {
			outcome = operationOutcomeError
		}
		trafficAllocationRuns.WithLabelValues(outcome, mode).Inc()
		trafficAllocationDuration.Observe(durationSeconds(started))
		trafficRequestedQPS.Observe(float64(requestedQPS))
		trafficAllocatedQPS.Observe(float64(allocatedQPS))
		observability.EndSpan(
			span,
			operationErr,
			attribute.String("traffic.mode", mode),
			attribute.Int("traffic.allocated_qps", allocatedQPS),
		)
		observeOperation(componentTraffic, "allocate", operationErr)
	}()

	// 收集租户下所有有效实例及其分数
	instances, err := r.collectTrafficInstances(ctx, tenant.Name)
	if err != nil {
		return err
	}
	if len(instances) == 0 {
		return nil
	}

	mode = trafficAllocationMode(requestedQPS, instances)
	allocations, _ := allocateTraffic(requestedQPS, instances)
	updateErrors := make([]error, 0)
	for _, instance := range instances {
		if err := r.updateInstanceQPS(ctx, instance.name, allocations[instance.name]); err != nil {
			updateErrors = append(updateErrors, fmt.Errorf("update QPS for %q: %w", instance.name, err))
			continue
		}
		allocatedQPS += allocations[instance.name]
	}
	return errors.Join(updateErrors...)
}

// trafficAllocationMode 返回流量分配模式：zero_traffic、score_weighted 或 equal_fallback
func trafficAllocationMode(totalQPS int, instances []instanceData) string {
	if len(instances) == 0 {
		return "no_instances"
	}
	if totalQPS == 0 {
		return "zero_traffic"
	}
	for _, instance := range instances {
		if instance.score > 0 {
			return "score_weighted"
		}
	}
	return "equal_fallback"
}

type instanceData struct {
	name  string
	score int
}

// collectTrafficInstances 获取租户下所有有效实例和它们的分数，过期或无效的分数设为 0。
func (r *TrafficReconciler) collectTrafficInstances(ctx context.Context, tenantName string) ([]instanceData, error) {
	var instances platformv1.SimulatorInstanceList
	if err := r.List(ctx, &instances, client.MatchingFields{trafficTenantIndex: tenantName}); err != nil {
		return nil, fmt.Errorf("list simulator instances for tenant %q: %w", tenantName, err)
	}

	now := r.currentTime()
	result := make([]instanceData, 0, len(instances.Items))
	for i := range instances.Items {
		instance := &instances.Items[i]
		// 跳过已删除或副本为 0 的实例
		if !instance.DeletionTimestamp.IsZero() || instance.Spec.Replicas == 0 {
			continue
		}
		score := 0
		// 只有运行中且指标新鲜的实例其分数才有效
		if instance.Status.Phase == phaseRunning &&
			metricIsFresh(instance.Status.ObservedAt, now) &&
			instance.Status.Score != nil && *instance.Status.Score > 0 {
			score = *instance.Status.Score
		}
		result = append(result, instanceData{name: instance.Name, score: score})
	}
	// 按名字排序保证稳定
	slices.SortFunc(result, func(left, right instanceData) int {
		return cmp.Compare(left.name, right.name)
	})
	return result, nil
}

type trafficRemainder struct {
	name     string
	fraction float64 // 分配后的小数部分，用于 Largest Remainder 方法
}

// allocateTraffic 使用 Largest Remainder 算法分配 QPS。
// 所有分数为 0 的实例权重视为 1 平均分配；否则只有分数 >0 的实例能分到流量。
// 返回 map 和分配的总 QPS（一定等于 totalQPS）。
func allocateTraffic(totalQPS int, instances []instanceData) (map[string]int, int) {
	totalQPS = nonNegative(totalQPS)
	allocations := make(map[string]int, len(instances))
	if len(instances) == 0 {
		return allocations, 0
	}

	// 按名称排序
	sorted := append([]instanceData(nil), instances...)
	slices.SortFunc(sorted, func(left, right instanceData) int {
		return cmp.Compare(left.name, right.name)
	})
	for _, instance := range sorted {
		allocations[instance.name] = 0
	}
	if totalQPS == 0 {
		return allocations, 0
	}

	totalWeight := 0.0
	weights := make([]float64, len(sorted))
	for i, instance := range sorted {
		if instance.score > 0 {
			weights[i] = float64(instance.score)
			totalWeight += weights[i]
		}
	}
	// 如果所有分数都为零，则权重都为 1，平均分配
	if totalWeight == 0 {
		for i := range weights {
			weights[i] = 1
		}
		totalWeight = float64(len(weights))
	}

	assigned := 0
	remainders := make([]trafficRemainder, 0, len(sorted))
	for i, instance := range sorted {
		exact := float64(totalQPS) * weights[i] / totalWeight
		whole := int(math.Floor(exact))
		allocations[instance.name] = whole
		assigned += whole
		remainders = append(remainders, trafficRemainder{
			name:     instance.name,
			fraction: exact - float64(whole),
		})
	}
	// 按小数部分从大到小排序，将剩余 QPS 逐个分给小数部分最大的实例
	slices.SortStableFunc(remainders, func(left, right trafficRemainder) int {
		if left.fraction != right.fraction {
			return cmp.Compare(right.fraction, left.fraction)
		}
		return cmp.Compare(left.name, right.name)
	})
	for i := 0; i < totalQPS-assigned; i++ {
		allocations[remainders[i%len(remainders)].name]++
	}
	return allocations, totalQPS
}

// updateInstanceQPS 更新单个实例的 QPS，冲突自动重试。
func (r *TrafficReconciler) updateInstanceQPS(ctx context.Context, instanceName string, qps int) error {
	qps = nonNegative(qps)
	return retryOnConflict(func() error {
		var instance platformv1.SimulatorInstance
		if err := r.Get(ctx, client.ObjectKey{Name: instanceName}, &instance); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if !instance.DeletionTimestamp.IsZero() || instance.Spec.Traffic.QPS == qps {
			return nil
		}
		before := instance.DeepCopy()
		instance.Spec.Traffic.QPS = qps
		return r.Patch(ctx, &instance, client.MergeFrom(before))
	})
}

// 清理该租户下实例遗留的 traffic finalizer
func (r *TrafficReconciler) removeLegacyTrafficFinalizers(ctx context.Context, tenantName string) error {
	return removeLegacyInstanceFinalizers(ctx, r.Client, trafficTenantIndex, tenantName, trafficFinalizer)
}

func (r *TrafficReconciler) mapInstanceToTenant(ctx context.Context, obj client.Object) []reconcile.Request {
	return mapSimulatorInstanceToTenant(ctx, obj)
}

func (r *TrafficReconciler) currentTime() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// metricIsFresh 检查指标时间戳是否在有效期内
func metricIsFresh(observedAt *metav1.Time, now time.Time) bool {
	if observedAt == nil || observedAt.IsZero() {
		return false
	}
	age := now.Sub(observedAt.Time)
	return age >= -instanceMetricMaxAge && age <= instanceMetricMaxAge
}

func (r *TrafficReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := registerFieldIndexes(
		context.Background(),
		mgr,
		componentTraffic,
		simulatorInstanceFieldIndex(trafficTenantIndex, func(instance *platformv1.SimulatorInstance) string {
			return instance.Spec.TenantRef.Name
		}),
	); err != nil {
		return err
	}

	// 实例分数、指标新鲜度、Phase、副本数、租户引用或删除时间变化时重新分配
	instanceChanged := lifecyclePredicate(func(e event.UpdateEvent) bool {
		oldInstance, oldOK := e.ObjectOld.(*platformv1.SimulatorInstance)
		newInstance, newOK := e.ObjectNew.(*platformv1.SimulatorInstance)
		if !oldOK || !newOK {
			return false
		}
		return !optionalEqual(oldInstance.Status.Score, newInstance.Status.Score) ||
			!optionalEqual(oldInstance.Status.ObservedAt, newInstance.Status.ObservedAt) ||
			oldInstance.Status.Phase != newInstance.Status.Phase ||
			oldInstance.Spec.Replicas != newInstance.Spec.Replicas ||
			oldInstance.Spec.TenantRef.Name != newInstance.Spec.TenantRef.Name ||
			!oldInstance.DeletionTimestamp.Equal(newInstance.DeletionTimestamp)
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("traffic").
		For(&platformv1.Tenant{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&platformv1.SimulatorInstance{},
			handler.EnqueueRequestsFromMapFunc(r.mapInstanceToTenant),
			builder.WithPredicates(instanceChanged),
		).
		Complete(r)
}
