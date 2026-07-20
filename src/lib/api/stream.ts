import type { StreamEvent } from "../../types";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function parseStreamEvent<T = unknown>(data: string): StreamEvent<T> | null {
  try {
    const value = JSON.parse(data) as unknown;
    if (!isRecord(value)) {
      return null;
    }
    if (typeof value.type !== "string" || typeof value.timestamp !== "string" || !("payload" in value)) {
      return null;
    }
    return {
      type: value.type,
      timestamp: value.timestamp,
      payload: value.payload as T,
    };
  } catch {
    return null;
  }
}
