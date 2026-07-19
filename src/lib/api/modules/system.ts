import type {
  ApiMetricsSnapshot,
  BuildInfo,
  ClusterInfo,
  ClusterStats,
  DiagnosticsResult,
  AutonomousRemediationReport,
  ExperimentalStatus,
  FleetDriftReport,
  HealthStatus,
  NodeTelemetryReport,
  PredictionsResult,
  ProductionReadinessStatus,
  PredictorModelHealth,
  RightsizingOverview,
  RuntimeStatus,
  SLOOverview,
} from "../../../types";
import { ApiError, apiRoute, requestJson, requestPredictions } from "../core";

async function requestHealthStatus(url: string): Promise<HealthStatus> {
  const response = await fetch(url, {
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
    },
  });
  const payload = (await response.json()) as unknown;
  if (isHealthStatus(payload)) {
    return payload;
  }
  if (!response.ok) {
    throw new ApiError(`Request failed with status ${response.status}`, response.status);
  }
  throw new ApiError(`Unexpected response shape from ${url}`, 502);
}

async function requestProductionReadiness(url: string): Promise<ProductionReadinessStatus> {
  const response = await fetch(url, {
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
    },
  });
  const payload = (await response.json()) as unknown;
  if (isProductionReadinessStatus(payload)) {
    return payload;
  }
  if (!response.ok) {
    throw new ApiError(`Request failed with status ${response.status}`, response.status);
  }
  throw new ApiError(`Unexpected response shape from ${url}`, 502);
}

function isHealthStatus(value: unknown): value is HealthStatus {
  return typeof value === "object" && value !== null && "status" in value && "checks" in value && "timestamp" in value;
}

function isProductionReadinessStatus(value: unknown): value is ProductionReadinessStatus {
  return (
    typeof value === "object" &&
    value !== null &&
    "status" in value &&
    "generatedAt" in value &&
    "blockers" in value &&
    "warnings" in value &&
    "stores" in value
  );
}

export const systemApi = {
  getVersion: () => requestJson<BuildInfo>(apiRoute("/version")),
  getHealth: () => requestJson<HealthStatus>(apiRoute("/healthz")),
  getReadiness: () => requestJson<HealthStatus>(apiRoute("/readyz")),
  getEnterpriseReadiness: () => requestHealthStatus(apiRoute("/readiness/enterprise")),
  getProductionReadiness: () => requestProductionReadiness(apiRoute("/readiness/production")),
  getRuntimeStatus: () => requestJson<RuntimeStatus>(apiRoute("/runtime")),
  getClusterInfo: () => requestJson<ClusterInfo>(apiRoute("/cluster-info")),
  getApiMetrics: (signal?: AbortSignal) => requestJson<ApiMetricsSnapshot>(apiRoute("/metrics"), { signal }),
  getSLOOverview: (signal?: AbortSignal) => requestJson<SLOOverview>(apiRoute("/slo"), { signal }),
  getRightsizingOverview: (signal?: AbortSignal) =>
    requestJson<RightsizingOverview>(apiRoute("/rightsizing"), { signal }),
  getStats: (signal?: AbortSignal) => requestJson<ClusterStats>(apiRoute("/stats"), { signal }),
  getDiagnostics: (signal?: AbortSignal) => requestJson<DiagnosticsResult>(apiRoute("/diagnostics"), { signal }),
  getPredictions: (force = false): Promise<PredictionsResult> => requestPredictions(force),
  getPredictorModelHealth: (signal?: AbortSignal) =>
    requestJson<PredictorModelHealth>(apiRoute("/predictor/model"), { signal }),
  getExperimentalStatus: (signal?: AbortSignal) =>
    requestJson<ExperimentalStatus>(apiRoute("/experimental"), { signal }),
  getNodeTelemetryReport: (signal?: AbortSignal) =>
    requestJson<NodeTelemetryReport>(apiRoute("/experimental/ebpf/nodes"), { signal }),
  getFleetDriftReport: (signal?: AbortSignal) =>
    requestJson<FleetDriftReport>(apiRoute("/experimental/fleet-drift"), { signal }),
  proposeAutonomousRemediation: () =>
    requestJson<AutonomousRemediationReport>(apiRoute("/experimental/autonomous-remediation/propose"), {
      method: "POST",
    }),
};
