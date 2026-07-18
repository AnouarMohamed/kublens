import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useRuntimeStatus } from "./useRuntimeStatus";

const mockAPI = vi.hoisted(() => ({
  getRuntimeStatus: vi.fn(),
}));

vi.mock("../../lib/api", () => ({
  api: mockAPI,
}));

describe("useRuntimeStatus", () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it("does not fetch when auth is loading or read access is missing", () => {
    renderHook(() => useRuntimeStatus({ authLoading: true, canRead: true }));
    renderHook(() => useRuntimeStatus({ authLoading: false, canRead: false }));
    expect(mockAPI.getRuntimeStatus).not.toHaveBeenCalled();
  });

  it("loads runtime status when access is granted", async () => {
    mockAPI.getRuntimeStatus.mockResolvedValue({
      mode: "demo",
      devMode: false,
      insecure: true,
      isRealCluster: false,
      authEnabled: false,
      writeActionsEnabled: false,
      databaseDriver: "sqlite",
      enterpriseStorage: true,
      predictorEnabled: true,
      predictorHealthy: true,
      predictorMode: "deterministic",
      assistantEnabled: false,
      ragEnabled: true,
      alertsEnabled: false,
      warnings: [],
    });

    const { result } = renderHook(() => useRuntimeStatus({ authLoading: false, canRead: true }));
    await waitFor(() => {
      expect(result.current?.mode).toBe("demo");
    });
  });
});
