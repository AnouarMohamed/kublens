package httpapi

import (
	"fmt"
	"net/http"
	"time"

	ghostengine "kubelens-backend/internal/ghost"
	"kubelens-backend/internal/model"
	"kubelens-backend/internal/remediation"
)

func (s *Server) handleGhostTopology(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.currentGhostTopology(r))
}

func (s *Server) handleGhostSimulation(w http.ResponseWriter, r *http.Request) {
	var req model.GhostSimulationRequest
	if err := s.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	normalized, err := ghostengine.NormalizeRequest(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	topology := s.currentGhostTopology(r)
	var result model.GhostSimulationResult
	var simulated bool
	if s.ghostClient != nil {
		var simErr error
		result, simErr = s.ghostClient.Simulate(r.Context(), normalized, topology)
		if simErr != nil {
			s.logger.Warn("ghost engine gRPC simulation failed, falling back to in-memory simulation", "error", simErr)
		} else {
			simulated = true
		}
	}

	if !simulated {
		result = ghostengine.SimulateNodeDrain(topology, normalized, s.now())
	}

	// Automated Remediation Proposal for favorable verdicts
	if (result.Verdict.Severity == "warning" || result.Verdict.Severity == "info") && s.remediations != nil {
		proposal := model.RemediationProposal{
			ID:        fmt.Sprintf("ghost-%d", s.now().UnixNano()),
			Kind:      model.RemediationKind(req.Action),
			Status:    "proposed",
			Resource:  req.NodeName,
			Reason:    result.Verdict.Summary,
			RiskLevel: "low", // Map simulation warning/info to low-risk proposal
			CreatedAt: s.now().Format(time.RFC3339),
		}
		s.remediations.SaveProposals([]model.RemediationProposal{proposal})
		s.logger.Info("automated remediation proposal created from ghost simulation", "id", proposal.ID, "resource", proposal.Resource)

		// Automatically generate GitOps artifact
		artifact := remediation.BuildGitOpsArtifact(proposal, collectGitOpsWorkloadInventory(r.Context(), s.cluster), s.now())
		_, err := upsertRemediationGitOpsArtifactWithContext(r.Context(), s.remediations, proposal.ID, artifact, "system/ghost-engine")
		if err != nil {
			s.logger.Error("failed to persist ghost simulation gitops artifact", "proposal_id", proposal.ID, "error", err)
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) currentGhostTopology(r *http.Request) model.GhostTopology {
	if snapshot, ok := s.cluster.StateSnapshot(r.Context()); ok {
		return ghostengine.BuildTopologyFromState(snapshot, s.now())
	}
	pods, nodes := s.cluster.Snapshot(r.Context())
	return ghostengine.BuildTopologyFromSummaries(pods, nodes, s.now())
}
