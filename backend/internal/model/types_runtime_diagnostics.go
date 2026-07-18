package model

// DiagnosticSeverity models issue criticality for cluster diagnostics.
type DiagnosticSeverity string

const (
	SeverityCritical DiagnosticSeverity = "critical"
	SeverityWarning  DiagnosticSeverity = "warning"
	SeverityInfo     DiagnosticSeverity = "info"
)

type ClusterInfo struct {
	IsRealCluster bool `json:"isRealCluster"`
}

type BuildInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"builtAt"`
}

type RuntimeStatus struct {
	Mode                string   `json:"mode"`
	DevMode             bool     `json:"devMode"`
	Insecure            bool     `json:"insecure"`
	IsRealCluster       bool     `json:"isRealCluster"`
	AuthEnabled         bool     `json:"authEnabled"`
	WriteActionsEnabled bool     `json:"writeActionsEnabled"`
	DatabaseDriver      string   `json:"databaseDriver"`
	EnterpriseStorage   bool     `json:"enterpriseStorage"`
	PredictorEnabled    bool     `json:"predictorEnabled"`
	PredictorHealthy    bool     `json:"predictorHealthy"`
	PredictorLastError  string   `json:"predictorLastError,omitempty"`
	PredictorMode       string   `json:"predictorMode"`
	GhostEnabled        bool     `json:"ghostEnabled"`
	GhostHealthy        bool     `json:"ghostHealthy"`
	GhostLastError      string   `json:"ghostLastError,omitempty"`
	AssistantEnabled    bool     `json:"assistantEnabled"`
	RAGEnabled          bool     `json:"ragEnabled"`
	AlertsEnabled       bool     `json:"alertsEnabled"`
	Warnings            []string `json:"warnings"`
}

type HealthCheck struct {
	Name        string `json:"name"`
	OK          bool   `json:"ok"`
	Message     string `json:"message"`
	LastSuccess string `json:"lastSuccess,omitempty"`
	LastFailure string `json:"lastFailure,omitempty"`
}

type HealthStatus struct {
	Status    string        `json:"status"`
	Timestamp string        `json:"timestamp"`
	Checks    []HealthCheck `json:"checks"`
	Build     BuildInfo     `json:"build"`
}

type DiagnosticIssue struct {
	Severity       DiagnosticSeverity `json:"severity"`
	Resource       string             `json:"resource,omitempty"`
	Namespace      string             `json:"namespace,omitempty"`
	Message        string             `json:"message"`
	Evidence       []string           `json:"evidence,omitempty"`
	Recommendation string             `json:"recommendation"`
	Source         string             `json:"source,omitempty"`
}

type DiagnosticsResult struct {
	Summary        string            `json:"summary"`
	Timestamp      string            `json:"timestamp"`
	CriticalIssues int               `json:"criticalIssues"`
	WarningIssues  int               `json:"warningIssues"`
	HealthScore    int               `json:"healthScore"`
	Issues         []DiagnosticIssue `json:"issues"`
}

type RiskCheck struct {
	Name       string `json:"name"`
	Category   string `json:"category,omitempty"`
	Passed     bool   `json:"passed"`
	Detail     string `json:"detail"`
	Suggestion string `json:"suggestion"`
	Score      int    `json:"score"`
}

type RiskReport struct {
	Score   int         `json:"score"`
	Level   string      `json:"level"`
	Summary string      `json:"summary"`
	Checks  []RiskCheck `json:"checks"`
}

type RiskAnalyzeRequest struct {
	Manifest string `json:"manifest"`
}

type ResourceApplyRiskResponse struct {
	Message       string     `json:"message"`
	RequiresForce bool       `json:"requiresForce"`
	Report        RiskReport `json:"report"`
}
