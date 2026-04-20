package model

type GitOpsSupportLevel string

const (
	GitOpsSupportPatchReady GitOpsSupportLevel = "patch_ready"
	GitOpsSupportAdvisory   GitOpsSupportLevel = "advisory"
)

type GitOpsArtifact struct {
	SupportLevel    GitOpsSupportLevel `json:"supportLevel"`
	Strategy        string             `json:"strategy"`
	Summary         string             `json:"summary"`
	BranchName      string             `json:"branchName"`
	PRTitle         string             `json:"prTitle"`
	CommitMessage   string             `json:"commitMessage"`
	TargetPath      string             `json:"targetPath"`
	TargetKind      string             `json:"targetKind,omitempty"`
	TargetNamespace string             `json:"targetNamespace,omitempty"`
	TargetName      string             `json:"targetName,omitempty"`
	Format          string             `json:"format"`
	ArtifactBody    string             `json:"artifactBody"`
	Instructions    []string           `json:"instructions"`
}

type RemediationGitOpsArtifact struct {
	ProposalID  string         `json:"proposalId"`
	Artifact    GitOpsArtifact `json:"artifact"`
	GeneratedBy string         `json:"generatedBy"`
	GeneratedAt string         `json:"generatedAt"`
	UpdatedAt   string         `json:"updatedAt"`
}

type RightsizingStatus string

const (
	RightsizingStatusOverprovisioned  RightsizingStatus = "overprovisioned"
	RightsizingStatusUnderprovisioned RightsizingStatus = "underprovisioned"
	RightsizingStatusMissingGuardrail RightsizingStatus = "missing_guardrails"
	RightsizingStatusBalanced         RightsizingStatus = "balanced"
)

type RightsizingRecommendation struct {
	ID                       string            `json:"id"`
	Namespace                string            `json:"namespace"`
	Pod                      string            `json:"pod"`
	WorkloadKind             string            `json:"workloadKind,omitempty"`
	WorkloadName             string            `json:"workloadName,omitempty"`
	Status                   RightsizingStatus `json:"status"`
	RiskLevel                string            `json:"riskLevel"`
	Summary                  string            `json:"summary"`
	QoSClass                 string            `json:"qosClass,omitempty"`
	ContainerCount           int               `json:"containerCount"`
	CPUUsage                 string            `json:"cpuUsage"`
	MemoryUsage              string            `json:"memoryUsage"`
	RequestCPU               string            `json:"requestCpu"`
	RequestMemory            string            `json:"requestMemory"`
	LimitCPU                 string            `json:"limitCpu"`
	LimitMemory              string            `json:"limitMemory"`
	RecommendedRequestCPU    string            `json:"recommendedRequestCpu"`
	RecommendedRequestMemory string            `json:"recommendedRequestMemory"`
	RecommendedLimitCPU      string            `json:"recommendedLimitCpu"`
	RecommendedLimitMemory   string            `json:"recommendedLimitMemory"`
	ReclaimableCPU           string            `json:"reclaimableCpu"`
	ReclaimableMemory        string            `json:"reclaimableMemory"`
	Confidence               int               `json:"confidence"`
	Artifact                 *GitOpsArtifact   `json:"artifact,omitempty"`
}

type RightsizingOverview struct {
	GeneratedAt          string                      `json:"generatedAt"`
	Summary              string                      `json:"summary"`
	SavingsOpportunities int                         `json:"savingsOpportunities"`
	Underprovisioned     int                         `json:"underprovisioned"`
	MissingGuardrails    int                         `json:"missingGuardrails"`
	Balanced             int                         `json:"balanced"`
	ReclaimableCPU       string                      `json:"reclaimableCpu"`
	ReclaimableMemory    string                      `json:"reclaimableMemory"`
	Items                []RightsizingRecommendation `json:"items"`
}
