// Package crashloop_analyzer flags pod restart loops and OOM crashes.
package crashloop_analyzer

import (
	"fmt"

	"kubelens-backend/internal/intelligence"
	"kubelens-backend/internal/intelligence/rules"
	"kubelens-backend/internal/state"
)

type Plugin struct{}

// New returns a crash-loop analyzer plugin instance.
func New() Plugin { return Plugin{} }

// Name returns the stable plugin identifier.
func (Plugin) Name() string { return "crashloop_analyzer" }

// Analyze emits diagnostics for OOMKilled and crash-looping pods.
func (Plugin) Analyze(snapshot state.ClusterState) []intelligence.Diagnostic {
	diagnostics := make([]intelligence.Diagnostic, 0)

	for _, pod := range snapshot.Pods {
		if rules.IsOOMKilled(pod) {
			diagnostics = append(diagnostics, intelligence.Diagnostic{
				Severity:       intelligence.SeverityCritical,
				Resource:       pod.Name,
				Namespace:      pod.Namespace,
				Message:        "Pod restarting due to memory limit exceeded",
				Evidence:       buildPodEvidence(pod, "Termination reason: OOMKilled"),
				Recommendation: "Increase memory limits or investigate memory leaks and retry behavior.",
			})
			continue
		}

		if pod.RecentRestarts >= 3 {
			diagnostics = append(diagnostics, intelligence.Diagnostic{
				Severity:       intelligence.SeverityWarning,
				Resource:       pod.Name,
				Namespace:      pod.Namespace,
				Message:        "Pod restart velocity is high",
				Evidence:       buildPodEvidence(pod, fmt.Sprintf("%d restart-related warning events observed in the last 15 minutes.", pod.RecentRestarts)),
				Recommendation: "Inspect recent events, probes, and dependency outages to stop repeated restarts before the workload degrades further.",
			})
			continue
		}

		if rules.IsCrashLoop(pod) {
			diagnostics = append(diagnostics, intelligence.Diagnostic{
				Severity:       intelligence.SeverityWarning,
				Resource:       pod.Name,
				Namespace:      pod.Namespace,
				Message:        "Pod is in a crash loop",
				Evidence:       buildPodEvidence(pod, "Container is restarting repeatedly"),
				Recommendation: "Inspect container logs, probes, and dependency readiness; verify secrets and config.",
			})
		}
	}

	return diagnostics
}

// buildPodEvidence composes human-readable evidence fields for crash diagnostics.
func buildPodEvidence(pod state.PodInfo, extra string) []string {
	evidence := []string{
		fmt.Sprintf("Restart count: %d", pod.Restarts),
	}
	if extra != "" {
		evidence = append(evidence, extra)
	}
	for _, container := range pod.Containers {
		if container.WaitingReason != "" {
			evidence = append(evidence, fmt.Sprintf("Container %s waiting: %s", container.Name, container.WaitingReason))
		}
		if container.TerminatedReason != "" {
			evidence = append(evidence, fmt.Sprintf("Container %s terminated: %s", container.Name, container.TerminatedReason))
		}
	}
	return evidence
}
