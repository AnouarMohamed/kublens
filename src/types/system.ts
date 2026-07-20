import type { RAGMetricsSummary } from "./assistant";
import type { RemediationProposal } from "./incident";

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
  databaseDriver: string;
  databaseMigrations: boolean;
  enterpriseStorage: boolean;
  memoryStore: string;
  memoryDurable: boolean;
  auditStore: string;
  auditDurable: boolean;
  auditSigned: boolean;
  predictorEnabled: boolean;
  predictorHealthy: boolean;
  predictorLastError?: string;
  predictorMode: "deterministic" | "shadow" | "blended" | string;
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

export interface HealthzStatus {
  status: "ok" | string;
  timestamp: string;
  version: string;
  commit: string;
}

export interface ProductionReadinessStatus {
  status: "ready" | "degraded" | "blocked" | string;
  generatedAt: string;
  summary: string;
  mode: string;
  blockers: ProductionReadinessIssue[];
  warnings: ProductionReadinessIssue[];
  checks: ProductionReadinessCheck[];
  stores: ProductionStorePosture;
  dependencies: ProductionDependencyPosture;
  runbooks: ProductionRunbookLink[];
  build: BuildInfo;
}

export interface ProductionReadinessIssue {
  key: string;
  severity: "blocker" | "warning" | string;
  message: string;
  recommendation: string;
}

export interface ProductionReadinessCheck {
  name: string;
  ok: boolean;
  severity: "blocker" | "warning" | "info" | string;
  message: string;
}

export interface ProductionStorePosture {
  databaseDriver: string;
  enterpriseStorage: boolean;
  migrationsEnabled: boolean;
  memoryStore: string;
  memoryDurable: boolean;
  auditStore: string;
  auditDurable: boolean;
  auditSigned: boolean;
  auditSinkFailures: number;
  auditSinkLastError?: string;
}

export interface ProductionDependencyPosture {
  cluster: ProductionDependencyStatus;
  predictor: ProductionDependencyStatus;
  ghost: ProductionDependencyStatus;
  alerts: ProductionDependencyStatus;
}

export interface ProductionDependencyStatus {
  enabled: boolean;
  healthy: boolean;
  message: string;
  lastSuccess?: string;
  lastFailure?: string;
}

export interface ProductionRunbookLink {
  title: string;
  path: string;
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

export interface PredictorModelHealth {
  source: string;
  mode: "deterministic" | "shadow" | "blended" | string;
  enabled: boolean;
  usableForBlending: boolean;
  modelLoaded: boolean;
  metadataLoaded: boolean;
  modelVersion: string;
  stale: boolean;
  maxModelAgeHours: number;
  minFeatureCompleteness: number;
  requiredFeatures: string[];
  calibratedThreshold?: number;
  calibrationMethod: string;
  evaluationMetrics: Record<string, number>;
  promotionGates: Record<string, number>;
  lastError: string;
}

export interface ExperimentalStatus {
  generatedAt: string;
  features: ExperimentalFeatureStatus[];
}

export interface ExperimentalFeatureStatus {
  name: string;
  enabled: boolean;
  experimental: boolean;
  maturity: string;
  message: string;
  limitations: string[];
}

export interface NodeTelemetryReport {
  generatedAt: string;
  enabled: boolean;
  experimental: boolean;
  source: string;
  agentConnected: boolean;
  lastReceivedAt?: string;
  summary: string;
  nodes: NodeTelemetryItem[];
  limitations: string[];
}

export interface NodeTelemetryIngestRequest {
  agentId?: string;
  source?: string;
  capturedAt?: string;
  nodes: NodeTelemetryItem[];
}

export interface NodeTelemetryItem {
  node: string;
  status: string;
  cpuUsage: string;
  memoryUsage: string;
  warningEvents: number;
  pressureSignals: string[];
  observedWorkload: number;
}

export interface FleetDriftReport {
  generatedAt: string;
  enabled: boolean;
  experimental: boolean;
  baseline: string;
  compared: number;
  items: FleetDriftItem[];
  limitations: string[];
}

export interface FleetDriftProposalReport {
  generatedAt: string;
  enabled: boolean;
  experimental: boolean;
  proposals: RemediationProposal[];
  limitations: string[];
}

export interface FleetDriftItem {
  cluster: string;
  severity: string;
  summary: string;
  signals: string[];
}

export interface AutonomousRemediationPolicy {
  enabled: boolean;
  experimental: boolean;
  minRiskScore: number;
  maxProposals: number;
  requiresWriteGate: boolean;
  requiresHumanReview: boolean;
  blockedReasons: string[];
}

export interface AutonomousRemediationReport {
  generatedAt: string;
  enabled: boolean;
  experimental: boolean;
  policy: AutonomousRemediationPolicy;
  proposals: RemediationProposal[];
  limitations: string[];
}
