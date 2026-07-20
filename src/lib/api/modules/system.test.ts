import { afterEach, describe, expect, it, vi } from "vitest";
import { systemApi } from "./system";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("systemApi validators", () => {
  it("accepts lightweight healthz responses", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            status: "ok",
            timestamp: "2026-01-01T00:00:00Z",
            version: "dev",
            commit: "local",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );

    await expect(systemApi.getHealth()).resolves.toMatchObject({ status: "ok", version: "dev" });
  });

  it("rejects malformed runtime responses", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ mode: "prod", warnings: "none" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    await expect(systemApi.getRuntimeStatus()).rejects.toMatchObject({
      status: 502,
      message: "Unexpected response shape from /api/runtime",
    });
  });
});
