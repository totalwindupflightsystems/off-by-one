// Package metrics records per-solve and host-pressure observability
// data for the off-by-one cron loop (DF-OFF-BY-ONE-32).
//
// Why a separate package: the cron loop already keeps coarse counters
// (attempts / success / failed / skipped) in internal/cron.Metrics,
// but the 2026-10-03 RAM incident showed those are not enough — a
// solve that eats 47 GB looks identical to a healthy one in
// attempt counts, and the only signal was a human noticing "RAM
// almost full". This package owns the per-solve record (peak RSS,
// wall time, test-run count, kill flag) and the host-pressure skip
// counter, so a recurrence is caught by structured log lines and a
// queryable endpoint instead of by observation.
//
// Everything here is add-only and non-blocking: Record* take a lock,
// mutate, and return; readers use Snapshot / Recent, which copy under
// the same lock. Nothing in this package ever blocks the solve loop
// beyond a mutex hold measured in nanoseconds.
//
// The zero Observer is usable and records nothing (nil-receiver-safe
// methods are NOT provided — use New() to get a working Observer).
// A nil *Observer is however tolerated by the cron loop wiring
// (see cron.Config.Observer): the loop calls the n.method forms,
// which are no-ops on a nil receiver, so "metrics disabled" is the
// zero config rather than a separate flag.
package metrics

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/solver"
)

// SolvePeakRSSBuckets are the obo_solve_peak_rss_bytes histogram bucket
// upper bounds in bytes (OBS-OB-003, docs/observability-spec.md §3.3):
// 256 MiB, 512 MiB, 1 GiB, 2.5 GiB, 4 GiB. The top bound is the default
// alert threshold, so every solve that would have paged someone lands in
// the last finite bucket or in +Inf.
var SolvePeakRSSBuckets = []uint64{256 << 20, 512 << 20, 1 << 30, 2684354560, 4 << 30}

// DefaultRSSAlertBytes is the default over-budget threshold: 4 GiB.
// The unit-level cap (DF-OFF-BY-ONE-30) bounds a single solve at
// RLIMIT_AS = 6 GiB by default; a solve that peaks above 4 GiB is
// surviving the cap but is close enough to it (and to the host's
// swap) that it must page someone. The threshold is fixed at
// construction time in cmd/off-by-one/main.go; there is no flag or
// env override.
const DefaultRSSAlertBytes uint64 = 4 << 30

// SolveRecord is one solve's resource footprint. PeakRSSBytes is the
// high-water mark of the sandboxed process tree's resident set,
// sampled while pi-agent ran (see sandbox.Sampler). TestRuns counts
// `go test`-style invocations pi-agent made (parsed from its stdout);
// it is the field that was 47 GB on the incident solve. Killed reports
// whether the solve died on its memory cap (exec.ExitError) rather
// than finishing. Rejected reports whether the harness refused the
// solve AFTER it returned success because its observed peak RSS
// exceeded the configured reject threshold (DF-OFF-BY-ONE-31); a
// rejected solve is never committed to the graph, so Success is false
// for it even though the solver exited cleanly.
type SolveRecord struct {
	SubmissionID   string `json:"submission_id"`
	ProblemClass   string `json:"problem_class"`
	Model          string `json:"model"`
	PeakRSSBytes   uint64 `json:"peak_rss_bytes"`
	WallTimeMS     int64  `json:"wall_time_ms"`
	TestRuns       int    `json:"test_runs"`
	Killed         bool   `json:"killed"`
	Success        bool   `json:"success"`
	Rejected       bool   `json:"rejected"`
	OverBudget     bool   `json:"over_budget"`
	AlertBytesUsed uint64 `json:"alert_threshold_bytes"`
}

