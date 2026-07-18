package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	ghostengine "kubelens-backend/internal/ghost"
	"kubelens-backend/internal/model"
	"kubelens-backend/internal/remediation"
)

type GhostController struct {
	cluster        ClusterReader
	ghostClient    ghostClient
	runs           ghostSimulationStore
	remediations   remediationStore
	logger         *slog.Logger
	now            func() time.Time
	decodeJSONBody func(*http.Request, any) error
}

const defaultGhostSimulationLimit = 200

type ghostSimulationStore interface {
	Save(record model.GhostSimulationRecord) model.GhostSimulationRecord
	List(limit int) []model.GhostSimulationRecord
	Get(id string) (model.GhostSimulationRecord, bool)
}

type memoryGhostSimulationStore struct {
	mu       sync.RWMutex
	maxItems int
	items    []model.GhostSimulationRecord
}

func newMemoryGhostSimulationStore(maxItems int) *memoryGhostSimulationStore {
	if maxItems <= 0 {
		maxItems = defaultGhostSimulationLimit
	}
	return &memoryGhostSimulationStore{
		maxItems: maxItems,
		items:    make([]model.GhostSimulationRecord, 0, maxItems),
	}
}

func NewGhostController(
	cluster ClusterReader,
	ghostClient ghostClient,
	runs ghostSimulationStore,
	remediations remediationStore,
	logger *slog.Logger,
	now func() time.Time,
	decode func(*http.Request, any) error,
) *GhostController {
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	if decode == nil {
		decode = decodeJSONBody
	}
	return &GhostController{
		cluster:        cluster,
		ghostClient:    ghostClient,
		runs:           runs,
		remediations:   remediations,
		logger:         logger,
		now:            now,
		decodeJSONBody: decode,
	}
}

func (gc *GhostController) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/topology", gc.handleGhostTopology)
	r.Get("/simulations", gc.handleGhostSimulationList)
	r.Post("/simulations", gc.handleGhostSimulation)
	r.Get("/simulations/{id}", gc.handleGhostSimulationGet)
	return r
}

func (gc *GhostController) handleGhostTopology(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, gc.currentGhostTopology(r))
}

