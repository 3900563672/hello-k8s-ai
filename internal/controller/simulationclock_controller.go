package controller

import (
	"context"
	"errors"
	"fmt"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
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

// SimulationClockReconciler 把集群级倍速配置同步到全部 SimulatorInstance。
// Simulator 直接读取实例字段，因此运行中调整倍速不需要重建 Pod。
type SimulationClockReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=platform.study.com,resources=simulationclocks,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=platform.study.com,resources=simulationclocks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.study.com,resources=simulatorinstances,verbs=get;list;watch;update;patch

func (r *SimulationClockReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	if req.Name != platformv1.DefaultSimulationClockName {
		return ctrl.Result{}, nil
	}

	var simulationClock platformv1.SimulationClock
	if err := r.Get(ctx, req.NamespacedName, &simulationClock); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("get SimulationClock %q: %w", req.Name, err)
		}
		defaultClock := &platformv1.SimulationClock{
			ObjectMeta: metav1.ObjectMeta{Name: platformv1.DefaultSimulationClockName},
			Spec: platformv1.SimulationClockSpec{
				Rate: platformv1.DefaultSimulationRate,
			},
		}
		if err := r.Create(ctx, defaultClock); err != nil && !apierrors.IsAlreadyExists(err) {
			return ctrl.Result{}, fmt.Errorf("create default SimulationClock: %w", err)
		}
		return ctrl.Result{}, nil
	}

	rate := simulationClock.Spec.Rate
	if rate < platformv1.DefaultSimulationRate || rate > platformv1.MaxSimulationRate {
		message := fmt.Sprintf(
			"spec.rate 必须在 %d 到 %d 之间，当前为 %d",
			platformv1.DefaultSimulationRate,
			platformv1.MaxSimulationRate,
			rate,
		)
		return ctrl.Result{}, r.updateSimulationClockStatus(
			ctx,
			simulationClock.Name,
			simulationClock.Generation,
			0,
			0,
			0,
			metav1.ConditionFalse,
			"InvalidRate",
			message,
		)
	}

	var instances platformv1.SimulatorInstanceList
	if err := r.List(ctx, &instances); err != nil {
		return ctrl.Result{}, fmt.Errorf("list SimulatorInstances: %w", err)
	}

	total := 0
	synchronized := 0
	syncErrors := make([]error, 0)
	for i := range instances.Items {
		instance := &instances.Items[i]
		if !instance.DeletionTimestamp.IsZero() {
			continue
		}
		total++
		updated, err := r.synchronizeInstanceRate(ctx, instance.Name, rate)
		if err != nil {
			syncErrors = append(syncErrors, err)
			continue
		}
		if updated {
			synchronized++
		}
	}

	conditionStatus := metav1.ConditionTrue
	reason := "InstancesSynchronized"
	message := fmt.Sprintf("%d 个 SimulatorInstance 已同步为 %dx", synchronized, rate)
	appliedRate := rate
	if len(syncErrors) > 0 {
		conditionStatus = metav1.ConditionFalse
		reason = "SynchronizationFailed"
		message = fmt.Sprintf("%d/%d 个 SimulatorInstance 已同步为 %dx", synchronized, total, rate)
		appliedRate = simulationClock.Status.AppliedRate
	}
	if err := r.updateSimulationClockStatus(
		ctx,
		simulationClock.Name,
		simulationClock.Generation,
		appliedRate,
		synchronized,
		total,
		conditionStatus,
		reason,
		message,
	); err != nil {
		syncErrors = append(syncErrors, err)
	}
	if err := errors.Join(syncErrors...); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// synchronizeInstanceRate 只修改 spec.timeScale，避免覆盖其他 Controller 管理的字段。
func (r *SimulationClockReconciler) synchronizeInstanceRate(
	ctx context.Context,
	name string,
	rate int,
) (bool, error) {
	synchronized := false
	err := retryOnConflict(func() error {
		var instance platformv1.SimulatorInstance
		if err := r.Get(ctx, client.ObjectKey{Name: name}, &instance); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if !instance.DeletionTimestamp.IsZero() {
			return nil
		}
		if instance.Spec.TimeScale == rate {
			synchronized = true
			return nil
		}
		before := instance.DeepCopy()
		instance.Spec.TimeScale = rate
		if err := r.Patch(ctx, &instance, client.MergeFrom(before)); err != nil {
			return err
		}
		synchronized = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("synchronize SimulatorInstance %q rate: %w", name, err)
	}
	return synchronized, nil
}

func (r *SimulationClockReconciler) updateSimulationClockStatus(
	ctx context.Context,
	name string,
	observedGeneration int64,
	appliedRate int,
	synchronizedInstances int,
	totalInstances int,
	conditionStatus metav1.ConditionStatus,
	reason string,
	message string,
) error {
	return retryOnConflict(func() error {
		var latest platformv1.SimulationClock
		if err := r.Get(ctx, client.ObjectKey{Name: name}, &latest); err != nil {
			return err
		}
		before := latest.DeepCopy()
		latest.Status.ObservedGeneration = observedGeneration
		latest.Status.AppliedRate = appliedRate
		latest.Status.SynchronizedInstances = synchronizedInstances
		latest.Status.TotalInstances = totalInstances
		meta.SetStatusCondition(&latest.Status.Conditions, metav1.Condition{
			Type:               conditionTypeReady,
			Status:             conditionStatus,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: observedGeneration,
		})
		if equality.Semantic.DeepEqual(before.Status, latest.Status) {
			return nil
		}
		return r.Status().Patch(ctx, &latest, client.MergeFrom(before))
	})
}

func (r *SimulationClockReconciler) SetupWithManager(mgr ctrl.Manager) error {
	instanceRateChanged := lifecyclePredicate(func(e event.UpdateEvent) bool {
		oldInstance, oldOK := e.ObjectOld.(*platformv1.SimulatorInstance)
		newInstance, newOK := e.ObjectNew.(*platformv1.SimulatorInstance)
		return oldOK && newOK &&
			(oldInstance.Spec.TimeScale != newInstance.Spec.TimeScale ||
				!oldInstance.DeletionTimestamp.Equal(newInstance.DeletionTimestamp))
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("simulationclock").
		For(
			&platformv1.SimulationClock{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&platformv1.SimulatorInstance{},
			handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
				return []reconcile.Request{{
					NamespacedName: client.ObjectKey{Name: platformv1.DefaultSimulationClockName},
				}}
			}),
			builder.WithPredicates(instanceRateChanged),
		).
		Complete(r)
}
