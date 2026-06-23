import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, apiRoute, requestJson } from "./core";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("apiRoute", () => {
  it("builds static OpenAPI routes", () => {
    expect(apiRoute("/healthz")).toBe("/api/healthz");
  });

  it("replaces and encodes path parameters", () => {
    expect(apiRoute("/pods/{namespace}/{name}", { namespace: "team a", name: "api/server" })).toBe(
      "/api/pods/team%20a/api%2Fserver",
    );
  });

  it("throws when required parameters are missing", () => {
    expect(() => apiRoute("/incidents/{id}", {})).toThrow(/Missing path param "id"/);
  });

  it("throws when unexpected parameters are provided", () => {
    expect(() => apiRoute("/stats", { id: "unexpected" })).toThrow(/Unexpected path param "id"/);
  });
});

describe("requestJson", () => {
  it("rejects unexpected response shapes when a validator is provided", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ enabled: "yes" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    await expect(
      requestJson<{ enabled: boolean }>("/api/test", undefined, (value): value is { enabled: boolean } => {
        return typeof value === "object" && value !== null && "enabled" in value && value.enabled === true;
      }),
    ).rejects.toMatchObject(new ApiError("Unexpected response shape from /api/test", 502));
  });
});
