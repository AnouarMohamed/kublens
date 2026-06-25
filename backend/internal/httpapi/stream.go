package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"kubelens-backend/internal/events"
	"kubelens-backend/internal/model"
)

const streamHeartbeatInterval = 20 * time.Second
const streamSnapshotEventLimit = 32 // Keeps initial snapshot useful without over-buffering; too low drops context, too high increases memory/network burst.

type StreamController struct {
	cluster            ClusterReader
	eventBus           *events.Bus
	now                func() time.Time
	clusterStats       func(context.Context) model.ClusterStats
	trustedCSRFDomains []string
}

func NewStreamController(
	cluster ClusterReader,
	eventBus *events.Bus,
	now func() time.Time,
	clusterStats func(context.Context) model.ClusterStats,
	trustedCSRFDomains []string,
) *StreamController {
	if now == nil {
		now = time.Now
	}
	if clusterStats == nil {
		clusterStats = func(context.Context) model.ClusterStats { return model.ClusterStats{} }
	}
	return &StreamController{
		cluster:            cluster,
		eventBus:           eventBus,
		now:                now,
		clusterStats:       clusterStats,
		trustedCSRFDomains: append([]string(nil), trustedCSRFDomains...),
	}
}

func (sc *StreamController) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", sc.handleStream)
	r.Get("/ws", sc.handleStreamWebSocket)
	return r
}

func (sc *StreamController) handleStream(w http.ResponseWriter, r *http.Request) {
	if sc.eventBus == nil {
		writeError(w, http.StatusServiceUnavailable, "event stream is not configured")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	subID, ch := sc.eventBus.Subscribe()
	defer sc.eventBus.Unsubscribe(subID)

	_ = writeSSE(w, "connected", events.Event{
		Type:      "connected",
		Timestamp: sc.now().UTC().Format(time.RFC3339),
		Payload: map[string]string{
			"message": "stream established",
		},
	})
	flusher.Flush()

	sc.sendInitialStreamSnapshot(r.Context(), func(evt events.Event) error {
		return writeSSE(w, evt.Type, evt)
	})
	flusher.Flush()

	ticker := time.NewTicker(streamHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			if err := writeSSE(w, event.Type, event); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			_, _ = w.Write([]byte(": ping\n\n"))
			flusher.Flush()
		}
	}
}

func (sc *StreamController) handleStreamWebSocket(w http.ResponseWriter, r *http.Request) {
	if sc.eventBus == nil {
		writeError(w, http.StatusServiceUnavailable, "event stream is not configured")
		return
	}

	if err := validateCSRFSameOrigin(r, sc.trustedCSRFDomains); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx := r.Context()
	subID, ch := sc.eventBus.Subscribe()
	defer sc.eventBus.Unsubscribe(subID)

	_ = wsjson.Write(ctx, conn, events.Event{
		Type:      "connected",
		Timestamp: sc.now().UTC().Format(time.RFC3339),
		Payload: map[string]string{
			"message": "stream established",
		},
	})

	sc.sendInitialStreamSnapshot(ctx, func(evt events.Event) error {
		return wsjson.Write(ctx, conn, evt)
	})

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			if err := wsjson.Write(ctx, conn, event); err != nil {
				return
			}
		}
	}
}

func (sc *StreamController) sendInitialStreamSnapshot(ctx context.Context, send func(events.Event) error) {
	stats := sc.clusterStats(ctx)
	_ = send(events.Event{
		Type:      "stats",
		Timestamp: sc.now().UTC().Format(time.RFC3339),
		Payload:   stats,
	})

	clusterEvents := trimEvents(sc.cluster.ListClusterEvents(ctx), streamSnapshotEventLimit)
	if len(clusterEvents) > 0 {
		_ = send(events.Event{
			Type:      "cluster_events",
			Timestamp: sc.now().UTC().Format(time.RFC3339),
			Payload:   clusterEvents,
		})
	}
}

func writeSSE(w http.ResponseWriter, event string, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Emit event/data in a single write to avoid partially written SSE frames.
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", sanitizeSSEField(event), encoded); err != nil {
		return err
	}
	return nil
}

func sanitizeSSEField(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "message"
	}
	return strings.ReplaceAll(trimmed, "\n", " ")
}

func trimEvents(items []model.K8sEvent, limit int) []model.K8sEvent {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return append([]model.K8sEvent(nil), items[:limit]...)
}

func timeoutUnlessPath(timeout time.Duration, skip func(path string) bool) func(http.Handler) http.Handler {
	base := middleware.Timeout(timeout)
	return func(next http.Handler) http.Handler {
		withTimeout := base(next)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip != nil && skip(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			withTimeout.ServeHTTP(w, r)
		})
	}
}
