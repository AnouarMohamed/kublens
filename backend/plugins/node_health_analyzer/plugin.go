// Package node_health_analyzer reports node readiness and pressure issues.
package node_health_analyzer

import (
	"fmt"
	"strings"

	"kubelens-backend/internal/intelligence"
	"kubelens-backend/internal/intelligence/rules"
	"kubelens-backend/internal/state"
)

type Plugin struct{}

// New returns a node health analyzer plugin instance.
func New() Plugin { return Plugin{} }

// Name returns the stable plugin identifier.
func (Plugin) Name() string { return "node_health_analyzer" }

// Analyze emits diagnostics for NotReady nodes and pressure conditions.
func (Plugin) Analyze(snapshot state.ClusterState) []intelligence.Diagnostic {
	diagnostics := make([]intelligence.Diagnostic, 0)

	for _, node := range snapshot.Nodes {
		if rules.IsNodeNotReady(node) {
			diagnostics = append(diagnostics, intelligence.Diagnostic{
				Severity:       intelligence.SeverityCritical,
				Resource:       node.Name,
				Message:        "Node is not ready",
				Evidence:       []string{"Node Ready condition is false."},
				Recommendation: "Inspect kubelet health, connectivity, and node pressure conditions.",
			})
		}

		for _, cond := range []string{"MemoryPressure", "DiskPressure", "PIDPressure"} {
			if !rules.NodeHasPressure(node, cond) {
				continue
			}
			diagnostics = append(diagnostics, intelligence.Diagnostic{
				Severity:       intelligence.SeverityWarning,
				Resource:       node.Name,
				Message:        fmt.Sprintf("Node reporting %s", cond),
				Evidence:       []string{fmt.Sprintf("%s condition is True.", cond)},
				Recommendation: "Drain workloads or increase node capacity before scheduling additional pods.",
			})
		}

		if percent, ok := cpuUsagePercent(node); ok && percent > 85 {
			diagnostics = append(diagnostics, intelligence.Diagnostic{
				Severity:       intelligence.SeverityWarning,
				Resource:       node.Name,
				Message:        "Node CPU utilization is high",
				Evidence:       []string{fmt.Sprintf("CPU usage is %d%% of allocatable capacity (%dm/%dm).", percent, node.Usage.CPUMilli, cpuCapacity(node))},
				Recommendation: "Shift workloads, scale node capacity, or investigate noisy neighbors before saturation impacts scheduling.",
			})
		}

		if node.Unschedulable && !nodeInMaintenanceMode(node) && activePodCount(snapshot, node.Name) == 0 {
			diagnostics = append(diagnostics, intelligence.Diagnostic{
				Severity:       intelligence.SeverityWarning,
				Resource:       node.Name,
				Message:        "Cordoned node has no active workload pods",
				Evidence:       []string{"Node is marked unschedulable.", "No non-terminal pods are currently assigned to the node."},
				Recommendation: "Uncordon the node when maintenance is complete or mark it explicitly for maintenance to avoid silent capacity loss.",
			})
		}
	}

	return diagnostics
}

func cpuUsagePercent(node state.NodeInfo) (int64, bool) {
	capacity := cpuCapacity(node)
	if capacity <= 0 || node.Usage.CPUMilli <= 0 {
		return 0, false
	}
	return (node.Usage.CPUMilli * 100) / capacity, true
}

func cpuCapacity(node state.NodeInfo) int64 {
	if node.Allocatable.CPUMilli > 0 {
		return node.Allocatable.CPUMilli
	}
	return node.Capacity.CPUMilli
}

func activePodCount(snapshot state.ClusterState, nodeName string) int {
	total := 0
	for _, pod := range snapshot.Pods {
		if pod.NodeName != nodeName {
			continue
		}
		if strings.EqualFold(pod.Phase, "Succeeded") || strings.EqualFold(pod.Phase, "Failed") {
			continue
		}
		total++
	}
	return total
}

func nodeInMaintenanceMode(node state.NodeInfo) bool {
	for _, role := range node.Roles {
		if strings.Contains(strings.ToLower(strings.TrimSpace(role)), "maintenance") {
			return true
		}
	}
	for key, value := range node.Labels {
		if strings.Contains(strings.ToLower(key), "maintenance") || strings.Contains(strings.ToLower(value), "maintenance") {
			return true
		}
	}
	for _, taint := range node.Taints {
		if strings.Contains(strings.ToLower(taint), "maintenance") {
			return true
		}
	}
	return false
}