// Tick outcome vocabulary — the closed `outcome` label set of
// obo_cron_ticks_total (OBS-OB-002, docs/observability-spec.md §3.2).
// Every tick of the cron loop settles on exactly one of these; an
// outcome outside the set is dropped, never added as a label value
// (spec §3.5: closed enumerations only).
const (
	// TickOutcomeSolved is a tick that solved and committed an entry.
	TickOutcomeSolved = "solved"
	// TickOutcomeEmpty is a tick that found nothing in the queue.
	TickOutcomeEmpty = "empty"
	// TickOutcomeIdleGated is a tick the host-pressure gate deferred:
	// the loop is alive, but it deliberately did no work. This is the
	// outcome that makes starvation alertable — the whole point of
	// OBS-OB-002.
	TickOutcomeIdleGated = "idle_gated"
	// TickOutcomeError is a tick that failed for any other reason.
	TickOutcomeError = "error"
)

// TickOutcomes is the complete `outcome` label set for
// obo_cron_ticks_total. Every member is emitted on every scrape, at
// zero before its first occurrence, so rate()-based alerts have a
// series to fire on (a missing series is invisible to rate()).
var TickOutcomes = []string{
	TickOutcomeSolved,
	TickOutcomeEmpty,
	TickOutcomeIdleGated,
	TickOutcomeError,
}

// SkipReasons is the complete `reason` label set for
// obo_cron_skip_reason_total — the same vocabulary TrimSpaceReason
// produces from the gate's parts, and the same strings the deferral
// log line names. A reason outside this set is dropped rather than
// widening the exposition's cardinality.
var SkipReasons = []string{"load", "memory", "load+memory"}

// DiscoveryOutcomes is the complete `outcome` label set for
// obo_discovery_requests_total (OBS-OB-004, docs/observability-spec.md
// §3.4): a discover request either resolved an exact answer (hit) or
// found nothing for the requested class (miss). Invalid requests (400)
// and internal errors (500) are counted as neither — they measure the
// caller and the store, not discovery outcomes (see RecordDiscovery).
var DiscoveryOutcomes = []string{"hit", "miss"}

// IsTickOutcome reports whether outcome is a member of TickOutcomes.
func IsTickOutcome(outcome string) bool { return containsString(TickOutcomes, outcome) }

// IsSkipReason reports whether reason is a member of SkipReasons.
func IsSkipReason(reason string) bool { return containsString(SkipReasons, reason) }

// IsDiscoveryOutcome reports whether outcome is a member of
// DiscoveryOutcomes.
func IsDiscoveryOutcome(outcome string) bool {
	return containsString(DiscoveryOutcomes, outcome)
}

func containsString(set []string, want string) bool {
	for _, v := range set {
		if v == want {
			return true
		}
	}
	return false
}

// SchedulerHealth is the scrape-time view of the loop's scheduler
// state (OBS-OB-002, spec §3.2): cumulative tick outcomes, cumulative
// skip reasons, the last host reading, and the last tick's wall clock.
// Ticks and SkipReasons are copies — mutating them never touches the
// Observer.
type SchedulerHealth struct {
	// Ticks counts every tick by outcome (TickOutcomes members only).
	Ticks map[string]int64
	// SkipReasons counts host-gate skips by reason (SkipReasons
	// members only).
	SkipReasons map[string]int64
	// Host is the latest gate reading; HostOK is false before the
	// first tick reached the gate, in which case the host gauges are
	// omitted from the exposition rather than reported as 0.
	Host   HostSnapshot
	HostOK bool
	// LastTick is the wall clock stamped at the top of the most
	// recent tick; LastTickOK is false before the loop ever ticked.
	LastTick   time.Time
	LastTickOK bool
}

// HostSnapshot is one host-pressure observation taken when the gate
// runs. Load1 is the 1-minute load average; MemUsedFraction is
// MemAvailableBytes-derived (1.0 = full). Reason is the gate's skip
// reason ("load", "memory", "load+memory") for the run that skipped.
type HostSnapshot struct {
	Load1             float64 `json:"load1"`
	MemUsedFraction   float64 `json:"mem_used_fraction"`
	MemTotalBytes     uint64  `json:"mem_total_bytes"`
	MemAvailableBytes uint64  `json:"mem_available_bytes"`
	Reason            string  `json:"reason,omitempty"`
}

