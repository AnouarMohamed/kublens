package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"kubelens-backend/internal/model"
	"kubelens-backend/internal/redact"
)

const (
	defaultPredictorTimeout = 4 * time.Second
	maxPredictionItems      = 10
)

type predictorRequest struct {
	Pods      []model.PodSummary  `json:"pods"`
	Nodes     []model.NodeSummary `json:"nodes"`
	Events    []model.K8sEvent    `json:"events"`
	Timestamp string              `json:"timestamp"`
}

type predictionProvider interface {
	Predict(ctx context.Context, input predictorRequest) (model.PredictionsResult, error)
}

type predictorModelHealthProvider interface {
	ModelHealth(ctx context.Context) (model.PredictorModelHealth, error)
}

type PredictionController struct {
	cluster              ClusterReader
	predictor            predictionProvider
	logger               *slog.Logger
	now                  func() time.Time
	predictionsFromCache func() (model.PredictionsResult, bool)
	storePredictions     func(model.PredictionsResult)
	recordSuccess        func()
	recordFailure        func(error)
}

func NewPredictionController(
	cluster ClusterReader,
	predictor predictionProvider,
	logger *slog.Logger,
	now func() time.Time,
	predictionsFromCache func() (model.PredictionsResult, bool),
	storePredictions func(model.PredictionsResult),
	recordSuccess func(),
	recordFailure func(error),
) *PredictionController {
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	if predictionsFromCache == nil {
		predictionsFromCache = func() (model.PredictionsResult, bool) { return model.PredictionsResult{}, false }
	}
	if storePredictions == nil {
		storePredictions = func(model.PredictionsResult) {}
	}
	if recordSuccess == nil {
		recordSuccess = func() {}
	}
	if recordFailure == nil {
		recordFailure = func(error) {}
	}
	return &PredictionController{
		cluster:              cluster,
		predictor:            predictor,
		logger:               logger,
		now:                  now,
		predictionsFromCache: predictionsFromCache,
		storePredictions:     storePredictions,
		recordSuccess:        recordSuccess,
		recordFailure:        recordFailure,
	}
}

type predictorClient struct {
	baseURL      string
	sharedSecret string
	client       *http.Client
}

