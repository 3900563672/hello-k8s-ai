package main

import "testing"

func TestDerivedSimulatorServiceAccount(t *testing.T) {
	tests := map[string]string{
		"controller-manager":              "simulator-sa",
		"hello-k8s-ai-controller-manager": "hello-k8s-ai-simulator-sa",
		"custom-manager":                  "simulator-sa",
	}
	for input, want := range tests {
		if got := derivedSimulatorServiceAccount(input); got != want {
			t.Fatalf("derivedSimulatorServiceAccount(%q) = %q, want %q", input, got, want)
		}
	}
}
