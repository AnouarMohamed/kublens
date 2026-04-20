package model

type SLOStatus string

const (
	SLOStatusHealthy  SLOStatus = "healthy"
	SLOStatusAtRisk   SLOStatus = "at_risk"
	SLOStatusBreached SLOStatus = "breached"
)

type SLOSignal struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Tone  string `json:"tone"`
}

type SLOObjective struct {
	ID                     string      `json:"id"`
	Name                   string      `json:"name"`
	Category               string      `json:"category"`
	Summary                string      `json:"summary"`
	Window                 string      `json:"window"`
	Status                 SLOStatus   `json:"status"`
	TargetValue            string      `json:"targetValue"`
	CurrentValue           string      `json:"currentValue"`
	TargetPercent          float64     `json:"targetPercent"`
	CurrentPercent         float64     `json:"currentPercent"`
	ErrorBudgetUsedPercent float64     `json:"errorBudgetUsedPercent"`
	BudgetRemainingPercent float64     `json:"budgetRemainingPercent"`
	BurnRate               float64     `json:"burnRate"`
	Signals                []SLOSignal `json:"signals"`
}

type SLOOverview struct {
	GeneratedAt        string         `json:"generatedAt"`
	Summary            string         `json:"summary"`
	HealthyObjectives  int            `json:"healthyObjectives"`
	AtRiskObjectives   int            `json:"atRiskObjectives"`
	BreachedObjectives int            `json:"breachedObjectives"`
	Alerts             []string       `json:"alerts"`
	Objectives         []SLOObjective `json:"objectives"`
}
