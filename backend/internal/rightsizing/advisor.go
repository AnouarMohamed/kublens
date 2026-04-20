package rightsizing

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"kubelens-backend/internal/gitops"
	"kubelens-backend/internal/model"
)

type podResources struct {
	requestCPUMilli int
	requestMemoryMi int
	limitCPUMilli   int
	limitMemoryMi   int
}

type containerRecommendation struct {
	name            string
	requestCPUMilli int
	requestMemoryMi int
	limitCPUMilli   int
	limitMemoryMi   int
}

func BuildOverview(
	pods []model.PodSummary,
	details map[string]model.PodDetail,
	inventory gitops.WorkloadInventory,
	now time.Time,
) model.RightsizingOverview {
	items := make([]model.RightsizingRecommendation, 0, len(pods))
	overprovisioned := 0
	underprovisioned := 0
	missingGuardrails := 0
	balanced := 0
	totalReclaimCPU := 0
	totalReclaimMemory := 0

	for _, pod := range pods {
		if pod.Status != model.PodStatusRunning {
			continue
		}
		key := pod.Namespace + "/" + pod.Name
		detail, ok := details[key]
		if !ok {
			continue
		}

		item, include := buildRecommendation(pod, detail, inventory)
		if !include {
			continue
		}

		switch item.Status {
		case model.RightsizingStatusOverprovisioned:
			overprovisioned++
		case model.RightsizingStatusUnderprovisioned:
			underprovisioned++
		case model.RightsizingStatusMissingGuardrail:
			missingGuardrails++
		default:
			balanced++
		}
		if reclaimCPU, ok := parseCPUMilli(item.ReclaimableCPU); ok {
			totalReclaimCPU += reclaimCPU
		}
		if reclaimMemory, ok := parseMemoryMi(item.ReclaimableMemory); ok {
			totalReclaimMemory += reclaimMemory
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		left := rightsizingRank(items[i].Status)
		right := rightsizingRank(items[j].Status)
		if left != right {
			return left > right
		}
		if items[i].Confidence != items[j].Confidence {
			return items[i].Confidence > items[j].Confidence
		}
		if items[i].Namespace == items[j].Namespace {
			return items[i].Pod < items[j].Pod
		}
		return items[i].Namespace < items[j].Namespace
	})

	summary := fmt.Sprintf(
		"%d savings opportunities, %d underprovisioned workloads, %d missing guardrail candidates, %d balanced workloads.",
		overprovisioned,
		underprovisioned,
		missingGuardrails,
		balanced,
	)

	return model.RightsizingOverview{
		GeneratedAt:          now.UTC().Format(time.RFC3339),
		Summary:              summary,
		SavingsOpportunities: overprovisioned,
		Underprovisioned:     underprovisioned,
		MissingGuardrails:    missingGuardrails,
		Balanced:             balanced,
		ReclaimableCPU:       formatCPUMilli(totalReclaimCPU),
		ReclaimableMemory:    formatMemoryMi(totalReclaimMemory),
		Items:                items,
	}
}

