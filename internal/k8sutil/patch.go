package k8sutil

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RetryOnConflict 使用 client-go 默认退避策略重试冲突。
func RetryOnConflict(fn func() error) error {
	return retry.RetryOnConflict(retry.DefaultRetry, fn)
}

// PatchWithRetry 以最新对象为基础做修改，冲突自动重试。
// newObj 返回目标类型的零值对象（如 &platformv1.X{}）；mutate 只允许修改业务字段；
// write 决定写入方式（Status 或普通 Patch）。没有任何实际变化时不会发起 API 请求。
func PatchWithRetry[T client.Object](
	ctx context.Context,
	c client.Client,
	name string,
	ignoreNotFound bool,
	newObj func() T,
	mutate func(obj T) error,
	write func(c client.Client, obj T, before client.Object) error,
) error {
	return RetryOnConflict(func() error {
		obj := newObj()
		if err := c.Get(ctx, client.ObjectKey{Name: name}, obj); err != nil {
			if ignoreNotFound && apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		before, ok := obj.DeepCopyObject().(client.Object)
		if !ok {
			return fmt.Errorf("object %T does not implement client.Object", obj)
		}
		if err := mutate(obj); err != nil {
			return err
		}
		// 状态没变就不发 API 了
		if equality.Semantic.DeepEqual(before, obj) {
			return nil
		}
		return write(c, obj, before)
	})
}

// PatchStatusWithRetry 是 PatchWithRetry 的状态子资源版本，只修改 Status 字段。
func PatchStatusWithRetry[T client.Object](
	ctx context.Context,
	c client.Client,
	name string,
	ignoreNotFound bool,
	newObj func() T,
	mutate func(obj T) error,
) error {
	return PatchWithRetry(ctx, c, name, ignoreNotFound, newObj, mutate,
		func(c client.Client, obj T, before client.Object) error {
			return c.Status().Patch(ctx, obj, client.MergeFrom(before))
		})
}
