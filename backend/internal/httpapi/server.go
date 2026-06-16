package httpapi

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"kubelens-backend/internal/ai"
	storesql "kubelens-backend/internal/db"
	"kubelens-backend/internal/events"
	"kubelens-backend/internal/intelligence"
	"kubelens-backend/internal/model"
	"kubelens-backend/internal/state"
)

type ClusterReader interface {
	IsRealCluster() bool
	Snapshot(ctx context.Context) ([]model.PodSummary, []model.NodeSummary)
	ListNamespaces(ctx context.Context) []string
	ListResources(ctx context.Context, kind string) ([]model.ResourceRecord, error)
	ListClusterEvents(ctx context.Context) []model.K8sEvent
	GetResourceYAML(ctx context.Context, kind, namespace, name string) (string, error)
	ApplyResourceYAML(ctx context.Context, kind, namespace, name, manifestYAML string) (model.ActionResult, error)
	ScaleResource(ctx context.Context, kind, namespace, name string, replicas int32) (model.ActionResult, error)
	RestartResource(ctx context.Context, kind, namespace, name string) (model.ActionResult, error)
	RollbackResource(ctx context.Context, kind, namespace, name string) (model.ActionResult, error)
	PodEvents(ctx context.Context, namespace, name string) []model.K8sEvent
	PodLogs(ctx context.Context, namespace, name, container string, lines int) string
	StreamPodLogs(ctx context.Context, namespace, name, container string, tailLines int, follow bool) (io.ReadCloser, error)
	PodDetail(ctx context.Context, namespace, name string) (model.PodDetail, error)
	NodeDetail(ctx context.Context, name string) (model.NodeDetail, error)
	CreatePod(ctx context.Context, req model.PodCreateRequest) (model.ActionResult, error)
	RestartPod(ctx context.Context, namespace, name string) (model.ActionResult, error)
	DeletePod(ctx context.Context, namespace, name string) (model.ActionResult, error)
	CordonNode(ctx context.Context, name string) (model.ActionResult, error)
	StateSnapshot(ctx context.Context) (state.ClusterState, bool)
}

type Server struct {
	cluster        ClusterReader
	now            func() time.Time
	logger         *slog.Logger
	metrics        *requestMetrics
	runtime        model.RuntimeStatus
	auth           authRuntime
	authLogin      *authLoginProtection
	trustedProxies trustedProxyMatcher
	limiter        rateLimiter
	writesOn       bool
	anonPerms      []string
	sqliteDB       *sql.DB
	audit          *auditLog
	eventBus       *events.Bus
	alerts         alertDispatcher
	alertLifecycle alertLifecycleStateStore
	ai             ai.Provider
	aiTTL          time.Duration
	docs           docsRetriever
	memory         memoryStore
	incidents      incidentStore
	remediations   remediationStore
	riskGuard      riskAnalyzer
	postmortems    postmortemStore
	chatops        chatopsNotifier
	predictor      predictionProvider
	buildInfo      model.BuildInfo
	intel          *intelligence.Analyzer
	ghostClient    ghostClient

	predictionsTTL   time.Duration
	predictionsMu    sync.RWMutex
	predictionsCache predictionsCacheEntry

	predictorHealthMu sync.RWMutex
	predictorHealth   predictorHealthState
}

type predictionsCacheEntry struct {
	data      model.PredictionsResult
	expiresAt time.Time
}

type predictorHealthState struct {
	enabled     bool
	lastSuccess time.Time
	lastFailure time.Time
	lastError   string
}

type docsRetriever interface {
	Enabled() bool
	Retrieve(ctx context.Context, query string, limit int) []model.DocumentationReference
}

type memoryStore interface {
	Search(query string) []model.MemoryRunbook
	IncrementUsage(id string) bool
	CreateRunbook(req model.MemoryRunbookUpsertRequest) (model.MemoryRunbook, error)
	UpdateRunbook(id string, req model.MemoryRunbookUpsertRequest) (model.MemoryRunbook, error)
	ListFixes() []model.MemoryFixPattern
	RecordFix(req model.MemoryFixCreateRequest, recordedBy string) (model.MemoryFixPattern, error)
}

type incidentStore interface {
	Create(incident model.Incident) model.Incident
	List() []model.Incident
	Get(id string) (model.Incident, bool)
	PatchStepStatus(id string, stepID string, target model.RunbookStepStatus) (model.Incident, error)
	Resolve(id string) (model.Incident, error)
	AssociateRemediation(incidentID string, proposalID string) error
}

