package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"time"

	"kubelens-backend/internal/model"
)

func Load() (Config, error) {
	cfg := Config{}
	now := time.Now().UTC()

	mode := parseMode(os.Getenv("APP_MODE"))
	devMode := parseBoolDefault(os.Getenv("DEV_MODE"), false)
	p := profileForMode(mode)

	cfg.Mode = mode
	cfg.DevMode = devMode
	cfg.Port = parsePort(os.Getenv("PORT"))
	cfg.DistDir = strings.TrimSpace(defaultIfEmpty(os.Getenv("DIST_DIR"), "dist"))
	cfg.Cluster = parseClusterConfig()
	cfg.Build = model.BuildInfo{
		Version: defaultIfEmpty(strings.TrimSpace(os.Getenv("APP_VERSION")), "dev"),
		Commit:  defaultIfEmpty(strings.TrimSpace(os.Getenv("APP_COMMIT")), "local"),
		BuiltAt: defaultIfEmpty(strings.TrimSpace(os.Getenv("APP_BUILT_AT")), now.Format(time.RFC3339)),
	}

	embeddingAPIKey := strings.TrimSpace(os.Getenv("ASSISTANT_EMBEDDING_API_KEY"))
	embeddingBaseURL := strings.TrimSpace(firstNonEmpty(
		os.Getenv("OLLAMA_BASE_URL"),
		os.Getenv("ASSISTANT_EMBEDDING_BASE_URL"),
	))
	embeddingModel := strings.TrimSpace(firstNonEmpty(
		os.Getenv("OLLAMA_EMBEDDING_MODEL"),
		os.Getenv("ASSISTANT_EMBEDDING_MODEL"),
	))
	if embeddingBaseURL != "" && embeddingModel == "" {
		embeddingModel = "nomic-embed-text"
	}

	cfg.Assistant = AssistantConfig{
		Provider:           strings.ToLower(strings.TrimSpace(defaultIfEmpty(os.Getenv("ASSISTANT_PROVIDER"), "none"))),
		Timeout:            parseSecondsAsDuration(os.Getenv("ASSISTANT_TIMEOUT_SECONDS"), 8*time.Second),
		APIBaseURL:         strings.TrimSpace(defaultIfEmpty(os.Getenv("ASSISTANT_API_BASE_URL"), "https://api.openai.com/v1")),
		APIKey:             strings.TrimSpace(os.Getenv("ASSISTANT_API_KEY")),
		Model:              strings.TrimSpace(os.Getenv("ASSISTANT_MODEL")),
		Temperature:        parseFloatDefault(os.Getenv("ASSISTANT_TEMPERATURE"), 0.2),
		AssistantMaxTokens: getEnvInt("ASSISTANT_MAX_TOKENS", 2048),
		RAGEnabled:         parseBoolDefault(os.Getenv("ASSISTANT_RAG_ENABLED"), p.ragEnabled),
		PromptTimeout:      parseSecondsAsDuration(os.Getenv("ASSISTANT_PROMPT_TIMEOUT_SECONDS"), 8*time.Second),
		EmbeddingModel:     embeddingModel,
		EmbeddingBaseURL:   embeddingBaseURL,
		EmbeddingAPIKey:    embeddingAPIKey,
	}

	cfg.Predictor = PredictorConfig{
		BaseURL:                strings.TrimSpace(os.Getenv("PREDICTOR_BASE_URL")),
		Timeout:                parseSecondsAsDuration(os.Getenv("PREDICTOR_TIMEOUT_SECONDS"), 4*time.Second),
		SharedSecret:           strings.TrimSpace(os.Getenv("PREDICTOR_SHARED_SECRET")),
		Mode:                   strings.ToLower(strings.TrimSpace(defaultIfEmpty(os.Getenv("PREDICTOR_MODE"), "deterministic"))),
		ModelMetadataPath:      strings.TrimSpace(os.Getenv("PREDICTOR_MODEL_METADATA_PATH")),
		MinFeatureCompleteness: parseFloatDefault(os.Getenv("PREDICTOR_MIN_FEATURE_COMPLETENESS"), 0.80),
		MaxModelAge:            parseHoursAsDuration(os.Getenv("PREDICTOR_MAX_MODEL_AGE_HOURS"), 168*time.Hour),
	}
	if cfg.Predictor.MinFeatureCompleteness < 0 {
		cfg.Predictor.MinFeatureCompleteness = 0
	} else if cfg.Predictor.MinFeatureCompleteness > 1 {
		cfg.Predictor.MinFeatureCompleteness = 1
	}
	cfg.Ghost = GhostConfig{
		Enabled:    parseBoolDefault(os.Getenv("GHOST_ENABLED"), true),
		EngineAddr: strings.TrimSpace(os.Getenv("GHOST_ENGINE_ADDR")),
		Timeout:    parseSecondsAsDuration(os.Getenv("GHOST_ENGINE_TIMEOUT_SECONDS"), 5*time.Second),
	}

	dbPath := strings.TrimSpace(firstNonEmpty(
		os.Getenv("DB_PATH"),
		"data/kubelens.db",
	))
	cfg.DBPath = dbPath
	cfg.Database = DatabaseConfig{
		Driver:         strings.ToLower(strings.TrimSpace(defaultIfEmpty(os.Getenv("DATABASE_DRIVER"), "sqlite"))),
		URL:            strings.TrimSpace(os.Getenv("DATABASE_URL")),
		SQLitePath:     dbPath,
		MigrationsAuto: parseBoolDefault(os.Getenv("DATABASE_MIGRATIONS_AUTO"), true),
	}

	cfg.Memory = MemoryConfig{
		FilePath: strings.TrimSpace(firstNonEmpty(
			os.Getenv("MEMORY_FILE_PATH"),
			"data/memory-runbooks.json",
		)),
	}

	cfg.ChatOps = ChatOpsConfig{
		SlackWebhookURL: strings.TrimSpace(os.Getenv("CHATOPS_SLACK_WEBHOOK_URL")),
		BaseURL: strings.TrimSpace(defaultIfEmpty(
			os.Getenv("CHATOPS_BASE_URL"),
			"http://localhost:5173",
		)),
		NotifyIncidents:      parseBoolDefault(os.Getenv("CHATOPS_NOTIFY_INCIDENTS"), true),
		NotifyRemediations:   parseBoolDefault(os.Getenv("CHATOPS_NOTIFY_REMEDIATIONS"), true),
		NotifyPostmortems:    parseBoolDefault(os.Getenv("CHATOPS_NOTIFY_POSTMORTEMS"), true),
		NotifyAssistantFinds: parseBoolDefault(os.Getenv("CHATOPS_NOTIFY_ASSISTANT_FINDINGS"), false),
	}

	authEnabled := parseBoolDefault(os.Getenv("AUTH_ENABLED"), p.authEnabled)
	tokens := parseAuthTokens(os.Getenv("AUTH_TOKENS"))

	oidcProvider := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		os.Getenv("AUTH_PROVIDER"),
		os.Getenv("AUTH_OIDC_PROVIDER"),
	)))
	oidcIssuer := strings.TrimSpace(os.Getenv("AUTH_OIDC_ISSUER_URL"))
	oidcClientID := strings.TrimSpace(os.Getenv("AUTH_OIDC_CLIENT_ID"))
	oidcUsernameClaim := strings.TrimSpace(os.Getenv("AUTH_OIDC_USERNAME_CLAIM"))
	oidcRoleClaim := strings.TrimSpace(os.Getenv("AUTH_OIDC_ROLE_CLAIM"))
	oidcEnabled := parseBoolDefault(os.Getenv("AUTH_OIDC_ENABLED"), false)
	if oidcProvider != "" || oidcIssuer != "" {
		oidcEnabled = true
	}

	if authEnabled && len(tokens) == 0 {
		if oidcEnabled {
			// OIDC auth does not require static tokens.
		} else if devMode {
			tokens = []AuthToken{
				{Token: "kubelens-viewer", User: "viewer", Role: "viewer"},
				{Token: "kubelens-operator", User: "operator", Role: "operator"},
				{Token: "kubelens-admin", User: "admin", Role: "admin"},
			}
		} else {
			return Config{}, errors.New("AUTH_ENABLED=true requires AUTH_TOKENS unless DEV_MODE=true")
		}
	}
	cfg.Auth = AuthConfig{
		Enabled:            authEnabled,
		AllowHeaderToken:   parseBoolDefault(os.Getenv("AUTH_ALLOW_HEADER_TOKEN"), devMode),
		TrustedCSRFDomains: parseCSV(os.Getenv("AUTH_TRUSTED_CSRF_DOMAINS")),
		TrustedProxyCIDRs:  parseCSV(os.Getenv("AUTH_TRUSTED_PROXY_CIDRS")),
		Tokens:             tokens,
		OIDC: AuthOIDCConfig{
			Enabled:       oidcEnabled,
			Provider:      oidcProvider,
			IssuerURL:     oidcIssuer,
			ClientID:      oidcClientID,
			UsernameClaim: oidcUsernameClaim,
			RoleClaim:     oidcRoleClaim,
		},
	}

	cfg.RateLimit = RateLimitConfig{
		Enabled:  parseBoolDefault(os.Getenv("RATE_LIMIT_ENABLED"), p.rateLimitEnabled),
		Requests: parseIntDefault(os.Getenv("RATE_LIMIT_REQUESTS"), p.rateLimitRequests),
		Window:   parseSecondsAsDuration(os.Getenv("RATE_LIMIT_WINDOW_SECONDS"), time.Duration(p.rateLimitWindowSec)*time.Second),
	}

	cfg.Audit = AuditConfig{
		MaxItems: parseIntDefault(os.Getenv("AUDIT_MAX_ITEMS"), 500),
		FilePath: strings.TrimSpace(os.Getenv("AUDIT_LOG_FILE")),
	}

	cfg.Alerts = AlertsConfig{
		Timeout:             parseSecondsAsDuration(os.Getenv("ALERT_TIMEOUT_SECONDS"), 5*time.Second),
		AlertmanagerURL:     strings.TrimSpace(os.Getenv("ALERTMANAGER_WEBHOOK_URL")),
		SlackWebhookURL:     strings.TrimSpace(os.Getenv("SLACK_WEBHOOK_URL")),
		PagerDutyEventsURL:  strings.TrimSpace(defaultIfEmpty(os.Getenv("PAGERDUTY_EVENTS_URL"), "https://events.pagerduty.com/v2/enqueue")),
		PagerDutyRoutingKey: strings.TrimSpace(os.Getenv("PAGERDUTY_ROUTING_KEY")),
	}

	tracingEndpoint := firstNonEmpty(
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"),
		os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	)
	tracingProtocol := firstNonEmpty(
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL"),
		os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"),
	)
	tracingInsecure := parseBoolDefault(
		firstNonEmpty(
			os.Getenv("OTEL_EXPORTER_OTLP_TRACES_INSECURE"),
			os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"),
		),
		true,
	)
	tracingService := defaultIfEmpty(strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")), "kubelens-backend")
	tracingSample := parseFloatDefault(os.Getenv("OTEL_TRACES_SAMPLE_RATIO"), 1.0)
	if tracingSample < 0 {
		tracingSample = 0
	} else if tracingSample > 1 {
		tracingSample = 1
	}
	cfg.Tracing = TracingConfig{
		Endpoint:    strings.TrimSpace(tracingEndpoint),
		Protocol:    strings.TrimSpace(tracingProtocol),
		Insecure:    tracingInsecure,
		ServiceName: tracingService,
		SampleRatio: tracingSample,
	}

	cfg.WriteActionsEnabled = parseBoolDefault(os.Getenv("WRITE_ACTIONS_ENABLED"), p.writeActions)
	cfg.AnonymousPermissions = anonymousPermissionsFor(cfg)

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func getEnvInt(key string, fallback int) int {
	return parseIntDefault(os.Getenv(key), fallback)
}

func validate(cfg Config) error {
	if cfg.Mode == ModeProd && cfg.DevMode {
		return errors.New("DEV_MODE=true is not allowed when APP_MODE=prod")
	}

	if cfg.Mode == ModeProd && !cfg.Auth.Enabled {
		return errors.New("APP_MODE=prod requires AUTH_ENABLED=true")
	}
	if cfg.Mode == ModeProd && cfg.Auth.AllowHeaderToken {
		return errors.New("APP_MODE=prod does not allow AUTH_ALLOW_HEADER_TOKEN=true")
	}
	if cfg.Auth.Enabled && len(cfg.Auth.Tokens) == 0 && !cfg.Auth.OIDC.Enabled {
		return errors.New("AUTH_ENABLED=true requires AUTH_TOKENS or AUTH_OIDC_* configuration")
	}
	if cfg.Auth.Enabled && cfg.Auth.OIDC.Enabled && strings.TrimSpace(cfg.Auth.OIDC.ClientID) == "" {
		return errors.New("AUTH_OIDC_CLIENT_ID is required when OIDC auth is enabled")
	}
	for _, raw := range cfg.Auth.TrustedProxyCIDRs {
		if _, err := netip.ParsePrefix(strings.TrimSpace(raw)); err != nil {
			return fmt.Errorf("invalid AUTH_TRUSTED_PROXY_CIDRS entry %q: %w", raw, err)
		}
	}

	if cfg.WriteActionsEnabled && !cfg.Auth.Enabled {
		return errors.New("WRITE_ACTIONS_ENABLED=true requires AUTH_ENABLED=true")
	}

	if cfg.Assistant.Provider != "" && cfg.Assistant.Provider != "none" && cfg.Assistant.Provider != "openai_compatible" {
		return fmt.Errorf("unsupported ASSISTANT_PROVIDER: %s", cfg.Assistant.Provider)
	}
	if cfg.Database.Driver != "sqlite" && cfg.Database.Driver != "postgres" {
		return fmt.Errorf("unsupported DATABASE_DRIVER: %s", cfg.Database.Driver)
	}
	if cfg.Database.Driver == "postgres" && strings.TrimSpace(cfg.Database.URL) == "" {
		return errors.New("DATABASE_URL is required when DATABASE_DRIVER=postgres")
	}
	if cfg.Predictor.Mode != "deterministic" && cfg.Predictor.Mode != "shadow" && cfg.Predictor.Mode != "blended" {
		return fmt.Errorf("unsupported PREDICTOR_MODE: %s", cfg.Predictor.Mode)
	}

	return nil
}
