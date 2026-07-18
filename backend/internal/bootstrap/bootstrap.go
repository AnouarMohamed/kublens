package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"kubelens-backend/internal/ai"
	"kubelens-backend/internal/alerts"
	"kubelens-backend/internal/auth"
	"kubelens-backend/internal/chatops"
	"kubelens-backend/internal/cluster"
	"kubelens-backend/internal/config"
	storesql "kubelens-backend/internal/db"
	"kubelens-backend/internal/events"
	"kubelens-backend/internal/ghost"
	"kubelens-backend/internal/httpapi"
	"kubelens-backend/internal/incident"
	"kubelens-backend/internal/intelligence"
	"kubelens-backend/internal/memory"
	"kubelens-backend/internal/observability"
	"kubelens-backend/internal/postmortem"
	"kubelens-backend/internal/rag"
	"kubelens-backend/internal/remediation"
	"kubelens-backend/internal/riskguard"
	"kubelens-backend/plugins"
	"kubelens-backend/plugins/crashloop_analyzer"
	"kubelens-backend/plugins/image_pull_analyzer"
	"kubelens-backend/plugins/node_health_analyzer"
	"kubelens-backend/plugins/pod_health_analyzer"
	"kubelens-backend/plugins/resource_analyzer"
	"kubelens-backend/plugins/scheduling_analyzer"
)

type Result struct {
	Server   *http.Server
	Warnings []string
	Shutdown func(context.Context) error
}

func Build(cfg config.Config) (Result, error) {
	warnings := make([]string, 0, 8)
	eventBus := events.NewBus(64)

	sqliteDB, err := storesql.Open(context.Background(), cfg.DBPath)
	if err != nil {
		return Result{}, fmt.Errorf("initialize sqlite store: %w", err)
	}

	shutdownTracing := func(context.Context) error { return nil }
	if strings.TrimSpace(cfg.Tracing.Endpoint) != "" {
		shutdown, err := observability.InitTracing(context.Background(), cfg.Tracing)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("tracing init warning: %v", err))
		} else {
			shutdownTracing = shutdown
		}
	}

	clusterSvc, initErr := cluster.NewServiceWithOptions(
		cfg.Cluster.KubeconfigData,
		cluster.WithEventBus(eventBus),
	)
	if initErr != nil {
		warnings = append(warnings, fmt.Sprintf("cluster initialization warning: %v", initErr))
	}

	clusterContexts := map[string]httpapi.ClusterReader{
		"default": clusterSvc,
	}
	for name, payload := range cfg.Cluster.Contexts {
		svc, err := cluster.NewServiceWithOptions(payload, cluster.WithEventBus(eventBus))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("cluster context %s warning: %v", name, err))
		}
		clusterContexts[name] = svc
	}

	stateCtx, stateCancel := context.WithCancel(context.Background())
	if err := clusterSvc.Start(stateCtx); err != nil {
		warnings = append(warnings, fmt.Sprintf("cluster state cache warning: %v", err))
	}
	for name, reader := range clusterContexts {
		if name == "default" {
			continue
		}
		if svc, ok := reader.(*cluster.Service); ok {
			if err := svc.Start(stateCtx); err != nil {
				warnings = append(warnings, fmt.Sprintf("cluster state cache (%s) warning: %v", name, err))
			}
		}
	}

	aiProvider, providerErr := buildAIProvider(cfg.Assistant)
	if providerErr != nil {
		warnings = append(warnings, fmt.Sprintf("assistant provider warning: %v", providerErr))
	}

	var embeddingClient *rag.EmbeddingClient
	if strings.TrimSpace(cfg.Assistant.EmbeddingBaseURL) != "" {
		client, err := rag.NewEmbeddingClient(
			cfg.Assistant.EmbeddingBaseURL,
			cfg.Assistant.EmbeddingModel,
			nil,
		)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("assistant embedding warning: %v", err))
		} else {
			embeddingClient = client
		}
	}

	ragger := rag.NewService(rag.Config{
		Enabled:         cfg.Assistant.RAGEnabled,
		EmbeddingClient: embeddingClient,
	})
	memoryStore := memory.NewWithEmbeddings(cfg.Memory.FilePath, nil, embeddingClient)
	incidentStore := incident.NewStore(sqliteDB, incident.DefaultStoreLimit, nil)
	remediationStore := remediation.NewStore(sqliteDB, remediation.DefaultStoreLimit, nil)
	postmortemStore := postmortem.NewStore(sqliteDB, postmortem.DefaultStoreLimit, nil)
	riskAnalyzer := riskguard.NewAnalyzer()
	chatopsNotifier := chatops.NewSlackNotifier(chatops.Config{
		SlackWebhookURL:      cfg.ChatOps.SlackWebhookURL,
		BaseURL:              cfg.ChatOps.BaseURL,
		NotifyIncidents:      cfg.ChatOps.NotifyIncidents,
		NotifyRemediations:   cfg.ChatOps.NotifyRemediations,
		NotifyPostmortems:    cfg.ChatOps.NotifyPostmortems,
		NotifyAssistantFinds: cfg.ChatOps.NotifyAssistantFinds,
	}, nil, nil)
	alertDispatcher := alerts.New(alerts.Config{
		AlertmanagerURL:     cfg.Alerts.AlertmanagerURL,
		SlackWebhookURL:     cfg.Alerts.SlackWebhookURL,
		PagerDutyEventsURL:  cfg.Alerts.PagerDutyEventsURL,
		PagerDutyRoutingKey: cfg.Alerts.PagerDutyRoutingKey,
		Timeout:             cfg.Alerts.Timeout,
	})

	runtime := config.RuntimeStatus(cfg, clusterSvc.IsRealCluster(), alertDispatcher.Enabled())
	diagnosticAnalyzer := buildAnalyzer()

	var ghostClient *ghost.Client
	if cfg.Ghost.Enabled && cfg.Ghost.EngineAddr != "" {
		ghostClient = ghost.NewClient(cfg.Ghost.EngineAddr, cfg.Ghost.Timeout)
	}

	handler := httpapi.New(
		clusterSvc,
		httpapi.WithGhostClient(ghostClient),
		httpapi.WithAIProvider(aiProvider),
		httpapi.WithAITimeout(cfg.Assistant.Timeout),
		httpapi.WithDocsRetriever(ragger),
		httpapi.WithMemoryStore(memoryStore),
		httpapi.WithIncidentStore(incidentStore),
		httpapi.WithRemediationStore(remediationStore),
		httpapi.WithRiskAnalyzer(riskAnalyzer),
		httpapi.WithPostmortemStore(postmortemStore),
		httpapi.WithChatOpsNotifier(chatopsNotifier),
		httpapi.WithPredictor(cfg.Predictor.BaseURL, cfg.Predictor.Timeout, cfg.Predictor.SharedSecret),
		httpapi.WithAuth(toHTTPAuth(cfg.Auth)),
		httpapi.WithTrustedProxyCIDRs(cfg.Auth.TrustedProxyCIDRs),
		httpapi.WithRateLimit(httpapi.RateLimitConfig{
			Enabled:  cfg.RateLimit.Enabled,
			Requests: cfg.RateLimit.Requests,
			Window:   cfg.RateLimit.Window,
		}),
		httpapi.WithAuditConfig(httpapi.AuditConfig{
			MaxItems:   cfg.Audit.MaxItems,
			FilePath:   cfg.Audit.FilePath,
			SigningKey: cfg.Audit.SigningKey,
		}),
		httpapi.WithAlertDispatcher(alertDispatcher),
		httpapi.WithSQLiteDB(sqliteDB),
		httpapi.WithEventBus(eventBus),
		httpapi.WithIntelligence(diagnosticAnalyzer),
		httpapi.WithClusterContexts(httpapi.ClusterContextsConfig{
			Default: "default",
			Readers: clusterContexts,
		}),
		httpapi.WithBuildInfo(cfg.Build),
		httpapi.WithRuntimeStatus(runtime),
		httpapi.WithWriteActionsEnabled(cfg.WriteActionsEnabled),
		httpapi.WithAnonymousPermissions(cfg.AnonymousPermissions),
	)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler.Router(filepath.Clean(cfg.DistDir)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return Result{
		Server:   server,
		Warnings: warnings,
		Shutdown: func(ctx context.Context) error {
			stateCancel()
			return errors.Join(
				shutdownTracing(ctx),
				sqliteDB.Close(),
			)
		},
	}, nil
}