type remediationStore interface {
	SaveProposals(proposals []model.RemediationProposal) []model.RemediationProposal
	List() []model.RemediationProposal
	Get(id string) (model.RemediationProposal, bool)
	Approve(id string, user string) (model.RemediationProposal, error)
	Reject(id string, user string, reason string) (model.RemediationProposal, error)
	MarkExecuted(id string, user string, result string) (model.RemediationProposal, error)
	GetGitOpsArtifact(proposalID string) (model.RemediationGitOpsArtifact, bool)
	UpsertGitOpsArtifact(proposalID string, artifact model.GitOpsArtifact, generatedBy string) (model.RemediationGitOpsArtifact, error)
}

type riskAnalyzer interface {
	Analyze(manifest string, pods []model.PodSummary, nodes []model.NodeSummary) model.RiskReport
}

type postmortemStore interface {
	Create(postmortem model.Postmortem) (model.Postmortem, error)
	List() []model.Postmortem
	Get(id string) (model.Postmortem, bool)
	GetByIncidentID(incidentID string) (model.Postmortem, bool)
}

type chatopsNotifier interface {
	Enabled() bool
	NotifyIncident(ctx context.Context, incident model.Incident)
	NotifyRemediation(ctx context.Context, proposal model.RemediationProposal)
	NotifyPostmortem(ctx context.Context, postmortem model.Postmortem)
	NotifyAssistantFinding(ctx context.Context, finding string, resources []string)
}

type alertDispatcher interface {
	Dispatch(ctx context.Context, req model.AlertDispatchRequest) model.AlertDispatchResponse
	Enabled() bool
}

type alertLifecycleStateStore interface {
	List(ctx context.Context) []model.NodeAlertLifecycle
	Upsert(ctx context.Context, req model.NodeAlertLifecycleUpdateRequest, actor string) (model.NodeAlertLifecycle, error)
}

type Option func(*Server)

func WithAIProvider(provider ai.Provider) Option {
	return func(s *Server) {
		s.ai = provider
	}
}

func WithAITimeout(timeout time.Duration) Option {
	return func(s *Server) {
		if timeout > 0 {
			s.aiTTL = timeout
		}
	}
}

func WithDocsRetriever(retriever docsRetriever) Option {
	return func(s *Server) {
		s.docs = retriever
	}
}

func WithMemoryStore(store memoryStore) Option {
	return func(s *Server) {
		s.memory = store
	}
}

func WithIncidentStore(store incidentStore) Option {
	return func(s *Server) {
		s.incidents = store
	}
}

func WithRemediationStore(store remediationStore) Option {
	return func(s *Server) {
		s.remediations = store
	}
}

func WithRiskAnalyzer(analyzer riskAnalyzer) Option {
	return func(s *Server) {
		s.riskGuard = analyzer
	}
}

func WithPostmortemStore(store postmortemStore) Option {
	return func(s *Server) {
		s.postmortems = store
	}
}

func WithChatOpsNotifier(notifier chatopsNotifier) Option {
	return func(s *Server) {
		s.chatops = notifier
	}
}

func WithPredictor(baseURL string, timeout time.Duration, sharedSecret string) Option {
	return func(s *Server) {
		if baseURL == "" {
			return
		}
		s.predictor = newPredictorClient(baseURL, timeout, sharedSecret)
		s.predictorHealthMu.Lock()
		s.predictorHealth.enabled = true
		s.predictorHealthMu.Unlock()
	}
}

type ghostClient interface {
	Simulate(ctx context.Context, req model.GhostSimulationRequest, topology model.GhostTopology) (model.GhostSimulationResult, error)
}

func WithGhostClient(client ghostClient) Option {
	return func(s *Server) {
		s.ghostClient = client
	}
}

func WithAlertDispatcher(dispatcher alertDispatcher) Option {
	return func(s *Server) {
		s.alerts = dispatcher
	}
}

func WithSQLiteDB(handle *sql.DB) Option {
	return func(s *Server) {
		s.sqliteDB = handle
	}
}

func WithEventBus(bus *events.Bus) Option {
	return func(s *Server) {
		s.eventBus = bus
	}
}

