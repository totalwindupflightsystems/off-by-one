// Package api implements the Off-by-One HTTP API. The routes and
// request/response shapes are defined in pkg/api/openapi.yaml and
// referenced by both the Muster MCP auto-config and the web UI.
//
// The Server is a thin wrapper around net/http. Handlers live in
// handlers.go; this file is just the wiring (mux, listen, graceful
// shutdown) and the small shared helpers (JSON read/write, error
// formatting, content-type).
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/export"
	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
	"github.com/totalwindupflightsystems/off-by-one/internal/metrics"
)

// Server holds the dependencies the HTTP handlers need. The graph
// Store and ingest Queue are shared with the cron loop (read-write
// from both sides); the embedded OpenAPI spec is served as JSON.
type Server struct {
	Store *graph.Store
	Queue *ingest.Queue

	// OpenAPISpec is the raw YAML or JSON bytes of the spec. The
	// /openapi.json endpoint serves this verbatim. Pre-loaded at
	// construction so the handler is a single write.
	OpenAPISpec []byte

	// ExportLocalDir is the working directory for git export clones.
	// When empty, POST /api/v1/export returns 501.
	ExportLocalDir string

	// ImportLocalDir is the working directory for git import clones.
	// When empty, POST /api/v1/import returns 501.
	ImportLocalDir string

	// AttachmentsDir is the directory where uploaded file attachments
	// are stored. Created on first use. When empty, multipart uploads
	// are silently accepted but files are discarded (only JSON body
	// is processed).
	AttachmentsDir string

	// ReadOnly disables all mutating endpoints (submit/discover/export/
	// import/queue writes and the /ws/chat AI agent). Used for public
	// catalog deployments: the corpus is served read-only and no LLM
	// keys are present on the box.
	ReadOnly bool

	// SolverAvailable reports whether a solver (bwrap + pi-agent) is
	// wired up. When false the cron loop is not running and queued
	// submissions will not be processed — surfaced on /api/v1/stats
	// so users can tell why their submission is stuck.
	SolverAvailable bool

	// Observer records per-solve metrics and host-pressure skips.
	// nil disables /metrics endpoint.
	Observer *metrics.Observer

	// StartedAt is set in New and used by /health to report uptime.
	StartedAt time.Time
}

// New builds a Server. The OpenAPI spec is optional — when nil the
// /openapi.json endpoint returns 501 Not Implemented.
func New(store *graph.Store, queue *ingest.Queue, spec []byte) *Server {
	return &Server{
		Store:       store,
		Queue:       queue,
		OpenAPISpec: spec,
		StartedAt:   time.Now(),
	}
}

// Handler returns the *http.ServeMux with all routes registered.
// We use net/http.ServeMux directly (no gorilla/mux, no chi) to keep
// the binary small. Path patterns use the {param} syntax Go 1.22+
// supports natively; we just register a couple of leading-prefix
// catch-alls for the /api/v1 prefix.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Routes from pkg/api/openapi.yaml §10. The path patterns use
	// Go 1.22+ {param} syntax; the stdlib ServeMux parses them
	// natively and dispatches to handlers.
	mux.HandleFunc("POST /api/v1/problems/submit", s.handleSubmitProblem)
	mux.HandleFunc("POST /api/v1/problems/discover", s.handleDiscover)
	mux.HandleFunc("GET /api/v1/problems", s.handleListProblems)
	mux.HandleFunc("GET /api/v1/problems/{class}", s.handleGetProblemClass)
	mux.HandleFunc("GET /api/v1/problems/{class}/answers", s.handleListAnswers)
	mux.HandleFunc("GET /api/v1/problems/{class}/answers/{id}", s.handleGetAnswer)
	mux.HandleFunc("GET /api/v1/problems/{class}/related", s.handleGetRelated)
	mux.HandleFunc("GET /api/v1/queue", s.handleListQueue)
	mux.HandleFunc("GET /api/v1/queue/{submission_id}", s.handleGetQueueStatus)
	mux.HandleFunc("GET /api/v1/taxonomy", s.handleTaxonomy)
	mux.HandleFunc("GET /api/v1/stats", s.handleStats)
	mux.HandleFunc("POST /api/v1/export", s.handleExport)
	mux.HandleFunc("POST /api/v1/import", s.handleImport)
	mux.HandleFunc("GET /openapi.json", s.handleOpenAPI)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /metrics", s.handleMetrics)

	// In read-only (public catalog) mode, block all mutating endpoints
	// and the AI chat WebSocket before they reach the mux.
	if s.ReadOnly {
		return sanitizeReadOnlyJSON(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/ws/chat" {
				writeError(w, http.StatusForbidden, "read_only", "AI agent disabled in read-only catalog mode")
				return
			}
			if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/") && !readOnlyAllowedPost(r.URL.Path) {
				writeError(w, http.StatusForbidden, "read_only", "catalog is read-only — submissions go through the upstream lab")
				return
			}
			mux.ServeHTTP(w, r)
		}))
	}

	return mux
}

