package diagnostics

import (
	"strings"
	"testing"
	"time"

	"kubelens-backend/internal/intelligence"
	"kubelens-backend/internal/state"
	"kubelens-backend/plugins/crashloop_analyzer"
	"kubelens-backend/plugins/node_health_analyzer"
	"kubelens-backend/plugins/pod_health_analyzer"
	"kubelens-backend/plugins/resource_analyzer"
	"kubelens-backend/plugins/scheduling_analyzer"
)

func TestPodHealthAnalyzerDetectsProbeFailures(t *testing.T) {
	snapshot := state.ClusterState{
		Pods: map[string]state.PodInfo{
			"prod/api": {
				Name:      "api",
				Namespace: "prod",
				Phase:     "Running",
				Containers: []state.ContainerInfo{
					{Name: "api", WaitingReason: "RunContainerError"},
				},
			},
		},
		Events: []state.EventInfo{
			{
				Type:               "Warning",
				Reason:             "Unhealthy",
				Message:            "Liveness probe failed: Get http://10.0.0.10:8080/healthz",
				Namespace:          "prod",
				InvolvedObjectKind: "Pod",
				InvolvedObjectName: "api",
			},
		},
	}

	diags := pod_health_analyzer.New().Analyze(snapshot)
	diag := requireDiagnostic(t, diags, "Pod failing health probes", "prod", "api")
	if diag.Severity != intelligence.SeverityWarning {
		t.Fatalf("severity = %s, want %s", diag.Severity, intelligence.SeverityWarning)
	}
	assertEvidenceContains(t, diag.Evidence, "RunContainerError")
	assertEvidenceContains(t, diag.Evidence, "probe failed")
}

func TestPodHealthAnalyzerDetectsEvictedPods(t *testing.T) {
	snapshot := state.ClusterState{
		Pods: map[string]state.PodInfo{
			"prod/api": {
				Name:          "api",
				Namespace:     "prod",
				Phase:         "Failed",
				StatusReason:  "Evicted",
				StatusMessage: "The node had condition: [DiskPressure].",
			},
		},
	}

	diags := pod_health_analyzer.New().Analyze(snapshot)
	diag := requireDiagnostic(t, diags, "Pod was evicted from its node", "prod", "api")
	if diag.Severity != intelligence.SeverityCritical {
		t.Fatalf("severity = %s, want %s", diag.Severity, intelligence.SeverityCritical)
	}
	assertEvidenceContains(t, diag.Evidence, "DiskPressure")
}

func TestSchedulingAnalyzerSurfacesUnschedulableEvidence(t *testing.T) {
	snapshot := state.ClusterState{
		Pods: map[string]state.PodInfo{
			"prod/api": {
				Name:      "api",
				Namespace: "prod",
				Phase:     "Pending",
			},
		},
		Events: []state.EventInfo{
			{
				Type:               "Warning",
				Reason:             "FailedScheduling",
				Message:            "0/3 nodes are available: 3 Insufficient memory.",
				Namespace:          "prod",
				InvolvedObjectKind: "Pod",
				InvolvedObjectName: "api",
			},
		},
	}

	diags := scheduling_analyzer.New().Analyze(snapshot)
	diag := requireDiagnostic(t, diags, "Pod pending due to unschedulable placement", "prod", "api")
	assertEvidenceContains(t, diag.Evidence, "Insufficient memory")
}

func TestResourceAnalyzerFlagsRunningPodsWithoutRequests(t *testing.T) {
	snapshot := state.ClusterState{
		Pods: map[string]state.PodInfo{
			"prod/api": {
				Name:      "api",
				Namespace: "prod",
				Phase:     "Running",
			},
		},
	}

	diags := resource_analyzer.New().Analyze(snapshot)
	diag := requireDiagnostic(t, diags, "Running pod has no resource requests", "prod", "api")
	if diag.Severity != intelligence.SeverityWarning {
		t.Fatalf("severity = %s, want %s", diag.Severity, intelligence.SeverityWarning)
	}
	assertEvidenceContains(t, diag.Evidence, "CPU and memory requests are not set")
}

