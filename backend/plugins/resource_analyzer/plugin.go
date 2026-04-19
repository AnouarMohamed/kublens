// Package resource_analyzer evaluates pod resource requests, limits, and usage.
package resource_analyzer

import (
	"fmt"
	"strings"

	"kubelens-backend/internal/intelligence"
	"kubelens-backend/internal/state"
)

type Plugin struct{}

// New returns a resource analyzer plugin instance.
func New() Plugin { return Plugin{} }

// Name returns the stable plugin identifier.
func (Plugin) Name() string { return "resource_analyzer" }

// Analyze emits diagnostics for missing resource controls and usage overruns.
func (Plugin) Analyze(snapshot state.ClusterState) []intelligence.Diagnostic {
	diagnostics := make([]intelligence.Diagnostic, 0)

	for _, pod := range snapshot.Pods {
		limits := pod.ResourceLimits
		requests := pod.ResourceRequests

		if strings.EqualFold(pod.Phase, "Running") && requests.CPUMilli == 0 && requests.MemoryBytes == 0 {
			evidence := []string{"CPU and memory requests are not set for this running pod."}
			if limits.CPUMilli == 0 && limits.MemoryBytes == 0 {
				evidence = append(evidence, "CPU and memory limits are also unset.")
			}
			diagnostics = append(diagnostics, intelligence.Diagnostic{
				Severity:       intelligence.SeverityWarning,
				Resource:       pod.Name,
				Namespace:      pod.Namespace,
				Message:        "Running pod has no resource requests",
				Evidence:       evidence,
				Recommendation: "Define CPU and memory requests and set matching limits to improve scheduling accuracy and protect cluster stability.",
			})
		}

		if limits.MemoryBytes == 0 {
			diagnostics = append(diagnostics, intelligence.Diagnostic{
				Severity:       intelligence.SeverityWarning,
				Resource:       pod.Name,
				Namespace:      pod.Namespace,
				Message:        "Pod missing memory limits",
				Evidence:       []string{"Memory limit is not set for one or more containers."},
				Recommendation: "Define memory limits to prevent noisy-neighbor impact and OOM storms.",
			})
		}
		if limits.CPUMilli == 0 {
			diagnostics = append(diagnostics, intelligence.Diagnostic{
				Severity:       intelligence.SeverityInfo,
				Resource:       pod.Name,
				Namespace:      pod.Namespace,
				Message:        "Pod missing CPU limits",
				Evidence:       []string{"CPU limit is not set for one or more containers."},
				Recommendation: "Define CPU limits to prevent runaway compute and set fair scheduling bounds.",
			})
		}
		if requests.MemoryBytes == 0 {
			diagnostics = append(diagnostics, intelligence.Diagnostic{
				Severity:       intelligence.SeverityInfo,
				Resource:       pod.Name,
				Namespace:      pod.Namespace,
				Message:        "Pod missing memory requests",
				Evidence:       []string{"Memory request is not set for one or more containers."},
				Recommendation: "Define memory requests to improve bin packing and scheduling accuracy.",
			})
		}

		if limits.MemoryBytes > 0 && pod.Usage.MemoryBytes > limits.MemoryBytes {
			diagnostics = append(diagnostics, intelligence.Diagnostic{
				Severity:       intelligence.SeverityCritical,
				Resource:       pod.Name,
				Namespace:      pod.Namespace,
				Message:        "Pod memory usage exceeded limits",
				Evidence:       []string{fmt.Sprintf("Usage %d bytes exceeds limit %d bytes.", pod.Usage.MemoryBytes, limits.MemoryBytes)},
				Recommendation: "Increase memory limits or investigate memory leaks before restarting.",
			})
		}
		if limits.CPUMilli > 0 && pod.Usage.CPUMilli > limits.CPUMilli {
			diagnostics = append(diagnostics, intelligence.Diagnostic{
				Severity:       intelligence.SeverityWarning,
				Resource:       pod.Name,
				Namespace:      pod.Namespace,
				Message:        "Pod CPU usage exceeded limits",
				Evidence:       []string{fmt.Sprintf("Usage %dm exceeds limit %dm.", pod.Usage.CPUMilli, limits.CPUMilli)},
				Recommendation: "Adjust CPU limits or investigate spikes and throttling behavior.",
			})
		}
	}

	return diagnostics
}
