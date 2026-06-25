package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	ghostengine "kubelens-backend/internal/ghost"
	"kubelens-backend/internal/model"
	"kubelens-backend/internal/remediation"
)

type GhostController struct {
	cluster        ClusterReader
	ghostClient    ghostClient
	remediations   remediationStore
	logger         *slog.Logger
	now            func() time.Time
	decodeJSONBody func(*http.Request, any) error
}

func NewGhostController(
	cluster ClusterReader,
	ghostClient ghostClient,
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
		remediations:   remediations,
		logger:         logger,
		now:            now,
		decodeJSONBody: decode,
	}
}

func (gc *GhostController) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/topology", gc.handleGhostTopology)
	r.Post("/simulations", gc.handleGhostSimulation)
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
		}
	}

	if !simulated {
		result = ghostengine.SimulateNodeDrain(topology, normalized, gc.now())
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

func (gc *GhostController) currentGhostTopology(r *http.Request) model.GhostTopology {
	if snapshot, ok := gc.cluster.StateSnapshot(r.Context()); ok {
		return ghostengine.BuildTopologyFromState(snapshot, gc.now())
	}
	pods, nodes := gc.cluster.Snapshot(r.Context())
	return ghostengine.BuildTopologyFromSummaries(pods, nodes, gc.now())
}
