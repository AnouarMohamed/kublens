package config

import (
	"time"

	"kubelens-backend/internal/model"
)

type Mode string

const (
	ModeDev  Mode = "dev"
	ModeDemo Mode = "demo"
	ModeProd Mode = "prod"
)

type AuthToken struct {
	Token string
	User  string
	Role  string
}

type Config struct {
	Port    int
	DistDir string
	Build   model.BuildInfo

	Mode    Mode
	DevMode bool

	Cluster ClusterConfig

	Assistant AssistantConfig
	Predictor PredictorConfig
	Ghost     GhostConfig
	DBPath    string
	Memory    MemoryConfig
	ChatOps   ChatOpsConfig
	Auth      AuthConfig
	RateLimit RateLimitConfig
	Audit     AuditConfig
	Alerts    AlertsConfig
	Tracing   TracingConfig

	WriteActionsEnabled  bool
	AnonymousPermissions []string
}

type ClusterConfig struct {
	KubeconfigData string
	Contexts       map[string]string
}

type AssistantConfig struct {
	Provider           string
	Timeout            time.Duration
	APIBaseURL         string
	APIKey             string
	Model              string
	Temperature        float64
	AssistantMaxTokens int
	RAGEnabled         bool
	PromptTimeout      time.Duration
	EmbeddingModel     string
	EmbeddingBaseURL   string
	EmbeddingAPIKey    string
}

type PredictorConfig struct {
	BaseURL      string
	Timeout      time.Duration
	SharedSecret string
}

type GhostConfig struct {
	Enabled    bool
	EngineAddr string
	Timeout    time.Duration
}

type MemoryConfig struct {
	FilePath string
}

type ChatOpsConfig struct {
	SlackWebhookURL      string
	BaseURL              string
	NotifyIncidents      bool
	NotifyRemediations   bool
	NotifyPostmortems    bool
	NotifyAssistantFinds bool
}

type AuthConfig struct {
	Enabled            bool
	AllowHeaderToken   bool
	TrustedCSRFDomains []string
	TrustedProxyCIDRs  []string
	Tokens             []AuthToken
	OIDC               AuthOIDCConfig
}

type AuthOIDCConfig struct {
	Enabled       bool
	Provider      string
	IssuerURL     string
	ClientID      string
	UsernameClaim string
	RoleClaim     string
}

type RateLimitConfig struct {
	Enabled  bool
	Requests int
	Window   time.Duration
}

type AuditConfig struct {
	MaxItems int
	FilePath string
}

type AlertsConfig struct {
	Timeout             time.Duration
	AlertmanagerURL     string
	SlackWebhookURL     string
	PagerDutyEventsURL  string
	PagerDutyRoutingKey string
}

type TracingConfig struct {
	Endpoint    string
	Protocol    string
	Insecure    bool
	ServiceName string
	SampleRatio float64
}

type profile struct {
	authEnabled        bool
	rateLimitEnabled   bool
	rateLimitRequests  int
	rateLimitWindowSec int
	writeActions       bool
	ragEnabled         bool
}
