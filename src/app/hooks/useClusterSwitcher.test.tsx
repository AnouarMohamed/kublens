import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useClusterSwitcher } from "./useClusterSwitcher";
import type { ClusterContextList } from "../../types";
import type { Dispatch, SetStateAction } from "react";

const mockAPI = vi.hoisted(() => ({
  selectCluster: vi.fn(),
}));

vi.mock("../../lib/api", () => ({
  api: mockAPI,
}));

describe("useClusterSwitcher", () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it("switches cluster and increments refresh key", async () => {
    const setClusterContexts = vi.fn() as unknown as Dispatch<SetStateAction<ClusterContextList | null>>;
    const onMessage = vi.fn();
    mockAPI.selectCluster.mockResolvedValue({ selected: "prod" });

    const { result } = renderHook(() =>
      useClusterSwitcher({
        clusterContexts: { selected: "default", items: [{ name: "default", isRealCluster: false }] },
        setClusterContexts,
        onMessage,
      }),
    );

    await act(async () => {
      await result.current.selectCluster("prod");
    });

    await waitFor(() => {
      expect(result.current.clusterRefreshKey).toBe(1);
      expect(onMessage).toHaveBeenCalledWith("Switched to cluster: prod");
    });
  });

  it("reports selection errors", async () => {
    const setClusterContexts = vi.fn();
    const onMessage = vi.fn();
    mockAPI.selectCluster.mockRejectedValue(new Error("selection failed"));

    const { result } = renderHook(() =>
      useClusterSwitcher({
        clusterContexts: { selected: "default", items: [{ name: "default", isRealCluster: false }] },
        setClusterContexts,
        onMessage,
      }),
    );

    await act(async () => {
      await result.current.selectCluster("broken");
    });

    expect(onMessage).toHaveBeenCalledWith("selection failed");
  });

  it("exposes a manual refresh trigger", () => {
    const setClusterContexts = vi.fn();
    const onMessage = vi.fn();

    const { result } = renderHook(() =>
      useClusterSwitcher({
        clusterContexts: { selected: "default", items: [{ name: "default", isRealCluster: false }] },
        setClusterContexts,
        onMessage,
      }),
    );

    act(() => {
      result.current.refreshCluster();
    });

    expect(result.current.clusterRefreshKey).toBe(1);
  });
});
