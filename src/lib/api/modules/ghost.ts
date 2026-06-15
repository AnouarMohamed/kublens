import type { GhostSimulationRequest, GhostSimulationResult, GhostTopology } from "../../../types";
import { apiRoute, requestJson } from "../core";

export const ghostApi = {
  getGhostTopology: (signal?: AbortSignal) => requestJson<GhostTopology>(apiRoute("/ghost/topology"), { signal }),
  simulateGhostScenario: (payload: GhostSimulationRequest) =>
    requestJson<GhostSimulationResult>(apiRoute("/ghost/simulations"), {
      method: "POST",
      body: JSON.stringify(payload),
    }),
};
