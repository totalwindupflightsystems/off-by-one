// Package main — Off-by-One pre-solve lab entry point.
//
// Embeds the OpenAPI spec from pkg/api/openapi.yaml and serves it at
// /openapi.json so Muster (and any other OpenAPI consumer) can auto-discover
// and auto-configure the API surface.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/web"
	"github.com/totalwindupflightsystems/off-by-one/pkg/api"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "0.1.0-dev"

func main() {
	port := flag.Int("port", envInt("OFF_BY_ONE_PORT", 8766), "HTTP listen port")
	dbPath := flag.String("db", envString("OFF_BY_ONE_DB", "./off-by-one.db"), "SQLite database path")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("off-by-one %s\n", version)
		return
	}

	// Validate the embedded OpenAPI spec at startup. A malformed spec
	// here is a build-time bug, but we fail loud rather than serve a
	// broken document silently.
	if _, err := api.JSONBytes(); err != nil {
		log.Fatalf("openapi spec invalid: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", openapiHandler)
	mux.HandleFunc("/health", healthHandler)

	// The web handler serves the SPA shell and static assets. It
	// only matches paths that don't collide with the API routes
	// registered above (the web handler explicitly returns 404 for
	// /api/* and known API paths). We compose the two with a small
	// dispatcher: the API mux is consulted first; if the mux did
	// not match (its default 404), the web handler gets a chance.
	webHandler := web.Handler()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The stdlib mux returns 404 for unmatched paths; the
		// cheapest way to detect that is to ask the mux but capture
		// the status code via a ResponseWriter wrapper. We do it
		// the simple way here: only defer to the web handler when
		// the request path is not an API path.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			mux.ServeHTTP(w, r)
			return
		}
		// /openapi.json and /health are explicitly handled by mux.
		// For everything else, the web handler serves the SPA shell
		// or static assets.
		switch r.URL.Path {
		case "/openapi.json", "/health":
			mux.ServeHTTP(w, r)
			return
		}
		webHandler.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", *port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("off-by-one %s listening on :%d (db=%s)", version, *port, *dbPath)
	log.Printf("OpenAPI spec at http://localhost:%d/openapi.json (sha256=%s)", *port, api.SHA256()[:12])

	// Graceful shutdown on SIGINT / SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Printf("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// openapiHandler serves the embedded OpenAPI 3.0.3 spec at /openapi.json.
//
// Muster (and any other OpenAPI consumer) reads `paths.*` directly from the
// JSON document. We parse the YAML at startup (api.JSONBytes) and serve the
// re-emitted JSON here.
//
// On parse failure — which would be a build-time bug — we fall back to
// serving the raw YAML as application/x-yaml. Muster accepts both.
func openapiHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	etag := `"` + api.SHA256() + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("X-Off-by-One-Version", version)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "public, max-age=300")

	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	body, err := api.JSONBytes()
	if err != nil {
		log.Printf("openapi parse failed, falling back to YAML: %v", err)
		w.Header().Set("Content-Type", "application/x-yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(api.YAMLBytes())
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// healthHandler returns 200 with a minimal status payload. Used by load
// balancers, the supervisor cron, and the GitReins Tier 1 health check.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status":"ok","version":%q}`, version)
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return def
}

func envString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}