func buildRecommendation(
	pod model.PodSummary,
	detail model.PodDetail,
	inventory gitops.WorkloadInventory,
) (model.RightsizingRecommendation, bool) {
	usageCPU, cpuKnown := parseCPUMilli(pod.CPU)
	usageMemory, memoryKnown := parseMemoryMi(pod.Memory)
	current := aggregateResources(detail)
	if len(detail.Containers) == 0 {
		return model.RightsizingRecommendation{}, false
	}

	status := classifyStatus(current, usageCPU, cpuKnown, usageMemory, memoryKnown)
	recommended := recommendResources(status, current, usageCPU, cpuKnown, usageMemory, memoryKnown)
	reclaimCPU := maxInt(current.requestCPUMilli-recommended.requestCPUMilli, 0)
	reclaimMemory := maxInt(current.requestMemoryMi-recommended.requestMemoryMi, 0)

	target, targetOK := gitops.ResolvePodWorkload(pod.Namespace, pod.Name, inventory)
	artifact := buildArtifact(status, pod, detail, target, targetOK, recommended)

	item := model.RightsizingRecommendation{
		ID:                       gitops.Slug(pod.Namespace + "-" + pod.Name),
		Namespace:                pod.Namespace,
		Pod:                      pod.Name,
		Status:                   status,
		RiskLevel:                riskForStatus(status),
		Summary:                  summaryForStatus(status, recommended, current),
		QoSClass:                 inferQoSClass(detail),
		ContainerCount:           maxInt(len(detail.Containers), 1),
		CPUUsage:                 formatCPUMilli(usageCPU),
		MemoryUsage:              formatMemoryMi(usageMemory),
		RequestCPU:               formatCPUMilli(current.requestCPUMilli),
		RequestMemory:            formatMemoryMi(current.requestMemoryMi),
		LimitCPU:                 formatCPUMilli(current.limitCPUMilli),
		LimitMemory:              formatMemoryMi(current.limitMemoryMi),
		RecommendedRequestCPU:    formatCPUMilli(recommended.requestCPUMilli),
		RecommendedRequestMemory: formatMemoryMi(recommended.requestMemoryMi),
		RecommendedLimitCPU:      formatCPUMilli(recommended.limitCPUMilli),
		RecommendedLimitMemory:   formatMemoryMi(recommended.limitMemoryMi),
		ReclaimableCPU:           formatCPUMilli(reclaimCPU),
		ReclaimableMemory:        formatMemoryMi(reclaimMemory),
		Confidence:               confidenceForRecommendation(current, cpuKnown, memoryKnown, targetOK, len(detail.Containers)),
		Artifact:                 artifact,
	}
	if targetOK {
		item.WorkloadKind = target.Kind
		item.WorkloadName = target.Name
	}
	return item, true
}

func classifyStatus(current podResources, usageCPU int, cpuKnown bool, usageMemory int, memoryKnown bool) model.RightsizingStatus {
	if current.requestCPUMilli == 0 || current.requestMemoryMi == 0 || current.limitCPUMilli == 0 || current.limitMemoryMi == 0 {
		return model.RightsizingStatusMissingGuardrail
	}

	if (cpuKnown && current.limitCPUMilli > 0 && usageCPU >= int(math.Round(float64(current.limitCPUMilli)*0.9))) ||
		(cpuKnown && current.requestCPUMilli > 0 && usageCPU >= int(math.Round(float64(current.requestCPUMilli)*1.3))) ||
		(memoryKnown && current.limitMemoryMi > 0 && usageMemory >= int(math.Round(float64(current.limitMemoryMi)*0.9))) ||
		(memoryKnown && current.requestMemoryMi > 0 && usageMemory >= int(math.Round(float64(current.requestMemoryMi)*1.3))) {
		return model.RightsizingStatusUnderprovisioned
	}

	if (cpuKnown && current.requestCPUMilli >= 150 && usageCPU <= int(math.Round(float64(current.requestCPUMilli)*0.35))) ||
		(memoryKnown && current.requestMemoryMi >= 128 && usageMemory <= int(math.Round(float64(current.requestMemoryMi)*0.35))) {
		return model.RightsizingStatusOverprovisioned
	}

	return model.RightsizingStatusBalanced
}

