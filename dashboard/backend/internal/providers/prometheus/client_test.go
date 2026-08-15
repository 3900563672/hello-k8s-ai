package prometheus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestParseSeriesDropsInvalidSamples(t *testing.T) {
	raw := json.RawMessage(`[
		{"metric":{"tenant":"tenant-a"},"values":[[1723464000.25,"12.5"],[1723464005,"NaN"],[1723464010,"13"]]}
	]`)
	series, err := parseSeries("matrix", raw)
	if err != nil {
		t.Fatalf("parseSeries returned an error: %v", err)
	}
	if len(series) != 1 || len(series[0].Points) != 2 {
		t.Fatalf("unexpected parsed series: %#v", series)
	}
	if series[0].Labels["tenant"] != "tenant-a" || series[0].Points[0].Value != 12.5 {
		t.Fatalf("series labels or value were not preserved: %#v", series[0])
	}
}

func TestQueryRangeClassifiesInvalidMetric(t *testing.T) {
	client := &Client{enabled: true, maxWindow: time.Hour}
	_, err := client.QueryRange(context.Background(), Query{MetricID: "arbitrary.promql"})
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("unknown metric error = %v, want ErrInvalidQuery", err)
	}
}

func TestSelectorIsStableAndEscapesValues(t *testing.T) {
	result := selector("metric_name", map[string]string{
		"tenant": "tenant-a", "model": "model\"quoted", "node": "",
	})
	want := `metric_name{model="model\"quoted",tenant="tenant-a"}`
	if result != want {
		t.Fatalf("selector = %q, want %q", result, want)
	}
}

func TestSimulatorQPSDeduplicatesReplicaTargets(t *testing.T) {
	definition := metricCatalog()["simulator.qps"]
	result := definition.build(map[string]string{
		"tenant": "tenant-a",
		"model":  "model-a",
	})
	want := `max by (tenant, model, simulator_instance) (hello_k8s_ai_simulator_assigned_qps{model="model-a",tenant="tenant-a"})`
	if result != want {
		t.Fatalf("simulator.qps query = %q, want %q", result, want)
	}
}

func TestSimulatorTimeScaleUsesCurrentReporterGauge(t *testing.T) {
	definition := metricCatalog()["simulator.timeScale"]
	result := definition.build(map[string]string{
		"tenant":             "tenant-a",
		"simulator_instance": "instance-a",
	})
	want := `max by (tenant, model, simulator_instance) (hello_k8s_ai_simulator_time_scale{simulator_instance="instance-a",tenant="tenant-a"})`
	if result != want {
		t.Fatalf("simulator.timeScale query = %q, want %q", result, want)
	}
}

func TestWorkerMetricIgnoresUnsupportedEntityFilters(t *testing.T) {
	definition := metricCatalog()["worker.gpuUsed"]
	result := definition.build(map[string]string{
		"tenant": "tenant-a",
		"node":   "node-a",
	})
	want := `hello_k8s_ai_worker_node_gpu_units_used{node="node-a"}`
	if result != want {
		t.Fatalf("worker.gpuUsed query = %q, want %q", result, want)
	}
}