func TestPodHealthAnalyzerDetectsTerminatingPods(t *testing.T) {
	deletionTimestamp := time.Date(2026, time.April, 19, 20, 0, 0, 0, time.UTC)
	snapshot := state.ClusterState{
		Pods: map[string]state.PodInfo{
			"prod/api": {
				Name:              "api",
				Namespace:         "prod",
				Phase:             "Running",
				DeletionTimestamp: &deletionTimestamp,
			},
		},
	}

	diags := pod_health_analyzer.New().Analyze(snapshot)
	diag := requireDiagnostic(t, diags, "Pod stuck terminating", "prod", "api")
	assertEvidenceContains(t, diag.Evidence, deletionTimestamp.Format("2006-01-02T15:04:05Z"))
}

func TestCrashloopAnalyzerDetectsHighRestartVelocity(t *testing.T) {
	snapshot := state.ClusterState{
		Pods: map[string]state.PodInfo{
			"prod/api": {
				Name:           "api",
				Namespace:      "prod",
				Phase:          "Running",
				Restarts:       1,
				RecentRestarts: 3,
			},
		},
	}

	diags := crashloop_analyzer.New().Analyze(snapshot)
	diag := requireDiagnostic(t, diags, "Pod restart velocity is high", "prod", "api")
	assertEvidenceContains(t, diag.Evidence, "last 15 minutes")
}

func TestNodeHealthAnalyzerDetectsPressureSignals(t *testing.T) {
	testCases := []struct {
		name    string
		node    state.NodeInfo
		message string
	}{
		{
			name: "memory pressure",
			node: state.NodeInfo{
				Name: "node-1",
				Conditions: []state.ConditionInfo{
					{Type: "MemoryPressure", Status: "True"},
				},
			},
			message: "Node reporting MemoryPressure",
		},
		{
			name: "disk pressure",
			node: state.NodeInfo{
				Name: "node-2",
				Conditions: []state.ConditionInfo{
					{Type: "DiskPressure", Status: "True"},
				},
			},
			message: "Node reporting DiskPressure",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := state.ClusterState{
				Nodes: map[string]state.NodeInfo{
					tc.node.Name: tc.node,
				},
			}

			diags := node_health_analyzer.New().Analyze(snapshot)
			requireDiagnostic(t, diags, tc.message, "", tc.node.Name)
		})
	}
}

func TestNodeHealthAnalyzerDetectsHighCPUUtilization(t *testing.T) {
	snapshot := state.ClusterState{
		Nodes: map[string]state.NodeInfo{
			"node-1": {
				Name:        "node-1",
				Allocatable: state.ResourceQuantities{CPUMilli: 1000},
				Usage:       state.ResourceQuantities{CPUMilli: 900},
			},
		},
	}

	diags := node_health_analyzer.New().Analyze(snapshot)
	diag := requireDiagnostic(t, diags, "Node CPU utilization is high", "", "node-1")
	assertEvidenceContains(t, diag.Evidence, "90%")
}

func TestNodeHealthAnalyzerDetectsIdleCordonedNodes(t *testing.T) {
	snapshot := state.ClusterState{
		Nodes: map[string]state.NodeInfo{
			"node-1": {
				Name:          "node-1",
				Unschedulable: true,
			},
		},
	}

	diags := node_health_analyzer.New().Analyze(snapshot)
	requireDiagnostic(t, diags, "Cordoned node has no active workload pods", "", "node-1")
}

func requireDiagnostic(t *testing.T, diags []intelligence.Diagnostic, message, namespace, resource string) intelligence.Diagnostic {
	t.Helper()

	for _, diag := range diags {
		if diag.Message != message {
			continue
		}
		if diag.Namespace != namespace {
			continue
		}
		if diag.Resource != resource {
			continue
		}
		return diag
	}

	t.Fatalf("diagnostic %q for %s/%s not found in %+v", message, namespace, resource, diags)
	return intelligence.Diagnostic{}
}

func assertEvidenceContains(t *testing.T, evidence []string, token string) {
	t.Helper()

	for _, item := range evidence {
		if strings.Contains(item, token) {
			return
		}
	}

	t.Fatalf("expected evidence containing %q in %+v", token, evidence)
}