func recommendResources(
	status model.RightsizingStatus,
	current podResources,
	usageCPU int,
	cpuKnown bool,
	usageMemory int,
	memoryKnown bool,
) podResources {
	recommended := current

	minCPU := 50
	minMemory := 64
	if !cpuKnown {
		usageCPU = 0
	}
	if !memoryKnown {
		usageMemory = 0
	}

	switch status {
	case model.RightsizingStatusMissingGuardrail:
		recommended.requestCPUMilli = ensurePositive(current.requestCPUMilli, maxInt(int(math.Ceil(float64(usageCPU)*1.25)), 100))
		recommended.requestMemoryMi = ensurePositive(current.requestMemoryMi, maxInt(int(math.Ceil(float64(usageMemory)*1.25)), 128))
		recommended.limitCPUMilli = ensurePositive(current.limitCPUMilli, maxInt(int(math.Ceil(float64(recommended.requestCPUMilli)*1.6)), int(math.Ceil(float64(usageCPU)*1.8))))
		recommended.limitMemoryMi = ensurePositive(current.limitMemoryMi, maxInt(int(math.Ceil(float64(recommended.requestMemoryMi)*1.5)), int(math.Ceil(float64(usageMemory)*1.8))))
	case model.RightsizingStatusUnderprovisioned:
		recommended.requestCPUMilli = maxInt(current.requestCPUMilli, int(math.Ceil(float64(usageCPU)*1.25)))
		recommended.requestMemoryMi = maxInt(current.requestMemoryMi, int(math.Ceil(float64(usageMemory)*1.25)))
		recommended.limitCPUMilli = maxInt(current.limitCPUMilli, maxInt(int(math.Ceil(float64(recommended.requestCPUMilli)*1.5)), int(math.Ceil(float64(usageCPU)*1.7))))
		recommended.limitMemoryMi = maxInt(current.limitMemoryMi, maxInt(int(math.Ceil(float64(recommended.requestMemoryMi)*1.4)), int(math.Ceil(float64(usageMemory)*1.7))))
	case model.RightsizingStatusOverprovisioned:
		if cpuKnown {
			recommended.requestCPUMilli = minPositive(
				current.requestCPUMilli,
				maxInt(int(math.Ceil(float64(usageCPU)*1.3)), minCPU),
			)
			recommended.limitCPUMilli = minPositive(
				current.limitCPUMilli,
				maxInt(int(math.Ceil(float64(usageCPU)*1.8)), recommended.requestCPUMilli),
			)
		}
		if memoryKnown {
			recommended.requestMemoryMi = minPositive(
				current.requestMemoryMi,
				maxInt(int(math.Ceil(float64(usageMemory)*1.35)), minMemory),
			)
			recommended.limitMemoryMi = minPositive(
				current.limitMemoryMi,
				maxInt(int(math.Ceil(float64(usageMemory)*1.7)), recommended.requestMemoryMi),
			)
		}
	default:
		return recommended
	}

	recommended.requestCPUMilli = maxInt(recommended.requestCPUMilli, minCPU)
	recommended.requestMemoryMi = maxInt(recommended.requestMemoryMi, minMemory)
	recommended.limitCPUMilli = maxInt(recommended.limitCPUMilli, recommended.requestCPUMilli)
	recommended.limitMemoryMi = maxInt(recommended.limitMemoryMi, recommended.requestMemoryMi)
	return recommended
}

func buildArtifact(
	status model.RightsizingStatus,
	pod model.PodSummary,
	detail model.PodDetail,
	target gitops.WorkloadTarget,
	targetOK bool,
	recommended podResources,
) *model.GitOpsArtifact {
	if status == model.RightsizingStatusBalanced {
		return nil
	}

	branch := fmt.Sprintf(
		"rightsizing/%s-%s",
		gitops.Slug(pod.Namespace),
		gitops.Slug(pod.Name),
	)
	prTitle := fmt.Sprintf("Rightsize %s/%s resource requests", strings.TrimSpace(pod.Namespace), strings.TrimSpace(pod.Name))
	commitMessage := fmt.Sprintf("chore(rightsizing): tune %s/%s resources", strings.TrimSpace(pod.Namespace), strings.TrimSpace(pod.Name))

	if targetOK {
		containers := distributeContainerRecommendations(detail, recommended)
		body := renderRightsizingPatch(target, containers)
		artifact := gitops.BuildPatchArtifact(
			fmt.Sprintf("Apply rightsizing recommendations for %s/%s through the workload manifest.", pod.Namespace, pod.Name),
			"rightsizing_patch",
			target,
			branch,
			prTitle,
			commitMessage,
			body,
			[]string{
				"Review the generated resource requests and limits against recent workload peaks.",
				"Commit the patch in the workload repository and let the GitOps controller reconcile it.",
				"Monitor restart rate and error budget after rollout before closing the recommendation.",
			},
		)
		return &artifact
	}

	target = gitops.WorkloadTarget{Namespace: pod.Namespace, Name: pod.Name}
	body := strings.Join([]string{
		"# Rightsizing Advisory",
		"",
		fmt.Sprintf("- Pod: %s/%s", strings.TrimSpace(pod.Namespace), strings.TrimSpace(pod.Name)),
		fmt.Sprintf("- Recommended requests: CPU %s, Memory %s", formatCPUMilli(recommended.requestCPUMilli), formatMemoryMi(recommended.requestMemoryMi)),
		fmt.Sprintf("- Recommended limits: CPU %s, Memory %s", formatCPUMilli(recommended.limitCPUMilli), formatMemoryMi(recommended.limitMemoryMi)),
		"",
		"Resolve the owning workload manifest first, then translate the recommendation into the corresponding Deployment, StatefulSet, or DaemonSet template.",
	}, "\n")
	artifact := gitops.BuildAdvisoryArtifact(
		fmt.Sprintf("Workload ownership for %s/%s could not be resolved automatically.", pod.Namespace, pod.Name),
		"rightsizing_advisory",
		target,
		branch,
		prTitle,
		commitMessage,
		body,
		[]string{
			"Locate the owning workload manifest for the pod before opening a pull request.",
			"Apply the recommended requests and limits to the pod template containers in Git, not directly in-cluster.",
		},
	)
	return &artifact
}

