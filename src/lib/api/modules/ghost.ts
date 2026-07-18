import type {
  GhostSimulationListResponse,
  GhostSimulationRecord,
  GhostSimulationRequest,
  GhostSimulationResult,
  GhostTopology,
} from "../../../types";
import { apiRoute, requestJson } from "../core";

export const ghostApi = {
  getGhostTopology: (signal?: AbortSignal) => requestJson<GhostTopology>(apiRoute("/ghost/topology"), { signal }),
  listGhostSimulations: (limit = 50, signal?: AbortSignal) =>
    requestJson<GhostSimulationListResponse>(`${apiRoute("/ghost/simulations")}?limit=${limit}`, { signal }),
  getGhostSimulation: (id: string, signal?: AbortSignal) =>
    requestJson<GhostSimulationRecord>(apiRoute("/ghost/simulations/{id}", { id }), { signal }),
  simulateGhostScenario: (payload: GhostSimulationRequest) =>
    requestJson<GhostSimulationResult>(apiRoute("/ghost/simulations"), {
      method: "POST",
      body: JSON.stringify(payload),
    }),
};
