import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import IncidentWorkbench from "..";
import type { RemediationProposal } from "../../../types";

const mockAPI = vi.hoisted(() => ({
  getRuntimeStatus: vi.fn(),
  getEnterpriseReadiness: vi.fn(),
  getDiagnostics: vi.fn(),
  getPredictions: vi.fn(),
  listRemediation: vi.fn(),
  listGhostSimulations: vi.fn(),
  getAuditLog: vi.fn(),
}));

const mockAuth = vi.hoisted(() => ({
  can: vi.fn(),
  isLoading: false,
}));

const mockNavigation = vi.hoisted(() => ({
  navigateToView: vi.fn(),
}));

vi.mock("../../../lib/api", () => ({
  api: mockAPI,
}));

vi.mock("../../../context/AuthSessionContext", () => ({
  useAuthSession: () => mockAuth,
}));

vi.mock("../../../app/viewNavigation", () => mockNavigation);

function remediation(overrides: Partial<RemediationProposal> = {}): RemediationProposal {
  return {
    id: "proposal-1",
    kind: "restart_pod",
    status: "proposed",
    incidentId: "incident-1",
    resource: "payment-gateway",
    namespace: "prod",
    reason: "CrashLoopBackOff",
    riskLevel: "high",
    dryRunResult: "",
    executionResult: "",
    createdAt: "2026-07-18T12:00:00Z",
    updatedAt: "2026-07-18T12:00:00Z",
    approvedBy: "",
    approvedAt: "",
    rejectedBy: "",
    rejectedAt: "",
    rejectedReason: "",
    executedBy: "",
    executedAt: "",
    ...overrides,
  };
}

describe("IncidentWorkbench", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    mockAuth.can.mockReturnValue(true);
    mockAPI.getRuntimeStatus.mockResolvedValue({
      mode: "prod",
      devMode: false,
      insecure: false,
      isRealCluster: true,
      authEnabled: true,
      writeActionsEnabled: true,
      databaseDriver: "sqlite",
      enterpriseStorage: true,
      predictorEnabled: true,
      predictorHealthy: true,
      predictorMode: "shadow",
      ghostEnabled: true,
      ghostHealthy: true,
      assistantEnabled: false,
      ragEnabled: false,
      alertsEnabled: true,
      warnings: [],
    });
    mockAPI.getEnterpriseReadiness.mockResolvedValue({
      status: "degraded",
      timestamp: "2026-07-18T12:00:00Z",
      build: { version: "dev", commit: "abc1234", builtAt: "" },
      checks: [
        { name: "auth", ok: true, message: "Authentication is enabled." },
        { name: "predictor", ok: false, message: "Predictor is in shadow mode." },
      ],
    });
    mockAPI.getDiagnostics.mockResolvedValue({
      summary: "One critical issue",
      timestamp: "2026-07-18T12:00:00Z",
      criticalIssues: 1,
      warningIssues: 2,
      healthScore: 72,
      issues: [],
    });
    mockAPI.getPredictions.mockResolvedValue({
      source: "python-fastapi",
      generatedAt: "2026-07-18T12:00:00Z",
      items: [
        {
          id: "pod-payment",
          resourceKind: "Pod",
          resource: "payment-gateway",
          namespace: "prod",
          riskScore: 91,
          confidence: 88,
          summary: "payment-gateway shows elevated risk.",
          recommendation: "Inspect pod events and logs.",
          signals: [{ key: "mlShadowRisk", value: "97%" }],
        },
      ],
    });
    mockAPI.listRemediation.mockResolvedValue([remediation()]);
    mockAPI.listGhostSimulations.mockResolvedValue({
      total: 1,
      items: [
        {
          id: "ghost-1",
          createdAt: "2026-07-18T12:02:00Z",
          request: { action: "node_drain", nodeName: "node-a", horizonSeconds: 900 },
          topologyHash: "abc123456789",
          result: {
            id: "ghost-1",
            action: "node_drain",
            generatedAt: "2026-07-18T12:02:00Z",
            horizonSeconds: 900,
            engine: "in-memory",
            topologyHash: "abc123456789",
            confidence: 64,
            limitations: ["Topology came from summary fallback data."],
            verdict: {
              severity: "warning",
              summary: "Drain simulation can move 1 pod from node-a.",
              recommendations: [],
            },
            frames: [{ offsetSeconds: 0, nodes: [], pods: [], events: [] }],
          },
        },
      ],
    });
    mockAPI.getAuditLog.mockResolvedValue({
      total: 1,
      items: [
        {
          id: "audit-1",
          timestamp: "2026-07-18T12:04:00Z",
          method: "POST",
          path: "/api/remediation/propose",
          status: 200,
          durationMs: 12,
          bytes: 200,
          success: true,
          hash: "abcdef1234567890",
        },
      ],
    });
  });

  it("renders the incident workflow queue and routes actions", async () => {
    render(<IncidentWorkbench />);

    await waitFor(() => {
      expect(screen.getByText("Incident Workbench")).toBeInTheDocument();
      expect(screen.getAllByText("payment-gateway").length).toBeGreaterThan(0);
    });

    expect(screen.getByText("shadow")).toBeInTheDocument();
    expect(screen.getByText("mlShadowRisk: 97%")).toBeInTheDocument();
    expect(screen.getByText("Drain simulation can move 1 pod from node-a.")).toBeInTheDocument();
    expect(screen.getByText("abcdef123456")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /^Ghost$/i }));
    expect(mockNavigation.navigateToView).toHaveBeenCalledWith("ghost");
  });
});
