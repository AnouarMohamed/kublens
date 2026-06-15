import type { RAGMetricsSummary } from "./assistant";

export interface BuildInfo {
  version: string;
  commit: string;
  builtAt: string;
}

export interface RuntimeStatus {
  // Keep string fallback to tolerate forward-compatible backend modes from newer servers.
  mode: "dev" | "demo" | "prod" | string;
  devMode: boolean;
  insecure: boolean;
  isRealCluster: boolean;
  authEnabled: boolean;
  writeActionsEnabled: boolean;
  predictorEnabled: boolean;
  predictorHealthy: boolean;
  predictorLastError?: string;
  ghostEnabled: boolean;
  ghostHealthy: boolean;
  ghostLastError?: string;
  assistantEnabled: boolean;
  ragEnabled: boolean;
  alertsEnabled: boolean;
  warnings: string[];
}

export interface HealthCheck {
  name: string;
  ok: boolean;
  message: string;
  lastSuccess?: string;
  lastFailure?: string;
}

export interface HealthStatus {
  status: "ok" | "degraded" | "not-ready" | string;
  timestamp: string;
  checks: HealthCheck[];
  build: BuildInfo;
}

export interface ApiRouteMetrics {
  route: string;
  requests: number;
  errors: number;
  bytes: number;
  status2xx: number;
  status3xx: number;
  status4xx: number;
  status5xx: number;
  avgLatencyMs: number;
  maxLatencyMs: number;
}

export interface ApiMetricsSnapshot {
  uptimeSeconds: number;
  inFlight: number;
  totalRequests: number;
  totalErrors: number;
  totalBytes: number;
  avgLatencyMs: number;
  maxLatencyMs: number;
  routes: ApiRouteMetrics[];
  rag: RAGMetricsSummary;
}