func distributeContainerRecommendations(detail model.PodDetail, recommended podResources) []containerRecommendation {
	if len(detail.Containers) == 0 {
		return []containerRecommendation{{
			name:            "main",
			requestCPUMilli: recommended.requestCPUMilli,
			requestMemoryMi: recommended.requestMemoryMi,
			limitCPUMilli:   recommended.limitCPUMilli,
			limitMemoryMi:   recommended.limitMemoryMi,
		}}
	}

	requestCPUWeights := make([]int, 0, len(detail.Containers))
	requestMemoryWeights := make([]int, 0, len(detail.Containers))
	limitCPUWeights := make([]int, 0, len(detail.Containers))
	limitMemoryWeights := make([]int, 0, len(detail.Containers))
	for _, container := range detail.Containers {
		requestCPUWeights = append(requestCPUWeights, parseContainerCPU(container, true))
		requestMemoryWeights = append(requestMemoryWeights, parseContainerMemory(container, true))
		limitCPUWeights = append(limitCPUWeights, parseContainerCPU(container, false))
		limitMemoryWeights = append(limitMemoryWeights, parseContainerMemory(container, false))
	}

	requestCPUShares := distributeInt(recommended.requestCPUMilli, requestCPUWeights)
	requestMemoryShares := distributeInt(recommended.requestMemoryMi, requestMemoryWeights)
	limitCPUShares := distributeInt(recommended.limitCPUMilli, limitCPUWeights)
	limitMemoryShares := distributeInt(recommended.limitMemoryMi, limitMemoryWeights)

	out := make([]containerRecommendation, 0, len(detail.Containers))
	for index, container := range detail.Containers {
		name := strings.TrimSpace(container.Name)
		if name == "" {
			name = fmt.Sprintf("container-%d", index+1)
		}
		out = append(out, containerRecommendation{
			name:            name,
			requestCPUMilli: requestCPUShares[index],
			requestMemoryMi: requestMemoryShares[index],
			limitCPUMilli:   maxInt(limitCPUShares[index], requestCPUShares[index]),
			limitMemoryMi:   maxInt(limitMemoryShares[index], requestMemoryShares[index]),
		})
	}
	return out
}