// Observer aggregates per-solve records and host-pressure skips.
// Construct with New; all methods are safe for concurrent use.
type Observer struct {
	mu sync.Mutex

	alertBytes uint64
	logger     *log.Logger

	records     []SolveRecord
	skips       int64
	lastSkipped *HostSnapshot
	lastHost    *HostSnapshot

	// discoveryHits/discoveryMisses are request-path counters for
	// POST /api/v1/problems/discover. They are deliberately separate
	// from the /api/v1/stats hit_rate ratio, which is
	// verified_answers/total_answers over the catalog — not a
	// discovery-outcome measure.
	discoveryHits   int64
	discoveryMisses int64

	// Scheduler-health totals (OBS-OB-002, spec §3.2): cumulative tick
	// outcomes, cumulative gate skip reasons, and the wall clock of the
	// last tick. Kept here (not in cron.Metrics) because the exposition
	// already reads this Observer and because the skip-reason tally
	// belongs on the same path that records the skip. Both maps are
	// keyed by the CLOSED vocabularies above — no caller can widen them.
	tickCounts   map[string]int64
	reasonCounts map[string]int64
	lastTick     time.Time

	// deferralCounts counts how often the host-pressure gate actually
	// POSTPONED waiting work — one increment per entry it stamped
	// deferred — keyed by the same closed reason vocabulary the gate
	// writes into queue_entries.gate_reason (SkipReasons). It is the
	// obo_queue_deferred_total source (OBS-OB-004, spec §3.4); a
	// reason outside the set is dropped, never added as a series.
	deferralCounts map[string]int64

	// Cumulative per-solve resource and failure-cause totals feeding
	// obo_solve_* on GET /metrics/prom (OBS-OB-003). They are kept apart
	// from o.records because records is a bounded (and trimmable) window:
	// a counter derived from it would go backwards. peakBuckets is the
	// cumulative count per SolvePeakRSSBuckets entry plus a final +Inf
	// slot, i.e. len(SolvePeakRSSBuckets)+1 entries.
	killed        int64
	overBudget    int64
	rejected      int64
	peakBuckets   []uint64
	peakSum       uint64
	peakCount     int64
	failureCauses map[string]int64
}

// SolveResources is the cumulative resource + failure-attribution view
// of every solve the Observer has recorded. It exists for the Prometheus
// exposition (OBS-OB-003): the JSON Snapshot keeps its historical shape
// (a bounded window plus last/last-host), while these totals never go
// backwards.
type SolveResources struct {
	// PeakRSSBucketCounts are CUMULATIVE counts: entry i counts solves
	// whose peak was <= SolvePeakRSSBuckets[i]; the final entry is the
	// +Inf bucket, so PeakRSSBucketCounts[len-1] == PeakRSSCount.
	PeakRSSBucketCounts []uint64
	// PeakRSSSum is the sum of every observed peak in bytes.
	PeakRSSSum uint64
	// PeakRSSCount is the number of observations (one per solve, failed
	// solves and harness rejections included — they consumed memory too).
	PeakRSSCount int64
	// Killed counts solves that died on their memory cap.
	Killed int64
	// OverBudget counts solves whose peak exceeded the alert threshold.
	OverBudget int64
	// Rejected counts harness-side rejections (peak above the reject
	// threshold, result never committed).
	Rejected int64
	// Failures counts failures by cause. Every key is a member of
	// solver.FailureCauses; a nil map means "no failures recorded".
	Failures map[string]int64
}