func newPredictorClient(baseURL string, timeout time.Duration, sharedSecret string) *predictorClient {
	if timeout <= 0 {
		timeout = defaultPredictorTimeout
	}

	return &predictorClient{
		baseURL:      strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		sharedSecret: strings.TrimSpace(sharedSecret),
		client: &http.Client{
			Timeout:   timeout,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}
}

func (p *predictorClient) Predict(ctx context.Context, input predictorRequest) (model.PredictionsResult, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return model.PredictionsResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/predict", bytes.NewReader(body))
	if err != nil {
		return model.PredictionsResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.sharedSecret != "" {
		req.Header.Set("X-Predictor-Secret", p.sharedSecret)
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := p.client.Do(req)
	if err != nil {
		return model.PredictionsResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return model.PredictionsResult{}, fmt.Errorf("predictor status %d", resp.StatusCode)
	}

	var out model.PredictionsResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return model.PredictionsResult{}, err
	}
	if out.GeneratedAt == "" {
		out.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if out.Source == "" {
		out.Source = "python-service"
	}

	return out, nil
}

func (p *predictorClient) ModelHealth(ctx context.Context) (model.PredictorModelHealth, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/model", nil)
	if err != nil {
		return model.PredictorModelHealth{}, err
	}
	if p.sharedSecret != "" {
		req.Header.Set("X-Predictor-Secret", p.sharedSecret)
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := p.client.Do(req)
	if err != nil {
		return model.PredictorModelHealth{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return model.PredictorModelHealth{}, fmt.Errorf("predictor model status %d", resp.StatusCode)
	}

	var out model.PredictorModelHealth
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return model.PredictorModelHealth{}, err
	}
	out.Source = fallbackHealthSource(out.Source, "python-service")
	if out.RequiredFeatures == nil {
		out.RequiredFeatures = []string{}
	}
	if out.EvaluationMetrics == nil {
		out.EvaluationMetrics = map[string]float64{}
	}
	if out.PromotionGates == nil {
		out.PromotionGates = map[string]float64{}
	}
	return out, nil
}

func (s *Server) handlePredictorModelHealth(w http.ResponseWriter, r *http.Request) {
	runtime := s.runtimeSnapshot()
	fallback := model.PredictorModelHealth{
		Source:                 "backend",
		Mode:                   fallbackHealthSource(runtime.PredictorMode, "deterministic"),
		Enabled:                runtime.PredictorEnabled,
		UsableForBlending:      false,
		ModelLoaded:            false,
		MetadataLoaded:         false,
		ModelVersion:           "deterministic",
		Stale:                  false,
		MaxModelAgeHours:       0,
		MinFeatureCompleteness: 0,
		RequiredFeatures:       []string{},
		CalibrationMethod:      "",
		EvaluationMetrics:      map[string]float64{},
		PromotionGates:         map[string]float64{},
		LastError:              runtime.PredictorLastError,
	}
	provider, ok := s.predictor.(predictorModelHealthProvider)
	if !ok || s.predictor == nil {
		writeJSON(w, http.StatusOK, fallback)
		return
	}

	health, err := provider.ModelHealth(r.Context())
	if err != nil {
		s.recordPredictorFailure(err)
		fallback.Source = "backend-unavailable"
		fallback.LastError = redact.Error(err)
		writeJSON(w, http.StatusOK, fallback)
		return
	}
	s.recordPredictorSuccess()
	writeJSON(w, http.StatusOK, health)
}

func (pc *PredictionController) handlePredictions(w http.ResponseWriter, r *http.Request) {
	forceRefresh := queryBool(r, "force")
	if !forceRefresh {
		if cached, ok := pc.predictionsFromCache(); ok {
			writeJSON(w, http.StatusOK, cached)
			return
		}
	}

	pods, nodes := pc.cluster.Snapshot(r.Context())
	events := pc.cluster.ListClusterEvents(r.Context())
	request := predictorRequest{
		Pods:      pods,
		Nodes:     nodes,
		Events:    events,
		Timestamp: pc.now().UTC().Format(time.RFC3339),
	}

	if pc.predictor != nil {
		predictions, err := pc.predictor.Predict(r.Context(), request)
		if err == nil {
			pc.recordSuccess()
			pc.storePredictions(predictions)
			writeJSON(w, http.StatusOK, predictions)
			return
		}
		pc.recordFailure(err)
		pc.logger.Warn("predictor service unavailable, using local fallback", "error", redact.Error(err))
	}

	fallback := buildLocalPredictions(pods, nodes, events, pc.now())
	pc.storePredictions(fallback)
	writeJSON(w, http.StatusOK, fallback)
}

func queryBool(r *http.Request, key string) bool {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func fallbackHealthSource(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func (s *Server) invalidatePredictionsCache() {
	if s.predictionsTTL <= 0 {
		return
	}

	s.predictionsMu.Lock()
	s.predictionsCache = predictionsCacheEntry{}
	s.predictionsMu.Unlock()
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

type eventIndex struct {
	exactMap     map[string][]model.K8sEvent
	fallbackList []model.K8sEvent
}

func buildEventIndex(events []model.K8sEvent) eventIndex {
	exactMap := make(map[string][]model.K8sEvent)
	var fallbackList []model.K8sEvent

	for _, event := range events {
		eventType := strings.ToLower(strings.TrimSpace(event.Type))
		reason := strings.ToLower(strings.TrimSpace(event.Reason))
		if eventType != "warning" && reason != "backoff" && reason != "failed" && reason != "unhealthy" && reason != "oomkilled" {
			continue
		}

		resName := strings.ToLower(strings.TrimSpace(event.Resource))
		if resName != "" {
			nsName := strings.ToLower(strings.TrimSpace(event.Namespace))
			key := nsName + "/" + resName
			exactMap[key] = append(exactMap[key], event)
		} else {
			fallbackList = append(fallbackList, event)
		}
	}

	return eventIndex{
		exactMap:     exactMap,
		fallbackList: fallbackList,
	}
}

func countResourceWarningEventsIndexed(idx eventIndex, resource, namespace string) int {
	resourceName := strings.ToLower(strings.TrimSpace(resource))
	if resourceName == "" {
		return 0
	}
	namespaceName := strings.ToLower(strings.TrimSpace(namespace))
	total := 0

	key := namespaceName + "/" + resourceName
	if exactEvents, ok := idx.exactMap[key]; ok {
		for _, event := range exactEvents {
			if event.Count > 1 {
				total += int(event.Count)
			} else {
				total++
			}
		}
	}

	for _, event := range idx.fallbackList {
		eventNamespace := strings.ToLower(strings.TrimSpace(event.Namespace))
		if namespaceName != "" && eventNamespace != "" && eventNamespace != namespaceName {
			continue
		}

		haystack := strings.ToLower(strings.TrimSpace(event.Reason + " " + event.Message + " " + event.From))
		pattern := `\b` + regexp.QuoteMeta(resourceName) + `\b`
		matched, _ := regexp.MatchString(pattern, haystack)
		if matched {
			if event.Count > 1 {
				total += int(event.Count)
			} else {
				total++
			}
		}
	}

	return total
}

func buildLocalPredictions(pods []model.PodSummary, nodes []model.NodeSummary, events []model.K8sEvent, now time.Time) model.PredictionsResult {
	items := make([]model.IncidentPrediction, 0, len(pods)+len(nodes))

	podCPUThreshold := getEnvInt("PREDICTOR_POD_CPU_THRESHOLD_MILLI", 400)
	podMemThreshold := getEnvInt("PREDICTOR_POD_MEM_THRESHOLD_MI", 512)
	nodeCPUThreshold := getEnvInt("PREDICTOR_NODE_CPU_THRESHOLD_PCT", 90)
	nodeMemThreshold := getEnvInt("PREDICTOR_NODE_MEM_THRESHOLD_PCT", 90)
	nodeCPUTrendThreshold := getEnvInt("PREDICTOR_NODE_CPU_TREND_THRESHOLD_PCT", 20)
	nodeCPUTrendBase := getEnvInt("PREDICTOR_NODE_CPU_TREND_BASE_PCT", 80)

	idx := buildEventIndex(events)

	for _, pod := range pods {
		score := 0
		signals := make([]model.PredictionSignal, 0, 4)
		resourceWarnings := countResourceWarningEventsIndexed(idx, pod.Name, pod.Namespace)
		cpuMilli, cpuKnown := parseCPUMilli(pod.CPU)
		memMi, memKnown := parseMemoryMi(pod.Memory)
		cpuSignal := false
		memSignal := false

		switch pod.Status {
		case model.PodStatusFailed:
			score += 60
			signals = append(signals, model.PredictionSignal{Key: "status", Value: "Failed"})
		case model.PodStatusPending:
			score += 35
			signals = append(signals, model.PredictionSignal{Key: "status", Value: "Pending"})
		case model.PodStatusUnknown:
			score += 20
			signals = append(signals, model.PredictionSignal{Key: "status", Value: "Unknown"})
		}

		if pod.Restarts > 0 {
			restartRisk := int(pod.Restarts) * 8
			if restartRisk > 40 {
				restartRisk = 40
			}
			score += restartRisk
			signals = append(signals, model.PredictionSignal{Key: "restarts", Value: strconv.Itoa(int(pod.Restarts))})
		}

		cpuReqVal, cpuReqKnown := parseCPUMilli(pod.CPURequest)
		cpuLimitVal, cpuLimitKnown := parseCPUMilli(pod.CPULimit)
		cpuTriggered := false
		if cpuKnown {
			if cpuReqKnown && cpuReqVal > 0 {
				if cpuMilli >= int(float64(cpuReqVal)*0.90) {
					cpuTriggered = true
				}
			} else if cpuLimitKnown && cpuLimitVal > 0 {
				if cpuMilli >= int(float64(cpuLimitVal)*0.90) {
					cpuTriggered = true
				}
			} else {
				if cpuMilli >= podCPUThreshold {
					cpuTriggered = true
				}
			}
		}
		if cpuTriggered {
			score += 10
			signals = append(signals, model.PredictionSignal{Key: "cpu", Value: pod.CPU})
			cpuSignal = true
		}

		memReqVal, memReqKnown := parseMemoryMi(pod.MemoryRequest)
		memLimitVal, memLimitKnown := parseMemoryMi(pod.MemoryLimit)
		memTriggered := false
		if memKnown {
			if memReqKnown && memReqVal > 0 {
				if memMi >= int(float64(memReqVal)*0.90) {
					memTriggered = true
				}
			} else if memLimitKnown && memLimitVal > 0 {
				if memMi >= int(float64(memLimitVal)*0.90) {
					memTriggered = true
				}
			} else {
				if memMi >= podMemThreshold {
					memTriggered = true
				}
			}
		}
		if memTriggered {
			score += 10
			signals = append(signals, model.PredictionSignal{Key: "memory", Value: pod.Memory})
			memSignal = true
		}

		if resourceWarnings > 0 && pod.Status != model.PodStatusRunning {
			score += minInt(12, resourceWarnings*2)
		}

		score = clampInt(score, 0, 100)
		if score < 35 {
			continue
		}

		recommendation := "Review recent pod events and logs; verify dependencies and resource requests."
		if pod.Status == model.PodStatusPending {
			recommendation = "Inspect scheduler constraints, image pull status, and resource requests."
		} else if pod.Status == model.PodStatusFailed {
			recommendation = "Investigate crash causes, validate probes, and consider rollback to last healthy revision."
		}

		confidence := confidenceFromEvidence(evidenceProfile{
			strongStatus:      pod.Status == model.PodStatusFailed || pod.Status == model.PodStatusPending,
			signalCount:       len(signals),
			metricKnown:       boolToInt(cpuKnown) + boolToInt(memKnown),
			metricSignalCount: boolToInt(cpuSignal) + boolToInt(memSignal),
			warningMatches:    resourceWarnings,
			restartSignal:     pod.Restarts > 0,
		})
		items = append(items, model.IncidentPrediction{
			ID:             "pod-" + pod.ID,
			ResourceKind:   "Pod",
			Resource:       pod.Name,
			Namespace:      pod.Namespace,
			RiskScore:      score,
			Confidence:     confidence,
			Summary:        fmt.Sprintf("%s pod with %d restarts and status %s.", pod.Name, pod.Restarts, pod.Status),
			Recommendation: recommendation,
			Signals:        signals,
		})
	}

	for _, node := range nodes {
		score := 0
		signals := make([]model.PredictionSignal, 0, 3)
		cpu, cpuKnown := parsePercent(node.CPUUsage)
		mem, memKnown := parsePercent(node.MemUsage)
		resourceWarnings := countResourceWarningEventsIndexed(idx, node.Name, "")
		cpuTrend := cpuTrendDelta(node.CPUHistory)
		cpuSignal := false
		memSignal := false

		if node.Status == model.NodeStatusNotReady {
			score += 75
			signals = append(signals, model.PredictionSignal{Key: "status", Value: "NotReady"})
		}

		if cpuKnown && cpu >= float64(nodeCPUThreshold) {
			score += 20
			signals = append(signals, model.PredictionSignal{Key: "cpuUsage", Value: node.CPUUsage})
			cpuSignal = true
		}

		if memKnown && mem >= float64(nodeMemThreshold) {
			score += 20
			signals = append(signals, model.PredictionSignal{Key: "memUsage", Value: node.MemUsage})
			memSignal = true
		}

		if cpuTrend >= nodeCPUTrendThreshold && cpuKnown && cpu >= float64(nodeCPUTrendBase) {
			score += 10
			signals = append(signals, model.PredictionSignal{Key: "cpuTrend", Value: fmt.Sprintf("+%d%%", cpuTrend)})
		}

		if resourceWarnings > 0 && node.Status != model.NodeStatusReady {
			score += minInt(10, resourceWarnings*2)
		}

		score = clampInt(score, 0, 100)
		if score < 45 {
			continue
		}

		confidence := confidenceFromEvidence(evidenceProfile{
			strongStatus:      node.Status == model.NodeStatusNotReady,
			signalCount:       len(signals),
			metricKnown:       boolToInt(cpuKnown) + boolToInt(memKnown),
			metricSignalCount: boolToInt(cpuSignal) + boolToInt(memSignal),
			warningMatches:    resourceWarnings,
		})
		recommendation := "Inspect kubelet health, node conditions, and workload pressure before scheduling more pods."
		if cpuTrend >= nodeCPUTrendThreshold && cpuKnown && cpu >= float64(nodeCPUTrendBase) {
			recommendation = "CPU usage is trending up quickly; review noisy neighbors and consider scaling."
		}

		items = append(items, model.IncidentPrediction{
			ID:             "node-" + strings.ToLower(node.Name),
			ResourceKind:   "Node",
			Resource:       node.Name,
			RiskScore:      score,
			Confidence:     confidence,
			Summary:        fmt.Sprintf("Node %s shows elevated operational risk.", node.Name),
			Recommendation: recommendation,
			Signals:        signals,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RiskScore == items[j].RiskScore {
			return items[i].Confidence > items[j].Confidence
		}
		return items[i].RiskScore > items[j].RiskScore
	})

	if len(items) > maxPredictionItems {
		items = items[:maxPredictionItems]
	}

	return model.PredictionsResult{
		Source:      "local-fallback",
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Items:       items,
	}
}

func countResourceWarningEvents(events []model.K8sEvent, resource, namespace string) int {
	resourceName := strings.ToLower(strings.TrimSpace(resource))
	if resourceName == "" {
		return 0
	}
	namespaceName := strings.ToLower(strings.TrimSpace(namespace))
	total := 0

	for _, event := range events {
		eventType := strings.ToLower(strings.TrimSpace(event.Type))
		reason := strings.ToLower(strings.TrimSpace(event.Reason))
		if eventType != "warning" && reason != "backoff" && reason != "failed" && reason != "unhealthy" && reason != "oomkilled" {
			continue
		}

		eventNamespace := strings.ToLower(strings.TrimSpace(event.Namespace))
		if namespaceName != "" && eventNamespace != "" && eventNamespace != namespaceName {
			continue
		}

		eventResource := strings.ToLower(strings.TrimSpace(event.Resource))
		if eventResource != "" {
			if eventResource != resourceName {
				continue
			}
		} else {
			haystack := strings.ToLower(strings.TrimSpace(event.Reason + " " + event.Message + " " + event.From))
			pattern := `\b` + regexp.QuoteMeta(resourceName) + `\b`
			matched, _ := regexp.MatchString(pattern, haystack)
			if !matched {
				continue
			}
		}

		if event.Count > 1 {
			total += int(event.Count)
		} else {
			total++
		}
	}

	return total
}

type evidenceProfile struct {
	strongStatus      bool
	signalCount       int
	metricKnown       int
	metricSignalCount int
	warningMatches    int
	restartSignal     bool
}

func confidenceFromEvidence(profile evidenceProfile) int {
	confidence := 35
	if profile.strongStatus {
		confidence += 18
	}

	confidence += minInt(24, profile.signalCount*6)
	confidence += minInt(16, profile.metricKnown*8)
	confidence += minInt(10, profile.metricSignalCount*5)
	confidence += minInt(12, profile.warningMatches*3)
	if profile.restartSignal {
		confidence += 6
	}

	if profile.signalCount <= 1 {
		confidence -= 8
	}
	if profile.metricKnown == 0 && !profile.strongStatus {
		confidence -= 10
	}

	return clampInt(confidence, 35, 96)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func parseCPUMilli(raw string) (int, bool) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" || value == "n/a" {
		return 0, false
	}

	if strings.HasSuffix(value, "m") {
		numeric := strings.TrimSuffix(value, "m")
		parsed, err := strconv.ParseFloat(numeric, 64)
		if err != nil {
			return 0, false
		}
		return int(parsed), true
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return int(parsed * 1000), true
}

func parseMemoryMi(raw string) (int, bool) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" || value == "n/a" {
		return 0, false
	}

	switch {
	case strings.HasSuffix(value, "mi"):
		numeric := strings.TrimSuffix(value, "mi")
		parsed, err := strconv.ParseFloat(numeric, 64)
		if err != nil {
			return 0, false
		}
		return int(parsed), true
	case strings.HasSuffix(value, "gi"):
		numeric := strings.TrimSuffix(value, "gi")
		parsed, err := strconv.ParseFloat(numeric, 64)
		if err != nil {
			return 0, false
		}
		return int(parsed * 1024), true
	case strings.HasSuffix(value, "ki"):
		numeric := strings.TrimSuffix(value, "ki")
		parsed, err := strconv.ParseFloat(numeric, 64)
		if err != nil {
			return 0, false
		}
		return int(parsed / 1024), true
	default:
		numeric := strings.TrimRight(value, "b")
		parsed, err := strconv.ParseFloat(numeric, 64)
		if err != nil {
			return 0, false
		}
		// If no explicit unit is provided, treat as bytes.
		return int(parsed / (1024 * 1024)), true
	}
}

func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func cpuTrendDelta(points []model.CPUPoint) int {
	if len(points) < 2 {
		return 0
	}
	start := points[0].Value
	end := points[len(points)-1].Value
	if end < start {
		return 0
	}
	return end - start
}

func (s *Server) predictionsFromCache() (model.PredictionsResult, bool) {
	if s.predictionsTTL <= 0 {
		return model.PredictionsResult{}, false
	}

	now := s.now()

	s.predictionsMu.RLock()
	defer s.predictionsMu.RUnlock()

	if now.After(s.predictionsCache.expiresAt) {
		return model.PredictionsResult{}, false
	}

	return clonePredictionsResult(s.predictionsCache.data), true
}

func (s *Server) storePredictions(result model.PredictionsResult) {
	if s.predictionsTTL <= 0 {
		return
	}

	s.predictionsMu.Lock()
	s.predictionsCache = predictionsCacheEntry{
		data:      clonePredictionsResult(result),
		expiresAt: s.now().Add(s.predictionsTTL),
	}
	s.predictionsMu.Unlock()
}

func clonePredictionsResult(in model.PredictionsResult) model.PredictionsResult {
	out := in
	out.Items = make([]model.IncidentPrediction, len(in.Items))
	for i := range in.Items {
		out.Items[i] = in.Items[i]
		out.Items[i].Signals = append([]model.PredictionSignal(nil), in.Items[i].Signals...)
	}
	return out
}
