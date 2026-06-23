import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type {
  Incident,
  IncidentEvidenceBundle,
  IncidentReplay,
  RemediationProposal,
  RunbookStep,
  TimelineEntry,
} from "../../../types";
import { useIncidentData } from "./useIncidentData";

const mockAPI = vi.hoisted(() => ({
  listIncidents: vi.fn(),
  listRemediation: vi.fn(),
  getIncident: vi.fn(),
  getIncidentReplay: vi.fn(),
  getIncidentEvidence: vi.fn(),
  createIncident: vi.fn(),
  updateIncidentStep: vi.fn(),
  resolveIncident: vi.fn(),
  generatePostmortem: vi.fn(),
  recordMemoryFix: vi.fn(),
}));

const mockAuth = vi.hoisted(() => ({
  can: vi.fn(),
  isLoading: false,
}));

vi.mock("../../../lib/api", () => ({
  api: mockAPI,
}));

vi.mock("../../../context/AuthSessionContext", () => ({
  useAuthSession: () => mockAuth,
}));

const timelineEntry: TimelineEntry = {
  timestamp: "2026-06-23T09:59:00Z",
  kind: "diagnostic",
  source: "diagnostics",
  summary: "Pod restart spike",
  resource: "default/api-7d9f",
  severity: "critical",
};

const firstStep: RunbookStep = {
  id: "step-1",
  title: "Inspect pod",
  description: "Check restart reason",
  command: "kubectl describe pod api-7d9f",
  status: "pending",
  mandatory: true,
};

function incident(overrides: Partial<Incident> = {}): Incident {
  return {
    id: "incident-1",
    title: "API pod crash loop",
    severity: "critical",
    status: "open",
    summary: "API pod is restarting repeatedly.",
    openedAt: "2026-06-23T10:00:00Z",
    resolvedAt: "",
    timeline: [timelineEntry],
    runbook: [firstStep],
    affectedResources: ["default/api-7d9f"],
    associatedRemediationIds: [],
    ...overrides,
  };
}

function replay(incidentID = "incident-1"): IncidentReplay {
  return {
    incidentId: incidentID,
    incidentTitle: "API pod crash loop",
    status: "open",
    generatedAt: "2026-06-23T10:01:00Z",
    startedAt: "2026-06-23T10:00:00Z",
    endedAt: "2026-06-23T10:01:00Z",
    duration: "1m",
    frames: [],
  };
}

function evidence(incidentID = "incident-1"): IncidentEvidenceBundle {
  return {
    incidentId: incidentID,
    incidentTitle: "API pod crash loop",
    generatedAt: "2026-06-23T10:01:00Z",
    summary: "Evidence bundle",
    affectedResources: ["default/api-7d9f"],
    diagnostics: [timelineEntry],
    events: [],
    predictions: [],
    actions: [],
    audit: [],
    remediations: [],
    markdown: "# Evidence",
  };
}

function proposal(overrides: Partial<RemediationProposal> = {}): RemediationProposal {
  return {
    id: "proposal-1",
    kind: "restart_pod",
    status: "executed",
    incidentId: "incident-1",
    resource: "api-7d9f",
    namespace: "default",
    reason: "CrashLoopBackOff",
    riskLevel: "high",
    dryRunResult: "restart pod default/api-7d9f",
    executionResult: "pod restarted",
    createdAt: "2026-06-23T10:00:00Z",
    updatedAt: "2026-06-23T10:05:00Z",
    approvedBy: "sre",
    approvedAt: "2026-06-23T10:02:00Z",
    rejectedBy: "",
    rejectedAt: "",
    rejectedReason: "",
    executedBy: "sre",
    executedAt: "2026-06-23T10:05:00Z",
    ...overrides,
  };
}

describe("useIncidentData", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    mockAuth.can.mockReturnValue(true);
    mockAPI.listIncidents.mockResolvedValue([incident()]);
    mockAPI.listRemediation.mockResolvedValue([]);
    mockAPI.getIncident.mockResolvedValue(incident());
    mockAPI.getIncidentReplay.mockResolvedValue(replay());
    mockAPI.getIncidentEvidence.mockResolvedValue(evidence());
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("updates selected incident and incident list after a runbook step change", async () => {
    const openIncident = incident();
    const updatedIncident = incident({
      runbook: [{ ...firstStep, status: "in_progress" }],
    });

    mockAPI.listIncidents.mockResolvedValueOnce([openIncident]).mockResolvedValue([updatedIncident]);
    mockAPI.getIncident.mockResolvedValue(openIncident);
    mockAPI.updateIncidentStep.mockResolvedValue(updatedIncident);

    const { result } = renderHook(() => useIncidentData());

    await waitFor(() => {
      expect(result.current.incidents).toHaveLength(1);
    });

    await act(async () => {
      await result.current.loadIncidentDetail(openIncident.id);
    });

    await act(async () => {
      await result.current.applyStepStatus(firstStep, "in_progress");
    });

    expect(mockAPI.updateIncidentStep).toHaveBeenCalledWith(openIncident.id, firstStep.id, {
      status: "in_progress",
    });
    expect(result.current.selected?.runbook[0]?.status).toBe("in_progress");
    expect(result.current.incidents[0]?.runbook[0]?.status).toBe("in_progress");
    expect(result.current.message).toContain("updated to in_progress");
  });

  it("creates a fix form for resolved incidents with executed remediations", async () => {
    const resolvedIncident = incident({
      status: "resolved",
      resolvedAt: "2026-06-23T10:08:00Z",
      associatedRemediationIds: ["proposal-1"],
      runbook: [{ ...firstStep, status: "done" }],
    });

    mockAPI.listIncidents.mockResolvedValue([resolvedIncident]);
    mockAPI.getIncident.mockResolvedValue(resolvedIncident);
    mockAPI.getIncidentReplay.mockResolvedValue(replay(resolvedIncident.id));
    mockAPI.getIncidentEvidence.mockResolvedValue(evidence(resolvedIncident.id));
    mockAPI.listRemediation.mockResolvedValue([proposal()]);

    const { result } = renderHook(() => useIncidentData());

    await waitFor(() => {
      expect(result.current.incidents[0]?.status).toBe("resolved");
    });

    await act(async () => {
      await result.current.loadIncidentDetail(resolvedIncident.id);
    });

    await waitFor(() => {
      expect(result.current.fixForm).toMatchObject({
        incidentId: resolvedIncident.id,
        proposalId: "proposal-1",
        resource: "default/api-7d9f",
        kind: "restart_pod",
      });
    });

    await act(async () => {
      result.current.dismissFixPrompt();
    });

    expect(result.current.fixForm).toBeNull();
    expect(result.current.fixPromptDismissed).toBe(true);
  });
});
