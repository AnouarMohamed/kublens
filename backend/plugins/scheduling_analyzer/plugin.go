// Package scheduling_analyzer detects pods pending because scheduling failed.
package scheduling_analyzer

import (
	"strings"

	"kubelens-backend/internal/intelligence"
	"kubelens-backend/internal/intelligence/rules"
	"kubelens-backend/internal/state"
	"kubelens-backend/plugins"
)

type Plugin struct{}

// New returns a scheduling analyzer plugin instance.
func New() Plugin { return Plugin{} }

// Name returns the stable plugin identifier.
func (Plugin) Name() string { return "scheduling_analyzer" }

// Analyze emits diagnostics for pods blocked by FailedScheduling events.
func (Plugin) Analyze(snapshot state.ClusterState) []intelligence.Diagnostic {
	diagnostics := make([]intelligence.Diagnostic, 0)

	for _, pod := range snapshot.Pods {
		if !rules.IsPending(pod) {
			continue
		}
		events := plugins.PodEvents(snapshot, pod.Namespace, pod.Name)
		evidence := unschedulableEvidence(events)
		if len(evidence) == 0 {
			continue
		}
		diagnostics = append(diagnostics, intelligence.Diagnostic{
			Severity:       intelligence.SeverityWarning,
			Resource:       pod.Name,
			Namespace:      pod.Namespace,
			Message:        "Pod pending due to unschedulable placement",
			Evidence:       evidence,
			Recommendation: "Review node capacity, resource requests, taints/tolerations, and node selectors.",
		})
	}

	return diagnostics
}

func unschedulableEvidence(events []state.EventInfo) []string {
	out := make([]string, 0, 2)
	for _, event := range events {
		lowerReason := strings.ToLower(event.Reason)
		lowerMessage := strings.ToLower(event.Message)
		if !strings.Contains(lowerReason, "failedscheduling") &&
			!strings.Contains(lowerReason, "unschedulable") &&
			!strings.Contains(lowerMessage, "insufficient") &&
			!strings.Contains(lowerMessage, "unschedulable") {
			continue
		}

		message := strings.TrimSpace(event.Message)
		if message == "" {
			message = strings.TrimSpace(event.Reason)
		}
		if message == "" {
			continue
		}
		out = append(out, message)
		if len(out) == 2 {
			break
		}
	}
	return out
}
