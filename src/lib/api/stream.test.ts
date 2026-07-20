import { describe, expect, it } from "vitest";
import { parseStreamEvent } from "./stream";

describe("parseStreamEvent", () => {
  it("accepts valid stream events", () => {
    expect(
      parseStreamEvent<{ ok: boolean }>(
        '{"type":"connected","timestamp":"2026-01-01T00:00:00Z","payload":{"ok":true}}',
      ),
    ).toEqual({
      type: "connected",
      timestamp: "2026-01-01T00:00:00Z",
      payload: { ok: true },
    });
  });

  it("rejects malformed stream frames", () => {
    expect(parseStreamEvent("not-json")).toBeNull();
    expect(parseStreamEvent("[]")).toBeNull();
    expect(parseStreamEvent('{"type":"connected","payload":{}}')).toBeNull();
  });
});
