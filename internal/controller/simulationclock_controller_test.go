package controller

import (
	"context"
	"testing"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSimulationClockReconcilerCreatesDefaultClock(t *testing.T) {
	scheme := newControllerTestScheme(t)
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&platformv1.SimulationClock{}).
		Build()
	reconciler := &SimulationClockReconciler{Client: kubernetesClient, Scheme: scheme}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: platformv1.DefaultSimulationClockName},
	})
	if err != nil {
		t.Fatalf("reconcile default SimulationClock: %v", err)
	}
	var got platformv1.SimulationClock
	if err := kubernetesClient.Get(
		context.Background(),
		client.ObjectKey{Name: platformv1.DefaultSimulationClockName},
		&got,
	); err != nil {
		t.Fatalf("get default SimulationClock: %v", err)
	}
	if got.Spec.Rate != platformv1.DefaultSimulationRate {
		t.Fatalf("default rate = %d, want %d", got.Spec.Rate, platformv1.DefaultSimulationRate)
	}
}

func TestSimulationClockReconcilerSynchronizesOnlyOwnedInstanceField(t *testing.T) {
	scheme := newControllerTestScheme(t)
	simulationClock := &platformv1.SimulationClock{
		ObjectMeta: metav1.ObjectMeta{Name: platformv1.DefaultSimulationClockName},
		Spec:       platformv1.SimulationClockSpec{Rate: 5},
	}
	effectiveScore := 77
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "instance-a",
			Annotations: map[string]string{"example": "preserve"},
		},
		Spec: platformv1.SimulatorInstanceSpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
			ModelRef:  platformv1.ObjectRef{Name: "model-a"},
			Replicas:  3,
			Traffic:   platformv1.TrafficSpec{QPS: 19},
			TimeScale: 1,
		},
		Status: platformv1.SimulatorInstanceStatus{
			EffectiveScore:    &effectiveScore,
			AvailableReplicas: 2,
			Phase:             "Running",
		},
	}
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&platformv1.SimulationClock{}, &platformv1.SimulatorInstance{}).
		WithObjects(simulationClock, instance).
		Build()
	reconciler := &SimulationClockReconciler{Client: kubernetesClient, Scheme: scheme}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: platformv1.DefaultSimulationClockName},
	})
	if err != nil {
		t.Fatalf("synchronize SimulationClock: %v", err)
	}

	var gotInstance platformv1.SimulatorInstance
	if err := kubernetesClient.Get(
		context.Background(),
		client.ObjectKey{Name: instance.Name},
		&gotInstance,
	); err != nil {
		t.Fatalf("get synchronized instance: %v", err)
	}
	if gotInstance.Spec.TimeScale != 5 {
		t.Fatalf("timeScale = %d, want 5", gotInstance.Spec.TimeScale)
	}
	if gotInstance.Spec.Replicas != 3 || gotInstance.Spec.Traffic.QPS != 19 {
		t.Fatalf("another Controller field was overwritten: %#v", gotInstance.Spec)
	}
	if gotInstance.Annotations["example"] != "preserve" ||
		gotInstance.Status.EffectiveScore == nil ||
		*gotInstance.Status.EffectiveScore != effectiveScore ||
		gotInstance.Status.AvailableReplicas != 2 {
		t.Fatalf("metadata or status was overwritten: %#v", gotInstance)
	}

	var gotClock platformv1.SimulationClock
	if err := kubernetesClient.Get(
		context.Background(),
		client.ObjectKey{Name: simulationClock.Name},
		&gotClock,
	); err != nil {
		t.Fatalf("get reconciled clock: %v", err)
	}
	ready := meta.FindStatusCondition(gotClock.Status.Conditions, conditionTypeReady)
	if gotClock.Status.AppliedRate != 5 ||
		gotClock.Status.SynchronizedInstances != 1 ||
		gotClock.Status.TotalInstances != 1 ||
		ready == nil || ready.Status != metav1.ConditionTrue {
		t.Fatalf("unexpected clock status: %#v", gotClock.Status)
	}
}

func TestSimulationClockReconcilerReportsInvalidRateWithoutChangingInstances(t *testing.T) {
	scheme := newControllerTestScheme(t)
	simulationClock := &platformv1.SimulationClock{
		ObjectMeta: metav1.ObjectMeta{Name: platformv1.DefaultSimulationClockName},
		Spec:       platformv1.SimulationClockSpec{Rate: platformv1.MaxSimulationRate + 1},
	}
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-a"},
		Spec:       platformv1.SimulatorInstanceSpec{TimeScale: 3},
	}
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&platformv1.SimulationClock{}).
		WithObjects(simulationClock, instance).
		Build()
	reconciler := &SimulationClockReconciler{Client: kubernetesClient, Scheme: scheme}

	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: platformv1.DefaultSimulationClockName},
	})
	if err != nil {
		t.Fatalf("invalid rate should be reported through status: %v", err)
	}
	var gotInstance platformv1.SimulatorInstance
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: instance.Name}, &gotInstance); err != nil {
		t.Fatal(err)
	}
	if gotInstance.Spec.TimeScale != 3 {
		t.Fatalf("invalid rate changed instance to %d", gotInstance.Spec.TimeScale)
	}
	var gotClock platformv1.SimulationClock
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: simulationClock.Name}, &gotClock); err != nil {
		t.Fatal(err)
	}
	ready := meta.FindStatusCondition(gotClock.Status.Conditions, conditionTypeReady)
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason != "InvalidRate" {
		t.Fatalf("invalid-rate condition = %#v", ready)
	}
}
