package main

import (
	"context"
	"math/rand"
	"testing"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestColdStartFactorAtBoundaries(t *testing.T) {
	start := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		now  time.Time
		want float64
	}{
		{name: "before", now: start.Add(-time.Second), want: 0},
		{name: "half", now: start.Add(5 * time.Second), want: 0},
		{name: "three quarters", now: start.Add(7500 * time.Millisecond), want: 0.25},
		{name: "complete", now: start.Add(10 * time.Second), want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := coldStartFactorAt(test.now, start, 10000); got != test.want {
				t.Fatalf("factor = %v, want %v", got, test.want)
			}
		})
	}
}

func TestEngineProcessesMultipleCompletionsWithinOneStep(t *testing.T) {
	engine := newSimEngine(1, rand.New(rand.NewSource(7)))
	// 极低延迟 + 高容量，队列应该被清空并且有首 token 完成
	avgTTFT, queueLength, hasTTFT := engine.Step(
		time.Second,
		100,
		100,
		100,
		1,
		0,
		0,
		1,
	)
	if !hasTTFT {
		t.Fatal("expected completed first-token samples")
	}
	if avgTTFT < 0 || queueLength != 0 {
		t.Fatalf("avgTTFT=%d queue=%d, want a drained fast queue", avgTTFT, queueLength)
	}
}

func TestColdStartBacklogRecovers(t *testing.T) {
	engine := newSimEngine(2, rand.New(rand.NewSource(11)))
	// 冷启动因子为 0，请求应该排队但不会完成
	_, queued, completed := engine.Step(time.Second, 10, 0, 100, 1, 0, 1, 0)
	if completed || queued == 0 {
		t.Fatalf("cold step completed=%t queue=%d, want queued work and no TTFT", completed, queued)
	}

	// 随后恢复容量，积压请求应该被处理完
	_, remaining, completed := engine.StepRate(10*time.Second, 0.000001, 100, 100, 1, 0, 1, 1)
	if !completed {
		t.Fatal("backlog queued at factor zero should complete after capacity becomes available")
	}
	if remaining != 0 {
		t.Fatalf("remaining queue = %d, want 0", remaining)
	}
}

func TestPoissonLargeLambdaHasCorrectOrderOfMagnitude(t *testing.T) {
	rng := rand.New(rand.NewSource(19))
	const (
		lambda  = 1000.0
		samples = 2000
	)
	total := 0
	for range samples {
		total += poisson(rng, lambda)
	}
	mean := float64(total) / samples
	if mean < 985 || mean > 1015 {
		t.Fatalf("sample mean = %.2f, want near %.2f", mean, lambda)
	}
}

func TestPerformanceDefaults(t *testing.T) {
	got := withPerformanceDefaults(platformv1.PerformanceSpec{})
	if got.PrefillBaseMs != 50 || got.PrefillPerTokenUs != 500 || got.DecodePerTokenMs != 20 {
		t.Fatalf("defaults = %#v", got)
	}
}

func TestSimulatorStatusPatchPreservesControllerOwnedFields(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := platformv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-a"},
		Status: platformv1.SimulatorInstanceStatus{
			Phase: "Running",
			Conditions: []metav1.Condition{{
				Type: "Ready", Status: metav1.ConditionTrue, Reason: "DeploymentAvailable",
			}},
		},
	}
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&platformv1.SimulatorInstance{}).
		WithObjects(instance).
		Build()
	simulator := &Simulator{client: kubernetesClient, name: instance.Name, reporterID: "pod-a"}
	performance := &platformv1.InstancePerformance{
		Queue: &platformv1.InstancePerformanceMetric{Value: 3, Unit: "requests"},
	}
	observedAt := metav1.NewTime(time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC))

	// 模拟器写入 Score、Performance 和 ReporterID，不应该覆盖 Phase 和 Conditions
	if err := simulator.updateOwnedStatus(context.Background(), 42, performance, observedAt); err != nil {
		t.Fatalf("update owned status: %v", err)
	}

	var got platformv1.SimulatorInstance
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: instance.Name}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != "Running" || len(got.Status.Conditions) != 1 {
		t.Fatalf("controller-owned status was overwritten: %#v", got.Status)
	}
	if got.Status.Score == nil || *got.Status.Score != 42 || got.Status.Performance == nil {
		t.Fatalf("simulator-owned status was not written: %#v", got.Status)
	}
	if got.Status.ObservedAt == nil || !got.Status.ObservedAt.Equal(new(observedAt)) || got.Status.ReporterID != "pod-a" {
		t.Fatalf("reporter metadata was not written: %#v", got.Status)
	}
}

