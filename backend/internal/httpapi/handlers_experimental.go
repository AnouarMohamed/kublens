package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"kubelens-backend/internal/model"
)

const experimentalMaturity = "experimental"

func (s *Server) handleExperimentalStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, model.ExperimentalStatus{
		GeneratedAt: s.now().UTC().Format(time.RFC3339),
		Features: []model.ExperimentalFeatureStatus{
			s.experimentalFeatureStatus(
				"ebpf-node-telemetry",
				s.experimental.EBPFTelemetryEnabled,
				"Deep node telemetry ingestion is disabled.",
				"Deep node telemetry ingestion is enabled in compatibility mode.",
				ebpfLimitations(),
			),
			s.experimentalFeatureStatus(
				"fleet-drift-detection",
				s.experimental.FleetDriftEnabled,
				"Fleet drift detection is disabled.",
				"Fleet drift detection is enabled for configured cluster contexts.",
				fleetDriftLimitations(),
			),
			s.experimentalFeatureStatus(
				"autonomous-remediation-proposals",
				s.experimental.AutonomousRemediationEnabled,
				"Autonomous remediation proposal generation is disabled.",
				"Autonomous remediation proposal generation is enabled behind policy gates.",
				autonomousRemediationLimitations(),
			),
		},
	})
}

