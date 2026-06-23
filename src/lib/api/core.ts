import type { PredictionsResult } from "../../types";
import type { OpenAPIPathTemplate } from "./generated/openapi-contract";

const API_PREFIX = "/api";
const templateTokenPattern = /\{([A-Za-z0-9_]+)\}/g;
const unresolvedTemplatePattern = /\{[A-Za-z0-9_]+\}/;

type PathParamValue = string | number | boolean;
type PathParamMap = Record<string, PathParamValue>;
export type JsonValidator<T> = (value: unknown) => value is T;

/**
 * Represents a failed API request with an attached HTTP status code.
 */
export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

/**
 * Builds a URL under the `/api` prefix from raw path fragments.
 *
 * @param segments - Unencoded path segments.
 * @returns API-relative path.
 */
export function apiPath(...segments: string[]): string {
  if (segments.length === 0) {
    return API_PREFIX;
  }
  // Callers must pass raw path fragments (not pre-encoded) to avoid double-encoding.
  return `${API_PREFIX}/${segments.map(encodeURIComponent).join("/")}`;
}

/**
 * Builds a URL under the `/api` prefix from an OpenAPI path template.
 *
 * This enforces contract-aware routing for frontend API modules.
 *
 * @param template - Path template from generated OpenAPI contract.
 * @param params - Named template parameter values.
 * @returns API-relative path with encoded path parameters.
 */
export function apiRoute(template: OpenAPIPathTemplate, params: PathParamMap = {}): string {
  for (const key of Object.keys(params)) {
    if (!template.includes(`{${key}}`)) {
      throw new Error(`Unexpected path param "${key}" for template "${template}"`);
    }
  }

  const rendered = template.replace(templateTokenPattern, (_, key: string) => {
    const value = params[key];
    if (value === undefined || value === null || String(value).trim() === "") {
      throw new Error(`Missing path param "${key}" for template "${template}"`);
    }
    return encodeURIComponent(String(value));
  });

  if (unresolvedTemplatePattern.test(rendered)) {
    throw new Error(`Unresolved path template "${template}"`);
  }

  return `${API_PREFIX}${rendered}`;
}

/**
 * Attempts to parse a response body as JSON.
 *
 * @param response - Fetch response object.
 * @returns Parsed JSON value or `null` when parsing fails.
 */
async function parseJsonSafely(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch {
    return null;
  }
}

/**
 * Performs a JSON request and parses a JSON response.
 *
 * @typeParam T - Expected JSON payload type.
 * @param url - Request URL.
 * @param init - Optional fetch init.
 * @param validate - Optional runtime response validator.
 * @returns Parsed response payload.
 * @throws {ApiError} When the response status is non-2xx.
 */
export async function requestJson<T>(url: string, init?: RequestInit, validate?: JsonValidator<T>): Promise<T> {
  const response = await fetch(url, {
    credentials: "same-origin",
    ...init,
    headers: {
      ...(init?.headers ?? {}),
      "Content-Type": "application/json",
    },
  });

  if (!response.ok) {
    const payload = await parseJsonSafely(response);
    const message =
      typeof payload === "object" && payload !== null && "error" in payload && typeof payload.error === "string"
        ? payload.error
        : `Request failed with status ${response.status}`;

    throw new ApiError(message, response.status);
  }

  const payload = (await response.json()) as unknown;
  if (validate && !validate(payload)) {
    throw new ApiError(`Unexpected response shape from ${url}`, 502);
  }
  return payload as T;
}

/**
 * Performs a request and returns the response body as plain text.
 *
 * @param url - Request URL.
 * @returns Response body.
 * @throws {ApiError} When the response status is non-2xx.
 */
export async function requestText(url: string): Promise<string> {
  const response = await fetch(url, {
    credentials: "same-origin",
  });

  if (!response.ok) {
    throw new ApiError(`Request failed with status ${response.status}`, response.status);
  }

  return response.text();
}

/**
 * Loads predictions with backward-compatible fallback to legacy endpoint names.
 *
 * @param force - Whether to bypass server-side prediction caches.
 * @returns Prediction payload.
 */
export async function requestPredictions(force = false): Promise<PredictionsResult> {
  const suffix = force ? "?force=1" : "";
  try {
    return await requestJson<PredictionsResult>(`${apiRoute("/predictions")}${suffix}`);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      // Backward compatibility for pre-v0.2 backends; safe to remove after v1.0.
      return requestJson<PredictionsResult>(`${apiRoute("/predictive-incidents")}${suffix}`);
    }
    throw err;
  }
}

/**
 * Returns the SSE endpoint URL used for cluster event streams.
 */
export function buildStreamURL(): string {
  return apiRoute("/stream");
}

/**
 * Returns the WebSocket endpoint URL used for cluster event streams.
 */
export function buildStreamWSURL(): string {
  if (typeof window === "undefined") {
    return apiRoute("/stream/ws");
  }
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}${apiRoute("/stream/ws")}`;
}
