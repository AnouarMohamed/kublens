package ghost

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"kubelens-backend/internal/model"
)

const (
	ActionNodeDrain       = "node_drain"
	defaultHorizonSeconds = 900
	maxHorizonSeconds     = 1800
)

func NormalizeRequest(req model.GhostSimulationRequest) (model.GhostSimulationRequest, error) {
	req.Action = strings.TrimSpace(req.Action)
	if req.Action == "" {
		req.Action = ActionNodeDrain
	}
	if req.Action != ActionNodeDrain {
		return req, fmt.Errorf("unsupported ghost simulation action: %s", req.Action)
	}
	req.NodeName = strings.TrimSpace(req.NodeName)
	if req.NodeName == "" {
		return req, fmt.Errorf("nodeName is required")
	}
	if req.HorizonSeconds <= 0 {
		req.HorizonSeconds = defaultHorizonSeconds
	}
	if req.HorizonSeconds > maxHorizonSeconds {
		req.HorizonSeconds = maxHorizonSeconds
	}
	return req, nil
}

func SimulateNodeDrain(topology model.GhostTopology, req model.GhostSimulationRequest, now time.Time) model.GhostSimulationResult {
	req, _ = NormalizeRequest(req)
	initialTopology := topology
	nodeByName := make(map[string]model.GhostNode, len(topology.Nodes))
	pods := append([]model.GhostPod(nil), topology.Pods...)
	sort.Slice(pods, func(i, j int) bool { return pods[i].ID < pods[j].ID })
	for _, node := range topology.Nodes {
		nodeByName[node.Name] = node
	}

	events := []model.GhostTimelineEvent{}
	target, found := nodeByName[req.NodeName]
	if !found {
		return simulationResult(initialTopology, topology, req, now, "critical", "Target node was not found in the topology.", []string{
			"Refresh cluster state and rerun the simulation.",
		}, []model.GhostTimelineEvent{{
			Kind:     "blocked",
			Severity: "critical",
			Resource: req.NodeName,
			Message:  "Target node does not exist.",
		}})
	}
	target.Unschedulable = true
	nodeByName[req.NodeName] = target
	events = append(events, model.GhostTimelineEvent{
		Kind:     "node_cordoned",
		Severity: "info",
		Resource: req.NodeName,
		Message:  fmt.Sprintf("Simulation marks %s unschedulable before eviction.", req.NodeName),
	})

	unresolved := 0
	moved := 0
	for index := range pods {
		pod := pods[index]
		if pod.NodeName != req.NodeName {
			continue
		}
		destination, ok := chooseDestination(nodeByName, pod, req.NodeName)
		if !ok {
			pods[index].Status = "Pending"
			pods[index].NodeName = ""
			unresolved++
			events = append(events, model.GhostTimelineEvent{
				Kind:     "pod_pending",
				Severity: "critical",
				Resource: pod.ID,
				Message:  fmt.Sprintf("%s cannot be placed after draining %s.", pod.ID, req.NodeName),
			})
			continue
		}
		node := nodeByName[destination]
		node.Headroom.CPUMilli -= pod.Requests.CPUMilli
		node.Headroom.MemoryBytes -= pod.Requests.MemoryBytes
		nodeByName[destination] = node
		pods[index].NodeName = destination
		moved++
		events = append(events, model.GhostTimelineEvent{
			Kind:     "pod_rescheduled",
			Severity: "info",
			Resource: pod.ID,
			Message:  fmt.Sprintf("%s moves from %s to %s.", pod.ID, req.NodeName, destination),
		})
	}

	severity := "info"
	summary := fmt.Sprintf("Drain simulation can move %d pod(s) from %s.", moved, req.NodeName)
	recommendations := []string{"Review the simulated placements before running the real drain."}
	if unresolved > 0 {
		severity = "critical"
		summary = fmt.Sprintf("Drain simulation leaves %d pod(s) pending.", unresolved)
		recommendations = []string{
			"Add capacity or relax scheduling constraints before draining this node.",
			"Run a live drain preview to compare Kubernetes eviction blockers.",
		}
	} else if moved > 0 {
		severity = "warning"
		recommendations = append(recommendations, "Watch destination node headroom during the maintenance window.")
	}

	resultTopology := topology
	resultTopology.Nodes = nodesFromMap(nodeByName)
	resultTopology.Pods = pods
	return simulationResult(initialTopology, resultTopology, req, now, severity, summary, recommendations, events)
}

