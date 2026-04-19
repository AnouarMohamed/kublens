package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"kubelens-backend/internal/events"
	"kubelens-backend/internal/model"
)

const streamHeartbeatInterval = 20 * time.Second
const streamSnapshotEventLimit = 32 // Keeps initial snapshot useful without over-buffering; too low drops context, too high increases memory/network burst.

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	subID, ch := s.eventBus.Subscribe()
	defer s.eventBus.Unsubscribe(subID)

	_ = writeSSE(w, "connected", events.Event{
		Type:      "connected",
		Timestamp: s.now().UTC().Format(time.RFC3339),
		Payload: map[string]string{
			"message": "stream established",
		},
	})
	flusher.Flush()

	s.sendInitialStreamSnapshot(r.Context(), func(evt events.Event) error {
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

func (s *Server) handleStreamWebSocket(w http.ResponseWriter, r *http.Request) {
	if err := validateCSRFSameOrigin(r, s.auth.trustedCSRFDomains); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx := r.Context()
	subID, ch := s.eventBus.Subscribe()
	defer s.eventBus.Unsubscribe(subID)

	_ = wsjson.Write(ctx, conn, events.Event{
		Type:      "connected",
		Timestamp: s.now().UTC().Format(time.RFC3339),
		Payload: map[string]string{
			"message": "stream established",
		},
	})

	s.sendInitialStreamSnapshot(ctx, func(evt events.Event) error {
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

func (s *Server) sendInitialStreamSnapshot(ctx context.Context, send func(events.Event) error) {
	stats := s.currentClusterStats(ctx)
	_ = send(events.Event{
		Type:      "stats",
		Timestamp: s.now().UTC().Format(time.RFC3339),
		Payload:   stats,
	})

	clusterEvents := trimEvents(s.cluster.ListClusterEvents(ctx), streamSnapshotEventLimit)
	if len(clusterEvents) > 0 {
		_ = send(events.Event{
			Type:      "cluster_events",
			Timestamp: s.now().UTC().Format(time.RFC3339),
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
