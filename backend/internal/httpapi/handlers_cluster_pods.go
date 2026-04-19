package httpapi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"kubelens-backend/internal/apperrors"
	"kubelens-backend/internal/model"
)

func (s *Server) handleCreatePod(w http.ResponseWriter, r *http.Request) {
	var req model.PodCreateRequest
	if err := s.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := s.cluster.CreatePod(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.invalidatePredictionsCache()
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePodEvents(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	writeJSON(w, http.StatusOK, s.cluster.PodEvents(r.Context(), namespace, name))
}

func (s *Server) handlePodLogs(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	container := strings.TrimSpace(r.URL.Query().Get("container"))
	lines := parsePositiveIntWithMax(r.URL.Query().Get("lines"), 50, 500)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(s.cluster.PodLogs(r.Context(), namespace, name, container, lines)))
}

func (s *Server) handlePodLogsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}

	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	container := strings.TrimSpace(r.URL.Query().Get("container"))
	tailLines := parsePositiveIntWithMax(r.URL.Query().Get("tailLines"), 100, 1000)
	follow := parseBoolDefault(r.URL.Query().Get("follow"), true)

	stream, err := s.cluster.StreamPodLogs(r.Context(), namespace, name, container, tailLines, follow)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to stream pod logs")
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	streamCtx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	stopClose := context.AfterFunc(streamCtx, func() {
		_ = stream.Close()
	})
	defer stopClose()

	lineCh := make(chan string)
	errCh := make(chan error, 1)

	go func() {
		defer close(lineCh)

		scanner := bufio.NewScanner(stream)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case lineCh <- scanner.Text():
			case <-streamCtx.Done():
				return
			}
		}

		if err := scanner.Err(); err != nil && !errors.Is(streamCtx.Err(), context.Canceled) && !errors.Is(streamCtx.Err(), context.DeadlineExceeded) {
			errCh <- err
		}
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-streamCtx.Done():
			if errors.Is(streamCtx.Err(), context.DeadlineExceeded) {
				_ = writeSSELogLine(w, flusher, "[stream-timeout]")
			}
			return
		case err := <-errCh:
			if err != nil {
				s.logger.Warn("pod log stream ended with error", "namespace", namespace, "name", name, "error", err)
			}
			return
		case line, ok := <-lineCh:
			if !ok {
				return
			}
			if err := writeSSELogLine(w, flusher, line); err != nil {
				return
			}
		}
	}
}

func (s *Server) handlePodDescribe(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	pod, err := s.cluster.PodDetail(r.Context(), namespace, name)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Pod not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to describe pod")
		return
	}
	events := s.cluster.PodEvents(r.Context(), namespace, name)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(renderPodDescribe(pod, events)))
}

func (s *Server) handleRestartPod(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	result, err := s.cluster.RestartPod(r.Context(), namespace, name)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Pod not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.invalidatePredictionsCache()
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDeletePod(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	result, err := s.cluster.DeletePod(r.Context(), namespace, name)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Pod not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.invalidatePredictionsCache()
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handlePodDetail(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	pod, err := s.cluster.PodDetail(r.Context(), namespace, name)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Pod not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to fetch pod details")
		return
	}
	writeJSON(w, http.StatusOK, pod)
}

func writeSSELogLine(w http.ResponseWriter, flusher http.Flusher, line string) error {
	if _, err := fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(line, "\r", "")); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func parseBoolDefault(raw string, fallback bool) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}

	value, err := strconv.ParseBool(trimmed)
	if err != nil {
		return fallback
	}
	return value
}
