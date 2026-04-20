export type IncidentStatus = "open" | "resolved";
export type RunbookStepStatus = "pending" | "in_progress" | "done" | "skipped";
export type TimelineEntryKind = "diagnostic" | "event" | "prediction" | "action";

export interface TimelineEntry {
  timestamp: string;
  kind: TimelineEntryKind;
  source: string;
  summary: string;
  resource: string;
  severity: string;
}

export interface RunbookStep {
  id: string;
  title: string;
  description: string;
  command: string;
  status: RunbookStepStatus;
  mandatory: boolean;
}

export interface Incident {
  id: string;
  title: string;
  severity: string;
  status: IncidentStatus;
  summary: string;
  openedAt: string;
  resolvedAt: string;
  timeline: TimelineEntry[];
  runbook: RunbookStep[];
  affectedResources: string[];
  associatedRemediationIds: string[];
}

export interface IncidentStepStatusPatch {
  status: RunbookStepStatus;
}

export type RemediationKind = "restart_pod" | "cordon_node" | "rollback_deployment";

export interface RemediationProposal {
  id: string;
  kind: RemediationKind;
  status: string;
  incidentId: string;
  resource: string;
  namespace: string;
  reason: string;
  riskLevel: string;
  dryRunResult: string;
  executionResult: string;
  createdAt: string;
  updatedAt: string;
  approvedBy: string;
  approvedAt: string;
  rejectedBy: string;
  rejectedAt: string;
  rejectedReason: string;
  executedBy: string;
  executedAt: string;
}

export interface RemediationRejectRequest {
  reason: string;
}

export interface MemoryRunbook {
  id: string;
  title: string;
  tags: string[];
  description: string;
  steps: string[];
  usageCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface MemoryRunbookUpsertRequest {
  title: string;
  tags: string[];
  description: string;
  steps: string[];
}

export interface MemoryFixPattern {
  id: string;
  incidentId: string;
  proposalId: string;
  title: string;
  description: string;
  resource: string;
  kind: RemediationKind;
  recordedBy: string;
  recordedAt: string;
}

export interface MemoryFixCreateRequest {
  incidentId: string;
  proposalId: string;
  title: string;
  description: string;
  resource: string;
  kind: RemediationKind;
}

export type PostmortemMethod = "template" | "ai";

export interface Postmortem {
  id: string;
  incidentId: string;
  incidentTitle: string;
  severity: string;
  openedAt: string;
  resolvedAt: string;
  duration: string;
  generatedAt: string;
  method: PostmortemMethod;
  rootCause: string;
  impact: string;
  prevention: string;
  timelineMarkdown: string;
  runbookMarkdown: string;
  timeline: TimelineEntry[];
  runbook: RunbookStep[];
}

export interface IncidentReplayFrame {
  timestamp: string;
  offsetMinutes: number;
  kind: TimelineEntryKind;
  source: string;
  summary: string;
  resource: string;
  severity: string;
}

export interface IncidentReplay {
  incidentId: string;
  incidentTitle: string;
  status: IncidentStatus;
  generatedAt: string;
  startedAt: string;
  endedAt: string;
  duration: string;
  frames: IncidentReplayFrame[];
}

export interface IncidentEvidenceBundle {
  incidentId: string;
  incidentTitle: string;
  generatedAt: string;
  summary: string;
  affectedResources: string[];
  diagnostics: TimelineEntry[];
  events: TimelineEntry[];
  predictions: TimelineEntry[];
  actions: TimelineEntry[];
  audit: import("./audit").AuditEntry[];
  remediations: RemediationProposal[];
  postmortem?: Postmortem;
  markdown: string;
}
