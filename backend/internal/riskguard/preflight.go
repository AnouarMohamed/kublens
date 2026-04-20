package riskguard

import (
	"fmt"
	"strings"

	"kubelens-backend/internal/model"
)

type PolicyInventory struct {
	Namespaces      []model.ResourceRecord
	Workloads       []model.ResourceRecord
	ServiceAccounts []model.ResourceRecord
	NetworkPolicies []model.ResourceRecord
}

func BuildPolicyChecks(manifest string, pods []model.PodSummary, inventory PolicyInventory) []model.RiskCheck {
	spec, err := parseManifest(strings.TrimSpace(manifest))
	if err != nil {
		return nil
	}

	namespace := strings.TrimSpace(spec.Metadata.Namespace)
	checks := []model.RiskCheck{
		checkNamespaceExists(namespace, inventory.Namespaces),
		checkWorkloadCollision(spec, inventory.Workloads),
		checkServiceAccountExists(namespace, spec.Spec.Template.Spec.ServiceAccountName, inventory.ServiceAccounts),
		checkNetworkPolicyBaseline(namespace, inventory.NetworkPolicies),
		checkNamespaceRuntimePressure(namespace, pods),
	}
	return checks
}

func AugmentReport(report model.RiskReport, checks ...model.RiskCheck) model.RiskReport {
	if len(checks) == 0 {
		return report
	}

	combined := make([]model.RiskCheck, 0, len(report.Checks)+len(checks))
	combined = append(combined, report.Checks...)
	combined = append(combined, checks...)

	score := 0
	for _, check := range combined {
		if !check.Passed {
			score += check.Score
		}
	}
	score = clampScore(score)
	level, summary := summarizeRisk(score)
	report.Score = score
	report.Level = level
	report.Summary = summary
	report.Checks = combined
	return report
}

func checkNamespaceExists(namespace string, namespaces []model.ResourceRecord) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Namespace targeting",
		Category:   "Policy preflight",
		Passed:     true,
		Detail:     "Target namespace exists in the active cluster context.",
		Suggestion: "Create the namespace and baseline policy objects before rollout if this is a new environment.",
		Score:      14,
	}
	if namespace == "" {
		check.Passed = false
		check.Detail = "Manifest namespace is empty, so target policy boundaries cannot be verified."
		return check
	}
	for _, item := range namespaces {
		if strings.EqualFold(strings.TrimSpace(item.Name), namespace) {
			return check
		}
	}
	check.Passed = false
	check.Detail = fmt.Sprintf("Namespace %q is not present in the active cluster context.", namespace)
	return check
}

func checkWorkloadCollision(spec manifestSpec, workloads []model.ResourceRecord) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Live workload collision",
		Category:   "Policy preflight",
		Passed:     true,
		Detail:     "No existing workload with the same identity was detected.",
		Suggestion: "If this change targets an existing workload, review rollout strategy, disruption budget, and current readiness before apply.",
		Score:      6,
	}

	namespace := strings.TrimSpace(spec.Metadata.Namespace)
	name := strings.TrimSpace(spec.Metadata.Name)
	if namespace == "" || name == "" {
		check.Passed = false
		check.Detail = "Manifest identity is incomplete, so collision checks cannot be completed."
		return check
	}

	for _, item := range workloads {
		if strings.EqualFold(strings.TrimSpace(item.Namespace), namespace) && strings.EqualFold(strings.TrimSpace(item.Name), name) {
			check.Passed = false
			check.Detail = fmt.Sprintf("%s %s/%s already exists and this apply will mutate a live workload.", strings.TrimSpace(spec.Kind), namespace, name)
			return check
		}
	}
	return check
}

func checkServiceAccountExists(namespace string, serviceAccount string, accounts []model.ResourceRecord) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Service account binding",
		Category:   "Policy preflight",
		Passed:     true,
		Detail:     "Service account reference resolves in the target namespace.",
		Suggestion: "Create the service account and verify RBAC bindings before rollout.",
		Score:      12,
	}
	serviceAccount = strings.TrimSpace(serviceAccount)
	if serviceAccount == "" {
		check.Detail = "Manifest uses the namespace default service account."
		return check
	}
	for _, item := range accounts {
		if strings.EqualFold(strings.TrimSpace(item.Namespace), namespace) && strings.EqualFold(strings.TrimSpace(item.Name), serviceAccount) {
			return check
		}
	}
	check.Passed = false
	check.Detail = fmt.Sprintf("Service account %s/%s was not found in the active cluster context.", namespace, serviceAccount)
	return check
}

func checkNetworkPolicyBaseline(namespace string, policies []model.ResourceRecord) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Namespace network policy baseline",
		Category:   "Policy preflight",
		Passed:     true,
		Detail:     "At least one network policy exists in the target namespace.",
		Suggestion: "Confirm ingress and egress expectations before rollout when a namespace has no network policies.",
		Score:      4,
	}
	for _, item := range policies {
		if strings.EqualFold(strings.TrimSpace(item.Namespace), namespace) {
			return check
		}
	}
	check.Passed = false
	check.Detail = fmt.Sprintf("No network policy objects were found in namespace %q.", namespace)
	return check
}

func checkNamespaceRuntimePressure(namespace string, pods []model.PodSummary) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Namespace runtime pressure",
		Category:   "Policy preflight",
		Passed:     true,
		Detail:     "Namespace has no pending or failed pod pressure signals at preflight time.",
		Suggestion: "Stabilize namespace health before rollout when pending or failed pods are already accumulating.",
		Score:      8,
	}
	if namespace == "" {
		check.Passed = false
		check.Detail = "Manifest namespace is empty, so namespace pressure cannot be assessed."
		return check
	}

	pendingOrFailed := 0
	for _, pod := range pods {
		if !strings.EqualFold(strings.TrimSpace(pod.Namespace), namespace) {
			continue
		}
		if pod.Status == model.PodStatusPending || pod.Status == model.PodStatusFailed {
			pendingOrFailed++
		}
	}
	if pendingOrFailed > 0 {
		check.Passed = false
		check.Detail = fmt.Sprintf("Namespace %q currently has %d pending or failed pods.", namespace, pendingOrFailed)
	}
	return check
}
