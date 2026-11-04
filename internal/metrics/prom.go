// prom.go — Prometheus text-exposition surface (OBS-OB-001,
// docs/observability-spec.md §3.1/§4.1/§4.2).
//
// A private prometheus.Registry (never the global default, never the
// Go/process collectors) carrying exactly the five Tier-1 obo_*
// families, served by promhttp at GET /metrics/prom. The existing JSON
// GET /metrics (Observer.Snapshot) is untouched — the two surfaces
// coexist because dogfood docs and operator runbooks already consume
// the JSON one.
//
// Sources are pull-based at scrape time: the cron loop's counters
// (internal/cron.Metrics — previously dead code, "will eventually
// expose"), the queue depth (same query /api/v1/stats uses), and the
// solver-availability signal. All sources are optional: a zero
// PromConfig yields a valid exposition with every family present at
// zero/one values, which is what a nil *Prom serves.
package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// solveDurationBuckets are the obo_solve_duration_seconds bucket
// boundaries (seconds). Chosen around the observed solve-time mean of
// ~226s over 3354 completed solves (spec §3.1).
var solveDurationBuckets = []float64{30, 60, 120, 180, 300, 600, 1200, 2400}

// solveOutcomes is the fixed label vocabulary for obo_solves_total and
// obo_solve_duration_seconds. No free-text labels (spec §3.5).
var solveOutcomes = []string{"succeeded", "failed", "rejected"}

// PromSolveCounts is the scrape-time snapshot of the cron loop's solve
// counters, using internal/cron.Metrics semantics: Failed INCLUDES
// Rejected (a rejected solve is a failure that also increments the
// dedicated counter). The collector partitions them — see
// solvesTotalCollector.Collect.
type PromSolveCounts struct {
	Success  int64
	Failed   int64
	Rejected int64
}

// PromConfig supplies the live sources for the obo_* families. Every
// field is optional; a nil source serves the family at its zero value.
type PromConfig struct {
	// LoopRunning reports whether the cron loop was started (false
	// when main took the "no solver available" branch). Wired from
	// the same signal as api.Server.SolverAvailable.
	LoopRunning func() bool
	// SolveCounts returns the loop's solve counters (see
	// PromSolveCounts for the failed/rejected overlap).
	SolveCounts func() PromSolveCounts
	// QueuePending returns the pending/deferred/in_progress queue
	// depth — the same query /api/v1/stats serves as queue_depth, so
	// the two numbers agree at any instant.
	QueuePending func(ctx context.Context) (int64, error)
}

// Prom owns the private registry and its collectors. Construct once
// with NewProm and serve via Handler; collectors are built at
// construction, never per request.
type Prom struct {
	registry *prometheus.Registry
	handler  http.Handler
	duration *prometheus.HistogramVec
}

// NewProm builds the private registry with the five Tier-1 families.
// It never fails: MustRegister panics only on a programming error
// (duplicate/invalid descriptor), which the fixed definitions below
// cannot produce.
func NewProm(cfg PromConfig) *Prom {
	reg := prometheus.NewRegistry()
	p := &Prom{registry: reg}

	reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "obo_http_up",
		Help: "Whether the off-by-one HTTP API process is answering (always 1 when this exposition is served).",
	}, func() float64 { return 1 }))

	reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "obo_cron_loop_running",
		Help: "Whether the cron solve loop was started (1) or the process took the no-solver-available branch (0).",
	}, func() float64 {
		if cfg.LoopRunning != nil && cfg.LoopRunning() {
			return 1
		}
		return 0
	}))

	reg.MustRegister(&solvesTotalCollector{
		src: cfg.SolveCounts,
		desc: prometheus.NewDesc(
			"obo_solves_total",
			"Completed solves by outcome. Partitions the cron loop's attempt count: succeeded=SolveSuccess, rejected=SolveRejected, failed=SolveFailed-SolveRejected.",
			[]string{"outcome"}, nil,
		),
	})

	reg.MustRegister(&queuePendingCollector{
		src: cfg.QueuePending,
		desc: prometheus.NewDesc(
			"obo_queue_pending",
			"Queue entries pending, deferred, or in_progress — the same count /api/v1/stats serves as queue_depth.",
			nil, nil,
		),
	})

	p.duration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "obo_solve_duration_seconds",
		Help:    "Wall time per completed solve by outcome (buckets chosen around the ~226s observed mean).",
		Buckets: solveDurationBuckets,
	}, []string{"outcome"})
	// Pre-create one child per outcome so the family (and its
	// _bucket series) is present in the exposition even before the
	// first solve completes after a restart.
	for _, outcome := range solveOutcomes {
		p.duration.WithLabelValues(outcome)
	}
	reg.MustRegister(p.duration)

	p.handler = promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	return p
}

