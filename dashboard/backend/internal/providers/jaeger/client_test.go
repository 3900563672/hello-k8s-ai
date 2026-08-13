package jaeger

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSummarizeExtractsRootAndEntities(t *testing.T) {
	trace := legacyTrace{
		TraceID: "trace-1",
		Processes: map[string]legacyProcess{
			"p1": {ServiceName: "hello-k8s-ai-controller"},
			"p2": {ServiceName: "hello-k8s-ai-simulator"},
		},
		Spans: []legacySpan{
			{
				TraceID: "trace-1", SpanID: "child", ProcessID: "p2",
				OperationName: "simulator.tick", StartTime: 1_000_010, Duration: 2_000,
				References: []legacyReference{{RefType: "CHILD_OF", SpanID: "root"}},
				Tags: []legacyTag{
					{Key: "error", Value: true},
					{Key: "platform.simulator_instance.name", Value: "instance-a"},
				},
			},
			{
				TraceID: "trace-1", SpanID: "root", ProcessID: "p1",
				OperationName: "controller.reconcile", StartTime: 1_000_000, Duration: 5_000,
				Tags: []legacyTag{
					{Key: "platform.tenant.name", Value: "tenant-a"},
					{Key: "platform.model.name", Value: "model-a"},
				},
			},
		},
	}

	summary := summarize(trace)
	if summary.RootService != "hello-k8s-ai-controller" || summary.RootOperation != "controller.reconcile" {
		t.Fatalf("wrong root span summary: %#v", summary)
	}
	if summary.SpanCount != 2 || summary.ErrorSpanCount != 1 {
		t.Fatalf("wrong span counters: %#v", summary)
	}
	if summary.Entities["tenant"] != "tenant-a" || summary.Entities["model"] != "model-a" {
		t.Fatalf("entity attributes were not extracted: %#v", summary.Entities)
	}
	if summary.Entities["simulatorInstance"] != "instance-a" {
		t.Fatalf("child-span entity attribute was not extracted: %#v", summary.Entities)
	}
}

func TestSearchClassifiesInvalidWindow(t *testing.T) {
	client := &Client{enabled: true, maxWindow: time.Hour}
	at := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	_, err := client.Search(context.Background(), SearchRequest{Start: at, End: at})
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("invalid search error = %v, want ErrInvalidQuery", err)
	}
}

func TestDetailPreservesParentCallChain(t *testing.T) {
	trace := legacyTrace{
		TraceID: "trace-2",
		Processes: map[string]legacyProcess{
			"p1": {ServiceName: "hello-k8s-ai-controller"},
		},
		Spans: []legacySpan{
			{SpanID: "root", ProcessID: "p1", StartTime: 2_000_000, Duration: 10_000},
			{SpanID: "child", ProcessID: "p1", StartTime: 2_000_100, Duration: 5_000, References: []legacyReference{{RefType: "CHILD_OF", SpanID: "root"}}},
		},
	}
	result := detail(trace)
	if len(result.Spans) != 2 || result.Spans[1].ParentSpanID != "root" {
		t.Fatalf("parent call chain was not preserved: %#v", result.Spans)
	}
}
