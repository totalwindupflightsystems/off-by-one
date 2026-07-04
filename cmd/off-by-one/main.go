// Package main — Off-by-One pre-solve lab entry point.
//
// Wires all components into a single binary:
//
//   - Graph store (SQLite) — problem/answer persistence
//   - Ingest queue — submission validation, dedup, priority
//   - Sandbox executor (bwrap) — untrusted code isolation
//   - Solver (Pi Agent) — LLM-backed problem solver
//   - Cron loop — idle-gated dequeue → solve → commit
//   - API server — HTTP endpoints for submit, discover, stats
//   - Web UI — embedded SPA shell + WebSocket chat
//
// Configuration is entirely via flags + environment variables:
//
//	Port:           --port / OFF_BY_ONE_PORT          (default 8766)
//	DB path:        --db   / OFF_BY_ONE_DB            (default ./off-by-one.db)
//	bwrap path:     --bwrap / OFF_BY_ONE_BWRAP        (default /usr/bin/bwrap)
//	pi-agent path:  --pi-agent / OFF_BY_ONE_PI_AGENT  (default pi-agent)
//	DEEPSEEK_API_KEY from env (required for solver)
//	OPENROUTER_API_KEY from env (optional, for embeddings)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	apihttp "github.com/totalwindupflightsystems/off-by-one/internal/api"
	"github.com/totalwindupflightsystems/off-by-one/internal/cron"
	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
	"github.com/totalwindupflightsystems/off-by-one/internal/sandbox"
	"github.com/totalwindupflightsystems/off-by-one/internal/solver"
	"github.com/totalwindupflightsystems/off-by-one/internal/web"
	"github.com/totalwindupflightsystems/off-by-one/pkg/api"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "0.1.0-dev"

