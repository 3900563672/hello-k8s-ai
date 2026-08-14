package controller

import (
	"reflect"
	"testing"
)

func TestNodePlacementPlanRoundTripAndScaling(t *testing.T) {
	plan, err := newNodePlacementPlan(map[string]int{"node-b": 1, "node-a": 2})
	if err != nil {
		t.Fatalf("new placement plan: %v", err)
	}
	if plan.PrimaryNode != "node-a" {
		t.Fatalf("primary node = %q, want node-a", plan.PrimaryNode)
	}

	plan, err = addNodePlacement(plan, "node-c")
	if err != nil {
		t.Fatalf("add placement: %v", err)
	}
	if nodePlacementReplicaCount(plan) != 4 {
		t.Fatalf("replica count = %d, want 4", nodePlacementReplicaCount(plan))
	}
	if nodeName, found := scaleDownPlacementNode(plan); !found || nodeName != "node-c" {
		t.Fatalf("scale-down node = (%q, %t), want node-c", nodeName, found)
	}
	plan, err = removeNodePlacement(plan, "node-c")
	if err != nil {
		t.Fatalf("remove placement: %v", err)
	}

	payload, err := encodeNodePlacementPlan(plan)
	if err != nil {
		t.Fatalf("encode placement plan: %v", err)
	}
	decoded, persisted, err := decodeNodePlacementPlan(payload)
	if err != nil || !persisted {
		t.Fatalf("decode placement plan: persisted=%t err=%v", persisted, err)
	}
	if !reflect.DeepEqual(decoded, plan) {
		t.Fatalf("decoded plan = %#v, want %#v", decoded, plan)
	}
}

func TestNodePlacementPlanRejectsInvalidPayloads(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{name: "unknown version", payload: `{"version":2,"placements":[]}`},
		{name: "missing primary", payload: `{"version":1,"placements":[{"nodeName":"node-a","replicas":1}]}`},
		{name: "orphan primary", payload: `{"version":1,"primaryNode":"node-a","placements":[]}`},
		{name: "duplicate node", payload: `{"version":1,"primaryNode":"node-a","placements":[{"nodeName":"node-a","replicas":1},{"nodeName":"node-a","replicas":1}]}`},
		{name: "zero replicas", payload: `{"version":1,"primaryNode":"node-a","placements":[{"nodeName":"node-a","replicas":0}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := decodeNodePlacementPlan(test.payload); err == nil {
				t.Fatalf("decodeNodePlacementPlan(%s) succeeded, want error", test.payload)
			}
		})
	}
}
