export type GitOpsSupportLevel = "patch_ready" | "advisory";

export interface GitOpsArtifact {
  supportLevel: GitOpsSupportLevel;
  strategy: string;
  summary: string;
  branchName: string;
  prTitle: string;
  commitMessage: string;
  targetPath: string;
  targetKind?: string;
  targetNamespace?: string;
  targetName?: string;
  format: string;
  artifactBody: string;
  instructions: string[];
}

export interface RemediationGitOpsArtifact {
  proposalId: string;
  artifact: GitOpsArtifact;
  generatedBy: string;
  generatedAt: string;
  updatedAt: string;
}

export type RightsizingStatus = "overprovisioned" | "underprovisioned" | "missing_guardrails" | "balanced";

export interface RightsizingRecommendation {
  id: string;
  namespace: string;
  pod: string;
  workloadKind?: string;
  workloadName?: string;
  status: RightsizingStatus;
  riskLevel: string;
  summary: string;
  qosClass?: string;
  containerCount: number;
  cpuUsage: string;
  memoryUsage: string;
  requestCpu: string;
  requestMemory: string;
  limitCpu: string;
  limitMemory: string;
  recommendedRequestCpu: string;
  recommendedRequestMemory: string;
  recommendedLimitCpu: string;
  recommendedLimitMemory: string;
  reclaimableCpu: string;
  reclaimableMemory: string;
  confidence: number;
  artifact?: GitOpsArtifact;
}

export interface RightsizingOverview {
  generatedAt: string;
  summary: string;
  savingsOpportunities: number;
  underprovisioned: number;
  missingGuardrails: number;
  balanced: number;
  reclaimableCpu: string;
  reclaimableMemory: string;
  items: RightsizingRecommendation[];
}