func renderRightsizingPatch(target gitops.WorkloadTarget, containers []containerRecommendation) string {
	lines := []string{
		"apiVersion: apps/v1",
		fmt.Sprintf("kind: %s", workloadKindTitle(target.Kind)),
		"metadata:",
		fmt.Sprintf("  name: %s", target.Name),
		fmt.Sprintf("  namespace: %s", target.Namespace),
		"spec:",
		"  template:",
		"    spec:",
		"      containers:",
	}
	for _, container := range containers {
		lines = append(
			lines,
			fmt.Sprintf("        - name: %s", container.name),
			"          resources:",
			"            requests:",
			fmt.Sprintf("              cpu: %s", formatCPUMilli(container.requestCPUMilli)),
			fmt.Sprintf("              memory: %s", formatMemoryMi(container.requestMemoryMi)),
			"            limits:",
			fmt.Sprintf("              cpu: %s", formatCPUMilli(container.limitCPUMilli)),
			fmt.Sprintf("              memory: %s", formatMemoryMi(container.limitMemoryMi)),
		)
	}
	return strings.Join(lines, "\n")
}

func aggregateResources(detail model.PodDetail) podResources {
	out := podResources{}
	for _, container := range detail.Containers {
		out.requestCPUMilli += parseContainerCPU(container, true)
		out.requestMemoryMi += parseContainerMemory(container, true)
		out.limitCPUMilli += parseContainerCPU(container, false)
		out.limitMemoryMi += parseContainerMemory(container, false)
	}
	return out
}

func parseContainerCPU(container model.ContainerSpec, requests bool) int {
	if container.Resources == nil {
		return 0
	}
	if requests && container.Resources.Requests != nil {
		value, _ := parseCPUMilli(container.Resources.Requests.CPU)
		return value
	}
	if !requests && container.Resources.Limits != nil {
		value, _ := parseCPUMilli(container.Resources.Limits.CPU)
		return value
	}
	return 0
}

func parseContainerMemory(container model.ContainerSpec, requests bool) int {
	if container.Resources == nil {
		return 0
	}
	if requests && container.Resources.Requests != nil {
		value, _ := parseMemoryMi(container.Resources.Requests.Memory)
		return value
	}
	if !requests && container.Resources.Limits != nil {
		value, _ := parseMemoryMi(container.Resources.Limits.Memory)
		return value
	}
	return 0
}

func inferQoSClass(detail model.PodDetail) string {
	if len(detail.Containers) == 0 {
		return "Unknown"
	}

	allRequests := true
	allLimits := true
	guaranteed := true
	for _, container := range detail.Containers {
		reqCPU := parseContainerCPU(container, true)
		reqMemory := parseContainerMemory(container, true)
		limCPU := parseContainerCPU(container, false)
		limMemory := parseContainerMemory(container, false)
		if reqCPU == 0 || reqMemory == 0 {
			allRequests = false
		}
		if limCPU == 0 || limMemory == 0 {
			allLimits = false
		}
		if reqCPU == 0 || reqMemory == 0 || limCPU == 0 || limMemory == 0 || reqCPU != limCPU || reqMemory != limMemory {
			guaranteed = false
		}
	}
	switch {
	case guaranteed:
		return "Guaranteed"
	case allRequests || allLimits:
		return "Burstable"
	default:
		return "BestEffort"
	}
}

func summaryForStatus(status model.RightsizingStatus, recommended podResources, current podResources) string {
	switch status {
	case model.RightsizingStatusMissingGuardrail:
		return "Missing resource requests or limits. Add baseline guardrails so scheduling, autoscaling, and cost controls become predictable."
	case model.RightsizingStatusUnderprovisioned:
		return "Observed usage is pressing against current requests or limits. Increase the envelope before the workload churn turns into incident noise."
	case model.RightsizingStatusOverprovisioned:
		return fmt.Sprintf(
			"Current requests leave reclaimable headroom of %s CPU and %s memory based on live usage.",
			formatCPUMilli(maxInt(current.requestCPUMilli-recommended.requestCPUMilli, 0)),
			formatMemoryMi(maxInt(current.requestMemoryMi-recommended.requestMemoryMi, 0)),
		)
	default:
		return "Requests and limits are reasonably aligned with observed runtime usage."
	}
}

func riskForStatus(status model.RightsizingStatus) string {
	switch status {
	case model.RightsizingStatusUnderprovisioned:
		return "high"
	case model.RightsizingStatusMissingGuardrail:
		return "medium"
	default:
		return "low"
	}
}