// New returns an Observer that keeps the last keepRecent records and
// logs an ERROR line (to logger, or the standard logger when nil)
// whenever a solve's peak RSS exceeds alertBytes. alertBytes <= 0
// falls back to DefaultRSSAlertBytes.
func New(alertBytes uint64, logger *log.Logger) *Observer {
	if alertBytes == 0 {
		alertBytes = DefaultRSSAlertBytes
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Observer{
		alertBytes:     alertBytes,
		logger:         logger,
		peakBuckets:    make([]uint64, len(SolvePeakRSSBuckets)+1),
		failureCauses:  make(map[string]int64),
		tickCounts:     make(map[string]int64),
		reasonCounts:   make(map[string]int64),
		deferralCounts: make(map[string]int64),
	}
}

// RecordSolve appends rec, marks it over-budget when its peak RSS
// exceeds the alert threshold, and emits the over-budget ERROR line:
//
//	solve ALERT: peak RSS 5368709120 (5.0 GiB) exceeds 4294967296 (4.0 GiB) - problem_class=go-bbr-pacing submission=sub_x model=deepseek-v4-flash killed=false
//
// The line is the alerting contract: ops-side alerting tails ERROR
// lines; this package does not page anyone itself.
func (o *Observer) RecordSolve(rec SolveRecord) {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.alertBytesUsed(&rec)
	over := rec.PeakRSSBytes > o.alertBytes
	rec.OverBudget = over
	o.records = append(o.records, rec)
	// Cumulative resource totals (OBS-OB-003) — independent of the
	// bounded records window above.
	o.observePeakLocked(rec.PeakRSSBytes)
	if rec.Killed {
		o.killed++
	}
	if over {
		o.overBudget++
	}
	if rec.Rejected {
		o.rejected++
	}
	o.mu.Unlock()

	if over {
		// A zero-value Observer (struct literal, never New'ed) has no
		// logger; fall back to the standard one instead of panicking on
		// a nil *log.Logger.
		logger := o.logger
		if logger == nil {
			logger = log.Default()
		}
		logger.Printf("ERROR solve ALERT: peak RSS %d (%.1f GiB) exceeds %d (%.1f GiB) — problem_class=%s submission=%s model=%s killed=%t",
			rec.PeakRSSBytes, gib(rec.PeakRSSBytes),
			o.alertBytes, gib(o.alertBytes),
			rec.ProblemClass, rec.SubmissionID, rec.Model, rec.Killed)
	}
}

// observePeakLocked folds one solve's peak RSS into the cumulative
// bucket counters. Caller holds mu.
func (o *Observer) observePeakLocked(peak uint64) {
	if len(o.peakBuckets) != len(SolvePeakRSSBuckets)+1 {
		// A zero-value Observer (never New'ed) still has to produce a
		// consistent histogram rather than panicking on a short slice.
		o.peakBuckets = make([]uint64, len(SolvePeakRSSBuckets)+1)
	}
	for i, bound := range SolvePeakRSSBuckets {
		if peak <= bound {
			o.peakBuckets[i]++
		}
	}
	o.peakBuckets[len(SolvePeakRSSBuckets)]++
	o.peakSum += peak
	o.peakCount++
}

// RecordFailureCause counts n failures attributed to cause. A value
// outside the closed solver.FailureCauses set is coalesced to
// solver.CauseOther: the label set is closed by contract (spec §3.3), so
// a caller passing free text can never widen the exposition's
// cardinality. n <= 0 is a no-op (the stale-reaper calls this with the
// number of rows it reaped, which can legitimately be 0).
func (o *Observer) RecordFailureCause(cause string, n int64) {
	if o == nil || n <= 0 {
		return
	}
	if !solver.IsFailureCause(cause) {
		cause = solver.CauseOther
	}
	o.mu.Lock()
	if o.failureCauses == nil {
		o.failureCauses = make(map[string]int64)
	}
	o.failureCauses[cause] += n
	o.mu.Unlock()
}

// SolveResources returns the cumulative resource + failure-cause totals
// (OBS-OB-003). A copy: mutating the result never touches the Observer.
// Nil-receiver-safe — a nil Observer yields zeros with a full-length
// bucket slice so the caller's histogram is still well-formed.
func (o *Observer) SolveResources() SolveResources {
	buckets := make([]uint64, len(SolvePeakRSSBuckets)+1)
	if o == nil {
		return SolveResources{PeakRSSBucketCounts: buckets}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	copy(buckets, o.peakBuckets)
	failures := make(map[string]int64, len(o.failureCauses))
	for cause, n := range o.failureCauses {
		failures[cause] = n
	}
	return SolveResources{
		PeakRSSBucketCounts: buckets,
		PeakRSSSum:          o.peakSum,
		PeakRSSCount:        o.peakCount,
		Killed:              o.killed,
		OverBudget:          o.overBudget,
		Rejected:            o.rejected,
		Failures:            failures,
	}
}

// alertBytesUsed stamps the threshold the record was graded against
// onto the record itself, so a later threshold change cannot make
// stored records' OverBudget flag unverifiable. Caller holds mu.
func (o *Observer) alertBytesUsed(rec *SolveRecord) {
	rec.AlertBytesUsed = o.alertBytes
}

// RecordSkip counts a run skipped for host pressure and stores snap
// (with Reason filled) as both the last skip and the last host
// observation.
//
// The skip is also tallied into the reason counter behind
// obo_cron_skip_reason_total (OBS-OB-002). The vocabulary is CLOSED —
// TrimSpaceReason's three values — so a caller passing free text is
// dropped instead of creating a new series (spec §3.5). Dropping loses
// one sample; widening the label set would make the family's
// cardinality unbounded, which is worse and unrecoverable.
func (o *Observer) RecordSkip(snap HostSnapshot) {
	if o == nil {
		return
	}
	reason := snap.Reason
	o.mu.Lock()
	o.skips++
	cp := snap
	o.lastSkipped = &cp
	o.lastHost = &cp
	if IsSkipReason(reason) {
		if o.reasonCounts == nil {
			o.reasonCounts = make(map[string]int64)
		}
		o.reasonCounts[reason]++
	}
	o.mu.Unlock()
}

// RecordDeferred counts one entry the host-pressure gate actually
// POSTPONED (the loop's deferral stamp succeeded), under the gate's
// closed reason vocabulary — obo_queue_deferred_total (OBS-OB-004,
// spec §3.4).
//
// Unlike RecordSkip (which counts the decision) this counts the work
// the decision displaced: a gated tick with an empty queue postpones
// nothing and must not move this counter. A reason outside SkipReasons
// is dropped: the label set is closed by contract (spec §3.5), so free
// text can never widen the exposition's cardinality.
func (o *Observer) RecordDeferred(reason string) {
	if o == nil || !IsSkipReason(reason) {
		return
	}
	o.mu.Lock()
	if o.deferralCounts == nil {
		o.deferralCounts = make(map[string]int64)
	}
	o.deferralCounts[reason]++
	o.mu.Unlock()
}

// Deferrals returns the cumulative per-reason deferral counts. Every
// member of the closed SkipReasons vocabulary is present (at zero
// before it first fires), so the exposition has a series for rate() to
// alert on. Nil-receiver-safe.
func (o *Observer) Deferrals() map[string]int64 {
	out := make(map[string]int64, len(SkipReasons))
	for _, reason := range SkipReasons {
		out[reason] = 0
	}
	if o == nil {
		return out
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, reason := range SkipReasons {
		out[reason] = o.deferralCounts[reason]
	}
	return out
}

// RecordTickStart stamps the tick wall clock (gauge
// obo_cron_last_tick_timestamp_seconds) without counting an outcome.
//
// Why a separate call: a tick that dequeues and solves can hold the
// loop for minutes, and a freshness alert must still see the loop as
// alive while that solve runs. The loop stamps at the TOP of Tick;
// RecordTick then settles exactly one outcome on every return path.
// Neither call can double-count — this one counts nothing.
func (o *Observer) RecordTickStart() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.lastTick = time.Now()
	o.mu.Unlock()
}

// RecordTick stamps the tick wall clock and increments exactly one
// outcome counter for obo_cron_ticks_total. The loop calls it ONCE per
// tick, on every return path — including the gated and empty paths,
// because a starving loop still ticks while a dead loop does not.
//
// An outcome outside TickOutcomes is dropped (closed label set, spec
// §3.5); the stamp still lands, so an unknown value can never make a
// live loop look dead.
func (o *Observer) RecordTick(outcome string) {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.lastTick = time.Now()
	if IsTickOutcome(outcome) {
		if o.tickCounts == nil {
			o.tickCounts = make(map[string]int64)
		}
		o.tickCounts[outcome]++
	}
	o.mu.Unlock()
}

// LastHost returns the latest host observation and whether one has
// been taken yet (false before the first tick reached the gate).
func (o *Observer) LastHost() (HostSnapshot, bool) {
	if o == nil {
		return HostSnapshot{}, false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.lastHost == nil {
		return HostSnapshot{}, false
	}
	return *o.lastHost, true
}

// LastTick returns the last tick's wall clock and whether the loop has
// ticked at all.
func (o *Observer) LastTick() (time.Time, bool) {
	if o == nil {
		return time.Time{}, false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.lastTick.IsZero() {
		return time.Time{}, false
	}
	return o.lastTick, true
}

// SchedulerHealth returns a consistent copy of the scheduler-health
// totals (OBS-OB-002). Nil-receiver-safe: a nil Observer reports every
// counter at zero and both ok flags false, so the exposition keeps its
// shape (all label values present at 0) and omits only the gauges that
// would otherwise claim a reading nobody took.
func (o *Observer) SchedulerHealth() SchedulerHealth {
	h := SchedulerHealth{
		Ticks:       make(map[string]int64, len(TickOutcomes)),
		SkipReasons: make(map[string]int64, len(SkipReasons)),
	}
	if o == nil {
		return h
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, outcome := range TickOutcomes {
		h.Ticks[outcome] = o.tickCounts[outcome]
	}
	for _, reason := range SkipReasons {
		h.SkipReasons[reason] = o.reasonCounts[reason]
	}
	if o.lastHost != nil {
		h.Host = *o.lastHost
		h.HostOK = true
	}
	if !o.lastTick.IsZero() {
		h.LastTick = o.lastTick
		h.LastTickOK = true
	}
	return h
}

// RecordHost stores snap as the latest host observation without
// counting a skip (the healthy-path gate reading).
func (o *Observer) RecordHost(snap HostSnapshot) {
	if o == nil {
		return
	}
	o.mu.Lock()
	cp := snap
	o.lastHost = &cp
	o.mu.Unlock()
}

// RecordDiscovery counts one discover request outcome: found true when
// the request resolved an exact answer, false when the problem class
// was absent (the 404 not_found arm). Invalid requests (400) and
// internal errors (500) are not counted — they measure the caller and
// the store respectively, not discovery outcomes.
func (o *Observer) RecordDiscovery(found bool) {
	if o == nil {
		return
	}
	o.mu.Lock()
	if found {
		o.discoveryHits++
	} else {
		o.discoveryMisses++
	}
	o.mu.Unlock()
}

// DiscoveryCounts returns the cumulative discover-request outcomes:
// hits (an exact answer resolved) and misses (the class was absent).
// These feed obo_discovery_requests_total{outcome} (OBS-OB-004, spec
// §3.4). Distinct from the /api/v1/stats hit_rate ratio, which measures
// catalog coverage rather than discovery outcomes. Nil-receiver-safe.
func (o *Observer) DiscoveryCounts() (hits, misses int64) {
	if o == nil {
		return 0, 0
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.discoveryHits, o.discoveryMisses
}

// Skips returns the number of runs skipped for host pressure.
func (o *Observer) Skips() int64 {
	if o == nil {
		return 0
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.skips
}

// Recent returns up to the last n solve records, oldest first. A copy:
// mutating the result never touches the observer.
func (o *Observer) Recent(n int) []SolveRecord {
	if o == nil || n <= 0 {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.records) > n {
		o.records = o.records[len(o.records)-n:]
	}
	out := make([]SolveRecord, len(o.records))
	copy(out, o.records)
	return out
}

// Snapshot is the JSON shape served at GET /api/v1/metrics.
type Snapshot struct {
	// Total solves recorded since process start.
	SolveCount int `json:"solve_count"`
	// SkippedHostPressure counts cron/feeder runs skipped because the
	// host was under pressure (load or memory gate tripped).
	SkippedHostPressure int64 `json:"skipped_host_pressure"`
	// DiscoveryHits counts POST /api/v1/problems/discover requests that
	// resolved an exact answer. A request-path counter: distinct from
	// the /api/v1/stats hit_rate ratio (verified_answers/total_answers),
	// which measures catalog coverage, not discovery outcomes.
	DiscoveryHits int64 `json:"discovery_hits"`
	// DiscoveryMisses counts discover requests that found no answer for
	// the requested problem class (the 404 not_found arm). 400/500
	// responses are not counted.
	DiscoveryMisses int64 `json:"discovery_misses"`
	// OverBudget solves — peak RSS exceeded the alert threshold.
	OverBudget int `json:"over_budget"`
	// Rejected solves — the harness refused the solve because its
	// observed peak RSS exceeded the reject threshold, even though the
	// solver returned success (DF-OFF-BY-ONE-31).
	Rejected int `json:"rejected"`
	// PeakRSSBytesMax is the largest peak RSS observed; 0 before the
	// first solve.
	PeakRSSBytesMax uint64 `json:"peak_rss_bytes_max"`
	// AlertThresholdBytes is the live over-budget threshold.
	AlertThresholdBytes uint64 `json:"alert_threshold_bytes"`
	// LastSolve is the most recent record, when any solve ran.
	LastSolve *SolveRecord `json:"last_solve,omitempty"`
	// LastHost is the latest gate reading (skip or healthy check).
	LastHost *HostSnapshot `json:"last_host,omitempty"`
	// LastSkipReason is the reason string of the latest skip, empty
	// when nothing ever skipped.
	LastSkipReason string `json:"last_skip_reason,omitempty"`
}

// Snapshot returns the aggregate view. The bounded recent slice is
// not included here — use Recent for per-solve records.
func (o *Observer) Snapshot() Snapshot {
	if o == nil {
		return Snapshot{}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	snap := Snapshot{
		SolveCount:          len(o.records),
		SkippedHostPressure: o.skips,
		DiscoveryHits:       o.discoveryHits,
		DiscoveryMisses:     o.discoveryMisses,
		AlertThresholdBytes: o.alertBytes,
	}
	var maxRSS uint64
	for _, r := range o.records {
		if r.OverBudget {
			snap.OverBudget++
		}
		if r.Rejected {
			snap.Rejected++
		}
		if r.PeakRSSBytes > maxRSS {
			maxRSS = r.PeakRSSBytes
		}
	}
	snap.PeakRSSBytesMax = maxRSS
	if len(o.records) > 0 {
		last := o.records[len(o.records)-1]
		snap.LastSolve = &last
	}
	if o.lastHost != nil {
		cp := *o.lastHost
		snap.LastHost = &cp
	}
	if o.lastSkipped != nil {
		snap.LastSkipReason = o.lastSkipped.Reason
	}
	return snap
}

// AlertThreshold returns the live over-budget threshold in bytes.
func (o *Observer) AlertThreshold() uint64 {
	if o == nil {
		return 0
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.alertBytes
}

// gib converts bytes to GiB for the alert line.
func gib(b uint64) float64 { return float64(b) / (1 << 30) }

// TrimSpaceReason normalises a composite skip reason so the counter
// and log line always use the same vocabulary ("load", "memory",
// "load+memory").
func TrimSpaceReason(parts []string) string {
	return strings.Join(parts, "+")
}
