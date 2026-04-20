import type {
  MemoryFixCreateRequest,
  MemoryFixPattern,
  MemoryRunbook,
  MemoryRunbookUpsertRequest,
  RemediationGitOpsArtifact,
  RemediationProposal,
  RemediationRejectRequest,
  RiskAnalyzeRequest,
  RiskReport,
} from "../../../types";
import { apiRoute, requestJson } from "../core";

export const remediationApi = {
  proposeRemediation: () =>
    requestJson<RemediationProposal[]>(apiRoute("/remediation/propose"), {
      method: "POST",
      body: JSON.stringify({}),
    }),
  listRemediation: () => requestJson<RemediationProposal[]>(apiRoute("/remediation")),
  getRemediationGitOpsArtifact: (id: string) =>
    requestJson<RemediationGitOpsArtifact>(apiRoute("/remediation/{id}/gitops", { id })),
  generateRemediationGitOpsArtifact: (id: string) =>
    requestJson<RemediationGitOpsArtifact>(apiRoute("/remediation/{id}/gitops", { id }), {
      method: "POST",
      body: JSON.stringify({}),
    }),
  approveRemediation: (id: string) =>
    requestJson<RemediationProposal>(apiRoute("/remediation/{id}/approve", { id }), {
      method: "POST",
      body: JSON.stringify({}),
    }),
  executeRemediation: (id: string) =>
    requestJson<RemediationProposal>(apiRoute("/remediation/{id}/execute", { id }), {
      method: "POST",
      body: JSON.stringify({}),
    }),
  rejectRemediation: (id: string, payload: RemediationRejectRequest) =>
    requestJson<RemediationProposal>(apiRoute("/remediation/{id}/reject", { id }), {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  searchMemoryRunbooks: (query = "") => {
    const suffix = query.trim() === "" ? "" : `?q=${encodeURIComponent(query.trim())}`;
    return requestJson<MemoryRunbook[]>(`${apiRoute("/memory/runbooks")}${suffix}`);
  },
  createMemoryRunbook: (payload: MemoryRunbookUpsertRequest) =>
    requestJson<MemoryRunbook>(apiRoute("/memory/runbooks"), {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  updateMemoryRunbook: (id: string, payload: MemoryRunbookUpsertRequest) =>
    requestJson<MemoryRunbook>(apiRoute("/memory/runbooks/{id}", { id }), {
      method: "PUT",
      body: JSON.stringify(payload),
    }),
  listMemoryFixes: (query = "") => {
    const suffix = query.trim() === "" ? "" : `?q=${encodeURIComponent(query.trim())}`;
    return requestJson<MemoryFixPattern[]>(`${apiRoute("/memory/fixes")}${suffix}`);
  },
  recordMemoryFix: (payload: MemoryFixCreateRequest) =>
    requestJson<MemoryFixPattern>(apiRoute("/memory/fixes"), {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  analyzeRiskGuard: (payload: RiskAnalyzeRequest) =>
    requestJson<RiskReport>(apiRoute("/risk-guard/analyze"), {
      method: "POST",
      body: JSON.stringify(payload),
    }),
};
