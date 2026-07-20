import type {
  ApiMetricsSnapshot,
  BuildInfo,
  ClusterInfo,
  ClusterStats,
  DiagnosticsResult,
  AutonomousRemediationReport,
  ExperimentalStatus,
  FleetDriftReport,
  FleetDriftProposalReport,
  HealthStatus,
  HealthzStatus,
  NodeTelemetryIngestRequest,
  NodeTelemetryReport,
  PredictionsResult,
  ProductionReadinessStatus,
  PredictorModelHealth,
  RightsizingOverview,
  RuntimeStatus,
  SLOOverview,
} from "../../../types";
import { apiRoute, requestJson, requestPredictions } from "../core";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

function isHealthStatus(value: unknown): value is HealthStatus {
  return (
    isRecord(value) &&
    typeof value.status === "string" &&
    typeof value.timestamp === "string" &&
    Array.isArray(value.checks) &&
    value.checks.every(isHealthCheck) &&
    isBuildInfo(value.build)
  );
}

function isHealthzStatus(value: unknown): value is HealthzStatus {
  return (
    isRecord(value) &&
    typeof value.status === "string" &&
    typeof value.timestamp === "string" &&
    typeof value.version === "string" &&
    typeof value.commit === "string"
  );
}

function isHealthCheck(value: unknown): boolean {
  return (
    isRecord(value) &&
    typeof value.name === "string" &&
    typeof value.ok === "boolean" &&
    typeof value.message === "string" &&
    (value.lastSuccess === undefined || typeof value.lastSuccess === "string") &&
    (value.lastFailure === undefined || typeof value.lastFailure === "string")
  );
}

function isBuildInfo(value: unknown): value is BuildInfo {
  return (
    isRecord(value) &&
    typeof value.version === "string" &&
    typeof value.commit === "string" &&
    typeof value.builtAt === "string"
  );
}

function isRuntimeStatus(value: unknown): value is RuntimeStatus {
  return (
    isRecord(value) &&
    typeof value.mode === "string" &&
    typeof value.devMode === "boolean" &&
    typeof value.insecure === "boolean" &&
    typeof value.isRealCluster === "boolean" &&
    typeof value.authEnabled === "boolean" &&
    typeof value.writeActionsEnabled === "boolean" &&
    typeof value.databaseDriver === "string" &&
    typeof value.databaseMigrations === "boolean" &&
    typeof value.enterpriseStorage === "boolean" &&
    typeof value.memoryStore === "string" &&
    typeof value.memoryDurable === "boolean" &&
    typeof value.auditStore === "string" &&
    typeof value.auditDurable === "boolean" &&
    typeof value.auditSigned === "boolean" &&
    typeof value.predictorEnabled === "boolean" &&
    typeof value.predictorHealthy === "boolean" &&
    typeof value.predictorMode === "string" &&
    typeof value.ghostEnabled === "boolean" &&
    typeof value.ghostHealthy === "boolean" &&
    typeof value.assistantEnabled === "boolean" &&
    typeof value.ragEnabled === "boolean" &&
    typeof value.alertsEnabled === "boolean" &&
    isStringArray(value.warnings) &&
    (value.predictorLastError === undefined || typeof value.predictorLastError === "string") &&
    (value.ghostLastError === undefined || typeof value.ghostLastError === "string")
  );
}

function isProductionReadinessStatus(value: unknown): value is ProductionReadinessStatus {
  return (
    isRecord(value) &&
    typeof value.status === "string" &&
    typeof value.generatedAt === "string" &&
    typeof value.summary === "string" &&
    typeof value.mode === "string" &&
    Array.isArray(value.blockers) &&
    Array.isArray(value.warnings) &&
    Array.isArray(value.checks) &&
    isRecord(value.stores) &&
    isRecord(value.dependencies) &&
    Array.isArray(value.runbooks) &&
    isBuildInfo(value.build)
  );
}

export const systemApi = {
  getVersion: () => requestJson<BuildInfo>(apiRoute("/version")),
  getHealth: () => requestJson<HealthzStatus>(apiRoute("/healthz"), undefined, isHealthzStatus),
  getReadiness: () => requestJson<HealthStatus>(apiRoute("/readyz"), undefined, isHealthStatus),
  getEnterpriseReadiness: () => requestJson<HealthStatus>(apiRoute("/readiness/enterprise"), undefined, isHealthStatus),
  getProductionReadiness: () =>
    requestJson<ProductionReadinessStatus>(apiRoute("/readiness/production"), undefined, isProductionReadinessStatus),
  getRuntimeStatus: () => requestJson<RuntimeStatus>(apiRoute("/runtime"), undefined, isRuntimeStatus),
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
  submitNodeTelemetryReport: (payload: NodeTelemetryIngestRequest) =>
    requestJson<NodeTelemetryReport>(apiRoute("/experimental/ebpf/nodes"), {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  getFleetDriftReport: (signal?: AbortSignal) =>
    requestJson<FleetDriftReport>(apiRoute("/experimental/fleet-drift"), { signal }),
  proposeFleetDriftRemediation: () =>
    requestJson<FleetDriftProposalReport>(apiRoute("/experimental/fleet-drift/propose"), {
      method: "POST",
    }),
  proposeAutonomousRemediation: () =>
    requestJson<AutonomousRemediationReport>(apiRoute("/experimental/autonomous-remediation/propose"), {
      method: "POST",
    }),
};
