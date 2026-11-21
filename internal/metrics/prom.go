// prom.go — Prometheus text-exposition surface (OBS-OB-001,
// docs/observability-spec.md §3.1/§4.1/§4.2).
//
// A private prometheus.Registry (never the global default, never the
// Go/process collectors) carrying the obo_* families the observability
// spec catalogues — Tier 1 (§3.1), Tier 2 (§3.2/§3.3) and Tier 3
// (§3.4, the DB-derived gauges in prom_db.go) — served by promhttp at
// GET /metrics/prom. The existing JSON GET /metrics
// (Observer.Snapshot) is untouched — the two surfaces coexist because
// dogfood docs and operator runbooks already consume the JSON one.
//
// Sources are pull-based at scrape time for in-process counters, and
// CACHE-based for anything that would need a SQLite query or a stat():
// §4.4 forbids DB reads inside Collect(), so those gauges are refreshed
// on a ticker by Prom.StartDBRefresh and merely read during a scrape.
// All sources are optional: a zero PromConfig yields a valid exposition
// with every family present at zero/one values, which is what a nil
// *Prom serves.
package metrics

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/totalwindupflightsystems/off-by-one/internal/solver"
)

// ProductValue is the value of the const `product` label carried by
// every obo_* series (OBS-OB-005). It is the product identity, not the
// host name: one series set can be scraped from several instances and
// still aggregate correctly per product.
const ProductValue = "off-by-one"

// PromConstLabels are attached to EVERY series of the private registry
// via prometheus.WrapRegistererWith, so collectors never need to repeat
// them and a new family cannot forget them (OBS-OB-005). Emitting them
// at the source means the scrape config does not have to inject
// product/service labels, and a push/remote_write path keeps them too.
//
// `service` duplicates `product` deliberately: the fleet convention
// (spec §5) labels scrape targets with both, and a series carrying them
// already is unchanged by the scrape job's own label set.
var PromConstLabels = prometheus.Labels{
	"product": ProductValue,
	"service": ProductValue,
}

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

// PromDiscoveryCounts is the scrape-time snapshot of the discover
// request outcomes behind obo_discovery_requests_total (OBS-OB-004,
// spec §3.4). Both members are cumulative since process start.
type PromDiscoveryCounts struct {
	Hits   int64
	Misses int64
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
	// DBDerived reads the DB-derived Tier-3 gauges (OBS-OB-004, spec
	// §3.4): queue depth and oldest-pending age, catalogue verified
	// answers / hit rate, and the SQLite file sizes. It is called ONLY
	// by the refresher (RefreshDB / StartDBRefresh), never from a
	// Collect() path — §4.4 requires the DB reads to be cached so a
	// scrape storm cannot become a SQL storm. Calls are bounded to
	// 2s. A leg that could not be read sets its OK flag false and is
	// omitted from the exposition rather than reported as 0. Production
	// wires this in main (queue + graph store + the --db path).
	DBDerived func(ctx context.Context) DBDerived
	// SolveResources returns the cumulative per-solve resource totals
	// and failure-cause counts (OBS-OB-003, spec §3.3). Production
	// wires *metrics.Observer.SolveResources; nil serves every family at
	// zero so the exposition keeps its shape (a missing series is
	// invisible to rate(); a zero series is not).
	SolveResources func() SolveResources
	// Scheduler returns the loop's scheduler-health view (OBS-OB-002,
	// spec §3.2): tick outcomes, gate skip reasons, the last host
	// reading, and the last tick's wall clock. Production wires
	// *metrics.Observer.SchedulerHealth. nil serves every counter label
	// at zero and omits the gauges — a gauge with no reading must not
	// claim 0 (see schedulerHealthCollector).
	Scheduler func() SchedulerHealth
	// Deferrals returns the cumulative per-reason count of entries the
	// host-pressure gate postponed (OBS-OB-004, spec §3.4). Production
	// wires *metrics.Observer.Deferrals, which always reports every
	// member of the closed SkipReasons vocabulary. nil serves every
	// reason at zero.
	Deferrals func() map[string]int64
	// Discovery returns the cumulative discover-request outcomes
	// (OBS-OB-004, spec §3.4). Production wires
	// *metrics.Observer.DiscoveryCounts. nil serves both closed outcomes
	// at zero.
	Discovery func() PromDiscoveryCounts
}

