package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

const maxJSONRequestBody = 1 << 20 // 1 MiB

// decodeJSONBody decodes and validates a JSON payload against the destination struct.
func decodeJSONBody(r *http.Request, dst any) error {
	return decodeJSONBodyWithDebug(r, dst, false)
}

// decodeJSONBodyWithDebug decodes strict JSON and optionally returns detailed decode errors.
func decodeJSONBodyWithDebug(r *http.Request, dst any, debug bool) error {
	payload, err := io.ReadAll(io.LimitReader(r.Body, maxJSONRequestBody+1))
	if err != nil {
		return invalidJSONError(err, debug)
	}
	if len(payload) > maxJSONRequestBody {
		return errors.New("request body too large")
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return invalidJSONError(err, debug)
	}

	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return invalidJSONError(err, debug)
	}
	return nil
}

// invalidJSONError normalizes decode failures for public API responses.
func invalidJSONError(err error, debug bool) error {
	if !debug {
		return errors.New("invalid JSON body")
	}
	return fmt.Errorf("invalid JSON body: %w", err)
}

// decodeJSONBody decodes JSON using verbosity based on the current runtime mode.
func (s *Server) decodeJSONBody(r *http.Request, dst any) error {
	return decodeJSONBodyWithDebug(r, dst, s.runtime.Mode != "prod")
}

// writeJSON writes a JSON response with a stable content type.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	_ = encoder.Encode(payload)
}

// writeError writes API errors using the canonical {"error": "..."} shape.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// attachStatic serves dist assets and SPA fallback when a built frontend exists.
func attachStatic(r chi.Router, distDir string) {
	indexFile := filepath.Join(distDir, "index.html")
	if _, err := os.Stat(indexFile); err != nil {
		return
	}

	absDistDir, err := filepath.Abs(distDir)
	if err != nil {
		return
	}

	fileServer := http.FileServer(http.Dir(distDir))
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		if isAPIPath(req.URL.Path) {
			writeError(w, http.StatusNotFound, "Not found")
			return
		}

		trimmed := strings.TrimPrefix(req.URL.Path, "/")
		if trimmed == "" {
			http.ServeFile(w, req, indexFile)
			return
		}

		cleaned := filepath.Clean(trimmed)
		candidate := filepath.Join(absDistDir, cleaned)
		absCandidate, err := filepath.Abs(candidate)
		if err == nil {
			if rel, relErr := filepath.Rel(absDistDir, absCandidate); relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				if info, statErr := os.Stat(absCandidate); statErr == nil && !info.IsDir() {
					fileServer.ServeHTTP(w, req)
					return
				}
			}
		}

		http.ServeFile(w, req, indexFile)
	})
}
