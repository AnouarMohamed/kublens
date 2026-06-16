package httpapi

import (
	"net/http"

	ghostengine "kubelens-backend/internal/ghost"
	"kubelens-backend/internal/model"
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
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) currentGhostTopology(r *http.Request) model.GhostTopology {
	if snapshot, ok := s.cluster.StateSnapshot(r.Context()); ok {
		return ghostengine.BuildTopologyFromState(snapshot, s.now())
	}
	pods, nodes := s.cluster.Snapshot(r.Context())
	return ghostengine.BuildTopologyFromSummaries(pods, nodes, s.now())
}
