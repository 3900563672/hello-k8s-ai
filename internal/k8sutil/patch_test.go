package k8sutil

import (
	"errors"
	"testing"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestRetryOnConflict(t *testing.T) {
	calls := 0
	conflict := k8serrors.NewConflict(
		schema.GroupResource{Group: "platform.study.com", Resource: "simulatorinstances"},
		"instance-a",
		errors.New("conflict"),
	)
	// 第一次返回冲突，第二次成功，期望最终调用两次且无错
	err := RetryOnConflict(func() error {
		calls++
		if calls == 1 {
			return conflict
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("RetryOnConflict() error = %v, calls = %d; want nil, 2", err, calls)
	}
}
