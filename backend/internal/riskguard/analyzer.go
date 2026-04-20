package riskguard

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"kubelens-backend/internal/model"
)

type Analyzer struct{}

type manifestSpec struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string            `yaml:"name"`
		Namespace string            `yaml:"namespace"`
		Labels    map[string]string `yaml:"labels"`
	} `yaml:"metadata"`
	Spec struct {
		Replicas *int `yaml:"replicas"`
		Strategy struct {
			Type string `yaml:"type"`
		} `yaml:"strategy"`
		Template struct {
			Metadata struct {
				Labels map[string]string `yaml:"labels"`
			} `yaml:"metadata"`
			Spec struct {
				ServiceAccountName string            `yaml:"serviceAccountName"`
				NodeSelector       map[string]string `yaml:"nodeSelector"`
				Containers         []containerSpec   `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type containerSpec struct {
	Name            string `yaml:"name"`
	Image           string `yaml:"image"`
	ImagePullPolicy string `yaml:"imagePullPolicy"`
	Resources       struct {
		Requests resourceMap `yaml:"requests"`
		Limits   resourceMap `yaml:"limits"`
	} `yaml:"resources"`
	LivenessProbe   *yaml.Node `yaml:"livenessProbe"`
	ReadinessProbe  *yaml.Node `yaml:"readinessProbe"`
	SecurityContext struct {
		Privileged               *bool `yaml:"privileged"`
		RunAsNonRoot             *bool `yaml:"runAsNonRoot"`
		AllowPrivilegeEscalation *bool `yaml:"allowPrivilegeEscalation"`
	} `yaml:"securityContext"`
}

type resourceMap map[string]any

func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

func (a *Analyzer) Analyze(manifest string, pods []model.PodSummary, nodes []model.NodeSummary) model.RiskReport {
	spec, err := parseManifest(strings.TrimSpace(manifest))
	if err != nil {
		check := model.RiskCheck{
			Name:       "Manifest parse",
			Category:   "Manifest hygiene",
			Passed:     false,
			Detail:     fmt.Sprintf("manifest could not be parsed: %v", err),
			Suggestion: "Validate YAML structure and required indentation before retrying.",
			Score:      30,
		}
		score := clampScore(check.Score)
		level, summary := summarizeRisk(score)
		return model.RiskReport{
			Score:   score,
			Level:   level,
			Summary: summary,
			Checks:  []model.RiskCheck{check},
		}
	}

	checks := []model.RiskCheck{
		checkKindAndMetadata(spec, pods, nodes),
		checkReplicaSafety(spec, pods, nodes),
		checkContainerImages(spec, pods, nodes),
		checkResourceRequests(spec, pods, nodes),
		checkResourceLimits(spec, pods, nodes),
		checkHealthProbes(spec, pods, nodes),
		checkContainerSecurity(spec, pods, nodes),
		checkImagePullPolicy(spec, pods, nodes),
		checkNodeSelectorCompatibility(spec, pods, nodes),
		checkClusterPressureCompatibility(spec, pods, nodes),
	}

	score := 0
	for _, check := range checks {
		if !check.Passed {
			score += check.Score
		}
	}
	score = clampScore(score)
	level, summary := summarizeRisk(score)
	return model.RiskReport{
		Score:   score,
		Level:   level,
		Summary: summary,
		Checks:  checks,
	}
}

func Analyze(manifest string, pods []model.PodSummary, nodes []model.NodeSummary) model.RiskReport {
	return NewAnalyzer().Analyze(manifest, pods, nodes)
}

func parseManifest(manifest string) (manifestSpec, error) {
	var spec manifestSpec
	if strings.TrimSpace(manifest) == "" {
		return manifestSpec{}, fmt.Errorf("manifest is empty")
	}
	decoder := yaml.NewDecoder(strings.NewReader(manifest))
	if err := decoder.Decode(&spec); err != nil {
		return manifestSpec{}, err
	}
	return spec, nil
}

func checkKindAndMetadata(spec manifestSpec, _ []model.PodSummary, _ []model.NodeSummary) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Kind and metadata",
		Category:   "Manifest hygiene",
		Passed:     true,
		Detail:     "Kind, name, and namespace are present.",
		Suggestion: "Keep metadata explicit to avoid accidental cross-namespace deployment.",
		Score:      8,
	}
	if strings.TrimSpace(spec.Kind) == "" || strings.TrimSpace(spec.Metadata.Name) == "" || strings.TrimSpace(spec.Metadata.Namespace) == "" {
		check.Passed = false
		check.Detail = "Manifest should declare kind, metadata.name, and metadata.namespace."
		check.Suggestion = "Set kind/name/namespace explicitly in metadata."
	}
	return check
}

func checkReplicaSafety(spec manifestSpec, _ []model.PodSummary, _ []model.NodeSummary) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Replica safety",
		Category:   "Availability",
		Passed:     true,
		Detail:     "Replica count supports baseline availability.",
		Suggestion: "Use at least 2 replicas for user-facing workloads.",
		Score:      10,
	}
	if spec.Spec.Replicas == nil || *spec.Spec.Replicas < 2 {
		check.Passed = false
		check.Detail = "Single-replica workloads increase downtime risk during rollout or restart."
	}
	return check
}

func checkContainerImages(spec manifestSpec, _ []model.PodSummary, _ []model.NodeSummary) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Image tagging",
		Category:   "Manifest hygiene",
		Passed:     true,
		Detail:     "Container images use explicit immutable tags.",
		Suggestion: "Pin images to immutable version tags; avoid :latest.",
		Score:      8,
	}
	for _, container := range spec.Spec.Template.Spec.Containers {
		image := strings.TrimSpace(container.Image)
		if image == "" || strings.HasSuffix(strings.ToLower(image), ":latest") || !strings.Contains(image, ":") {
			check.Passed = false
			check.Detail = "At least one container uses missing/implicit tag or :latest."
			return check
		}
	}
	return check
}

func checkResourceRequests(spec manifestSpec, _ []model.PodSummary, _ []model.NodeSummary) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Resource requests",
		Category:   "Capacity",
		Passed:     true,
		Detail:     "CPU and memory requests are declared for all containers.",
		Suggestion: "Define cpu/memory requests to stabilize scheduling and autoscaling behavior.",
		Score:      10,
	}
	for _, container := range spec.Spec.Template.Spec.Containers {
		if !hasResourceKey(container.Resources.Requests, "cpu") || !hasResourceKey(container.Resources.Requests, "memory") {
			check.Passed = false
			check.Detail = "One or more containers are missing cpu/memory requests."
			return check
		}
	}
	return check
}

func checkResourceLimits(spec manifestSpec, _ []model.PodSummary, _ []model.NodeSummary) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Resource limits",
		Category:   "Capacity",
		Passed:     true,
		Detail:     "CPU and memory limits are declared for all containers.",
		Suggestion: "Set realistic cpu/memory limits to avoid noisy-neighbor impact and OOM surprises.",
		Score:      10,
	}
	for _, container := range spec.Spec.Template.Spec.Containers {
		if !hasResourceKey(container.Resources.Limits, "cpu") || !hasResourceKey(container.Resources.Limits, "memory") {
			check.Passed = false
			check.Detail = "One or more containers are missing cpu/memory limits."
			return check
		}
	}
	return check
}

func checkHealthProbes(spec manifestSpec, _ []model.PodSummary, _ []model.NodeSummary) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Health probes",
		Category:   "Availability",
		Passed:     true,
		Detail:     "Readiness and liveness probes are defined for all containers.",
		Suggestion: "Add readiness and liveness probes so rollouts and restarts are safe.",
		Score:      9,
	}
	for _, container := range spec.Spec.Template.Spec.Containers {
		if container.ReadinessProbe == nil || container.LivenessProbe == nil {
			check.Passed = false
			check.Detail = "At least one container is missing readiness/liveness probe configuration."
			return check
		}
	}
	return check
}

func checkContainerSecurity(spec manifestSpec, _ []model.PodSummary, _ []model.NodeSummary) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Container security context",
		Category:   "Security",
		Passed:     true,
		Detail:     "Security context settings avoid privileged escalation by default.",
		Suggestion: "Set runAsNonRoot=true, privileged=false, allowPrivilegeEscalation=false.",
		Score:      12,
	}
	for _, container := range spec.Spec.Template.Spec.Containers {
		if boolPtrValue(container.SecurityContext.Privileged, false) {
			check.Passed = false
			check.Detail = "Container runs privileged."
			return check
		}
		if !boolPtrValue(container.SecurityContext.RunAsNonRoot, false) {
			check.Passed = false
			check.Detail = "Container does not enforce runAsNonRoot."
			return check
		}
		if boolPtrValue(container.SecurityContext.AllowPrivilegeEscalation, true) {
			check.Passed = false
			check.Detail = "Container allows privilege escalation."
			return check
		}
	}
	return check
}

func checkImagePullPolicy(spec manifestSpec, _ []model.PodSummary, _ []model.NodeSummary) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Image pull policy",
		Category:   "Manifest hygiene",
		Passed:     true,
		Detail:     "Image pull policy is explicit and aligns with immutable tags.",
		Suggestion: "Use IfNotPresent for immutable tags and Always for frequently changing tags.",
		Score:      5,
	}
	for _, container := range spec.Spec.Template.Spec.Containers {
		policy := strings.TrimSpace(strings.ToLower(container.ImagePullPolicy))
		if policy == "" {
			check.Passed = false
			check.Detail = "At least one container omits imagePullPolicy."
			return check
		}
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(container.Image)), ":latest") && policy != "always" {
			check.Passed = false
			check.Detail = "Containers using :latest should set imagePullPolicy=Always."
			return check
		}
	}
	return check
}

func checkNodeSelectorCompatibility(spec manifestSpec, _ []model.PodSummary, nodes []model.NodeSummary) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Node selector compatibility",
		Category:   "Capacity",
		Passed:     true,
		Detail:     "Node selector appears compatible with current cluster nodes.",
		Suggestion: "Validate nodeSelector values against actual node labels in target cluster.",
		Score:      10,
	}
	if len(spec.Spec.Template.Spec.NodeSelector) == 0 {
		return check
	}
	if len(nodes) == 0 {
		check.Passed = false
		check.Detail = "Node selector is set but no node data is available to validate compatibility."
		return check
	}
	// NodeSummary does not expose labels; when selector is used, treat as medium risk due unverifiable placement.
	check.Passed = false
	check.Detail = "Node selector is set; label compatibility cannot be verified from current node summary payload."
	return check
}

func checkClusterPressureCompatibility(_ manifestSpec, pods []model.PodSummary, nodes []model.NodeSummary) model.RiskCheck {
	check := model.RiskCheck{
		Name:       "Cluster pressure compatibility",
		Category:   "Capacity",
		Passed:     true,
		Detail:     "Current cluster pressure is within safe bounds for additional rollout load.",
		Suggestion: "Avoid high-risk deploys during elevated pending/failed pod or NotReady node conditions.",
		Score:      12,
	}
	if len(pods) == 0 && len(nodes) == 0 {
		return check
	}

	var pendingFailed int
	for _, pod := range pods {
		if pod.Status == model.PodStatusPending || pod.Status == model.PodStatusFailed {
			pendingFailed++
		}
	}

	var notReady int
	for _, node := range nodes {
		if node.Status != model.NodeStatusReady {
			notReady++
		}
	}

	if pendingFailed >= 3 || notReady > 0 {
		check.Passed = false
		check.Detail = fmt.Sprintf("Cluster currently has %d pending/failed pods and %d non-ready nodes.", pendingFailed, notReady)
	}
	return check
}

func hasResourceKey(values resourceMap, key string) bool {
	if len(values) == 0 {
		return false
	}
	value, ok := values[key]
	if !ok {
		return false
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value)) != ""
}

func boolPtrValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func summarizeRisk(score int) (string, string) {
	switch {
	case score <= 25:
		return "LOW", "LOW — deploy with standard monitoring"
	case score <= 50:
		return "MEDIUM", "MEDIUM — review flagged issues before deploying"
	case score <= 75:
		return "HIGH", "HIGH — address critical issues before deploying"
	default:
		return "CRITICAL", "CRITICAL — do not deploy without resolving all critical issues"
	}
}