func chooseDestination(nodes map[string]model.GhostNode, pod model.GhostPod, sourceNode string) (string, bool) {
	candidates := nodesFromMap(nodes)
	for _, node := range candidates {
		if node.Name == sourceNode || node.Unschedulable || !strings.EqualFold(node.Status, "Ready") {
			continue
		}
		if !nodeSelectorMatches(pod.NodeSelector, node.Labels) {
			continue
		}
		if !toleratesNodeTaints(pod.Tolerations, node.Taints) {
			continue
		}
		if node.Headroom.CPUMilli < pod.Requests.CPUMilli || node.Headroom.MemoryBytes < pod.Requests.MemoryBytes {
			continue
		}
		return node.Name, true
	}
	return "", false
}

func nodeSelectorMatches(selector map[string]string, labels map[string]string) bool {
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func toleratesNodeTaints(tolerations []string, taints []string) bool {
	if len(taints) == 0 {
		return true
	}
	tolerated := make(map[string]struct{}, len(tolerations))
	for _, toleration := range tolerations {
		key := taintKey(toleration)
		if key != "" {
			tolerated[key] = struct{}{}
		}
	}
	for _, taint := range taints {
		key := taintKey(taint)
		if key == "" {
			continue
		}
		if _, ok := tolerated[key]; !ok {
			return false
		}
	}
	return true
}

func taintKey(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return ""
	}
	if index := strings.IndexAny(raw, "=:"); index >= 0 {
		return strings.TrimSpace(raw[:index])
	}
	return raw
}

func simulationResult(
	initialTopology model.GhostTopology,
	resultTopology model.GhostTopology,
	req model.GhostSimulationRequest,
	now time.Time,
	severity string,
	summary string,
	recommendations []string,
	events []model.GhostTimelineEvent,
) model.GhostSimulationResult {
	generatedAt := now.UTC().Format(time.RFC3339)
	for index := range events {
		events[index].Timestamp = generatedAt
	}
	return model.GhostSimulationResult{
		ID:             simulationID(req, generatedAt),
		Action:         req.Action,
		GeneratedAt:    generatedAt,
		HorizonSeconds: req.HorizonSeconds,
		Verdict: model.GhostSimulationVerdict{
			Severity:        severity,
			Summary:         summary,
			Recommendations: recommendations,
		},
		Frames: []model.GhostTimelineFrame{
			frameFromTopology(0, initialTopology, nil),
			frameFromTopology(req.HorizonSeconds, resultTopology, events),
		},
	}
}

func frameFromTopology(offset int, topology model.GhostTopology, events []model.GhostTimelineEvent) model.GhostTimelineFrame {
	return model.GhostTimelineFrame{
		OffsetSeconds: offset,
		Nodes:         frameNodes(topology.Nodes),
		Pods:          framePods(topology.Pods),
		Events:        append([]model.GhostTimelineEvent(nil), events...),
	}
}

func frameNodes(nodes []model.GhostNode) []model.GhostFrameNode {
	out := make([]model.GhostFrameNode, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, model.GhostFrameNode{
			Name:          node.Name,
			Status:        node.Status,
			Unschedulable: node.Unschedulable,
			Headroom:      node.Headroom,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func framePods(pods []model.GhostPod) []model.GhostFramePod {
	out := make([]model.GhostFramePod, 0, len(pods))
	for _, pod := range pods {
		out = append(out, model.GhostFramePod{
			ID:        pod.ID,
			Namespace: pod.Namespace,
			Name:      pod.Name,
			NodeName:  pod.NodeName,
			Status:    pod.Status,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func nodesFromMap(nodes map[string]model.GhostNode) []model.GhostNode {
	out := make([]model.GhostNode, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func simulationID(req model.GhostSimulationRequest, generatedAt string) string {
	hash := sha1.Sum([]byte(req.Action + "|" + req.NodeName + "|" + generatedAt))
	return "ghost-" + hex.EncodeToString(hash[:])[:12]
}
