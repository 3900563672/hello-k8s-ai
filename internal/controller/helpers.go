package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	"github.com/3900563672/hello-k8s-ai/internal/k8sutil"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const immediateRequeueAfter = time.Millisecond

type fieldIndex struct {
	object    client.Object
	field     string
	extractor client.IndexerFunc
}

// simulatorInstanceFieldIndex 建一个 SimulatorInstance 的字段索引。
// extract 从实例上取值，空字符串会被过滤掉不建索引。
func simulatorInstanceFieldIndex(
	field string,
	extract func(*platformv1.SimulatorInstance) string,
) fieldIndex {
	return fieldIndex{
		object: &platformv1.SimulatorInstance{},
		field:  field,
		extractor: func(obj client.Object) []string {
			return nonEmptyIndexValue(extract(obj.(*platformv1.SimulatorInstance)))
		},
	}
}

// registerFieldIndexes 批量注册字段索引，owner 只是用来在报错时标记是谁注册的。
func registerFieldIndexes(ctx context.Context, mgr ctrl.Manager, owner string, indexes ...fieldIndex) error {
	for _, index := range indexes {
		if err := mgr.GetFieldIndexer().IndexField(ctx, index.object, index.field, index.extractor); err != nil {
			return fmt.Errorf("register %s index %q: %w", owner, index.field, err)
		}
	}
	return nil
}

// registerSimulatorInstanceIndexes 一次性注册按租户和按模型的两个索引。
func registerSimulatorInstanceIndexes(
	ctx context.Context,
	mgr ctrl.Manager,
	owner string,
	tenantField string,
	modelField string,
) error {
	return registerFieldIndexes(
		ctx,
		mgr,
		owner,
		simulatorInstanceFieldIndex(tenantField, func(instance *platformv1.SimulatorInstance) string {
			return instance.Spec.TenantRef.Name
		}),
		simulatorInstanceFieldIndex(modelField, func(instance *platformv1.SimulatorInstance) string {
			return instance.Spec.ModelRef.Name
		}),
	)
}

// lifecyclePredicate Create 和 Delete 永远触发，Update 由调用方决定，Generic 忽略。
func lifecyclePredicate(update func(event.UpdateEvent) bool) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc:  update,
	}
}

// watchGenerationChanges 只在 spec 变化（generation 变了）时才触发 reconcile。
func watchGenerationChanges(
	controllerBuilder *builder.Builder,
	object client.Object,
	mapper handler.MapFunc,
) {
	controllerBuilder.Watches(
		object,
		handler.EnqueueRequestsFromMapFunc(mapper),
		builder.WithPredicates(predicate.GenerationChangedPredicate{}),
	)
}

// mapSimulatorInstanceToTenant 从实例对象上取出租户名，映射成租户的 reconcile 请求。
// 类型不匹配或租户名为空时不生成请求。
func mapSimulatorInstanceToTenant(_ context.Context, obj client.Object) []reconcile.Request {
	instance, ok := obj.(*platformv1.SimulatorInstance)
	if !ok || instance.Spec.TenantRef.Name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: instance.Spec.TenantRef.Name}}}
}

// removeLegacyInstanceFinalizers 清理租户下所有还挂着指定 finalizer 的实例。
// 只处理已经标记删除的实例，跳过还在用的。出错不中断，收集完一起返回。
func removeLegacyInstanceFinalizers(
	ctx context.Context,
	c client.Client,
	tenantIndex string,
	tenantName string,
	finalizer string,
) error {
	if tenantName == "" {
		return nil
	}

	// 用索引查出该租户下所有实例
	var instances platformv1.SimulatorInstanceList
	if err := c.List(ctx, &instances, client.MatchingFields{tenantIndex: tenantName}); err != nil {
		return err
	}

	cleanupErrors := make([]error, 0)
	for i := range instances.Items {
		instance := &instances.Items[i]
		// 只处理已进入删除流程且仍包含目标 finalizer 的实例。
		if instance.DeletionTimestamp.IsZero() || !controllerutil.ContainsFinalizer(instance, finalizer) {
			continue
		}
		if err := removeFinalizer(ctx, c, instance, finalizer); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}
	// 汇总全部错误后返回。
	return errors.Join(cleanupErrors...)
}

// optionalEqual 比较两个指针，两个都是 nil 算相等，只有一个 nil 算不等。
func optionalEqual[T comparable](left, right *T) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

// valueOrDefault 零值保护，value 是零值就退回 fallback。
func valueOrDefault[T comparable](value, fallback T) T {
	var zero T
	if value == zero {
		return fallback
	}
	return value
}

// addFinalizer 给对象加 finalizer，冲突自动重试。
// 每次重试都重新 Get 一遍，保证拿到的 ResourceVersion 是最新的。
func addFinalizer(ctx context.Context, c client.Client, obj client.Object, finalizer string) error {
	key := client.ObjectKeyFromObject(obj)
	return k8sutil.RetryOnConflict(func() error {
		// 每次重试都读取最新版本。
		if err := c.Get(ctx, key, obj); err != nil {
			return fmt.Errorf("get %T %s before adding finalizer: %w", obj, key, err)
		}
		// 已经有了就不重复加
		if controllerutil.ContainsFinalizer(obj, finalizer) {
			return nil
		}

		controllerutil.AddFinalizer(obj, finalizer)
		if err := c.Update(ctx, obj); err != nil {
			return fmt.Errorf("add finalizer %q to %T %s: %w", finalizer, obj, key, err)
		}
		return nil
	})
}

// removeFinalizer 移除对象的 finalizer，冲突自动重试。
// 对象已经不存在也视为成功，因为最终状态达到了。
func removeFinalizer(ctx context.Context, c client.Client, obj client.Object, finalizer string) error {
	key := client.ObjectKeyFromObject(obj)
	return k8sutil.RetryOnConflict(func() error {
		if err := c.Get(ctx, key, obj); err != nil {
			// 对象不存在时视为 finalizer 已移除。
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("get %T %s before removing finalizer: %w", obj, key, err)
		}
		// finalizer 已移除时无需写入。
		if !controllerutil.ContainsFinalizer(obj, finalizer) {
			return nil
		}

		controllerutil.RemoveFinalizer(obj, finalizer)
		if err := c.Update(ctx, obj); err != nil {
			return fmt.Errorf("remove finalizer %q from %T %s: %w", finalizer, obj, key, err)
		}
		return nil
	})
}