// Prom owns the private registry and its collectors. Construct once
// with NewProm and serve via Handler; collectors are built at
// construction, never per request.
type Prom struct {
	registry *prometheus.Registry
	handler  http.Handler
	duration *prometheus.HistogramVec

	// Tier-3 DB-derived gauge cache (OBS-OB-004, spec §3.4/§4.4):
	// dbSrc is read only by the refresher, db is read only by the
	// collector, and dbRefreshMu serialises refreshes so a slow
	// refresh cannot overlap the next tick.
	dbSrc       func(ctx context.Context) DBDerived
	db          *dbGaugeCache
	dbRefreshMu sync.Mutex
}

// NewProm builds the private registry with the Tier-1/2/3 families.
// It never fails: MustRegister panics only on a programming error
// (duplicate/invalid descriptor), which the fixed definitions below
// cannot produce.
//
// Every family is registered through a registerer wrapping the private
// registry with PromConstLabels (product/service), so all obo_* series
// inherit those labels without a single collector repeating them
// (OBS-OB-005). The wrap happens at construction, not at scrape time:
// a family added later gets the labels by construction too.
func NewProm(cfg PromConfig) *Prom {
	reg := prometheus.NewRegistry()
	regr := prometheus.WrapRegistererWith(PromConstLabels, reg)
	p := &Prom{registry: reg, dbSrc: cfg.DBDerived, db: newDBGaugeCache()}

	regr.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "obo_http_up",
		Help: "Whether the off-by-one HTTP API process is answering (always 1 when this exposition is served).",
	}, func() float64 { return 1 }))

	regr.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "obo_cron_loop_running",
		Help: "Whether the cron solve loop was started (1) or the process took the no-solver-available branch (0).",
	}, func() float64 {
		if cfg.LoopRunning != nil && cfg.LoopRunning() {
			return 1
		}
		return 0
	}))

	regr.MustRegister(&solvesTotalCollector{
		src: cfg.SolveCounts,
		desc: prometheus.NewDesc(
			"obo_solves_total",
			"Completed solves by outcome. Partitions the cron loop's attempt count: succeeded=SolveSuccess, rejected=SolveRejected, failed=SolveFailed-SolveRejected.",
			[]string{"outcome"}, nil,
		),
	})

	// Tier-3 DB-derived gauges (OBS-OB-004, spec §3.4/§4.4): served from
	// Prom.db, which only the refresher writes. Collect() never touches
	// SQLite or the filesystem, so a scrape storm cannot become a SQL
	// storm and the 50ms scrape budget holds regardless of DB size.
	regr.MustRegister(&dbGaugeCollector{
		cache: p.db,
		pendingDesc: prometheus.NewDesc(
			"obo_queue_pending",
			"Queue entries pending, deferred, or in_progress — the same count /api/v1/stats serves as queue_depth (refreshed from the DB every 15s, never queried per scrape).",
			nil, nil,
		),
		oldestPendingAgeDesc: prometheus.NewDesc(
			"obo_queue_oldest_pending_age_seconds",
			"Age in seconds of the oldest waiting entry (now - MIN(created_at) over status IN ('pending','deferred','in_progress')) — answers \"is a submission stuck?\". Refreshed from the DB every 15s.",
			nil, nil,
		),
		verifiedAnswersDesc: prometheus.NewDesc(
			"obo_catalog_verified_answers",
			"Answer nodes the catalogue counts as verified — the same figure /api/v1/stats serves as verified_answers (status verified/ci_passed and no failed result signature). Refreshed every 15s.",
			nil, nil,
		),
		hitRateDesc: prometheus.NewDesc(
			"obo_catalog_hit_rate",
			"Catalog hit rate: verified_answers / total_answers, 0 when the catalogue is empty — the same ratio /api/v1/stats serves as hit_rate. Refreshed every 15s.",
			nil, nil,
		),
		sizeDesc: prometheus.NewDesc(
			"obo_db_size_bytes",
			"On-disk size of the SQLite store by part (closed set: main, wal). A missing -wal file is a real 0 (the DB is not in WAL mode); a failed stat omits the part instead of claiming a size.",
			[]string{"kind"}, nil,
		),
	})

	regr.MustRegister(&deferredTotalCollector{
		src: cfg.Deferrals,
		desc: prometheus.NewDesc(
			"obo_queue_deferred_total",
			"Entries the host-pressure gate postponed, by gate reason (closed set: load, memory, load+memory). Counts displaced work, not gate decisions: a gated tick with an empty queue moves nothing here.",
			[]string{"reason"}, nil,
		),
	})

	regr.MustRegister(&discoveryTotalCollector{
		src: cfg.Discovery,
		desc: prometheus.NewDesc(
			"obo_discovery_requests_total",
			"Discover requests by outcome (closed set: hit, miss). Distinct from obo_catalog_hit_rate, which measures catalogue coverage rather than discovery outcomes.",
			[]string{"outcome"}, nil,
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
	regr.MustRegister(p.duration)

	// Tier 2 — solve resources and failure causes (OBS-OB-003, spec
	// §3.3). Sourced from the Observer's cumulative totals, which never
	// go backwards even though its per-solve record window is bounded.
	regr.MustRegister(&solvePeakRSSCollector{
		src: cfg.SolveResources,
		desc: prometheus.NewDesc(
			"obo_solve_peak_rss_bytes",
			"Distribution of solve peak RSS in bytes — catches a 47GB-style regression before the OOM. One observation per solve (failed solves and harness rejections included).",
			nil, nil,
		),
	})
	regr.MustRegister(&solveCountersCollector{
		src: cfg.SolveResources,
		specs: []solveCounterSpec{
			{
				desc: prometheus.NewDesc(
					"obo_solve_killed_total",
					"How often the per-solve memory cap actually killed a solve (SIGKILL/SIGSEGV from RLIMIT_AS or the kernel OOM killer).",
					nil, nil,
				),
				value: func(r SolveResources) int64 { return r.Killed },
			},
			{
				desc: prometheus.NewDesc(
					"obo_solve_over_budget_total",
					"How often a solve's peak RSS exceeded the alert threshold (4 GiB by default) without necessarily being killed.",
					nil, nil,
				),
				value: func(r SolveResources) int64 { return r.OverBudget },
			},
			{
				desc: prometheus.NewDesc(
					"obo_solve_rejected_total",
					"How often the harness refused a near-cap solve after it returned success (peak above the reject threshold; nothing committed).",
					nil, nil,
				),
				value: func(r SolveResources) int64 { return r.Rejected },
			},
		},
	})
	regr.MustRegister(&solveFailureCollector{
		src: cfg.SolveResources,
		desc: prometheus.NewDesc(
			"obo_solve_failure_total",
			"Failed solves by cause (closed set: timeout, solver_exit, toolchain_missing, invalid_problem, commit, stale_reaped, other). Classified at the failure site, never by re-parsing stored text.",
			[]string{"cause"}, nil,
		),
	})

	// Tier 2 — scheduler health and starvation (OBS-OB-002, spec §3.2).
	// This is the tier that makes "the loop is alive and answering HTTP
	// but never solving" visible: idle_gated ticks plus a fresh
	// last-tick timestamp while solves_total stays flat.
	regr.MustRegister(&schedulerHealthCollector{
		src: cfg.Scheduler,
		tickDesc: prometheus.NewDesc(
			"obo_cron_ticks_total",
			"Loop ticks by outcome (closed set: solved, empty, idle_gated, error). A starving loop keeps ticking with outcome=idle_gated; a dead loop stops advancing obo_cron_last_tick_timestamp_seconds.",
			[]string{"outcome"}, nil,
		),
		skipReasonDesc: prometheus.NewDesc(
			"obo_cron_skip_reason_total",
			"Host-gate skips by reason (closed set: load, memory, load+memory). When reason=load, the gate is admitting it: rising rate here with flat obo_solves_total is idle-by-gate, not idle-by-absence.",
			[]string{"reason"}, nil,
		),
		loadDesc: prometheus.NewDesc(
			"obo_host_load1",
			"Host 1-minute load average as read by the idle gate (mirrors HostSnapshot.Load1).",
			nil, nil,
		),
		memDesc: prometheus.NewDesc(
			"obo_host_mem_used_fraction",
			"Host memory used fraction (0-1) as read by the memory gate — 1.0 minus MemAvailable/MemTotal (mirrors HostSnapshot.MemUsedFraction).",
			nil, nil,
		),
		lastTickDesc: prometheus.NewDesc(
			"obo_cron_last_tick_timestamp_seconds",
			"Unix seconds of the most recent loop tick — advances on every tick including idle_gated and empty ones, so staleness means the loop is dead or hung rather than merely gated.",
			nil, nil,
		),
	})

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

// deferredTotalCollector emits obo_queue_deferred_total{reason} from the
// Observer's cumulative deferral tallies (OBS-OB-004, spec §3.4) over
// the CLOSED SkipReasons vocabulary — the same strings the gate writes
// into queue_entries.gate_reason. Every reason is present on every
// scrape, at 0 before it first fires, so rate() has a series to alert
// on. A reason outside the set can never appear: RecordDeferred drops
// it, and this collector only iterates the vocabulary.
type deferredTotalCollector struct {
	src  func() map[string]int64
	desc *prometheus.Desc
}

func (c *deferredTotalCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

func (c *deferredTotalCollector) Collect(ch chan<- prometheus.Metric) {
	var counts map[string]int64
	if c.src != nil {
		counts = c.src()
	}
	for _, reason := range SkipReasons {
		n := counts[reason]
		if n < 0 {
			n = 0
		}
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.CounterValue, float64(n), reason)
	}
}

// discoveryTotalCollector emits obo_discovery_requests_total{outcome}
// over the CLOSED two-value DiscoveryOutcomes vocabulary (OBS-OB-004,
// spec §3.4). Both outcomes are always present, so a hit-driven alert
// cannot be fooled by a series that only exists once a miss happened.
type discoveryTotalCollector struct {
	src  func() PromDiscoveryCounts
	desc *prometheus.Desc
}

func (c *discoveryTotalCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

func (c *discoveryTotalCollector) Collect(ch chan<- prometheus.Metric) {
	var counts PromDiscoveryCounts
	if c.src != nil {
		counts = c.src()
	}
	values := map[string]int64{
		DiscoveryOutcomes[0]: counts.Hits,   // "hit"
		DiscoveryOutcomes[1]: counts.Misses, // "miss"
	}
	for _, outcome := range DiscoveryOutcomes {
		n := values[outcome]
		if n < 0 {
			n = 0
		}
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.CounterValue, float64(n), outcome)
	}
}

// solvePeakRSSCollector emits obo_solve_peak_rss_bytes as a cumulative
// histogram built from the Observer's cumulative bucket counters
// (OBS-OB-003). MustNewConstHistogram takes the FINITE buckets as a
// map upper-bound → cumulative count and synthesises le="+Inf" from the
// sample count, so the +Inf series always equals the number of solves
// recorded — the invariant acceptance criterion 3 checks on the wire.
type solvePeakRSSCollector struct {
	src  func() SolveResources
	desc *prometheus.Desc
}

func (c *solvePeakRSSCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

func (c *solvePeakRSSCollector) Collect(ch chan<- prometheus.Metric) {
	var r SolveResources
	if c.src != nil {
		r = c.src()
	}
	buckets := make(map[float64]uint64, len(SolvePeakRSSBuckets))
	for i, bound := range SolvePeakRSSBuckets {
		var n uint64
		if i < len(r.PeakRSSBucketCounts) {
			n = r.PeakRSSBucketCounts[i]
		}
		buckets[float64(bound)] = n
	}
	count := r.PeakRSSCount
	if count < 0 {
		count = 0
	}
	ch <- prometheus.MustNewConstHistogram(c.desc, uint64(count), float64(r.PeakRSSSum), buckets)
}

// solveCounterSpec is one unlabelled counter derived from the resource
// snapshot.
type solveCounterSpec struct {
	desc  *prometheus.Desc
	value func(SolveResources) int64
}

// solveCountersCollector emits an arbitrary set of unlabelled counters
// from one source snapshot. Describe advertises every descriptor Collect
// can emit (a Collector that omits them fails registration).
type solveCountersCollector struct {
	src   func() SolveResources
	specs []solveCounterSpec
}

func (c *solveCountersCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, spec := range c.specs {
		ch <- spec.desc
	}
}

func (c *solveCountersCollector) Collect(ch chan<- prometheus.Metric) {
	var r SolveResources
	if c.src != nil {
		r = c.src()
	}
	for _, spec := range c.specs {
		v := spec.value(r)
		if v < 0 {
			v = 0
		}
		ch <- prometheus.MustNewConstMetric(spec.desc, prometheus.CounterValue, float64(v))
	}
}

// schedulerHealthCollector emits the five Tier-2 scheduler families
// (OBS-OB-002, spec §3.2): two labelled counters over CLOSED
// vocabularies plus three unlabelled gauges.
//
// Why a Collector and not GaugeFunc/CounterVec:
//   - A GaugeFunc always emits, so it cannot satisfy "no reading yet →
//     omit the gauge rather than emitting NaN". A nil source or an
//     empty host reading here omits the series instead.
//   - A CounterVec can only Add; the source is already cumulative, so
//     mirroring it into a vec would need Set (which counters lack) or
//     would double-count on every scrape. Const metrics read the
//     cumulative source exactly once per scrape with no state to drift.
//
// Both label vocabularies are emitted in full on every scrape, at 0
// before their first occurrence: a series that does not exist yet is
// invisible to rate(), which is precisely the alert that must fire the
// first time the gate closes.
type schedulerHealthCollector struct {
	src            func() SchedulerHealth
	tickDesc       *prometheus.Desc
	skipReasonDesc *prometheus.Desc
	loadDesc       *prometheus.Desc
	memDesc        *prometheus.Desc
	lastTickDesc   *prometheus.Desc
}

func (c *schedulerHealthCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.tickDesc
	ch <- c.skipReasonDesc
	ch <- c.loadDesc
	ch <- c.memDesc
	ch <- c.lastTickDesc
}

func (c *schedulerHealthCollector) Collect(ch chan<- prometheus.Metric) {
	var h SchedulerHealth
	if c.src != nil {
		h = c.src()
	}
	for _, outcome := range TickOutcomes {
		n := h.Ticks[outcome]
		if n < 0 {
			n = 0
		}
		ch <- prometheus.MustNewConstMetric(c.tickDesc, prometheus.CounterValue, float64(n), outcome)
	}
	for _, reason := range SkipReasons {
		n := h.SkipReasons[reason]
		if n < 0 {
			n = 0
		}
		ch <- prometheus.MustNewConstMetric(c.skipReasonDesc, prometheus.CounterValue, float64(n), reason)
	}

	// Gauges: no reading yet means OMIT, never 0. A zero load average
	// is a real (idle-host) value so it is emitted whenever a snapshot
	// exists; a zero memory fraction is not — a live host always has
	// memory in use, so 0 means "the memory probe was disabled or
	// failed" and claiming it would invert the gate's own judgement.
	if h.HostOK {
		ch <- prometheus.MustNewConstMetric(c.loadDesc, prometheus.GaugeValue, h.Host.Load1)
		if h.Host.MemUsedFraction > 0 {
			ch <- prometheus.MustNewConstMetric(c.memDesc, prometheus.GaugeValue, h.Host.MemUsedFraction)
		}
	}
	// Fractional unix seconds: a scrape-to-scrape delta must be visible
	// even when ticks are milliseconds apart (tests) — Prometheus
	// timestamps are float seconds, so this loses nothing.
	if h.LastTickOK {
		ch <- prometheus.MustNewConstMetric(c.lastTickDesc, prometheus.GaugeValue,
			float64(h.LastTick.UnixNano())/1e9)
	}
}

// solveFailureCollector emits obo_solve_failure_total{cause} over the
// CLOSED seven-value cause vocabulary (OBS-OB-003). Every cause is
// always present — a zero series is what makes rate()/absent() alerts
// expressible on a class that has not fired yet.
type solveFailureCollector struct {
	src  func() SolveResources
	desc *prometheus.Desc
}

func (c *solveFailureCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

func (c *solveFailureCollector) Collect(ch chan<- prometheus.Metric) {
	var counts map[string]int64
	if c.src != nil {
		counts = c.src().Failures
	}
	for _, cause := range solver.FailureCauses {
		n := counts[cause]
		if n < 0 {
			n = 0
		}
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.CounterValue, float64(n), cause)
	}
}
