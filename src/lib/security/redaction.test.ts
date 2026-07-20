import { describe, expect, it } from "vitest";
import { redactSensitiveText } from "./redaction";

describe("redactSensitiveText", () => {
  it("redacts bearer tokens, key-value secrets, and long opaque values", () => {
    const value =
      "Authorization: Bearer token-abc password=s3cr3t api_key=abcd token:1234 value=abcdefghijklmnopqrstuvwxyz123456";

    expect(redactSensitiveText(value)).toBe(
      "Authorization: Bearer [redacted] password=[redacted] api_key=[redacted] token:[redacted] value=[redacted]",
    );
  });
});
