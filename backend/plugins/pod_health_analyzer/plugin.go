// Package pod_health_analyzer reports pod lifecycle and probe health issues.
package pod_health_analyzer

import (
	"fmt"
	"strings"

	"kubelens-backend/internal/intelligence"
	"kubelens-backend/internal/state"
	"kubelens-backend/plugins"
)

type Plugin struct{}

// New returns a pod health analyzer plugin instance.
func New() Plugin { return Plugin{} }

// Name returns the stable plugin identifier.
func (Plugin) Name() string { return "pod_health_analyzer" }

// Analyze emits diagnostics for probe failures, evictions, and stuck termination.
func (Plugin) Analyze(snapshot state.ClusterState) []intelligence.Diagnostic {
	diagnostics := make([]intelligence.Diagnostic, 0)

	for _, pod := range snapshot.Pods {
		events := plugins.PodEvents(snapshot, pod.Namespace, pod.Name)

		if diag, ok := probeFailureDiagnostic(pod, events); ok {
			diagnostics = append(diagnostics, diag)
		}
		if diag, ok := evictedPodDiagnostic(pod); ok {
			diagnostics = append(diagnostics, diag)
		}
		if diag, ok := terminatingPodDiagnostic(pod); ok {
			diagnostics = append(diagnostics, diag)
		}
	}

	return diagnostics
}

func probeFailureDiagnostic(pod state.PodInfo, events []state.EventInfo) (intelligence.Diagnostic, bool) {
	evidence := make([]string, 0, 4)
	for _, container := range pod.Containers {
		if strings.EqualFold(container.WaitingReason, "RunContainerError") {
			evidence = append(evidence, fmt.Sprintf("Container %s waiting with RunContainerError.", container.Name))
		}
		if strings.Contains(strings.ToLower(container.TerminatedReason), "probe") {
			evidence = append(evidence, fmt.Sprintf("Container %s terminated due to %s.", container.Name, container.TerminatedReason))
		}
	}
	for _, event := range events {
		if strings.Contains(strings.ToLower(event.Message), "probe") || strings.Contains(strings.ToLower(event.Reason), "probe") {
			evidence = append(evidence, fmt.Sprintf("Event %s: %s", event.Reason, strings.TrimSpace(event.Message)))
		}
	}
	if len(evidence) == 0 {
		return intelligence.Diagnostic{}, false
	}

	return intelligence.Diagnostic{
		Severity:       intelligence.SeverityWarning,
		Resource:       pod.Name,
		Namespace:      pod.Namespace,
		Message:        "Pod failing health probes",
		Evidence:       dedupeEvidence(evidence),
		Recommendation: "Inspect liveness/readiness probe configuration, startup timing, and container dependencies before restarting.",
	}, true
}

func evictedPodDiagnostic(pod state.PodInfo) (intelligence.Diagnostic, bool) {
	if !strings.EqualFold(pod.Phase, "Failed") || !strings.EqualFold(pod.StatusReason, "Evicted") {
		return intelligence.Diagnostic{}, false
	}

	evidence := []string{"Pod phase is Failed with reason Evicted."}
	if strings.TrimSpace(pod.StatusMessage) != "" {
		evidence = append(evidence, strings.TrimSpace(pod.StatusMessage))
	}
	lowerMessage := strings.ToLower(pod.StatusMessage)
	switch {
	case strings.Contains(lowerMessage, "diskpressure"):
		evidence = append(evidence, "Eviction indicates disk pressure on the hosting node.")
	case strings.Contains(lowerMessage, "memorypressure"):
		evidence = append(evidence, "Eviction indicates memory pressure on the hosting node.")
	case strings.Contains(lowerMessage, "pressure"):
		evidence = append(evidence, "Eviction indicates node pressure impacted the workload.")
	}

	return intelligence.Diagnostic{
		Severity:       intelligence.SeverityCritical,
		Resource:       pod.Name,
		Namespace:      pod.Namespace,
		Message:        "Pod was evicted from its node",
		Evidence:       evidence,
		Recommendation: "Relieve node disk/memory pressure, reschedule the workload, and tighten resource requests to reduce eviction risk.",
	}, true
}

func terminatingPodDiagnostic(pod state.PodInfo) (intelligence.Diagnostic, bool) {
	if pod.DeletionTimestamp == nil || strings.EqualFold(pod.Phase, "Succeeded") {
		return intelligence.Diagnostic{}, false
	}

	evidence := []string{
		fmt.Sprintf("Deletion timestamp set at %s.", pod.DeletionTimestamp.UTC().Format("2006-01-02T15:04:05Z")),
		fmt.Sprintf("Pod phase remains %s.", pod.Phase),
	}
	if strings.TrimSpace(pod.StatusMessage) != "" {
		evidence = append(evidence, strings.TrimSpace(pod.StatusMessage))
	}

	return intelligence.Diagnostic{
		Severity:       intelligence.SeverityWarning,
		Resource:       pod.Name,
		Namespace:      pod.Namespace,
		Message:        "Pod stuck terminating",
		Evidence:       evidence,
		Recommendation: "Check finalizers, attached volumes, and kubelet/node health if the pod does not finish terminating promptly.",
	}, true
}

func dedupeEvidence(items []string) []string {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		clean := strings.TrimSpace(item)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}