// sharedZeroProm backs Handler/Gatherer on a nil *Prom: a zero-source
// registry, built once, so a Server with no Prom wired still answers
// GET /metrics/prom with 200 and every family present (unit tests
// construct Server directly).
var sharedZeroProm = NewProm(PromConfig{})

// Handler returns the promhttp exposition handler (Content-Type
// text/plain; version=0.0.4). Nil-receiver-safe: serves the shared
// zero-source registry.
func (p *Prom) Handler() http.Handler {
	if p == nil {
		return sharedZeroProm.handler
	}
	return p.handler
}

// Gatherer exposes the registry for tests that parse the exposition
// without going through HTTP. Nil-receiver-safe.
func (p *Prom) Gatherer() prometheus.Gatherer {
	if p == nil {
		return sharedZeroProm.registry
	}
	return p.registry
}

// ObserveSolveDuration records one completed solve's wall time under
// the given outcome label. Called once per solve from the cron loop's
// completion sites; nil-receiver-safe so an unwired loop is a no-op.
func (p *Prom) ObserveSolveDuration(outcome string, d time.Duration) {
	if p == nil {
		return
	}
	p.duration.WithLabelValues(outcome).Observe(d.Seconds())
}

// solvesTotalCollector emits obo_solves_total{outcome} from the cron
// loop's counters.
//
// Partition semantics (OBS-OB-001): cron.Metrics counts a rejected
// solve in BOTH SolveFailed and SolveRejected (see the SolveRejected
// field comment in internal/cron/loop.go). Emitting failed=SolveFailed
// and rejected=SolveRejected side by side would double-count under
// sum(), so the series are partitioned:
//
//	succeeded = SolveSuccess
//	rejected  = SolveRejected
//	failed    = SolveFailed - SolveRejected
//
// and sum(obo_solves_total) == SolveAttempts exactly.
type solvesTotalCollector struct {
	src  func() PromSolveCounts
	desc *prometheus.Desc
}

func (c *solvesTotalCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

func (c *solvesTotalCollector) Collect(ch chan<- prometheus.Metric) {
	var s PromSolveCounts
	if c.src != nil {
		s = c.src()
	}
	failed := s.Failed - s.Rejected
	if failed < 0 {
		// Defensive: an inconsistent source must not produce a
		// negative counter value.
		failed = 0
	}
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.CounterValue, float64(s.Success), "succeeded")
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.CounterValue, float64(failed), "failed")
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.CounterValue, float64(s.Rejected), "rejected")
}

// queuePendingCollector emits obo_queue_pending from queue.Depth. On a
// source error the series is omitted for that scrape — a missing
// series is honest, a fabricated 0 is not.
type queuePendingCollector struct {
	src  func(ctx context.Context) (int64, error)
	desc *prometheus.Desc
}

func (c *queuePendingCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

func (c *queuePendingCollector) Collect(ch chan<- prometheus.Metric) {
	if c.src == nil {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, 0)
		return
	}
	// The scrape has no request context of its own; bound the read so
	// a wedged store cannot hang the collector walk.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	n, err := c.src(ctx)
	cancel()
	if err != nil {
		return
	}
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, float64(n))
}
