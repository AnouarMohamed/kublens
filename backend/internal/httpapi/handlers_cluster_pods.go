package httpapi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"kubelens-backend/internal/apperrors"
	"kubelens-backend/internal/model"
)

type PodController struct {
	cluster                    ClusterReader
	logger                     *slog.Logger
	decodeJSONBody             func(*http.Request, any) error
	invalidatePredictionsCache func()
}

func NewPodController(
	cluster ClusterReader,
	logger *slog.Logger,
	decode func(*http.Request, any) error,
	invalidatePredictionsCache func(),
) *PodController {
	if decode == nil {
		decode = decodeJSONBody
	}
	if invalidatePredictionsCache == nil {
		invalidatePredictionsCache = func() {}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &PodController{
		cluster:                    cluster,
		logger:                     logger,
		decodeJSONBody:             decode,
		invalidatePredictionsCache: invalidatePredictionsCache,
	}
}

func (pc *PodController) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", pc.handlePods)
	r.Post("/", pc.handleCreatePod)
	r.Get("/{namespace}/{name}/events", pc.handlePodEvents)
	r.Get("/{namespace}/{name}/logs", pc.handlePodLogs)
	r.Get("/{namespace}/{name}/logs/stream", pc.handlePodLogsStream)
	r.Get("/{namespace}/{name}/describe", pc.handlePodDescribe)
	r.Post("/{namespace}/{name}/restart", pc.handleRestartPod)
	r.Delete("/{namespace}/{name}", pc.handleDeletePod)
	r.Get("/{namespace}/{name}", pc.handlePodDetail)
	return r
}

func (pc *PodController) handlePods(w http.ResponseWriter, r *http.Request) {
	pods, _ := pc.cluster.Snapshot(r.Context())
	writeJSON(w, http.StatusOK, pods)
}

func (pc *PodController) handleCreatePod(w http.ResponseWriter, r *http.Request) {
	var req model.PodCreateRequest
	if err := pc.decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := pc.cluster.CreatePod(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	pc.invalidatePredictionsCache()
	writeJSON(w, http.StatusOK, result)
}

func (pc *PodController) handlePodEvents(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	writeJSON(w, http.StatusOK, pc.cluster.PodEvents(r.Context(), namespace, name))
}

func (pc *PodController) handlePodLogs(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	container := strings.TrimSpace(r.URL.Query().Get("container"))
	lines := parsePositiveIntWithMax(r.URL.Query().Get("lines"), 50, 500)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(pc.cluster.PodLogs(r.Context(), namespace, name, container, lines)))
}

func (pc *PodController) handlePodLogsStream(w http.ResponseWriter, r *http.Request) {
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

	stream, err := pc.cluster.StreamPodLogs(r.Context(), namespace, name, container, tailLines, follow)
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
				pc.logger.Warn("pod log stream ended with error", "namespace", namespace, "name", name, "error", err)
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

func (pc *PodController) handlePodDescribe(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	pod, err := pc.cluster.PodDetail(r.Context(), namespace, name)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Pod not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to describe pod")
		return
	}
	events := pc.cluster.PodEvents(r.Context(), namespace, name)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(renderPodDescribe(pod, events)))
}

func (pc *PodController) handleRestartPod(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	result, err := pc.cluster.RestartPod(r.Context(), namespace, name)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Pod not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	pc.invalidatePredictionsCache()
	writeJSON(w, http.StatusOK, result)
}

func (pc *PodController) handleDeletePod(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	result, err := pc.cluster.DeletePod(r.Context(), namespace, name)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Pod not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	pc.invalidatePredictionsCache()
	writeJSON(w, http.StatusOK, result)
}

func (pc *PodController) handlePodDetail(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")

	pod, err := pc.cluster.PodDetail(r.Context(), namespace, name)
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
