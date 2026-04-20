package httpapi

import (
	"context"
	"net/http"
	"strings"

	"kubelens-backend/internal/model"
	"kubelens-backend/internal/riskguard"
)

func (s *Server) handleAnalyzeRiskGuard(w http.ResponseWriter, r *http.Request) {
	var req model.RiskAnalyzeRequest
	if err := s.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	manifest := strings.TrimSpace(req.Manifest)
	if manifest == "" {
		writeError(w, http.StatusBadRequest, "manifest is required")
		return
	}

	pods, nodes := s.cluster.Snapshot(r.Context())
	report := riskguard.Analyze(manifest, pods, nodes)
	if s.riskGuard != nil {
		report = s.riskGuard.Analyze(manifest, pods, nodes)
	}
	report = riskguard.AugmentReport(report, riskguard.BuildPolicyChecks(manifest, pods, s.collectRiskPolicyInventory(r.Context(), manifest))...)
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) evaluateManifestRisk(manifest string, pods []model.PodSummary, nodes []model.NodeSummary) model.RiskReport {
	if s.riskGuard != nil {
		return s.riskGuard.Analyze(manifest, pods, nodes)
	}
	return model.RiskReport{
		Score:   0,
		Level:   "LOW",
		Summary: "LOW — deploy with standard monitoring",
		Checks:  []model.RiskCheck{},
	}
}

func (s *Server) collectRiskPolicyInventory(ctx context.Context, manifest string) riskguard.PolicyInventory {
	kind := riskguardManifestKindToResourceKind(manifest)

	return riskguard.PolicyInventory{
		Namespaces:      listRiskPolicyResources(ctx, s.cluster, "namespaces"),
		Workloads:       listRiskPolicyResources(ctx, s.cluster, kind),
		ServiceAccounts: listRiskPolicyResources(ctx, s.cluster, "serviceaccounts"),
		NetworkPolicies: listRiskPolicyResources(ctx, s.cluster, "networkpolicies"),
	}
}

func listRiskPolicyResources(ctx context.Context, cluster ClusterReader, kind string) []model.ResourceRecord {
	trimmed := strings.TrimSpace(kind)
	if trimmed == "" {
		return nil
	}
	items, err := cluster.ListResources(ctx, trimmed)
	if err != nil {
		return nil
	}
	return items
}

func riskguardManifestKindToResourceKind(manifest string) string {
	normalized := strings.ToLower(manifest)
	switch {
	case strings.Contains(normalized, "\nkind: deployment"), strings.HasPrefix(strings.TrimSpace(normalized), "kind: deployment"):
		return "deployments"
	case strings.Contains(normalized, "\nkind: statefulset"), strings.HasPrefix(strings.TrimSpace(normalized), "kind: statefulset"):
		return "statefulsets"
	case strings.Contains(normalized, "\nkind: daemonset"), strings.HasPrefix(strings.TrimSpace(normalized), "kind: daemonset"):
		return "daemonsets"
	case strings.Contains(normalized, "\nkind: cronjob"), strings.HasPrefix(strings.TrimSpace(normalized), "kind: cronjob"):
		return "cronjobs"
	case strings.Contains(normalized, "\nkind: job"), strings.HasPrefix(strings.TrimSpace(normalized), "kind: job"):
		return "jobs"
	default:
		return ""
	}
}