func WithIntelligence(analyzer *intelligence.Analyzer) Option {
	return func(s *Server) {
		s.intel = analyzer
	}
}

func WithPredictionsTTL(ttl time.Duration) Option {
	return func(s *Server) {
		if ttl > 0 {
			s.predictionsTTL = ttl
		}
	}
}

func WithBuildInfo(info model.BuildInfo) Option {
	return func(s *Server) {
		if info.Version != "" {
			s.buildInfo.Version = info.Version
		}
		if info.Commit != "" {
			s.buildInfo.Commit = info.Commit
		}
		if info.BuiltAt != "" {
			s.buildInfo.BuiltAt = info.BuiltAt
		}
	}
}

func WithRuntimeStatus(status model.RuntimeStatus) Option {
	return func(s *Server) {
		s.runtime = status
	}
}

func WithWriteActionsEnabled(enabled bool) Option {
	return func(s *Server) {
		s.writesOn = enabled
	}
}

func WithAnonymousPermissions(permissions []string) Option {
	return func(s *Server) {
		s.anonPerms = append([]string(nil), permissions...)
	}
}

func New(clusterSvc ClusterReader, opts ...Option) *Server {
	return newServer(clusterSvc, time.Now, slog.New(slog.NewJSONHandler(os.Stdout, nil)), opts...)
}

func newServer(clusterSvc ClusterReader, now func() time.Time, logger *slog.Logger, opts ...Option) *Server {
	if now == nil {
		now = time.Now
	}
	if logger == nil {
		logger = slog.Default()
	}

	server := &Server{
		cluster:        clusterSvc,
		now:            now,
		logger:         logger,
		metrics:        newRequestMetrics(now),
		authLogin:      newAuthLoginProtection(defaultAuthLoginProtectionConfig()),
		audit:          newAuditLog(maxAuditLimit, "", logger),
		eventBus:       events.NewBus(64),
		aiTTL:          8 * time.Second,
		predictionsTTL: 8 * time.Second,
		buildInfo: model.BuildInfo{
			Version: "dev",
			Commit:  "local",
			BuiltAt: now().UTC().Format(time.RFC3339),
		},
		writesOn:  false,
		anonPerms: []string{"read", "assist", "stream"},
		runtime: model.RuntimeStatus{
			Mode:                "demo",
			Insecure:            true,
			WriteActionsEnabled: false,
			PredictorHealthy:    true,
		},
	}
	server.limiter.configure(RateLimitConfig{
		Enabled:  true,
		Requests: 300,
		Window:   time.Minute,
	})

	for _, opt := range opts {
		opt(server)
	}

	if server.eventBus == nil {
		server.eventBus = events.NewBus(64)
	}
	if server.alertLifecycle == nil {
		server.alertLifecycle = server.defaultAlertLifecycleStore()
	}

	return server
}

func (s *Server) defaultAlertLifecycleStore() alertLifecycleStateStore {
	if s.sqliteDB == nil {
		handle, err := storesql.Open(context.Background(), ":memory:")
		if err != nil {
			s.logger.Error("initialize alert lifecycle sqlite store", "error", err)
			return nil
		}
		s.sqliteDB = handle
	}

	return newAlertLifecycleStore(s.sqliteDB, defaultAlertLifecycleLimit, s.now)
}

func (s *Server) Router(distDir string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(s.securityHeadersMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(timeoutUnlessPath(20*time.Second, func(path string) bool {
		return strings.HasPrefix(path, apiStreamPrefix) || strings.HasSuffix(path, apiPodLogsStreamSuffix)
	}))
	r.Use(s.limiter.middlewareWithKey(s.now, s.clientIPFromRequest))
	r.Use(s.metrics.middleware(s.logger))
	r.Use(s.authMiddleware)
	r.Use(s.clusterMiddleware)
	r.Use(s.auditMiddleware)

	// Mutating endpoints are guarded centrally by auth middleware:
	// 1) RBAC role check via auth.RequiredRole
	// 2) environment write gate via s.writesOn
	// Non-mutating POST exceptions are documented in auth.RequiredRole.
	r.Route(apiMountPrefix, s.mountAPIRoutes)

	attachStatic(r, distDir)
	return otelhttp.NewHandler(
		r,
		"http.server",
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			route := routePattern(r)
			if route == "" {
				route = operation
			}
			return r.Method + " " + route
		}),
	)
}
