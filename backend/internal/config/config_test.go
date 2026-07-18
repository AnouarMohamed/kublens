package config

import "testing"

func TestLoadDefaultsDemoMode(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Mode != ModeDemo {
		t.Fatalf("mode = %s, want %s", cfg.Mode, ModeDemo)
	}
	if cfg.Auth.Enabled {
		t.Fatal("auth should be disabled by default in demo mode")
	}
	if cfg.WriteActionsEnabled {
		t.Fatal("write actions should be disabled by default")
	}
	if cfg.DBPath != "data/kubelens.db" {
		t.Fatalf("db path = %q, want data/kubelens.db", cfg.DBPath)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Fatalf("database driver = %q, want sqlite", cfg.Database.Driver)
	}
	if cfg.Predictor.Mode != "deterministic" {
		t.Fatalf("predictor mode = %q, want deterministic", cfg.Predictor.Mode)
	}
	if cfg.Assistant.AssistantMaxTokens != 2048 {
		t.Fatalf("assistant max tokens = %d, want 2048", cfg.Assistant.AssistantMaxTokens)
	}
}

func TestLoadProdRequiresAuth(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_MODE", "prod")
	t.Setenv("AUTH_ENABLED", "false")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when prod mode has auth disabled")
	}
}

func TestLoadAuthTokensRequiredOutsideDevMode(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("AUTH_ENABLED", "true")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when AUTH_ENABLED=true without AUTH_TOKENS")
	}
}

func TestLoadDevModeAllowsFallbackTokens(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_MODE", "dev")
	t.Setenv("DEV_MODE", "true")
	t.Setenv("AUTH_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Auth.Tokens) != 3 {
		t.Fatalf("fallback tokens = %d, want 3", len(cfg.Auth.Tokens))
	}
}

func TestWriteActionsRequireAuth(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("WRITE_ACTIONS_ENABLED", "true")
	t.Setenv("AUTH_ENABLED", "false")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when write actions enabled without auth")
	}
}

func TestProdDisallowsHeaderTokenAuth(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_MODE", "prod")
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("AUTH_TOKENS", "admin:admin:secret-token")
	t.Setenv("AUTH_ALLOW_HEADER_TOKEN", "true")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when prod mode enables AUTH_ALLOW_HEADER_TOKEN")
	}
}

func TestLoadRejectsPlannedPostgresDriver(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "postgres://kubelens@example/kubelens")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for planned postgres driver")
	}
	if err.Error() != "DATABASE_DRIVER=postgres is planned but not implemented; use sqlite for this release" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestLoadPredictorModeValidation(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("PREDICTOR_MODE", "autopilot")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for unsupported predictor mode")
	}
}

func TestLoadOIDCRequiresClientID(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("AUTH_PROVIDER", "oidc")
	t.Setenv("AUTH_OIDC_ISSUER_URL", "https://issuer.example")
	t.Setenv("AUTH_OIDC_ENABLED", "true")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when OIDC enabled without AUTH_OIDC_CLIENT_ID")
	}
}

func TestLoadOIDCWithClientIDAllowsTokenlessAuth(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("AUTH_PROVIDER", "oidc")
	t.Setenv("AUTH_OIDC_ISSUER_URL", "https://issuer.example")
	t.Setenv("AUTH_OIDC_CLIENT_ID", "kubelens-web")
	t.Setenv("AUTH_OIDC_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Auth.OIDC.Enabled {
		t.Fatal("expected OIDC to be enabled")
	}
	if cfg.Auth.OIDC.ClientID != "kubelens-web" {
		t.Fatalf("OIDC client id = %q, want kubelens-web", cfg.Auth.OIDC.ClientID)
	}
}

func TestLoadRejectsInvalidTrustedProxyCIDRs(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("AUTH_TRUSTED_PROXY_CIDRS", "10.0.0.0/8,not-a-cidr")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid AUTH_TRUSTED_PROXY_CIDRS entry")
	}
}

