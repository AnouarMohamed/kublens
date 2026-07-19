package model

type ProductionReadinessStatus struct {
	Status       string                      `json:"status"`
	GeneratedAt  string                      `json:"generatedAt"`
	Summary      string                      `json:"summary"`
	Mode         string                      `json:"mode"`
	Blockers     []ProductionReadinessIssue  `json:"blockers"`
	Warnings     []ProductionReadinessIssue  `json:"warnings"`
	Checks       []ProductionReadinessCheck  `json:"checks"`
	Stores       ProductionStorePosture      `json:"stores"`
	Dependencies ProductionDependencyPosture `json:"dependencies"`
	Runbooks     []ProductionRunbookLink     `json:"runbooks"`
	Build        BuildInfo                   `json:"build"`
}

type ProductionReadinessIssue struct {
	Key            string `json:"key"`
	Severity       string `json:"severity"`
	Message        string `json:"message"`
	Recommendation string `json:"recommendation"`
}

type ProductionReadinessCheck struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type ProductionStorePosture struct {
	DatabaseDriver     string `json:"databaseDriver"`
	EnterpriseStorage  bool   `json:"enterpriseStorage"`
	MigrationsEnabled  bool   `json:"migrationsEnabled"`
	MemoryStore        string `json:"memoryStore"`
	MemoryDurable      bool   `json:"memoryDurable"`
	AuditStore         string `json:"auditStore"`
	AuditDurable       bool   `json:"auditDurable"`
	AuditSigned        bool   `json:"auditSigned"`
	AuditSinkFailures  uint64 `json:"auditSinkFailures"`
	AuditSinkLastError string `json:"auditSinkLastError,omitempty"`
}

type ProductionDependencyPosture struct {
	Cluster   ProductionDependencyStatus `json:"cluster"`
	Predictor ProductionDependencyStatus `json:"predictor"`
	Ghost     ProductionDependencyStatus `json:"ghost"`
	Alerts    ProductionDependencyStatus `json:"alerts"`
}

type ProductionDependencyStatus struct {
	Enabled     bool   `json:"enabled"`
	Healthy     bool   `json:"healthy"`
	Message     string `json:"message"`
	LastSuccess string `json:"lastSuccess,omitempty"`
	LastFailure string `json:"lastFailure,omitempty"`
}

type ProductionRunbookLink struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}
