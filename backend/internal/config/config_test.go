package config

import "testing"

const prodStaticAuthTokens = "admin:admin:0123456789abcdef0123456789abcdef"

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
	if cfg.Memory.Store != "file" {
		t.Fatalf("memory store = %q, want file", cfg.Memory.Store)
	}
	if cfg.Audit.Store != "memory" {
		t.Fatalf("audit store = %q, want memory", cfg.Audit.Store)
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
	t.Setenv("AUTH_TOKENS", prodStaticAuthTokens)
	t.Setenv("AUTH_ALLOW_HEADER_TOKEN", "true")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when prod mode enables AUTH_ALLOW_HEADER_TOKEN")
	}
}

func TestLoadAcceptsPostgresDriver(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "postgres://kubelens@example/kubelens")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Database.Driver != "postgres" {
		t.Fatalf("database driver = %q, want postgres", cfg.Database.Driver)
	}
	if cfg.Database.URL != "postgres://kubelens@example/kubelens" {
		t.Fatalf("database URL = %q", cfg.Database.URL)
	}
}

func TestLoadPostgresRequiresDatabaseURL(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DATABASE_DRIVER", "postgres")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for postgres without DATABASE_URL")
	}
	if err.Error() != "DATABASE_URL is required when DATABASE_DRIVER=postgres" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestLoadProdRejectsMemorySQLiteStorage(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_MODE", "prod")
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("AUTH_TOKENS", prodStaticAuthTokens)
	t.Setenv("AUDIT_SIGNING_KEY", "audit-secret")
	t.Setenv("DB_PATH", ":memory:")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for memory-only prod sqlite storage")
	}
	if err.Error() != "APP_MODE=prod requires a durable DB_PATH when DATABASE_DRIVER=sqlite" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestLoadProdDefaultsToSQLMemoryAndAuditStores(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_MODE", "prod")
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("AUTH_TOKENS", prodStaticAuthTokens)
	t.Setenv("AUDIT_SIGNING_KEY", "audit-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Memory.Store != "sql" {
		t.Fatalf("memory store = %q, want sql", cfg.Memory.Store)
	}
	if cfg.Audit.Store != "sql" {
		t.Fatalf("audit store = %q, want sql", cfg.Audit.Store)
	}
}

func TestLoadProdRequiresStrongStaticTokens(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_MODE", "prod")
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("AUTH_TOKENS", "admin:admin:short-token")
	t.Setenv("AUDIT_SIGNING_KEY", "audit-secret")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for short prod static token")
	}
	if err.Error() != "APP_MODE=prod requires static auth tokens to be at least 32 characters" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestLoadProdRequiresSQLWorkflowStores(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr string
	}{
		{name: "memory store", key: "MEMORY_STORE", value: "file", wantErr: "APP_MODE=prod requires MEMORY_STORE=sql"},
		{name: "audit store", key: "AUDIT_STORE", value: "file", wantErr: "APP_MODE=prod requires AUDIT_STORE=sql"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearConfigEnv(t)
			t.Setenv("APP_MODE", "prod")
			t.Setenv("AUTH_ENABLED", "true")
			t.Setenv("AUTH_TOKENS", prodStaticAuthTokens)
			t.Setenv("AUDIT_SIGNING_KEY", "audit-secret")
			if tc.key == "AUDIT_STORE" {
				t.Setenv("AUDIT_LOG_FILE", "data/audit.log")
			}
			t.Setenv(tc.key, tc.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			if err.Error() != tc.wantErr {
				t.Fatalf("error = %q, want %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestLoadProdRequiresAuditSigningKey(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_MODE", "prod")
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("AUTH_TOKENS", prodStaticAuthTokens)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when prod audit signing key is missing")
	}
	if err.Error() != "APP_MODE=prod requires AUDIT_SIGNING_KEY" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestLoadRejectsInvalidMemoryStore(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("MEMORY_STORE", "redis")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for unsupported MEMORY_STORE")
	}
}

func TestLoadRejectsInvalidAuditStore(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("AUDIT_STORE", "stdout")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for unsupported AUDIT_STORE")
	}
}

func TestLoadAuditFileStoreRequiresPath(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("AUDIT_STORE", "file")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when AUDIT_STORE=file has no AUDIT_LOG_FILE")
	}
}

func TestLoadAuditLogFileDefaultsToFileStore(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("AUDIT_LOG_FILE", "data/audit.log")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Audit.Store != "file" {
		t.Fatalf("audit store = %q, want file", cfg.Audit.Store)
	}
}

func TestRuntimeStatusMarksFileSQLiteAsEnterpriseStorage(t *testing.T) {
	cfg := Config{
		Mode: ModeProd,
		Database: DatabaseConfig{
			Driver:     "sqlite",
			SQLitePath: "data/kubelens.db",
		},
		Auth: AuthConfig{Enabled: true},
		Predictor: PredictorConfig{
			Mode: "deterministic",
		},
	}

	runtime := RuntimeStatus(cfg, true, false)
	if !runtime.EnterpriseStorage {
		t.Fatal("expected file-backed sqlite to count as durable storage")
	}
}

