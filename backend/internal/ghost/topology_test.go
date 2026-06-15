package ghost

import (
	"testing"
	"time"

	"kubelens-backend/internal/model"
	"kubelens-backend/internal/state"
)

func TestBuildTopologyFromStateMapsServiceSelectors(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	topology := BuildTopologyFromState(state.ClusterState{
		Pods: map[string]state.PodInfo{
			"default/api": {
				Name:             "api",
				Namespace:        "default",
				NodeName:         "node-a",
				Phase:            "Running",
				Labels:           map[string]string{"app": "api"},
				ResourceRequests: state.ResourceQuantities{CPUMilli: 250, MemoryBytes: 256},
			},
		},
		Nodes: map[string]state.NodeInfo{
			"node-a": {
				Name:        "node-a",
				Status:      "Ready",
				Allocatable: state.ResourceQuantities{CPUMilli: 1000, MemoryBytes: 1024},
			},
		},
		Services: map[string]state.ServiceInfo{
			"default/api": {
				Name:      "api",
				Namespace: "default",
				Type:      "ClusterIP",
				Selector:  map[string]string{"app": "api"},
			},
		},
	}, now)

	if topology.GeneratedAt == "" {
		t.Fatal("expected generated timestamp")
	}
	if len(topology.Services) != 1 || len(topology.Services[0].PodRefs) != 1 || topology.Services[0].PodRefs[0] != "default/api" {
		t.Fatalf("service pod refs = %#v", topology.Services)
	}
	if len(topology.Edges) == 0 {
		t.Fatal("expected graph edges")
	}
}

func TestSimulateNodeDrainLeavesPodsPendingWhenNoCapacity(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	topology := model.GhostTopology{
		Nodes: []model.GhostNode{
			{
				Name:     "node-a",
				Status:   "Ready",
				Headroom: model.GhostResources{CPUMilli: 0, MemoryBytes: 0},
			},
			{
				Name:     "node-b",
				Status:   "Ready",
				Headroom: model.GhostResources{CPUMilli: 10, MemoryBytes: 10},
			},
		},
		Pods: []model.GhostPod{
			{
				ID:        "default/api",
				Name:      "api",
				Namespace: "default",
				NodeName:  "node-a",
				Status:    "Running",
				Requests:  model.GhostResources{CPUMilli: 100, MemoryBytes: 100},
			},
		},
	}

	result := SimulateNodeDrain(topology, model.GhostSimulationRequest{
		Action:   ActionNodeDrain,
		NodeName: "node-a",
	}, now)

	if result.Verdict.Severity != "critical" {
		t.Fatalf("severity = %q, want critical", result.Verdict.Severity)
	}
	if len(result.Frames) != 2 {
		t.Fatalf("frames = %d, want 2", len(result.Frames))
	}
	if got := result.Frames[1].Pods[0].Status; got != "Pending" {
		t.Fatalf("pod status = %q, want Pending", got)
	}
}
