package httpapi

import (
	"context"

	"kubelens-backend/internal/ai"
	"kubelens-backend/internal/diagnostics"
	"kubelens-backend/internal/intelligence"
	"kubelens-backend/internal/model"
	"kubelens-backend/internal/state"
)

func (s *Server) runDiagnostics(ctx context.Context) intelligence.Report {
	if s.intel == nil {
		pods, nodes := s.cluster.Snapshot(ctx)
		return intelligenceReportFromDiagnosticsResult(diagnostics.BuildDiagnostics(pods, nodes))
	}

	if snapshot, ok := s.cluster.StateSnapshot(ctx); ok {
		return s.intel.Analyze(snapshot)
	}

	// Fallback: build minimal state from current pod/node summaries.
	pods, nodes := s.cluster.Snapshot(ctx)
	minimal := buildStateFromSummaries(pods, nodes)
	return s.intel.Analyze(minimal)
}

func intelligenceReportFromDiagnosticsResult(result model.DiagnosticsResult) intelligence.Report {
	diags := make([]intelligence.Diagnostic, 0, len(result.Issues))
	for _, issue := range result.Issues {
		diags = append(diags, intelligence.Diagnostic{
			Severity:       intelligence.Severity(issue.Severity),
			Resource:       issue.Resource,
			Namespace:      issue.Namespace,
			Message:        issue.Message,
			Evidence:       append([]string(nil), issue.Evidence...),
			Recommendation: issue.Recommendation,
			Source:         issue.Source,
		})
	}
	return intelligence.Report{
		GeneratedAt:    result.Timestamp,
		Diagnostics:    diags,
		CriticalIssues: result.CriticalIssues,
		WarningIssues:  result.WarningIssues,
		HealthScore:    result.HealthScore,
		Summary:        result.Summary,
	}
}

func (s *Server) mapDiagnosticsReport(report intelligence.Report) model.DiagnosticsResult {
	issues := make([]model.DiagnosticIssue, 0, len(report.Diagnostics))
	for _, diag := range report.Diagnostics {
		issues = append(issues, model.DiagnosticIssue{
			Severity:       model.DiagnosticSeverity(diag.Severity),
			Resource:       diag.Resource,
			Namespace:      diag.Namespace,
			Message:        diag.Message,
			Evidence:       append([]string(nil), diag.Evidence...),
			Recommendation: diag.Recommendation,
			Source:         diag.Source,
		})
	}

	return model.DiagnosticsResult{
		Summary:        report.Summary,
		Timestamp:      report.GeneratedAt,
		CriticalIssues: report.CriticalIssues,
		WarningIssues:  report.WarningIssues,
		HealthScore:    report.HealthScore,
		Issues:         issues,
	}
}

func buildStateFromSummaries(pods []model.PodSummary, nodes []model.NodeSummary) state.ClusterState {
	snapshot := state.ClusterState{
		Pods:        map[string]state.PodInfo{},
		Nodes:       map[string]state.NodeInfo{},
		Deployments: map[string]state.DeploymentInfo{},
		Events:      []state.EventInfo{},
	}

	for _, pod := range pods {
		key := pod.Namespace + "/" + pod.Name
		snapshot.Pods[key] = state.PodInfo{
			UID:       pod.ID,
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Phase:     string(pod.Status),
			Restarts:  pod.Restarts,
		}
	}
	for _, node := range nodes {
		snapshot.Nodes[node.Name] = state.NodeInfo{
			Name:   node.Name,
			Status: string(node.Status),
		}
	}

	return snapshot
}

func mapDiagnosticsForAI(diags []intelligence.Diagnostic) []ai.DiagnosticBrief {
	if len(diags) == 0 {
		return nil
	}
	out := make([]ai.DiagnosticBrief, 0, len(diags))
	for _, diag := range diags {
		out = append(out, ai.DiagnosticBrief{
			Severity:       string(diag.Severity),
			Resource:       diag.Resource,
			Namespace:      diag.Namespace,
			Message:        diag.Message,
			Evidence:       append([]string(nil), diag.Evidence...),
			Recommendation: diag.Recommendation,
			Source:         diag.Source,
		})
	}
	return out
}

func filterDiagnosticsForResource(diags []ai.DiagnosticBrief, namespace, name string) []ai.DiagnosticBrief {
	if len(diags) == 0 {
		return nil
	}
	out := make([]ai.DiagnosticBrief, 0, 4)
	for _, diag := range diags {
		if diag.Resource == "" {
			continue
		}
		if name != "" && diag.Resource != name {
			continue
		}
		if namespace != "" && diag.Namespace != "" && diag.Namespace != namespace {
			continue
		}
		out = append(out, diag)
	}
	return out
}
