import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useAsyncResource } from "./useAsyncResource";

describe("useAsyncResource", () => {
  it("loads data and clears errors", async () => {
    const loader = vi.fn().mockResolvedValue("ready");

    const { result } = renderHook(() =>
      useAsyncResource({
        loader,
        fallbackError: "failed",
        initialData: "initial",
      }),
    );

    await waitFor(() => {
      expect(result.current.data).toBe("ready");
      expect(result.current.isLoading).toBe(false);
    });
    expect(result.current.error).toBeNull();
  });

  it("reports loader failures with fallback text", async () => {
    const loader = vi.fn().mockRejectedValue(new Error("network down"));

    const { result } = renderHook(() =>
      useAsyncResource({
        loader,
        fallbackError: "failed",
        initialData: "initial",
      }),
    );

    await waitFor(() => {
      expect(result.current.error).toBe("network down");
      expect(result.current.isLoading).toBe(false);
    });
    expect(result.current.data).toBe("initial");
  });

  it("uses disabled state without calling the loader", async () => {
    const loader = vi.fn().mockResolvedValue("ready");

    const { result } = renderHook(() =>
      useAsyncResource({
        loader,
        fallbackError: "failed",
        initialData: "initial",
        enabled: false,
        disabledData: "denied",
        disabledError: "authenticate first",
      }),
    );

    await waitFor(() => {
      expect(result.current.data).toBe("denied");
      expect(result.current.error).toBe("authenticate first");
      expect(result.current.isLoading).toBe(false);
    });
    expect(loader).not.toHaveBeenCalled();
  });

  it("ignores stale responses after a newer load starts", async () => {
    let resolveFirst: (value: string) => void = () => {};
    let resolveSecond: (value: string) => void = () => {};
    const loader = vi
      .fn()
      .mockImplementationOnce(
        () =>
          new Promise<string>((resolve) => {
            resolveFirst = resolve;
          }),
      )
      .mockImplementationOnce(
        () =>
          new Promise<string>((resolve) => {
            resolveSecond = resolve;
          }),
      );

    const { result } = renderHook(() =>
      useAsyncResource({
        loader,
        fallbackError: "failed",
        initialData: "initial",
        autoLoad: false,
      }),
    );

    await act(async () => {
      void result.current.load();
    });
    await act(async () => {
      void result.current.load();
    });
    await act(async () => {
      resolveFirst("stale");
      resolveSecond("fresh");
    });

    await waitFor(() => {
      expect(result.current.data).toBe("fresh");
    });
  });

  it("supports local data updates for action results", async () => {
    const loader = vi.fn().mockResolvedValue(["initial"]);

    const { result } = renderHook(() =>
      useAsyncResource({
        loader,
        fallbackError: "failed",
        initialData: [] as string[],
      }),
    );

    await waitFor(() => {
      expect(result.current.data).toEqual(["initial"]);
    });

    act(() => {
      result.current.updateData((current) => [...current, "mutated"]);
    });

    expect(result.current.data).toEqual(["initial", "mutated"]);
  });
});
