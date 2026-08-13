package controller

import (
	"context"
	"errors"
	"fmt"
	"reflect"
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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	tenantIndexField = "performancecollector.spec.tenantRef.name"
	// 旧版控制器遗留的 finalizer，只用于清理，不用于正常流程
	perfFinalizer            = "platform.study.com/performance-collector"
	performanceRefreshPeriod = 10 * time.Second
)

// PerformanceCollectorReconciler 负责将租户下所有 SimulatorInstance 的性能汇总到 TenantPerformance。
// 以 Tenant 为主键，这样 SimulatorInstance 删除时无需通过 finalizer 触发聚合，只需 Watch 实例变化映射到租户即可。
type PerformanceCollectorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Now    func() time.Time

	// tenantIndexField 允许在 fake-client 测试中注入索引名，生产环境用包级默认值。
	tenantIndexField string
}

// +kubebuilder:rbac:groups=platform.study.com,resources=tenants,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=simulatorinstances,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=platform.study.com,resources=tenantperformances,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=platform.study.com,resources=tenantperformances/status,verbs=get;update;patch

func (r *PerformanceCollectorReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (result ctrl.Result, reconcileErr error) {
	ctx, observation := beginReconcile(ctx, "performance-collector", req)
	defer func() { observation.finish(result, reconcileErr) }()

	logger := log.FromContext(ctx).WithValues("tenant", req.Name)

	var tenant platformv1.Tenant
	if err := r.Get(ctx, req.NamespacedName, &tenant); err != nil {
		if apierrors.IsNotFound(err) {
			// 租户被删了，清理所有实例上遗留的旧 finalizer
			return ctrl.Result{}, r.removeLegacyPerformanceFinalizers(ctx, req.Name)
		}
		return ctrl.Result{}, fmt.Errorf("get tenant %q: %w", req.Name, err)
	}
	if !tenant.DeletionTimestamp.IsZero() {
		// 租户正在删除，也做一次清理
		return ctrl.Result{}, r.removeLegacyPerformanceFinalizers(ctx, tenant.Name)
	}
	observation.span.SetAttributes(
		attribute.String(traceAttributeTenantName, tenant.Name),
		attribute.Int64("k8s.resource.generation", tenant.Generation),
	)

	// 保证 TenantPerformance 资源存在并绑定 OwnerReference
	ensureCtx, ensureSpan := startOperation(ctx, "performance-collector", "ensure-resource")
	err := r.ensureTenantPerformance(ensureCtx, &tenant)
	observability.EndSpan(ensureSpan, err)
	observeOperation("performance-collector", "ensure-resource", err)
	if err != nil {
		return ctrl.Result{}, err
	}
	// 重新计算并更新性能状态
	if err := r.recalculateTenantPerformance(ctx, tenant.Name); err != nil {
		logger.Error(err, "recalculate tenant performance")
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: performanceRefreshPeriod}, nil
}

// removeLegacyPerformanceFinalizers 清理租户下所有实例上遗留的 perfFinalizer。
func (r *PerformanceCollectorReconciler) removeLegacyPerformanceFinalizers(ctx context.Context, tenantName string) error {
	return removeLegacyInstanceFinalizers(ctx, r.Client, r.performanceTenantIndex(), tenantName, perfFinalizer)
}

func (r *PerformanceCollectorReconciler) performanceTenantIndex() string {
	if r.tenantIndexField != "" {
		return r.tenantIndexField
	}
	return tenantIndexField
}

// ensureTenantPerformance 确保每个租户都有一个 TenantPerformance，且 OwnerReference 指向该租户。
func (r *PerformanceCollectorReconciler) ensureTenantPerformance(ctx context.Context, tenant *platformv1.Tenant) error {
	performance := &platformv1.TenantPerformance{ObjectMeta: metav1.ObjectMeta{Name: tenant.Name}}
	_, err := controllerutil.CreateOrPatch(ctx, r.Client, performance, func() error {
		performance.Spec.TenantRef = platformv1.ObjectRef{Name: tenant.Name}
		if err := controllerutil.SetControllerReference(tenant, performance, r.Scheme); err != nil {
			return fmt.Errorf("set tenant owner: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("ensure tenant performance %q: %w", tenant.Name, err)
	}
	return nil
}

// recalculateTenantPerformance 收集租户下所有有效实例的性能数据，独立聚合 TTFT 和 Queue。
// 即使冷启动时 TTFT 缺失，队列数据仍然能体现压力，因此两个指标分开处理。
func (r *PerformanceCollectorReconciler) recalculateTenantPerformance(
	ctx context.Context,
	tenantName string,
) (operationErr error) {
	started := time.Now()
	freshSamples := 0
	ctx, span := startOperation(
		ctx,
		"performance-collector",
		"aggregate",
		attribute.String(traceAttributeTenantName, tenantName),
	)
	defer func() {
		performanceAggregationDuration.Observe(durationSeconds(started))
		performanceFreshSampleCount.Observe(float64(freshSamples))
		observability.EndSpan(span, operationErr, attribute.Int("performance.fresh_sample_count", freshSamples))
		observeOperation("performance-collector", "aggregate", operationErr)
	}()

	var instances platformv1.SimulatorInstanceList
	if err := r.List(ctx, &instances, client.MatchingFields{r.performanceTenantIndex(): tenantName}); err != nil {
		return fmt.Errorf("list simulator instances for tenant %q: %w", tenantName, err)
	}

	performance := make([]instancePerformance, 0, len(instances.Items))
	deleting := make([]*platformv1.SimulatorInstance, 0) // 待清理 finalizer 的实例
	var latestObservedAt *metav1.Time                    // 记录最新的观测时间戳
	now := r.currentTime()

	for i := range instances.Items {
		instance := &instances.Items[i]
		// 标记为删除的实例：计数后收集，等待移除 finalizer
		if !instance.DeletionTimestamp.IsZero() {
			performanceSamples.WithLabelValues("deleting").Inc()
			if controllerutil.ContainsFinalizer(instance, perfFinalizer) {
				deleting = append(deleting, instance)
			}
			continue
		}
		// 非 Running 状态不采信
		if instance.Status.Phase != phaseRunning {
			performanceSamples.WithLabelValues("not_running").Inc()
			continue
		}
		// 没有可用副本的实例不参与计算
		if instance.Status.AvailableReplicas == 0 {
			performanceSamples.WithLabelValues("unavailable").Inc()
			continue
		}
		// 指标太旧，视为过期
		if !metricIsFresh(instance.Status.ObservedAt, now) {
			performanceSamples.WithLabelValues("stale").Inc()
			continue
		}
		if instance.Status.Performance == nil {
			performanceSamples.WithLabelValues("missing_performance").Inc()
			continue
		}

		// 构造一次加权采样，Weight 用可用副本数
		sample := instancePerformance{Weight: instance.Status.AvailableReplicas}
		if instance.Status.Performance.TTFT != nil {
			sample.TTFT = instance.Status.Performance.TTFT.Value
			sample.HasTTFT = true
		}
		if instance.Status.Performance.Queue != nil {
			sample.Queue = instance.Status.Performance.Queue.Value
			sample.HasQueue = true
		}
		if sample.HasTTFT || sample.HasQueue {
			performance = append(performance, sample)
			freshSamples++
			performanceSamples.WithLabelValues("fresh").Inc()
			if latestObservedAt == nil || latestObservedAt.Before(instance.Status.ObservedAt) {
				latestObservedAt = instance.Status.ObservedAt.DeepCopy()
			}
		} else {
			performanceSamples.WithLabelValues("empty").Inc()
		}
	}

	avgTTFT, avgQueue, hasTTFT, hasQueue := calculatePerformanceSummary(performanceInput{Instances: performance})
	if err := r.updateTenantPerformanceStatus(
		ctx,
		tenantName,
		avgTTFT,
		avgQueue,
		hasTTFT,
		hasQueue,
		latestObservedAt,
		len(performance),
	); err != nil {
		return err
	}

	// 清理遗留的 finalizer，不阻塞主流程
	var cleanupErrors []error
	for _, instance := range deleting {
		if err := removeFinalizer(ctx, r.Client, instance, perfFinalizer); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	return errors.Join(cleanupErrors...)
}

// updateTenantPerformanceStatus 更新 TenantPerformance 的状态，含平均值、Phase、条件等。
func (r *PerformanceCollectorReconciler) updateTenantPerformanceStatus(
	ctx context.Context,
	tenantName string,
	avgTTFT int,
	avgQueue int,
	hasTTFT bool,
	hasQueue bool,
	observedAt *metav1.Time,
	sampleCount int,
) error {
	return retryOnConflict(func() error {
		var performance platformv1.TenantPerformance
		if err := r.Get(ctx, client.ObjectKey{Name: tenantName}, &performance); err != nil {
			return err
		}
		before := performance.DeepCopy()

		// 先清空旧指标，再根据实际可用性填写
		performance.Status.Performance = platformv1.PerformanceStatus{}
		performance.Status.ObservedAt = observedAt.DeepCopy()
		performance.Status.SampleCount = nonNegative(sampleCount)
		if hasTTFT {
			performance.Status.Performance.AvgTTFT = &platformv1.PerformanceMetric{Value: avgTTFT, Unit: "ms"}
		}
		if hasQueue {
			performance.Status.Performance.AvgQueue = &platformv1.PerformanceMetric{Value: avgQueue, Unit: "requests"}
		}

		// 只要有一个指标可用，Phase 就是 Running，否则 Stale
		condition := metav1.Condition{
			Type:               "MetricsReady",
			ObservedGeneration: performance.Generation,
		}
		if hasTTFT || hasQueue {
			performance.Status.Phase = phaseRunning
			condition.Status = metav1.ConditionTrue
			condition.Reason = "MetricsAvailable"
			condition.Message = "at least one fresh, running simulator metric is available"
		} else {
			performance.Status.Phase = "Stale"
			condition.Status = metav1.ConditionFalse
			condition.Reason = "NoMetrics"
			condition.Message = "no running simulator instance has a fresh performance metric"
		}
		meta.SetStatusCondition(&performance.Status.Conditions, condition)

		// 状态没变就不发了
		if reflect.DeepEqual(before.Status, performance.Status) {
			return nil
		}
		if err := r.Status().Patch(ctx, &performance, client.MergeFrom(before)); err != nil {
			return fmt.Errorf("patch tenant performance %q status: %w", tenantName, err)
		}
		return nil
	})
}

func (r *PerformanceCollectorReconciler) currentTime() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *PerformanceCollectorReconciler) mapInstanceToTenant(ctx context.Context, obj client.Object) []reconcile.Request {
	return mapSimulatorInstanceToTenant(ctx, obj)
}

func (r *PerformanceCollectorReconciler) mapPerformanceToTenant(_ context.Context, obj client.Object) []reconcile.Request {
	performance, ok := obj.(*platformv1.TenantPerformance)
	if !ok || performance.Spec.TenantRef.Name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: performance.Spec.TenantRef.Name}}}
}

func (r *PerformanceCollectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := registerFieldIndexes(
		context.Background(),
		mgr,
		"performance collector",
		simulatorInstanceFieldIndex(r.performanceTenantIndex(), func(instance *platformv1.SimulatorInstance) string {
			return instance.Spec.TenantRef.Name
		}),
	); err != nil {
		return err
	}

	// 实例的性能数据、状态、副本数等任何一个变化都要重新聚合
	performanceChanged := lifecyclePredicate(func(e event.UpdateEvent) bool {
		oldInstance, oldOK := e.ObjectOld.(*platformv1.SimulatorInstance)
		newInstance, newOK := e.ObjectNew.(*platformv1.SimulatorInstance)
		if !oldOK || !newOK {
			return false
		}
		var oldTTFT, oldQueue, newTTFT, newQueue *platformv1.InstancePerformanceMetric
		if oldInstance.Status.Performance != nil {
			oldTTFT = oldInstance.Status.Performance.TTFT
			oldQueue = oldInstance.Status.Performance.Queue
		}
		if newInstance.Status.Performance != nil {
			newTTFT = newInstance.Status.Performance.TTFT
			newQueue = newInstance.Status.Performance.Queue
		}
		return !optionalEqual(oldTTFT, newTTFT) ||
			!optionalEqual(oldQueue, newQueue) ||
			!optionalEqual(oldInstance.Status.ObservedAt, newInstance.Status.ObservedAt) ||
			oldInstance.Status.Phase != newInstance.Status.Phase ||
			oldInstance.Status.AvailableReplicas != newInstance.Status.AvailableReplicas ||
			oldInstance.Spec.Replicas != newInstance.Spec.Replicas ||
			oldInstance.Spec.TenantRef.Name != newInstance.Spec.TenantRef.Name ||
			!oldInstance.DeletionTimestamp.Equal(newInstance.DeletionTimestamp)
	})

	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		Named("performancecollector").
		For(&platformv1.Tenant{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&platformv1.SimulatorInstance{},
			handler.EnqueueRequestsFromMapFunc(r.mapInstanceToTenant),
			builder.WithPredicates(performanceChanged),
		)
	watchGenerationChanges(controllerBuilder, &platformv1.TenantPerformance{}, r.mapPerformanceToTenant)
	return controllerBuilder.Complete(r)
}