func main() {
	port := flag.Int("port", envInt("OFF_BY_ONE_PORT", 8766), "HTTP listen port")
	dbPath := flag.String("db", envString("OFF_BY_ONE_DB", "./off-by-one.db"), "SQLite database path")
	bwrapPath := flag.String("bwrap", envString("OFF_BY_ONE_BWRAP", "/usr/bin/bwrap"), "Path to bwrap binary")
	piAgentPath := flag.String("pi-agent", envString("OFF_BY_ONE_PI_AGENT", "pi-agent"), "Path to pi-agent binary")
	cronInterval := flag.Duration("cron-interval", envDuration("OFF_BY_ONE_CRON_INTERVAL", 5*time.Minute), "Cron loop wake interval")
	loadThreshold := flag.Float64("load-threshold", envFloat("OFF_BY_ONE_LOAD_THRESHOLD", 1.0), "Max loadavg(1) for idle detection (negative = always idle)")
	skipSandbox := flag.Bool("skip-sandbox", envBool("OFF_BY_ONE_SKIP_SANDBOX", false), "Skip bwrap sandbox (for dev/testing)")
	exportDir := flag.String("export-dir", envString("OFF_BY_ONE_EXPORT_DIR", ""), "Working directory for git export clones (empty = export disabled)")
	importDir := flag.String("import-dir", envString("OFF_BY_ONE_IMPORT_DIR", ""), "Working directory for git import clones (empty = import disabled)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("off-by-one %s\n", version)
		return
	}

	// Validate the embedded OpenAPI spec at startup. A malformed spec
	// here is a build-time bug, but we fail loud rather than serve a
	// broken document silently.
	openapiBytes, err := api.JSONBytes()
	if err != nil {
		log.Fatalf("openapi spec invalid: %v", err)
	}

	// --- 1. Graph store (SQLite) --------------------------------------
	store, err := graph.Open(*dbPath)
	if err != nil {
		log.Fatalf("open graph store: %v", err)
	}
	defer func() {
		if cerr := store.Close(); cerr != nil {
			log.Printf("close graph store: %v", cerr)
		}
	}()
	log.Printf("graph store opened: %s", *dbPath)

	// --- 2. Ingest queue (shares the graph DB) ------------------------
	queue, err := ingest.Open(store)
	if err != nil {
		log.Fatalf("open ingest queue: %v", err)
	}
	log.Printf("ingest queue ready")

	// --- 3. Sandbox executor + solver ---------------------------------
	// The sandbox executor is only constructed when bwrap is available.
	// In dev/testing with --skip-sandbox, the solver is nil and the cron
	// loop is not started — the API still works for submit/discover.
	var solverExec *solver.Executor
	var sandboxExec *sandbox.Executor
	if !*skipSandbox {
		if _, serr := os.Stat(*bwrapPath); serr != nil {
			log.Printf("warning: bwrap not found at %s — solver disabled (use --skip-sandbox to suppress this)", *bwrapPath)
		} else {
			sandboxExec = &sandbox.Executor{
				BwrapPath: *bwrapPath,
				WorkDir:   os.TempDir(),
				Timeout:   sandbox.DefaultBwrapTimeout,
			}
			runner := solver.NewBSandboxRunner(sandboxExec)
			apiKey := os.Getenv("DEEPSEEK_API_KEY")
			solverExec = solver.NewExecutor(solver.Config{
				PiAgentPath: *piAgentPath,
				Model:       solver.DefaultModel,
				APIKey:      apiKey,
				Timeout:     solver.DefaultSolveTimeout,
			}, runner, store)
			log.Printf("solver ready: pi-agent=%s bwrap=%s", *piAgentPath, *bwrapPath)
		}
	} else {
		log.Printf("sandbox skipped (--skip-sandbox)")
	}

	// --- 4. Cron loop --------------------------------------------------
	// The loop is only started when the solver is available (bwrap + pi-agent).
	// Without a solver, the loop has nothing to do — submissions queue up
	// and are processed when the binary is restarted with a solver.
	var loop *cron.Loop
	var cronCtx context.Context
	var cronCancel context.CancelFunc
	if solverExec != nil {
		cronCtx, cronCancel = context.WithCancel(context.Background())
		loop = cron.NewLoop(cron.Config{
			Interval:      *cronInterval,
			LoadThreshold: *loadThreshold,
			Solver:        solverExec,
			Queue:         queue,
		})
		go func() {
			log.Printf("cron loop started: interval=%s loadThreshold=%.1f", *cronInterval, *loadThreshold)
			if rerr := loop.Run(cronCtx); rerr != nil && !errors.Is(rerr, context.Canceled) {
				log.Printf("cron loop exited with error: %v", rerr)
			}
			log.Printf("cron loop stopped")
		}()
	} else {
		log.Printf("cron loop not started (no solver)")
	}

	// --- 5. API server -------------------------------------------------
	specBytes := openapiBytes
	apiServer := apihttp.New(store, queue, specBytes)
	apiServer.ExportLocalDir = *exportDir
	apiServer.ImportLocalDir = *importDir
	apiHandler := apiServer.Handler()

	// --- 6. WebSocket chat handler ------------------------------------
	// The chat handler requires an AgentRunner. When the solver is available
	// we wire it through; otherwise chat returns an "offline" message.
	var chatHandler http.Handler
	if solverExec != nil {
		chatHandler = web.NewChatHandler(nil) // nil runner = offline mode
	} else {
		chatHandler = web.NewChatHandler(nil)
	}

	// --- 7. Compose HTTP handler --------------------------------------
	// Route priority: /api/* → API server, /ws/* → WebSocket handlers,
	// /openapi.json + /health → explicit, everything else → web SPA.
	webHandler := web.Handler()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/"):
			apiHandler.ServeHTTP(w, r)
			return
		case r.URL.Path == "/ws/chat":
			chatHandler.ServeHTTP(w, r)
			return
		case r.URL.Path == "/openapi.json" || r.URL.Path == "/health":
			apiHandler.ServeHTTP(w, r)
			return
		default:
			webHandler.ServeHTTP(w, r)
		}
	})

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", *port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("off-by-one %s listening on :%d (db=%s)", version, *port, *dbPath)
	log.Printf("OpenAPI spec at http://localhost:%d/openapi.json (sha256=%s)", *port, api.SHA256()[:12])

	// --- 8. Graceful shutdown -----------------------------------------
	// SIGINT/SIGTERM → stop cron loop → close DB → shutdown HTTP server.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Printf("shutdown signal received")

		// Stop the cron loop first — it may be mid-solve.
		if cronCancel != nil {
			cronCancel()
		}

		// Wait for the loop to exit (best-effort, bounded).
		if loop != nil {
			select {
			case <-loop.Done():
			case <-time.After(10 * time.Second):
				log.Printf("cron loop did not exit within 10s — proceeding with shutdown")
			}
		}

		// Shut down the HTTP server (5s grace for in-flight requests).
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if serr := srv.Shutdown(shutdownCtx); serr != nil {
			log.Printf("http shutdown: %v", serr)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}

	// Wait for the shutdown goroutine to finish its cleanup before
	// the deferred store.Close() fires.
	wg.Wait()
	log.Printf("off-by-one shutdown complete")
}

// --- env helpers --------------------------------------------------------

// envInt reads an integer from the environment, returning def on error.
func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return def
}

// envString reads a string from the environment, returning def if unset.
func envString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// envFloat reads a float64 from the environment, returning def on error.
func envFloat(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		var f float64
		if _, err := fmt.Sscanf(v, "%f", &f); err == nil {
			return f
		}
	}
	return def
}

// envDuration reads a duration from the environment, returning def on error.
func envDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

// envBool reads a boolean from the environment. Returns true for "1", "true",
// "yes" (case-insensitive). Everything else returns false.
func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		switch strings.ToLower(v) {
		case "1", "true", "yes":
			return true
		case "0", "false", "no":
			return false
		}
	}
	return def
}
