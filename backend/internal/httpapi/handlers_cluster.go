package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"kubelens-backend/internal/apperrors"
	"kubelens-backend/internal/model"
)

func (s *Server) handleClusterInfo(w http.ResponseWriter, r *http.Request) {
	if selector, ok := s.cluster.(clusterSelector); ok {
		name := selector.ClusterName(r.Context())
		if info, found := selector.ClusterInfo(name); found {
			writeJSON(w, http.StatusOK, model.ClusterInfo{IsRealCluster: info.IsRealCluster})
			return
		}
	}

	writeJSON(w, http.StatusOK, model.ClusterInfo{IsRealCluster: s.cluster.IsRealCluster()})
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	snap := s.metrics.snapshot()
	snap.RAG = ragMetricsFromRetriever(s.docs)
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleNamespaces(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cluster.ListNamespaces(r.Context()))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cluster.ListClusterEvents(r.Context()))
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.currentClusterStats(r.Context()))
}

func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	report := s.runDiagnostics(r.Context())
	writeJSON(w, http.StatusOK, s.mapDiagnosticsReport(report))
}

func countPods(pods []model.PodSummary, status model.PodStatus) int {
	count := 0
	for _, pod := range pods {
		if pod.Status == status {
			count++
		}
	}
	return count
}

func (s *Server) currentClusterStats(ctx context.Context) model.ClusterStats {
	pods, nodes := s.cluster.Snapshot(ctx)
	return model.ClusterStats{
		Pods: model.PodStats{
			Total:   len(pods),
			Running: countPods(pods, model.PodStatusRunning),
			Pending: countPods(pods, model.PodStatusPending),
			Failed:  countPods(pods, model.PodStatusFailed),
		},
		Nodes: model.NodeStats{
			Total:    len(nodes),
			Ready:    countNodes(nodes, model.NodeStatusReady),
			NotReady: countNodesNotReady(nodes),
		},
		Cluster: clusterCapacityFromNodes(nodes, s.cluster.IsRealCluster()),
	}
}

func countNodes(nodes []model.NodeSummary, status model.NodeStatus) int {
	count := 0
	for _, node := range nodes {
		if node.Status == status {
			count++
		}
	}
	return count
}

func countNodesNotReady(nodes []model.NodeSummary) int {
	count := 0
	for _, node := range nodes {
		if node.Status != model.NodeStatusReady {
			count++
		}
	}
	return count
}

func clusterCapacityFromNodes(nodes []model.NodeSummary, isRealCluster bool) model.ClusterCapacity {
	if !isRealCluster {
		return model.ClusterCapacity{
			CPU:     "34%",
			Memory:  "58%",
			Storage: "22%",
		}
	}

	cpu, hasCPU := averageNodeUsage(nodes, func(node model.NodeSummary) string { return node.CPUUsage })
	memory, hasMemory := averageNodeUsage(nodes, func(node model.NodeSummary) string { return node.MemUsage })

	cpuValue := "N/A"
	if hasCPU {
		cpuValue = formatPercent(cpu)
	}

	memoryValue := "N/A"
	if hasMemory {
		memoryValue = formatPercent(memory)
	}

	return model.ClusterCapacity{
		CPU:     cpuValue,
		Memory:  memoryValue,
		Storage: "N/A",
	}
}

func averageNodeUsage(nodes []model.NodeSummary, read func(model.NodeSummary) string) (float64, bool) {
	var (
		total float64
		count float64
	)

	for _, node := range nodes {
		value, ok := parsePercent(read(node))
		if !ok {
			continue
		}
		total += value
		count++
	}

	if count == 0 {
		return 0, false
	}

	return total / count, true
}

func parsePercent(raw string) (float64, bool) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(raw, "%"))
	if trimmed == "" {
		return 0, false
	}

	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, false
	}
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}

	return value, true
}

func formatPercent(value float64) string {
	return strconv.Itoa(int(value+0.5)) + "%"
}

func parsePositiveIntWithMax(raw string, fallback int, max int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return fallback
	}
	if max > 0 && value > max {
		return max
	}
	return value
}

func handleActionError(w http.ResponseWriter, err error, notFoundMessage string) {
	if errors.Is(err, apperrors.ErrNotFound) {
		writeError(w, http.StatusNotFound, notFoundMessage)
		return
	}

	writeError(w, http.StatusBadRequest, err.Error())
}
