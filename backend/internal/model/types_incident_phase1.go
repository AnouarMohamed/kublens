package model

type IncidentReplayFrame struct {
	Timestamp     string            `json:"timestamp"`
	OffsetMinutes int               `json:"offsetMinutes"`
	Kind          TimelineEntryKind `json:"kind"`
	Source        string            `json:"source"`
	Summary       string            `json:"summary"`
	Resource      string            `json:"resource"`
	Severity      string            `json:"severity"`
}

type IncidentReplay struct {
	IncidentID    string                `json:"incidentId"`
	IncidentTitle string                `json:"incidentTitle"`
	Status        IncidentStatus        `json:"status"`
	GeneratedAt   string                `json:"generatedAt"`
	StartedAt     string                `json:"startedAt"`
	EndedAt       string                `json:"endedAt"`
	Duration      string                `json:"duration"`
	Frames        []IncidentReplayFrame `json:"frames"`
}

type IncidentEvidenceBundle struct {
	IncidentID        string                `json:"incidentId"`
	IncidentTitle     string                `json:"incidentTitle"`
	GeneratedAt       string                `json:"generatedAt"`
	Summary           string                `json:"summary"`
	AffectedResources []string              `json:"affectedResources"`
	Diagnostics       []TimelineEntry       `json:"diagnostics"`
	Events            []TimelineEntry       `json:"events"`
	Predictions       []TimelineEntry       `json:"predictions"`
	Actions           []TimelineEntry       `json:"actions"`
	Audit             []AuditEntry          `json:"audit"`
	Remediations      []RemediationProposal `json:"remediations"`
	Postmortem        *Postmortem           `json:"postmortem,omitempty"`
	Markdown          string                `json:"markdown"`
}