func TestSimulatorAppliesDynamicTimeScaleWithoutChangingWallClock(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := platformv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	effectiveScore := 100
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-rate"},
		Spec: platformv1.SimulatorInstanceSpec{
			ModelRef:  platformv1.ObjectRef{Name: "model-rate"},
			Traffic:   platformv1.TrafficSpec{QPS: 0},
			TimeScale: 2,
		},
		Status: platformv1.SimulatorInstanceStatus{
			EffectiveScore:   &effectiveScore,
			AvailableReplicas: 1,
		},
	}
	model := &platformv1.Model{
		ObjectMeta: metav1.ObjectMeta{Name: "model-rate"},
		Spec: platformv1.ModelSpec{
			MaxConcurrency: 1,
			ColdStartMs:    10000,
		},
	}
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&platformv1.SimulatorInstance{}).
		WithObjects(instance, model).
		Build()
	fixedWallTime := time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)
	simulator := &Simulator{
		client:     kubernetesClient,
		name:       instance.Name,
		interval:   time.Second,
		now:        func() time.Time { return fixedWallTime },
		reporterID: "pod-a",
	}

	if err := simulator.reconcile(context.Background()); err != nil {
		t.Fatalf("first reconcile at 2x: %v", err)
	}
	if simulator.simulationElapsed != 2*time.Second {
		t.Fatalf("elapsed = %s, want 2s", simulator.simulationElapsed)
	}

	var latest platformv1.SimulatorInstance
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: instance.Name}, &latest); err != nil {
		t.Fatal(err)
	}
	if latest.Status.Score == nil || *latest.Status.Score != 0 {
		t.Fatalf("score before cold start completes = %v, want 0", latest.Status.Score)
	}
	latest.Spec.TimeScale = 8
	if err := kubernetesClient.Update(context.Background(), &latest); err != nil {
		t.Fatalf("change timeScale at runtime: %v", err)
	}

	if err := simulator.reconcile(context.Background()); err != nil {
		t.Fatalf("second reconcile at 8x: %v", err)
	}
	if simulator.simulationElapsed != 10*time.Second {
		t.Fatalf("elapsed after dynamic change = %s, want 10s", simulator.simulationElapsed)
	}
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: instance.Name}, &latest); err != nil {
		t.Fatal(err)
	}
	if latest.Status.Score == nil || *latest.Status.Score != effectiveScore {
		t.Fatalf("score after simulated cold start = %v, want %d", latest.Status.Score, effectiveScore)
	}
	if latest.Status.ObservedAt == nil || !latest.Status.ObservedAt.Time.Equal(fixedWallTime) {
		t.Fatalf("observedAt = %v, want fixed wall time %s", latest.Status.ObservedAt, fixedWallTime)
	}
}

func TestTimeScaleBoundsAndSaturatedProgress(t *testing.T) {
	if got := normalizedTimeScale(0); got != platformv1.DefaultSimulationRate {
		t.Fatalf("normalized zero rate = %d", got)
	}
	if got := normalizedTimeScale(platformv1.MaxSimulationRate + 1); got != platformv1.MaxSimulationRate {
		t.Fatalf("normalized excessive rate = %d", got)
	}
	simulator := &Simulator{interval: 2 * time.Second}
	if step := simulator.advanceSimulationTime(5); step != 10*time.Second {
		t.Fatalf("step = %s, want 10s", step)
	}
	if simulator.simulationElapsed != 10*time.Second {
		t.Fatalf("elapsed = %s, want 10s", simulator.simulationElapsed)
	}
	maximumDuration := time.Duration(1<<63 - 1)
	simulator = &Simulator{interval: maximumDuration}
	if step := simulator.advanceSimulationTime(platformv1.MaxSimulationRate); step != maximumDuration {
		t.Fatalf("saturated step = %s, want maximum duration", step)
	}
	simulator.advanceSimulationTime(platformv1.MaxSimulationRate)
	if simulator.simulationElapsed != maximumDuration {
		t.Fatalf("saturated elapsed = %s, want maximum duration", simulator.simulationElapsed)
	}
}