func TestLoadOllamaEmbeddingDefaults(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Assistant.EmbeddingBaseURL != "http://localhost:11434" {
		t.Fatalf("embedding base URL = %q, want http://localhost:11434", cfg.Assistant.EmbeddingBaseURL)
	}
	if cfg.Assistant.EmbeddingModel != "nomic-embed-text" {
		t.Fatalf("embedding model = %q, want nomic-embed-text", cfg.Assistant.EmbeddingModel)
	}
}

func TestLoadAssistantMaxTokensOverride(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ASSISTANT_MAX_TOKENS", "4096")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Assistant.AssistantMaxTokens != 4096 {
		t.Fatalf("assistant max tokens = %d, want 4096", cfg.Assistant.AssistantMaxTokens)
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()

	keys := []string{
		"APP_MODE",
		"DEV_MODE",
		"PORT",
		"DIST_DIR",
		"KUBECONFIG_DATA",
		"KUBECONFIG_CONTEXTS",
		"APP_VERSION",
		"APP_COMMIT",
		"APP_BUILT_AT",
		"ASSISTANT_PROVIDER",
		"ASSISTANT_TIMEOUT_SECONDS",
		"ASSISTANT_API_BASE_URL",
		"ASSISTANT_API_KEY",
		"ASSISTANT_MODEL",
		"ASSISTANT_TEMPERATURE",
		"ASSISTANT_MAX_TOKENS",
		"ASSISTANT_RAG_ENABLED",
		"ASSISTANT_PROMPT_TIMEOUT_SECONDS",
		"ASSISTANT_EMBEDDING_MODEL",
		"ASSISTANT_EMBEDDING_BASE_URL",
		"ASSISTANT_EMBEDDING_API_KEY",
		"OLLAMA_BASE_URL",
		"OLLAMA_EMBEDDING_MODEL",
		"PREDICTOR_BASE_URL",
		"PREDICTOR_TIMEOUT_SECONDS",
		"PREDICTOR_SHARED_SECRET",
		"PREDICTOR_MODE",
		"PREDICTOR_MODEL_METADATA_PATH",
		"PREDICTOR_MIN_FEATURE_COMPLETENESS",
		"PREDICTOR_MAX_MODEL_AGE_HOURS",
		"GHOST_ENABLED",
		"GHOST_ENGINE_ADDR",
		"GHOST_ENGINE_TIMEOUT_SECONDS",
		"DATABASE_DRIVER",
		"DATABASE_URL",
		"DATABASE_MIGRATIONS_AUTO",
		"DB_PATH",
		"MEMORY_FILE_PATH",
		"CHATOPS_SLACK_WEBHOOK_URL",
		"CHATOPS_BASE_URL",
		"CHATOPS_NOTIFY_INCIDENTS",
		"CHATOPS_NOTIFY_REMEDIATIONS",
		"CHATOPS_NOTIFY_POSTMORTEMS",
		"CHATOPS_NOTIFY_ASSISTANT_FINDINGS",
		"AUTH_ENABLED",
		"AUTH_ALLOW_HEADER_TOKEN",
		"AUTH_TRUSTED_CSRF_DOMAINS",
		"AUTH_TRUSTED_PROXY_CIDRS",
		"AUTH_TOKENS",
		"AUTH_PROVIDER",
		"AUTH_OIDC_PROVIDER",
		"AUTH_OIDC_ENABLED",
		"AUTH_OIDC_ISSUER_URL",
		"AUTH_OIDC_CLIENT_ID",
		"AUTH_OIDC_USERNAME_CLAIM",
		"AUTH_OIDC_ROLE_CLAIM",
		"RATE_LIMIT_ENABLED",
		"RATE_LIMIT_REQUESTS",
		"RATE_LIMIT_WINDOW_SECONDS",
		"AUDIT_MAX_ITEMS",
		"AUDIT_LOG_FILE",
		"ALERT_TIMEOUT_SECONDS",
		"ALERTMANAGER_WEBHOOK_URL",
		"SLACK_WEBHOOK_URL",
		"PAGERDUTY_EVENTS_URL",
		"PAGERDUTY_ROUTING_KEY",
		"WRITE_ACTIONS_ENABLED",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}
}
