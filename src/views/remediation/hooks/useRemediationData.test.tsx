import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { RemediationGitOpsArtifact, RemediationProposal } from "../../../types";
import { useRemediationData } from "./useRemediationData";

const MockApiError = vi.hoisted(
  () =>
    class MockApiError extends Error {
      status: number;

      constructor(message: string, status: number) {
        super(message);
        this.status = status;
      }
    },
);

const mockAPI = vi.hoisted(() => ({
  listRemediation: vi.fn(),
  getRemediationGitOpsArtifact: vi.fn(),
  proposeRemediation: vi.fn(),
  approveRemediation: vi.fn(),
  generateRemediationGitOpsArtifact: vi.fn(),
  executeRemediation: vi.fn(),
  rejectRemediation: vi.fn(),
}));

const mockAuth = vi.hoisted(() => ({
  can: vi.fn(),
  isLoading: false,
}));

vi.mock("../../../lib/api", () => ({
  api: mockAPI,
  ApiError: MockApiError,
}));

vi.mock("../../../context/AuthSessionContext", () => ({
  useAuthSession: () => mockAuth,
}));

function proposal(overrides: Partial<RemediationProposal> = {}): RemediationProposal {
  return {
    id: "proposal-1",
    kind: "restart_pod",
    status: "proposed",
    incidentId: "incident-1",
    resource: "api-7d9f",
    namespace: "default",
    reason: "CrashLoopBackOff",
    riskLevel: "high",
    dryRunResult: "restart pod default/api-7d9f",
    executionResult: "",
    createdAt: "2026-06-23T10:00:00Z",
    updatedAt: "2026-06-23T10:00:00Z",
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

function gitOpsArtifact(proposalID = "proposal-1"): RemediationGitOpsArtifact {
  return {
    proposalId: proposalID,
    generatedBy: "system",
    generatedAt: "2026-06-23T10:01:00Z",
    updatedAt: "2026-06-23T10:01:00Z",
    artifact: {
      supportLevel: "patch_ready",
      strategy: "restart",
      summary: "Restart pod",
      branchName: "remediation/proposal-1",
      prTitle: "Restart pod",
      commitMessage: "Restart pod",
      targetPath: "clusters/default/pod.yaml",
      targetKind: "Pod",
      targetNamespace: "default",
      targetName: "api-7d9f",
      format: "yaml",
      artifactBody: "kind: Pod",
      instructions: ["Apply through GitOps"],
    },
  };
}

describe("useRemediationData", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    window.history.replaceState({}, "", "/");
    mockAuth.can.mockReturnValue(true);
    mockAPI.getRemediationGitOpsArtifact.mockResolvedValue(gitOpsArtifact());
  });

  afterEach(() => {
    vi.restoreAllMocks();
    window.history.replaceState({}, "", "/");
  });

  it("preserves approve query selection after proposals load", async () => {
    window.history.replaceState({}, "", "/?approve=proposal-2");
    mockAPI.listRemediation.mockResolvedValue([
      proposal({ id: "proposal-1", updatedAt: "2026-06-23T10:00:00Z" }),
      proposal({ id: "proposal-2", status: "approved", updatedAt: "2026-06-23T10:05:00Z" }),
    ]);

    const { result } = renderHook(() => useRemediationData());

    await waitFor(() => {
      expect(result.current.items).toHaveLength(2);
      expect(result.current.selectedID).toBe("proposal-2");
    });
  });

  it("updates proposal state through approve and execute actions", async () => {
    const initial = proposal();
    const approved = proposal({ status: "approved", approvedBy: "sre", approvedAt: "2026-06-23T10:02:00Z" });
    const executed = proposal({
      status: "executed",
      approvedBy: "sre",
      approvedAt: "2026-06-23T10:02:00Z",
      executedBy: "sre",
      executedAt: "2026-06-23T10:03:00Z",
      executionResult: "pod restarted",
    });

    mockAPI.listRemediation.mockResolvedValue([initial]);
    mockAPI.approveRemediation.mockResolvedValue(approved);
    mockAPI.executeRemediation.mockResolvedValue(executed);

    const { result } = renderHook(() => useRemediationData());

    await waitFor(() => {
      expect(result.current.items[0]?.status).toBe("proposed");
    });

    await act(async () => {
      await result.current.approveAndPrepareExecute(initial);
    });

    expect(result.current.items[0]?.status).toBe("approved");
    expect(result.current.executing?.status).toBe("approved");
    expect(result.current.message).toContain("Confirm execution");

    await act(async () => {
      await result.current.execute(result.current.executing!);
    });

    expect(result.current.items[0]?.status).toBe("executed");
    expect(result.current.executing).toBeNull();
    expect(result.current.message).toContain("executed");
  });

  it("clears reject form state after rejecting a proposal", async () => {
    const initial = proposal();
    const rejected = proposal({
      status: "rejected",
      rejectedBy: "sre",
      rejectedAt: "2026-06-23T10:04:00Z",
      rejectedReason: "Too risky",
    });

    mockAPI.listRemediation.mockResolvedValue([initial]);
    mockAPI.rejectRemediation.mockResolvedValue(rejected);

    const { result } = renderHook(() => useRemediationData());

    await waitFor(() => {
      expect(result.current.items).toHaveLength(1);
    });

    act(() => {
      result.current.setRejectingID(initial.id);
      result.current.setRejectReason("Too risky");
    });

    await act(async () => {
      await result.current.reject(initial.id, "Too risky");
    });

    expect(result.current.items[0]?.status).toBe("rejected");
    expect(result.current.rejectingID).toBeNull();
    expect(result.current.rejectReason).toBe("");
  });
});
