package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type streamRecordingClusterReader struct {
	testClusterReader
	streamData string
	container  string
	tailLines  int
	follow     bool
	onStream   func(context.Context, string, string, string, int, bool) (io.ReadCloser, error)
}

func (r *streamRecordingClusterReader) StreamPodLogs(
	ctx context.Context,
	namespace string,
	name string,
	container string,
	tailLines int,
	follow bool,
) (io.ReadCloser, error) {
	if r.onStream != nil {
		return r.onStream(ctx, namespace, name, container, tailLines, follow)
	}
	return io.NopCloser(strings.NewReader(r.streamData)), nil
}

func (r *streamRecordingClusterReader) captureStreamPodLogs(
	_ context.Context,
	_ string,
	_ string,
	container string,
	tailLines int,
	follow bool,
) (io.ReadCloser, error) {
	r.container = container
	r.tailLines = tailLines
	r.follow = follow
	return io.NopCloser(strings.NewReader(r.streamData)), nil
}

func TestHandlePodLogsStreamSSE(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	t.Run("streams log lines as server-sent events", func(t *testing.T) {
		reader := &streamRecordingClusterReader{streamData: "first line\nsecond line\n"}
		server := newServer(
			reader,
			nil,
			logger,
			WithAuth(AuthConfig{
				Enabled: true,
				Tokens: []AuthToken{
					{Token: "viewer-token", User: "viewer", Role: "viewer"},
				},
			}),
		)
		reader.onStream = reader.captureStreamPodLogs
		router := server.Router("")

		req := httptest.NewRequest(
			http.MethodGet,
			"/api/pods/production/payment-gateway/logs/stream?container=app&tailLines=3&follow=false",
			nil,
		)
		req.Header.Set("Authorization", "Bearer viewer-token")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status code = %d, want 200", rr.Code)
		}
		if got := rr.Header().Get("Content-Type"); got != "text/event-stream" {
			t.Fatalf("content-type = %q, want text/event-stream", got)
		}
		if got := rr.Header().Get("Cache-Control"); got != "no-cache" {
			t.Fatalf("cache-control = %q, want no-cache", got)
		}
		if got := rr.Header().Get("X-Accel-Buffering"); got != "no" {
			t.Fatalf("x-accel-buffering = %q, want no", got)
		}
		if rr.Body.String() != "data: first line\n\ndata: second line\n\n" {
			t.Fatalf("unexpected SSE body: %q", rr.Body.String())
		}
		if reader.container != "app" {
			t.Fatalf("container = %q, want app", reader.container)
		}
		if reader.tailLines != 3 {
			t.Fatalf("tailLines = %d, want 3", reader.tailLines)
		}
		if reader.follow {
			t.Fatal("follow = true, want false")
		}
	})

	t.Run("uses default follow and tail lines", func(t *testing.T) {
		reader := &streamRecordingClusterReader{streamData: "single line\n"}
		server := newServer(
			reader,
			nil,
			logger,
			WithAuth(AuthConfig{
				Enabled: true,
				Tokens: []AuthToken{
					{Token: "viewer-token", User: "viewer", Role: "viewer"},
				},
			}),
		)
		reader.onStream = reader.captureStreamPodLogs
		router := server.Router("")

		req := httptest.NewRequest(http.MethodGet, "/api/pods/production/payment-gateway/logs/stream", nil)
		req.Header.Set("Authorization", "Bearer viewer-token")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status code = %d, want 200", rr.Code)
		}
		if reader.tailLines != 100 {
			t.Fatalf("tailLines = %d, want 100", reader.tailLines)
		}
		if !reader.follow {
			t.Fatal("follow = false, want true")
		}
	})
}

func TestWriteSSELogLineSplitsEmbeddedNewlines(t *testing.T) {
	rr := httptest.NewRecorder()
	if err := writeSSELogLine(rr, flushRecorder{ResponseRecorder: rr}, "first\nsecond\r\nthird"); err != nil {
		t.Fatalf("write SSE log line: %v", err)
	}

	if got, want := rr.Body.String(), "data: first\ndata: second\ndata: third\n\n"; got != want {
		t.Fatalf("SSE payload = %q, want %q", got, want)
	}
}

type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (flushRecorder) Flush() {}
