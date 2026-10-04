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
)

// DefaultRSSAlertBytes is the default over-budget threshold: 4 GiB.
// The unit-level cap (DF-OFF-BY-ONE-30) bounds a single solve at
// RLIMIT_AS = 6 GiB by default; a solve that peaks above 4 GiB is
// surviving the cap but is close enough to it (and to the host's
// swap) that it must page someone. Configurable via
// -rss-alert-mb / OFF_BY_ONE_RSS_ALERT_MB.
const DefaultRSSAlertBytes uint64 = 4 << 30

// SolveRecord is one solve's resource footprint. PeakRSSBytes is the
// high-water mark of the sandboxed process tree's resident set,
// sampled while pi-agent ran (see sandbox.Sampler). TestRuns counts
// `go test`-style invocations pi-agent made (parsed from its stdout);
// it is the field that was 47 GB on the incident solve. Killed reports
// whether the solve died on its memory cap (exec.ExitError) rather
// than finishing.
type SolveRecord struct {
	SubmissionID   string `json:"submission_id"`
	ProblemClass   string `json:"problem_class"`
	Model          string `json:"model"`
	PeakRSSBytes   uint64 `json:"peak_rss_bytes"`
	WallTimeMS     int64  `json:"wall_time_ms"`
	TestRuns       int    `json:"test_runs"`
	Killed         bool   `json:"killed"`
	Success        bool   `json:"success"`
	OverBudget     bool   `json:"over_budget"`
	AlertBytesUsed uint64 `json:"alert_threshold_bytes"`
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
	return &Observer{alertBytes: alertBytes, logger: logger}
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
	o.mu.Unlock()

	if over {
		o.logger.Printf("ERROR solve ALERT: peak RSS %d (%.1f GiB) exceeds %d (%.1f GiB) — problem_class=%s submission=%s model=%s killed=%t",
			rec.PeakRSSBytes, gib(rec.PeakRSSBytes),
			o.alertBytes, gib(o.alertBytes),
			rec.ProblemClass, rec.SubmissionID, rec.Model, rec.Killed)
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
func (o *Observer) RecordSkip(snap HostSnapshot) {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.skips++
	cp := snap
	o.lastSkipped = &cp
	o.lastHost = &cp
	o.mu.Unlock()
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
	// OverBudget solves — peak RSS exceeded the alert threshold.
	OverBudget int `json:"over_budget"`
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
		AlertThresholdBytes: o.alertBytes,
	}
	var maxRSS uint64
	for _, r := range o.records {
		if r.OverBudget {
			snap.OverBudget++
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