func TestRuntimeStatusMarksPostgresAsEnterpriseStorage(t *testing.T) {
	cfg := Config{
		Mode: ModeProd,
		Database: DatabaseConfig{
			Driver: "postgres",
			URL:    "postgres://kubelens@example/kubelens",
		},
		Auth: AuthConfig{Enabled: true},
		Predictor: PredictorConfig{
			Mode: "deterministic",
		},
	}

	runtime := RuntimeStatus(cfg, true, false)
	if !runtime.EnterpriseStorage {
		t.Fatal("expected postgres to count as durable storage")
	}
}

func TestLoadAuditSigningKey(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("AUDIT_SIGNING_KEY", "audit-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Audit.SigningKey != "audit-secret" {
		t.Fatalf("audit signing key = %q, want audit-secret", cfg.Audit.SigningKey)
	}
}

func TestLoadPredictorModeValidation(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("PREDICTOR_MODE", "autopilot")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for unsupported predictor mode")
	}
}

func TestLoadExperimentalDefaultsDisabled(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Experimental.EBPFTelemetryEnabled {
		t.Fatal("eBPF telemetry should be disabled by default")
	}
	if cfg.Experimental.FleetDriftEnabled {
		t.Fatal("fleet drift should be disabled by default")
	}
	if cfg.Experimental.AutonomousRemediationEnabled {
		t.Fatal("autonomous remediation should be disabled by default")
	}
	if cfg.Experimental.AutonomousRemediationMinScore != 85 {
		t.Fatalf("min score = %d, want 85", cfg.Experimental.AutonomousRemediationMinScore)
	}
}

func TestLoadExperimentalOverrides(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("EXPERIMENTAL_EBPF_TELEMETRY_ENABLED", "true")
	t.Setenv("EXPERIMENTAL_FLEET_DRIFT_ENABLED", "true")
	t.Setenv("EXPERIMENTAL_AUTONOMOUS_REMEDIATION_ENABLED", "true")
	t.Setenv("AUTONOMOUS_REMEDIATION_MIN_RISK_SCORE", "140")
	t.Setenv("AUTONOMOUS_REMEDIATION_MAX_PROPOSALS", "0")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Experimental.EBPFTelemetryEnabled || !cfg.Experimental.FleetDriftEnabled ||
		!cfg.Experimental.AutonomousRemediationEnabled {
		t.Fatalf("experimental toggles not enabled: %+v", cfg.Experimental)
	}
	if cfg.Experimental.AutonomousRemediationMinScore != 100 {
		t.Fatalf("min score = %d, want clamp to 100", cfg.Experimental.AutonomousRemediationMinScore)
	}
	if cfg.Experimental.AutonomousRemediationMaxItems != 5 {
		t.Fatalf("max proposals = %d, want fallback 5", cfg.Experimental.AutonomousRemediationMaxItems)
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

func TestLoadOIDCRequiresIssuerForGenericProvider(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("AUTH_PROVIDER", "oidc")
	t.Setenv("AUTH_OIDC_CLIENT_ID", "kubelens-web")
	t.Setenv("AUTH_OIDC_ENABLED", "true")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when generic OIDC provider has no issuer")
	}
	if err.Error() != "AUTH_OIDC_ISSUER_URL is required for this OIDC provider" {
		t.Fatalf("error = %q", err.Error())
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

func TestLoadOIDCKnownProviderAllowsDerivedIssuer(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("AUTH_PROVIDER", "google")
	t.Setenv("AUTH_OIDC_CLIENT_ID", "kubelens-web")
	t.Setenv("AUTH_OIDC_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Auth.OIDC.Enabled {
		t.Fatal("expected OIDC to be enabled")
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
		"MEMORY_STORE",
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
		"AUDIT_STORE",
		"AUDIT_LOG_FILE",
		"ALERT_TIMEOUT_SECONDS",
		"ALERTMANAGER_WEBHOOK_URL",
		"SLACK_WEBHOOK_URL",
		"PAGERDUTY_EVENTS_URL",
		"PAGERDUTY_ROUTING_KEY",
		"EXPERIMENTAL_EBPF_TELEMETRY_ENABLED",
		"EXPERIMENTAL_FLEET_DRIFT_ENABLED",
		"EXPERIMENTAL_AUTONOMOUS_REMEDIATION_ENABLED",
		"AUTONOMOUS_REMEDIATION_MIN_RISK_SCORE",
		"AUTONOMOUS_REMEDIATION_MAX_PROPOSALS",
		"WRITE_ACTIONS_ENABLED",
		"AUDIT_SIGNING_KEY",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}
}