func buildAnalyzer() *intelligence.Analyzer {
	pluginList := []plugins.Plugin{
		crashloop_analyzer.New(),
		image_pull_analyzer.New(),
		node_health_analyzer.New(),
		pod_health_analyzer.New(),
		resource_analyzer.New(),
		scheduling_analyzer.New(),
	}

	runners := make([]intelligence.PluginRunner, 0, len(pluginList))
	for _, plugin := range pluginList {
		p := plugin
		runners = append(runners, intelligence.PluginRunner{
			Name:    p.Name(),
			Analyze: p.Analyze,
		})
	}

	return intelligence.NewAnalyzer(time.Now, runners...)
}

func toHTTPAuth(cfg config.AuthConfig) httpapi.AuthConfig {
	tokens := make([]httpapi.AuthToken, 0, len(cfg.Tokens))
	for _, token := range cfg.Tokens {
		tokens = append(tokens, httpapi.AuthToken{
			Token: token.Token,
			User:  token.User,
			Role:  token.Role,
		})
	}

	return httpapi.AuthConfig{
		Enabled:            cfg.Enabled,
		AllowHeaderToken:   cfg.AllowHeaderToken,
		TrustedCSRFDomains: cfg.TrustedCSRFDomains,
		Tokens:             tokens,
		OIDC: auth.OIDCConfig{
			Enabled:       cfg.OIDC.Enabled,
			Provider:      cfg.OIDC.Provider,
			IssuerURL:     cfg.OIDC.IssuerURL,
			ClientID:      cfg.OIDC.ClientID,
			UsernameClaim: cfg.OIDC.UsernameClaim,
			RoleClaim:     cfg.OIDC.RoleClaim,
		},
	}
}

func buildAIProvider(cfg config.AssistantConfig) (ai.Provider, error) {
	kind := cfg.Provider
	if kind == "" || kind == "none" {
		return nil, nil
	}

	switch kind {
	case "openai_compatible":
		provider, err := ai.NewOpenAICompatibleProvider(ai.OpenAICompatibleConfig{
			BaseURL:     cfg.APIBaseURL,
			APIKey:      cfg.APIKey,
			Model:       cfg.Model,
			Temperature: cfg.Temperature,
			MaxTokens:   cfg.AssistantMaxTokens,
		})
		if err != nil {
			return nil, err
		}
		return provider, nil
	default:
		return nil, errors.New("unsupported ASSISTANT_PROVIDER: " + kind)
	}
}
