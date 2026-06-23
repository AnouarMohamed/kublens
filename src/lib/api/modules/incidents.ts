import type {
  Incident,
  IncidentEvidenceBundle,
  IncidentReplay,
  IncidentStepStatusPatch,
  Postmortem,
} from "../../../types";
import { apiRoute, requestJson } from "../core";

export const incidentsApi = {
  createIncident: () =>
    requestJson<Incident>(apiRoute("/incidents"), {
      method: "POST",
      body: JSON.stringify({}),
    }),
  listIncidents: (signal?: AbortSignal) => requestJson<Incident[]>(apiRoute("/incidents"), { signal }),
  getIncident: (id: string) => requestJson<Incident>(apiRoute("/incidents/{id}", { id })),
  getIncidentReplay: (id: string) => requestJson<IncidentReplay>(apiRoute("/incidents/{id}/replay", { id })),
  getIncidentEvidence: (id: string) =>
    requestJson<IncidentEvidenceBundle>(apiRoute("/incidents/{id}/evidence", { id })),
  updateIncidentStep: (id: string, stepID: string, payload: IncidentStepStatusPatch) =>
    requestJson<Incident>(apiRoute("/incidents/{id}/steps/{step}", { id, step: stepID }), {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),
  resolveIncident: (id: string) =>
    requestJson<Incident>(apiRoute("/incidents/{id}/resolve", { id }), {
      method: "POST",
      body: JSON.stringify({}),
    }),
  generatePostmortem: (incidentID: string) =>
    requestJson<Postmortem>(apiRoute("/incidents/{id}/postmortem", { id: incidentID }), {
      method: "POST",
      body: JSON.stringify({}),
    }),
  listPostmortems: () => requestJson<Postmortem[]>(apiRoute("/postmortems")),
  getPostmortem: (id: string) => requestJson<Postmortem>(apiRoute("/postmortems/{id}", { id })),
};
