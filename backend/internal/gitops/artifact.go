package gitops

import (
	"fmt"
	"regexp"
	"strings"

	"kubelens-backend/internal/model"
)

type WorkloadInventory struct {
	Deployments  []model.ResourceRecord
	StatefulSets []model.ResourceRecord
	DaemonSets   []model.ResourceRecord
}

type WorkloadTarget struct {
	Kind      string
	Namespace string
	Name      string
}

var ordinalPodPattern = regexp.MustCompile(`^(.*)-(\d+)$`)

func ResolvePodWorkload(namespace string, podName string, inventory WorkloadInventory) (WorkloadTarget, bool) {
	ns := strings.TrimSpace(namespace)
	name := strings.TrimSpace(podName)
	if ns == "" || name == "" {
		return WorkloadTarget{}, false
	}

	if deploymentName := inferDeploymentName(name); deploymentName != "" {
		if target, ok := findTarget("deployment", ns, deploymentName, inventory.Deployments); ok {
			return target, true
		}
	}
	if statefulSetName := inferStatefulSetName(name); statefulSetName != "" {
		if target, ok := findTarget("statefulset", ns, statefulSetName, inventory.StatefulSets); ok {
			return target, true
		}
	}
	if target, ok := findLongestPrefixTarget("daemonset", ns, name, inventory.DaemonSets); ok {
		return target, true
	}
	if target, ok := findLongestPrefixTarget("deployment", ns, name, inventory.Deployments); ok {
		return target, true
	}
	return WorkloadTarget{}, false
}

func PathHint(target WorkloadTarget) string {
	if strings.TrimSpace(target.Name) == "" {
		return "ops/remediation/README.md"
	}
	kind := strings.ToLower(strings.TrimSpace(target.Kind))
	if kind == "" {
		kind = "resource"
	}
	ns := strings.TrimSpace(target.Namespace)
	if ns == "" {
		ns = "cluster"
	}
	return fmt.Sprintf("k8s/%s/%s-%s.yaml", slugify(ns), slugify(kind), slugify(target.Name))
}

func BuildPatchArtifact(
	summary string,
	strategy string,
	target WorkloadTarget,
	branch string,
	prTitle string,
	commitMessage string,
	patchBody string,
	instructions []string,
) model.GitOpsArtifact {
	return model.GitOpsArtifact{
		SupportLevel:    model.GitOpsSupportPatchReady,
		Strategy:        strings.TrimSpace(strategy),
		Summary:         strings.TrimSpace(summary),
		BranchName:      strings.TrimSpace(branch),
		PRTitle:         strings.TrimSpace(prTitle),
		CommitMessage:   strings.TrimSpace(commitMessage),
		TargetPath:      PathHint(target),
		TargetKind:      strings.TrimSpace(target.Kind),
		TargetNamespace: strings.TrimSpace(target.Namespace),
		TargetName:      strings.TrimSpace(target.Name),
		Format:          "yaml",
		ArtifactBody:    strings.TrimSpace(patchBody),
		Instructions:    cloneInstructions(instructions),
	}
}

func BuildAdvisoryArtifact(
	summary string,
	strategy string,
	target WorkloadTarget,
	branch string,
	prTitle string,
	commitMessage string,
	body string,
	instructions []string,
) model.GitOpsArtifact {
	return model.GitOpsArtifact{
		SupportLevel:    model.GitOpsSupportAdvisory,
		Strategy:        strings.TrimSpace(strategy),
		Summary:         strings.TrimSpace(summary),
		BranchName:      strings.TrimSpace(branch),
		PRTitle:         strings.TrimSpace(prTitle),
		CommitMessage:   strings.TrimSpace(commitMessage),
		TargetPath:      PathHint(target),
		TargetKind:      strings.TrimSpace(target.Kind),
		TargetNamespace: strings.TrimSpace(target.Namespace),
		TargetName:      strings.TrimSpace(target.Name),
		Format:          "md",
		ArtifactBody:    strings.TrimSpace(body),
		Instructions:    cloneInstructions(instructions),
	}
}

func Slug(value string) string {
	return slugify(value)
}

func findTarget(kind string, namespace string, name string, items []model.ResourceRecord) (WorkloadTarget, bool) {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Namespace), namespace) && strings.EqualFold(strings.TrimSpace(item.Name), name) {
			return WorkloadTarget{
				Kind:      kind,
				Namespace: namespace,
				Name:      name,
			}, true
		}
	}
	return WorkloadTarget{}, false
}

func findLongestPrefixTarget(kind string, namespace string, podName string, items []model.ResourceRecord) (WorkloadTarget, bool) {
	best := WorkloadTarget{}
	found := false
	for _, item := range items {
		if !strings.EqualFold(strings.TrimSpace(item.Namespace), namespace) {
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" || !strings.HasPrefix(strings.ToLower(podName), strings.ToLower(name+"-")) {
			continue
		}
		if !found || len(name) > len(best.Name) {
			best = WorkloadTarget{Kind: kind, Namespace: namespace, Name: name}
			found = true
		}
	}
	return best, found
}

func inferDeploymentName(podName string) string {
	parts := strings.Split(strings.TrimSpace(strings.ToLower(podName)), "-")
	if len(parts) < 3 {
		return ""
	}
	base := strings.Join(parts[:len(parts)-2], "-")
	return strings.Trim(base, "-")
}

func inferStatefulSetName(podName string) string {
	match := ordinalPodPattern.FindStringSubmatch(strings.TrimSpace(strings.ToLower(podName)))
	if len(match) != 3 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func cloneInstructions(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func slugify(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return "artifact"
	}
	var b strings.Builder
	lastDash := false
	for _, ch := range trimmed {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "artifact"
	}
	return out
}