func (gc *GhostController) handleGhostSimulation(w http.ResponseWriter, r *http.Request) {
	var req model.GhostSimulationRequest
	if err := gc.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	normalized, err := ghostengine.NormalizeRequest(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	topology := gc.currentGhostTopology(r)
	var result model.GhostSimulationResult
	var simulated bool
	if gc.ghostClient != nil {
		var simErr error
		result, simErr = gc.ghostClient.Simulate(r.Context(), normalized, topology)
		if simErr != nil {
			gc.logger.Warn("ghost engine gRPC simulation failed, falling back to in-memory simulation", "error", simErr)
		} else {
			simulated = true
			result.Engine = "grpc"
		}
	}

	if !simulated {
		result = ghostengine.SimulateNodeDrain(topology, normalized, gc.now())
		result.Engine = "in-memory"
	}
	result = annotateGhostSimulationResult(result, normalized, topology)
	if gc.runs != nil {
		record := model.GhostSimulationRecord{
			ID:           result.ID,
			CreatedAt:    result.GeneratedAt,
			Request:      normalized,
			TopologyHash: result.TopologyHash,
			Result:       result,
		}
		gc.runs.Save(record)
	}

	// Automated Remediation Proposal for favorable verdicts
	if (result.Verdict.Severity == "warning" || result.Verdict.Severity == "info") && gc.remediations != nil {
		proposal := model.RemediationProposal{
			ID:        fmt.Sprintf("ghost-%d", gc.now().UnixNano()),
			Kind:      model.RemediationKind(req.Action),
			Status:    "proposed",
			Resource:  req.NodeName,
			Reason:    result.Verdict.Summary,
			RiskLevel: "low", // Map simulation warning/info to low-risk proposal
			CreatedAt: gc.now().Format(time.RFC3339),
		}
		gc.remediations.SaveProposals([]model.RemediationProposal{proposal})
		gc.logger.Info("automated remediation proposal created from ghost simulation", "id", proposal.ID, "resource", proposal.Resource)

		// Automatically generate GitOps artifact
		artifact := remediation.BuildGitOpsArtifact(proposal, collectGitOpsWorkloadInventory(r.Context(), gc.cluster), gc.now())
		_, err := upsertRemediationGitOpsArtifactWithContext(r.Context(), gc.remediations, proposal.ID, artifact, "system/ghost-engine")
		if err != nil {
			gc.logger.Error("failed to persist ghost simulation gitops artifact", "proposal_id", proposal.ID, "error", err)
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (gc *GhostController) handleGhostSimulationList(w http.ResponseWriter, r *http.Request) {
	if gc.runs == nil {
		writeJSON(w, http.StatusOK, model.GhostSimulationListResponse{})
		return
	}
	limit := parsePositiveInt(r.URL.Query().Get("limit"), 50)
	items := gc.runs.List(limit)
	writeJSON(w, http.StatusOK, model.GhostSimulationListResponse{
		Total: len(items),
		Items: items,
	})
}

func (gc *GhostController) handleGhostSimulationGet(w http.ResponseWriter, r *http.Request) {
	if gc.runs == nil {
		writeError(w, http.StatusNotFound, "ghost simulation store unavailable")
		return
	}
	record, ok := gc.runs.Get(chi.URLParam(r, "id"))
	if !ok {
		writeError(w, http.StatusNotFound, "ghost simulation not found")
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (gc *GhostController) currentGhostTopology(r *http.Request) model.GhostTopology {
	if snapshot, ok := gc.cluster.StateSnapshot(r.Context()); ok {
		return ghostengine.BuildTopologyFromState(snapshot, gc.now())
	}
	pods, nodes := gc.cluster.Snapshot(r.Context())
	return ghostengine.BuildTopologyFromSummaries(pods, nodes, gc.now())
}

func (s *memoryGhostSimulationStore) Save(record model.GhostSimulationRecord) model.GhostSimulationRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = append(s.items, record)
	if overflow := len(s.items) - s.maxItems; overflow > 0 {
		s.items = append([]model.GhostSimulationRecord(nil), s.items[overflow:]...)
	}
	return record
}

func (s *memoryGhostSimulationStore) List(limit int) []model.GhostSimulationRecord {
	if limit <= 0 {
		limit = 50
	}
	if limit > s.maxItems {
		limit = s.maxItems
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	count := minInt(limit, len(s.items))
	out := make([]model.GhostSimulationRecord, 0, count)
	for i := len(s.items) - 1; i >= 0 && len(out) < count; i-- {
		out = append(out, s.items[i])
	}
	return out
}

func (s *memoryGhostSimulationStore) Get(id string) (model.GhostSimulationRecord, bool) {
	target := strings.TrimSpace(id)
	if target == "" {
		return model.GhostSimulationRecord{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := len(s.items) - 1; i >= 0; i-- {
		if s.items[i].ID == target {
			return s.items[i], true
		}
	}
	return model.GhostSimulationRecord{}, false
}

func annotateGhostSimulationResult(
	result model.GhostSimulationResult,
	req model.GhostSimulationRequest,
	topology model.GhostTopology,
) model.GhostSimulationResult {
	result.TopologyHash = ghostTopologyHash(topology)
	if strings.TrimSpace(result.Engine) == "" {
		result.Engine = "unknown"
	}
	result.Confidence = ghostSimulationConfidence(result, topology)
	result.Limitations = ghostSimulationLimitations(req, topology)
	return result
}

func ghostTopologyHash(topology model.GhostTopology) string {
	bytes, _ := json.Marshal(topology)
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func ghostSimulationConfidence(result model.GhostSimulationResult, topology model.GhostTopology) int {
	confidence := 72
	if topology.Source == "summary-fallback" {
		confidence = 52
	}
	if result.Engine == "grpc" {
		confidence += 8
	}
	if result.Verdict.Severity == "critical" {
		confidence -= 5
	}
	if confidence < 0 {
		return 0
	}
	if confidence > 100 {
		return 100
	}
	return confidence
}

func ghostSimulationLimitations(req model.GhostSimulationRequest, topology model.GhostTopology) []string {
	limitations := []string{
		"Scheduler model covers node readiness, unschedulability, node selectors, and requested CPU/memory headroom.",
		"PDB, priority/preemption, storage attachment, topology spread, and full kube-scheduler plugin parity are not complete.",
		"Network, disk, and cascade-failure propagation require the later eBPF and predictor bridge phases.",
	}
	if topology.Source == "summary-fallback" {
		limitations = append(limitations, "Topology came from summary fallback data, so service/ingress dependency confidence is reduced.")
	}
	if req.Action == ghostengine.ActionNodeDrain {
		limitations = append(limitations, "This result is a dry-run simulation and does not cordon, drain, or mutate the cluster.")
	}
	return limitations
}
