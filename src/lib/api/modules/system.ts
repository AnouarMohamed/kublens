import type {
  ApiMetricsSnapshot,
  BuildInfo,
  ClusterInfo,
  ClusterStats,
  DiagnosticsResult,
  HealthStatus,
  PredictionsResult,
  RightsizingOverview,
  RuntimeStatus,
  SLOOverview,
} from "../../../types";
import { apiRoute, requestJson, requestPredictions } from "../core";

export const systemApi = {
  getVersion: () => requestJson<BuildInfo>(apiRoute("/version")),
  getHealth: () => requestJson<HealthStatus>(apiRoute("/healthz")),
  getReadiness: () => requestJson<HealthStatus>(apiRoute("/readyz")),
  getRuntimeStatus: () => requestJson<RuntimeStatus>(apiRoute("/runtime")),
  getClusterInfo: () => requestJson<ClusterInfo>(apiRoute("/cluster-info")),
  getApiMetrics: (signal?: AbortSignal) => requestJson<ApiMetricsSnapshot>(apiRoute("/metrics"), { signal }),
  getSLOOverview: (signal?: AbortSignal) => requestJson<SLOOverview>(apiRoute("/slo"), { signal }),
  getRightsizingOverview: (signal?: AbortSignal) =>
    requestJson<RightsizingOverview>(apiRoute("/rightsizing"), { signal }),
  getStats: (signal?: AbortSignal) => requestJson<ClusterStats>(apiRoute("/stats"), { signal }),
  getDiagnostics: (signal?: AbortSignal) => requestJson<DiagnosticsResult>(apiRoute("/diagnostics"), { signal }),
  getPredictions: (force = false): Promise<PredictionsResult> => requestPredictions(force),
};