func (s *Server) handleExperimentalNodeTelemetry(w http.ResponseWriter, r *http.Request) {
	if !s.experimental.EBPFTelemetryEnabled {
		writeJSON(w, http.StatusOK, model.NodeTelemetryReport{
			GeneratedAt:    s.now().UTC().Format(time.RFC3339),
			Enabled:        false,
			Experimental:   true,
			Source:         "disabled",
			AgentConnected: false,
			Summary:        "eBPF node telemetry is disabled.",
			Nodes:          []model.NodeTelemetryItem{},
			Limitations:    ebpfLimitations(),
		})
		return
	}

	pods, nodes := s.cluster.Snapshot(r.Context())
	events := s.cluster.ListClusterEvents(r.Context())
	podsByNode := map[string]int{}
	for _, pod := range pods {
		if strings.TrimSpace(pod.NodeName) != "" {
			podsByNode[pod.NodeName]++
		}
	}

	items := make([]model.NodeTelemetryItem, 0, len(nodes))
	for _, node := range nodes {
		warnings := countResourceWarningEvents(events, node.Name, "")
		signals := nodePressureSignals(node, warnings)
		items = append(items, model.NodeTelemetryItem{
			Node:             node.Name,
			Status:           string(node.Status),
			CPUUsage:         node.CPUUsage,
			MemoryUsage:      node.MemUsage,
			WarningEvents:    warnings,
			PressureSignals:  signals,
			ObservedWorkload: podsByNode[node.Name],
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if len(items[i].PressureSignals) == len(items[j].PressureSignals) {
			return items[i].Node < items[j].Node
		}
		return len(items[i].PressureSignals) > len(items[j].PressureSignals)
	})

	writeJSON(w, http.StatusOK, model.NodeTelemetryReport{
		GeneratedAt:    s.now().UTC().Format(time.RFC3339),
		Enabled:        true,
		Experimental:   true,
		Source:         "kubernetes-compatibility",
		AgentConnected: false,
		Summary:        fmt.Sprintf("Compatibility telemetry summarized %d node(s).", len(items)),
		Nodes:          items,
		Limitations:    ebpfLimitations(),
	})
}

func (s *Server) handleExperimentalFleetDrift(w http.ResponseWriter, _ *http.Request) {
	if !s.experimental.FleetDriftEnabled {
		writeJSON(w, http.StatusOK, model.FleetDriftReport{
			GeneratedAt:  s.now().UTC().Format(time.RFC3339),
			Enabled:      false,
			Experimental: true,
			Baseline:     "",
			Compared:     0,
			Items:        []model.FleetDriftItem{},
			Limitations:  fleetDriftLimitations(),
		})
		return
	}

	selector, ok := s.cluster.(*routedCluster)
	if !ok || len(selector.readers) < 2 {
		writeJSON(w, http.StatusOK, model.FleetDriftReport{
			GeneratedAt:  s.now().UTC().Format(time.RFC3339),
			Enabled:      true,
			Experimental: true,
			Baseline:     "default",
			Compared:     0,
			Items: []model.FleetDriftItem{{
				Cluster:  "default",
				Severity: "info",
				Summary:  "Only one cluster context is configured.",
				Signals:  []string{"multi-cluster-context-required"},
			}},
			Limitations: fleetDriftLimitations(),
		})
		return
	}

	baselineName := selector.DefaultName()
	baselineReader := selector.readers[baselineName]
	baseline := captureFleetSnapshot(baselineReader)
	items := make([]model.FleetDriftItem, 0, len(selector.readers)-1)
	for _, name := range selector.Names() {
		if name == baselineName {
			continue
		}
		items = append(items, compareFleetSnapshot(name, baseline, captureFleetSnapshot(selector.readers[name])))
	}

	writeJSON(w, http.StatusOK, model.FleetDriftReport{
		GeneratedAt:  s.now().UTC().Format(time.RFC3339),
		Enabled:      true,
		Experimental: true,
		Baseline:     baselineName,
		Compared:     len(items),
		Items:        items,
		Limitations:  fleetDriftLimitations(),
	})
}

func (s *Server) handleAutonomousRemediationPropose(w http.ResponseWriter, r *http.Request) {
	policy := s.autonomousRemediationPolicy()
	report := model.AutonomousRemediationReport{
		GeneratedAt:  s.now().UTC().Format(time.RFC3339),
		Enabled:      s.experimental.AutonomousRemediationEnabled,
		Experimental: true,
		Policy:       policy,
		Proposals:    []model.RemediationProposal{},
		Limitations:  autonomousRemediationLimitations(),
	}
	if len(policy.BlockedReasons) > 0 {
		writeJSON(w, http.StatusOK, report)
		return
	}

	pods, nodes := s.cluster.Snapshot(r.Context())
	events := s.cluster.ListClusterEvents(r.Context())
	predictions := buildLocalPredictions(pods, nodes, events, s.now())
	candidates := make([]model.RemediationProposal, 0, policy.MaxProposals)
	for _, prediction := range predictions.Items {
		if prediction.RiskScore < policy.MinRiskScore {
			continue
		}
		proposal, ok := autonomousProposalFromPrediction(prediction, s.now, len(candidates)+1)
		if !ok {
			continue
		}
		candidates = append(candidates, proposal)
		if len(candidates) >= policy.MaxProposals {
			break
		}
	}
	if len(candidates) == 0 {
		writeJSON(w, http.StatusOK, report)
		return
	}

	saved := s.remediations.SaveProposals(candidates)
	report.Proposals = saved
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) experimentalFeatureStatus(
	name string,
	enabled bool,
	disabledMessage string,
	enabledMessage string,
	limitations []string,
) model.ExperimentalFeatureStatus {
	message := disabledMessage
	if enabled {
		message = enabledMessage
	}
	return model.ExperimentalFeatureStatus{
		Name:         name,
		Enabled:      enabled,
		Experimental: true,
		Maturity:     experimentalMaturity,
		Message:      message,
		Limitations:  limitations,
	}
}

func (s *Server) autonomousRemediationPolicy() model.AutonomousRemediationPolicy {
	minScore := s.experimental.AutonomousRemediationMinScore
	if minScore <= 0 {
		minScore = 85
	}
	maxItems := s.experimental.AutonomousRemediationMaxItems
	if maxItems <= 0 {
		maxItems = 5
	}
	blocked := make([]string, 0, 3)
	if !s.experimental.AutonomousRemediationEnabled {
		blocked = append(blocked, "feature-disabled")
	}
	if !s.writesOn {
		blocked = append(blocked, "write-gate-disabled")
	}
	if s.remediations == nil {
		blocked = append(blocked, "remediation-store-unavailable")
	}
	return model.AutonomousRemediationPolicy{
		Enabled:             s.experimental.AutonomousRemediationEnabled,
		Experimental:        true,
		MinRiskScore:        minScore,
		MaxProposals:        maxItems,
		RequiresWriteGate:   true,
		RequiresHumanReview: true,
		BlockedReasons:      blocked,
	}
}

type fleetSnapshot struct {
	podCount      int
	nodeCount     int
	namespaces    map[string]struct{}
	notReadyNodes int
}

func captureFleetSnapshot(reader ClusterReader) fleetSnapshot {
	out := fleetSnapshot{namespaces: map[string]struct{}{}}
	if reader == nil {
		return out
	}
	ctx := context.Background()
	pods, nodes := reader.Snapshot(ctx)
	out.podCount = len(pods)
	out.nodeCount = len(nodes)
	for _, node := range nodes {
		if node.Status != model.NodeStatusReady {
			out.notReadyNodes++
		}
	}
	for _, namespace := range reader.ListNamespaces(ctx) {
		trimmed := strings.TrimSpace(namespace)
		if trimmed != "" {
			out.namespaces[trimmed] = struct{}{}
		}
	}
	return out
}

func compareFleetSnapshot(name string, baseline fleetSnapshot, current fleetSnapshot) model.FleetDriftItem {
	signals := make([]string, 0, 4)
	if current.nodeCount != baseline.nodeCount {
		signals = append(signals, fmt.Sprintf("node-count %d vs baseline %d", current.nodeCount, baseline.nodeCount))
	}
	if current.podCount != baseline.podCount {
		signals = append(signals, fmt.Sprintf("pod-count %d vs baseline %d", current.podCount, baseline.podCount))
	}
	missingNamespaces := missingSetMembers(baseline.namespaces, current.namespaces)
	extraNamespaces := missingSetMembers(current.namespaces, baseline.namespaces)
	if len(missingNamespaces) > 0 {
		signals = append(signals, "missing-namespaces "+strings.Join(missingNamespaces, ","))
	}
	if len(extraNamespaces) > 0 {
		signals = append(signals, "extra-namespaces "+strings.Join(extraNamespaces, ","))
	}
	if current.notReadyNodes != baseline.notReadyNodes {
		signals = append(signals, fmt.Sprintf("not-ready-nodes %d vs baseline %d", current.notReadyNodes, baseline.notReadyNodes))
	}
	severity := "info"
	summary := "No fleet drift detected against baseline."
	if len(signals) > 0 {
		severity = "warning"
		summary = fmt.Sprintf("%d drift signal(s) detected against baseline.", len(signals))
	}
	return model.FleetDriftItem{
		Cluster:  name,
		Severity: severity,
		Summary:  summary,
		Signals:  signals,
	}
}

func missingSetMembers(left map[string]struct{}, right map[string]struct{}) []string {
	out := make([]string, 0)
	for value := range left {
		if _, ok := right[value]; !ok {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func nodePressureSignals(node model.NodeSummary, warnings int) []string {
	signals := make([]string, 0, 4)
	if node.Status != model.NodeStatusReady {
		signals = append(signals, "node-not-ready")
	}
	if cpu, ok := parsePercent(node.CPUUsage); ok && cpu >= 90 {
		signals = append(signals, "cpu-pressure")
	}
	if memory, ok := parsePercent(node.MemUsage); ok && memory >= 90 {
		signals = append(signals, "memory-pressure")
	}
	if warnings > 0 {
		signals = append(signals, "warning-events")
	}
	return signals
}

func autonomousProposalFromPrediction(
	prediction model.IncidentPrediction,
	now func() time.Time,
	sequence int,
) (model.RemediationProposal, bool) {
	kind := model.RemediationKind("")
	switch strings.ToLower(prediction.ResourceKind) {
	case "pod":
		if !predictionHasSignal(prediction, "status", "Failed") && !predictionHasKey(prediction, "restarts") {
			return model.RemediationProposal{}, false
		}
		kind = model.RemediationKindRestartPod
	case "node":
		if !predictionHasSignal(prediction, "status", "NotReady") {
			return model.RemediationProposal{}, false
		}
		kind = model.RemediationKindCordonNode
	default:
		return model.RemediationProposal{}, false
	}

	nowAt := now().UTC().Format(time.RFC3339)
	return model.RemediationProposal{
		ID:           fmt.Sprintf("auto-%d-%d", now().UTC().UnixNano(), sequence),
		Kind:         kind,
		Status:       "proposed",
		Namespace:    prediction.Namespace,
		Resource:     prediction.Resource,
		Reason:       prediction.Summary,
		RiskLevel:    riskLevelFromScore(prediction.RiskScore),
		DryRunResult: "proposal-only: requires human approval and GitOps review before execution",
		CreatedAt:    nowAt,
		UpdatedAt:    nowAt,
	}, true
}

func predictionHasKey(prediction model.IncidentPrediction, key string) bool {
	for _, signal := range prediction.Signals {
		if strings.EqualFold(signal.Key, key) {
			return true
		}
	}
	return false
}

func predictionHasSignal(prediction model.IncidentPrediction, key string, value string) bool {
	for _, signal := range prediction.Signals {
		if strings.EqualFold(signal.Key, key) && strings.EqualFold(signal.Value, value) {
			return true
		}
	}
	return false
}

func riskLevelFromScore(score int) string {
	switch {
	case score >= 90:
		return "critical"
	case score >= 75:
		return "high"
	case score >= 55:
		return "medium"
	default:
		return "low"
	}
}

func ebpfLimitations() []string {
	return []string{
		"Disabled by default and marked experimental.",
		"Kernel eBPF agent ingestion is not treated as production-ready until deployment, rollback, and privacy runbooks are complete.",
		"Compatibility reports use Kubernetes summary and event signals when no node agent is connected.",
	}
}

func fleetDriftLimitations() []string {
	return []string{
		"Disabled by default and marked experimental.",
		"Drift comparison is limited to configured cluster contexts and high-level inventory signals.",
		"Correction workflows remain proposal-only until rollback and ownership policies are complete.",
	}
}

func autonomousRemediationLimitations() []string {
	return []string{
		"Disabled by default and marked experimental.",
		"Generated actions are proposals only and require human approval before execution.",
		"The policy gate requires write actions to be enabled before proposal records are persisted.",
	}
}