func confidenceForRecommendation(current podResources, cpuKnown bool, memoryKnown bool, targetOK bool, containers int) int {
	confidence := 48
	if cpuKnown {
		confidence += 12
	}
	if memoryKnown {
		confidence += 12
	}
	if current.requestCPUMilli > 0 || current.requestMemoryMi > 0 {
		confidence += 8
	}
	if current.limitCPUMilli > 0 || current.limitMemoryMi > 0 {
		confidence += 8
	}
	if targetOK {
		confidence += 8
	}
	if containers == 1 {
		confidence += 4
	}
	return clampInt(confidence, 45, 95)
}

func parseCPUMilli(raw string) (int, bool) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" || value == "n/a" {
		return 0, false
	}
	if strings.HasSuffix(value, "m") {
		parsed, err := strconv.ParseFloat(strings.TrimSuffix(value, "m"), 64)
		if err != nil {
			return 0, false
		}
		return int(math.Round(parsed)), true
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return int(math.Round(parsed * 1000)), true
}

func parseMemoryMi(raw string) (int, bool) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" || value == "n/a" {
		return 0, false
	}
	switch {
	case strings.HasSuffix(value, "mi"):
		parsed, err := strconv.ParseFloat(strings.TrimSuffix(value, "mi"), 64)
		if err != nil {
			return 0, false
		}
		return int(math.Round(parsed)), true
	case strings.HasSuffix(value, "gi"):
		parsed, err := strconv.ParseFloat(strings.TrimSuffix(value, "gi"), 64)
		if err != nil {
			return 0, false
		}
		return int(math.Round(parsed * 1024)), true
	case strings.HasSuffix(value, "ki"):
		parsed, err := strconv.ParseFloat(strings.TrimSuffix(value, "ki"), 64)
		if err != nil {
			return 0, false
		}
		return int(math.Round(parsed / 1024)), true
	default:
		parsed, err := strconv.ParseFloat(strings.TrimRight(value, "b"), 64)
		if err != nil {
			return 0, false
		}
		return int(math.Round(parsed / (1024 * 1024))), true
	}
}

func distributeInt(total int, weights []int) []int {
	if len(weights) == 0 {
		return nil
	}
	if total <= 0 {
		return make([]int, len(weights))
	}

	sum := 0
	for _, weight := range weights {
		if weight > 0 {
			sum += weight
		}
	}
	if sum == 0 {
		sum = len(weights)
		weights = make([]int, len(weights))
		for index := range weights {
			weights[index] = 1
		}
	}

	out := make([]int, len(weights))
	remainder := total
	for index, weight := range weights {
		if index == len(weights)-1 {
			out[index] = remainder
			break
		}
		share := int(math.Round((float64(total) * float64(maxInt(weight, 1))) / float64(sum)))
		if share > remainder {
			share = remainder
		}
		out[index] = share
		remainder -= share
	}
	return out
}

func workloadKindTitle(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "deployment":
		return "Deployment"
	case "statefulset":
		return "StatefulSet"
	case "daemonset":
		return "DaemonSet"
	default:
		return "Deployment"
	}
}

func rightsizingRank(status model.RightsizingStatus) int {
	switch status {
	case model.RightsizingStatusUnderprovisioned:
		return 4
	case model.RightsizingStatusMissingGuardrail:
		return 3
	case model.RightsizingStatusOverprovisioned:
		return 2
	default:
		return 1
	}
}

func formatCPUMilli(value int) string {
	if value <= 0 {
		return "0m"
	}
	return fmt.Sprintf("%dm", value)
}

func formatMemoryMi(value int) string {
	if value <= 0 {
		return "0Mi"
	}
	if value%1024 == 0 {
		return fmt.Sprintf("%dGi", value/1024)
	}
	return fmt.Sprintf("%dMi", value)
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func ensurePositive(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func minPositive(current int, candidate int) int {
	if current <= 0 {
		return candidate
	}
	if candidate <= 0 {
		return current
	}
	if candidate < current {
		return candidate
	}
	return current
}
