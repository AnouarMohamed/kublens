package model

type PredictorModelHealth struct {
	Source                 string             `json:"source"`
	Mode                   string             `json:"mode"`
	Enabled                bool               `json:"enabled"`
	UsableForBlending      bool               `json:"usableForBlending"`
	ModelLoaded            bool               `json:"modelLoaded"`
	MetadataLoaded         bool               `json:"metadataLoaded"`
	ModelVersion           string             `json:"modelVersion"`
	Stale                  bool               `json:"stale"`
	MaxModelAgeHours       int                `json:"maxModelAgeHours"`
	MinFeatureCompleteness float64            `json:"minFeatureCompleteness"`
	RequiredFeatures       []string           `json:"requiredFeatures"`
	CalibratedThreshold    *float64           `json:"calibratedThreshold,omitempty"`
	CalibrationMethod      string             `json:"calibrationMethod"`
	EvaluationMetrics      map[string]float64 `json:"evaluationMetrics"`
	PromotionGates         map[string]float64 `json:"promotionGates"`
	LastError              string             `json:"lastError"`
}

type ExperimentalStatus struct {
	GeneratedAt string                      `json:"generatedAt"`
	Features    []ExperimentalFeatureStatus `json:"features"`
}

type ExperimentalFeatureStatus struct {
	Name         string   `json:"name"`
	Enabled      bool     `json:"enabled"`
	Experimental bool     `json:"experimental"`
	Maturity     string   `json:"maturity"`
	Message      string   `json:"message"`
	Limitations  []string `json:"limitations"`
}

type NodeTelemetryReport struct {
	GeneratedAt    string              `json:"generatedAt"`
	Enabled        bool                `json:"enabled"`
	Experimental   bool                `json:"experimental"`
	Source         string              `json:"source"`
	AgentConnected bool                `json:"agentConnected"`
	Summary        string              `json:"summary"`
	Nodes          []NodeTelemetryItem `json:"nodes"`
	Limitations    []string            `json:"limitations"`
}

type NodeTelemetryItem struct {
	Node             string   `json:"node"`
	Status           string   `json:"status"`
	CPUUsage         string   `json:"cpuUsage"`
	MemoryUsage      string   `json:"memoryUsage"`
	WarningEvents    int      `json:"warningEvents"`
	PressureSignals  []string `json:"pressureSignals"`
	ObservedWorkload int      `json:"observedWorkload"`
}

type FleetDriftReport struct {
	GeneratedAt  string           `json:"generatedAt"`
	Enabled      bool             `json:"enabled"`
	Experimental bool             `json:"experimental"`
	Baseline     string           `json:"baseline"`
	Compared     int              `json:"compared"`
	Items        []FleetDriftItem `json:"items"`
	Limitations  []string         `json:"limitations"`
}

type FleetDriftItem struct {
	Cluster  string   `json:"cluster"`
	Severity string   `json:"severity"`
	Summary  string   `json:"summary"`
	Signals  []string `json:"signals"`
}

type AutonomousRemediationPolicy struct {
	Enabled             bool     `json:"enabled"`
	Experimental        bool     `json:"experimental"`
	MinRiskScore        int      `json:"minRiskScore"`
	MaxProposals        int      `json:"maxProposals"`
	RequiresWriteGate   bool     `json:"requiresWriteGate"`
	RequiresHumanReview bool     `json:"requiresHumanReview"`
	BlockedReasons      []string `json:"blockedReasons"`
}

type AutonomousRemediationReport struct {
	GeneratedAt  string                      `json:"generatedAt"`
	Enabled      bool                        `json:"enabled"`
	Experimental bool                        `json:"experimental"`
	Policy       AutonomousRemediationPolicy `json:"policy"`
	Proposals    []RemediationProposal       `json:"proposals"`
	Limitations  []string                    `json:"limitations"`
}
