import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, beforeEach } from "vitest";
import { useCurrentView } from "./useCurrentView";

describe("useCurrentView", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("defaults to the incident workbench", () => {
    const { result } = renderHook(() => useCurrentView());
    expect(result.current.currentView).toBe("workbench");
  });

  it("persists selected view", async () => {
    const { result } = renderHook(() => useCurrentView());
    act(() => {
      result.current.setCurrentView("pods");
    });

    await waitFor(() => {
      expect(window.localStorage.getItem("k8s-ops.current-view.v1")).toBe("pods");
    });
  });
});