// sanitizeReadOnlyJSON protects every JSON read surface on the public
// catalog, including discover (POST is a pure read), queue metadata, and
// taxonomy. It works on parsed JSON values so escaping/serialization cannot
// bypass the same filter used by the corpus exporter.
func sanitizeReadOnlyJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &bufferedResponseWriter{header: make(http.Header)}
		next.ServeHTTP(recorder, r)
		for key, values := range recorder.header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		if !strings.HasPrefix(strings.ToLower(recorder.header.Get("Content-Type")), "application/json") {
			w.WriteHeader(recorder.statusCode())
			_, _ = w.Write(recorder.body.Bytes())
			return
		}
		body, err := export.SanitizePublicJSON(recorder.body.Bytes())
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":"public_response_sanitization_failed"}`)
			return
		}
		w.Header().Del("Content-Length")
		w.Header().Del("ETag")
		w.WriteHeader(recorder.statusCode())
		_, _ = w.Write(body)
	})
}

type bufferedResponseWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *bufferedResponseWriter) Header() http.Header { return w.header }

func (w *bufferedResponseWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
}

func (w *bufferedResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(p)
}

func (w *bufferedResponseWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// readOnlyAllowedPost reports whether a POST endpoint stays available in
// read-only catalog mode. Discovery is the only exemption: handleDiscover
// is a pure read — it runs the graph discovery query and returns a cached
// answer, never mutating state — so the agent-discovery workflow keeps
// working on public catalog deployments (OB-GAP-020). Every other POST
// (submit, export, import) is a write and stays blocked.
func readOnlyAllowedPost(path string) bool {
	return path == "/api/v1/problems/discover"
}

// ListenAndServe binds to addr and serves until ctx is cancelled.
// Returns http.ErrServerClosed on graceful shutdown.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return errors.Join(srv.Shutdown(shutdownCtx), <-errCh)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// --- JSON helpers --------------------------------------------------------

// writeJSON marshals v and writes it as the response. Content-Type
// is always application/json. Any encoding error becomes 500.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		// We can't change the status code now (WriteHeader already
		// committed it), but logging the encoding error is the
		// only honest action left.
		fmt.Printf("api: encode response: %v\n", err)
	}
}

// readJSON decodes the request body into v. Limits to 1MB — larger
// bodies are rejected with 400 to bound the memory footprint.
func readJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// writeError writes a JSON error response in the format the
// OpenAPI Error schema requires: {"error": code, "message": details}.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error":   code,
		"message": message,
	})
}

// writeOpenAPIError writes the OpenAPI spec bytes if available, or
// 501 if not. The spec is served verbatim from the bytes loaded at
// construction time.
func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	if len(s.OpenAPISpec) == 0 {
		writeError(w, http.StatusNotImplemented, "not_implemented", "OpenAPI spec not loaded")
		return
	}
	// The spec is YAML in the repo. Serve it as YAML with the
	// correct content type so Muster can parse it directly. If a
	// future task wants JSON output, run the conversion here.
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(s.OpenAPISpec)
}

// handleHealth returns a minimal status object with uptime. Used by
// docker/k8s liveness probes.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"uptime": time.Since(s.StartedAt).Truncate(time.Second).String(),
	})
}

// handleMetrics returns the metrics snapshot from the Observer.
// Returns 404 if Observer is nil (metrics not configured).
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if s.Observer == nil {
		writeError(w, http.StatusNotFound, "metrics_disabled", "metrics observer not configured")
		return
	}
	snap := s.Observer.Snapshot()
	writeJSON(w, http.StatusOK, snap)
}

// splitPath trims a leading slash and returns the path segments
// without empty strings (handles trailing slashes).
func splitPath(p string) []string {
	return strings.Split(strings.Trim(p, "/"), "/")
}
