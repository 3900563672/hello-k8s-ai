package controller

import (
	"context"
	"errors"
	"testing"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRetryOnConflict(t *testing.T) {
	calls := 0
	conflict := k8serrors.NewConflict(
		schema.GroupResource{Group: "platform.study.com", Resource: "simulatorinstances"},
		"instance-a",
		errors.New("conflict"),
	)
	// 第一次返回冲突，第二次成功，期望最终调用两次且无错
	err := retryOnConflict(func() error {
		calls++
		if calls == 1 {
			return conflict
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("retryOnConflict() error = %v, calls = %d; want nil, 2", err, calls)
	}
}

func TestFinalizerHelpersAreIdempotent(t *testing.T) {
	const finalizer = "test.platform.study.com/finalizer"
	scheme := newControllerTestScheme(t)
	instance := &platformv1.SimulatorInstance{ObjectMeta: metav1.ObjectMeta{Name: "instance-a"}}
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()
	ctx := context.Background()

	// 添加两次，应该幂等
	if err := addFinalizer(ctx, kubernetesClient, instance, finalizer); err != nil {
		t.Fatalf("add finalizer: %v", err)
	}
	if err := addFinalizer(ctx, kubernetesClient, instance, finalizer); err != nil {
		t.Fatalf("add finalizer again: %v", err)
	}
	if err := kubernetesClient.Get(ctx, client.ObjectKey{Name: instance.Name}, instance); err != nil {
		t.Fatal(err)
	}
	if len(instance.Finalizers) != 1 || instance.Finalizers[0] != finalizer {
		t.Fatalf("finalizers = %v, want [%s]", instance.Finalizers, finalizer)
	}

	// 移除两次，同样幂等
	if err := removeFinalizer(ctx, kubernetesClient, instance, finalizer); err != nil {
		t.Fatalf("remove finalizer: %v", err)
	}
	if err := removeFinalizer(ctx, kubernetesClient, instance, finalizer); err != nil {
		t.Fatalf("remove absent finalizer: %v", err)
	}
}
