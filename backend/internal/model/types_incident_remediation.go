package model

type PredictionSignal struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type IncidentPrediction struct {
	ID             string             `json:"id"`
	ResourceKind   string             `json:"resourceKind"`
	Resource       string             `json:"resource"`
	Namespace      string             `json:"namespace,omitempty"`
	RiskScore      int                `json:"riskScore"`
	Confidence     int                `json:"confidence"`
	Summary        string             `json:"summary"`
	Recommendation string             `json:"recommendation"`
	Signals        []PredictionSignal `json:"signals,omitempty"`
}

type PredictionsResult struct {
	Source      string               `json:"source"`
	GeneratedAt string               `json:"generatedAt"`
	Items       []IncidentPrediction `json:"items"`
}

type IncidentStatus string

const (
	IncidentStatusOpen     IncidentStatus = "open"
	IncidentStatusResolved IncidentStatus = "resolved"
)

type RunbookStepStatus string

const (
	RunbookStepStatusPending    RunbookStepStatus = "pending"
	RunbookStepStatusInProgress RunbookStepStatus = "in_progress"
	RunbookStepStatusDone       RunbookStepStatus = "done"
	RunbookStepStatusSkipped    RunbookStepStatus = "skipped"
)

type TimelineEntryKind string

const (
	TimelineEntryKindDiagnostic TimelineEntryKind = "diagnostic"
	TimelineEntryKindEvent      TimelineEntryKind = "event"
	TimelineEntryKindPrediction TimelineEntryKind = "prediction"
	TimelineEntryKindAction     TimelineEntryKind = "action"
)

type TimelineEntry struct {
	Timestamp string            `json:"timestamp"`
	Kind      TimelineEntryKind `json:"kind"`
	Source    string            `json:"source"`
	Summary   string            `json:"summary"`
	Resource  string            `json:"resource"`
	Severity  string            `json:"severity"`
}

type RunbookStep struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Command     string            `json:"command"`
	Status      RunbookStepStatus `json:"status"`
	Mandatory   bool              `json:"mandatory"`
}

type Incident struct {
	ID                       string          `json:"id"`
	Title                    string          `json:"title"`
	Severity                 string          `json:"severity"`
	Status                   IncidentStatus  `json:"status"`
	Summary                  string          `json:"summary"`
	OpenedAt                 string          `json:"openedAt"`
	ResolvedAt               string          `json:"resolvedAt"`
	Timeline                 []TimelineEntry `json:"timeline"`
	Runbook                  []RunbookStep   `json:"runbook"`
	AffectedResources        []string        `json:"affectedResources"`
	AssociatedRemediationIDs []string        `json:"associatedRemediationIds"`
}

type IncidentStepStatusPatch struct {
	Status RunbookStepStatus `json:"status"`
}

type RemediationKind string

const (
	RemediationKindRestartPod         RemediationKind = "restart_pod"
	RemediationKindCordonNode         RemediationKind = "cordon_node"
	RemediationKindRollbackDeployment RemediationKind = "rollback_deployment"
)

type RemediationProposal struct {
	ID              string          `json:"id"`
	Kind            RemediationKind `json:"kind"`
	Status          string          `json:"status"`
	IncidentID      string          `json:"incidentId"`
	Resource        string          `json:"resource"`
	Namespace       string          `json:"namespace"`
	Reason          string          `json:"reason"`
	RiskLevel       string          `json:"riskLevel"`
	DryRunResult    string          `json:"dryRunResult"`
	ExecutionResult string          `json:"executionResult"`
	CreatedAt       string          `json:"createdAt"`
	UpdatedAt       string          `json:"updatedAt"`
	ApprovedBy      string          `json:"approvedBy"`
	ApprovedAt      string          `json:"approvedAt"`
	RejectedBy      string          `json:"rejectedBy"`
	RejectedAt      string          `json:"rejectedAt"`
	RejectedReason  string          `json:"rejectedReason"`
	ExecutedBy      string          `json:"executedBy"`
	ExecutedAt      string          `json:"executedAt"`
}

type RemediationRejectRequest struct {
	Reason string `json:"reason"`
}

type MemoryRunbook struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Tags        []string  `json:"tags"`
	Description string    `json:"description"`
	Steps       []string  `json:"steps"`
	Embedding   []float32 `json:"embedding,omitempty"`
	UsageCount  int       `json:"usageCount"`
	CreatedAt   string    `json:"createdAt"`
	UpdatedAt   string    `json:"updatedAt"`
}

type MemoryRunbookUpsertRequest struct {
	Title       string   `json:"title"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
}

type MemoryFixPattern struct {
	ID          string          `json:"id"`
	IncidentID  string          `json:"incidentId"`
	ProposalID  string          `json:"proposalId"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Resource    string          `json:"resource"`
	Kind        RemediationKind `json:"kind"`
	RecordedBy  string          `json:"recordedBy"`
	RecordedAt  string          `json:"recordedAt"`
}

type MemoryFixCreateRequest struct {
	IncidentID  string          `json:"incidentId"`
	ProposalID  string          `json:"proposalId"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Resource    string          `json:"resource"`
	Kind        RemediationKind `json:"kind"`
}

type PostmortemMethod string

const (
	PostmortemMethodTemplate PostmortemMethod = "template"
	PostmortemMethodAI       PostmortemMethod = "ai"
)

type Postmortem struct {
	ID               string           `json:"id"`
	IncidentID       string           `json:"incidentId"`
	IncidentTitle    string           `json:"incidentTitle"`
	Severity         string           `json:"severity"`
	OpenedAt         string           `json:"openedAt"`
	ResolvedAt       string           `json:"resolvedAt"`
	Duration         string           `json:"duration"`
	GeneratedAt      string           `json:"generatedAt"`
	Method           PostmortemMethod `json:"method"`
	RootCause        string           `json:"rootCause"`
	Impact           string           `json:"impact"`
	Prevention       string           `json:"prevention"`
	TimelineMarkdown string           `json:"timelineMarkdown"`
	RunbookMarkdown  string           `json:"runbookMarkdown"`
	Timeline         []TimelineEntry  `json:"timeline"`
	Runbook          []RunbookStep    `json:"runbook"`
}
