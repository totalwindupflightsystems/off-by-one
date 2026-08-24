// Package cron implements the idle-gated pre-solve loop. The
// foreman wakes on a configurable interval, checks that the system
// is idle (load average below a threshold), then drains the queue
// one entry at a time through the solver and persists the result in
// the graph.
//
// Why a separate package: the loop is the only goroutine that turns
// "user submitted a problem" into "the graph has a candidate answer".
// The queue knows how to dequeue, the solver knows how to solve, the
// graph knows how to persist; this package is the small coordinator
// on top. Keeping it isolated makes it easy to mock all three
// dependencies in tests (loop_test.go uses fakes for each).
package cron

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
	"github.com/totalwindupflightsystems/off-by-one/internal/solver"
)

// Default wake interval. The spec recommends 5 minutes; tests use
// smaller values (10ms–1s) to keep the suite fast.
const DefaultInterval = 5 * time.Minute

// Default load-average threshold below which the system is
// considered idle. Linux loadavg(1) is a 1-minute exponentially
// damped average, so 1.0 means roughly "one runnable process on
// average." A 1-CPU box at load 0.5 has half a CPU to spare.
const DefaultLoadThreshold = 1.0

// ErrNoIdle is returned by IsIdle when the system load is at or
// above the configured threshold. The loop treats this as a soft
// signal to skip the tick.
var ErrNoIdle = errors.New("cron: system is not idle")

// Metrics is the live counters maintained by the loop. The fields
// are read with the atomic load helpers — no lock required for
// the hot-path metrics. TotalRuntime tracks the cumulative time
// the loop has spent inside Solve calls; it is updated under mu
// because it is read alongside the other metrics by Metrics().
type Metrics struct {
	SolveAttempts atomic.Int64
	SolveSuccess  atomic.Int64
	SolveFailed   atomic.Int64
	SkippedIdle   atomic.Int64
	SkippedEmpty  atomic.Int64

	mu           sync.Mutex
	totalRuntime time.Duration
}

// Metrics returns a point-in-time snapshot of the loop counters.
// The values are taken atomically (the Int64 fields) or under
// mu (totalRuntime) — the snapshot is internally consistent
// because we hold mu for the whole struct.
func (m *Metrics) Snapshot() MetricsSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempts := m.SolveAttempts.Load()
	success := m.SolveSuccess.Load()
	failed := m.SolveFailed.Load()
	var avgMS float64
	if attempts > 0 {
		avgMS = float64(m.totalRuntime.Milliseconds()) / float64(attempts)
	}
	return MetricsSnapshot{
		SolveAttempts:  attempts,
		SolveSuccess:   success,
		SolveFailed:    failed,
		SkippedIdle:    m.SkippedIdle.Load(),
		SkippedEmpty:   m.SkippedEmpty.Load(),
		AvgSolveMS:     avgMS,
		TotalRuntimeMS: m.totalRuntime.Milliseconds(),
	}
}

func (m *Metrics) recordSuccess(d time.Duration) {
	m.SolveAttempts.Add(1)
	m.SolveSuccess.Add(1)
	m.mu.Lock()
	m.totalRuntime += d
	m.mu.Unlock()
}

func (m *Metrics) recordFailure(d time.Duration) {
	m.SolveAttempts.Add(1)
	m.SolveFailed.Add(1)
	m.mu.Lock()
	m.totalRuntime += d
	m.mu.Unlock()
}

func (m *Metrics) recordIdleSkip()  { m.SkippedIdle.Add(1) }
func (m *Metrics) recordEmptySkip() { m.SkippedEmpty.Add(1) }

// MetricsSnapshot is the JSON-serialisable form returned by
// Snapshot(). It is what /api/v1/stats (or the local metrics
// endpoint) will eventually expose.
type MetricsSnapshot struct {
	SolveAttempts  int64   `json:"solve_attempts"`
	SolveSuccess   int64   `json:"solve_success"`
	SolveFailed    int64   `json:"solve_failed"`
	SkippedIdle    int64   `json:"skipped_idle"`
	SkippedEmpty   int64   `json:"skipped_empty"`
	AvgSolveMS     float64 `json:"avg_solve_ms"`
	TotalRuntimeMS int64   `json:"total_runtime_ms"`
}

// Config is the loop's runtime configuration. Zero values are
// filled in by ResolveConfig.
type Config struct {
	// Interval is the gap between ticks. Default: DefaultInterval.
	Interval time.Duration

	// LoadThreshold is the maximum loadavg(1) at which the
	// system is considered idle. Default: DefaultLoadThreshold.
	// Set to a negative number to disable the idle check.
	LoadThreshold float64

	// ReapAfter is the age at which an in_progress queue entry is
	// considered stale — orphaned by a restart or crash mid-solve —
	// and reaped (marked failed) at the start of each tick. Entries
	// newer than this are live solves and are never touched.
	// Default: 2 * solver.DefaultSolveTimeout.
	ReapAfter time.Duration

	// Solver is the problem solver. Required.
	Solver Solver

	// Queue is the submission queue. Required.
	Queue Queue

	// IdleProbe checks the system load. Default: probeLoadavg.
	// Tests substitute a fake to make the idle check deterministic.
	IdleProbe func() (float64, error)

	// Logger is the structured logger for loop events. nil → log.Default().
	Logger *log.Logger

	// Now is the time source. Default: time.Now. Tests override.
	Now func() time.Time

	// Sleep is the function used to wait between ticks. Default:
	// time.Sleep wrapped in a select on ctx.Done so the loop
	// can be cancelled mid-wait. Tests use a channel-based stub.
	Sleep func(ctx context.Context, d time.Duration) error
}

// Queue abstracts the part of the ingest queue the loop needs. The
// production type is *ingest.Queue; tests use a fake.
type Queue interface {
	Dequeue(ctx context.Context) (*ingest.Entry, error)
	MarkComplete(ctx context.Context, id string, answerID int64) error
	MarkFailed(ctx context.Context, id string, reason string) error
	SetStage(ctx context.Context, id, stage string) error
	// ReapStale marks in_progress entries older than olderThan as
	// failed and returns the number reaped. Called once per tick so
	// entries wedged by a restart mid-solve do not inflate Depth
	// forever.
	ReapStale(ctx context.Context, olderThan time.Duration) (int64, error)
}

// Solver abstracts the pi-agent executor. The production type is
// *solver.Executor; tests use a fake.
type Solver interface {
	Solve(ctx context.Context, sub *ingest.Entry) (*solver.Solution, error)
	Commit(ctx context.Context, sub *ingest.Entry, sol *solver.Solution) (int64, error)
}

// ResolveConfig returns a copy of cfg with zero values replaced
// by defaults.
func ResolveConfig(cfg Config) Config {
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultInterval
	}
	if cfg.LoadThreshold == 0 {
		cfg.LoadThreshold = DefaultLoadThreshold
	}
	if cfg.ReapAfter <= 0 {
		cfg.ReapAfter = 2 * solver.DefaultSolveTimeout
	}
	if cfg.IdleProbe == nil {
		cfg.IdleProbe = probeLoadavg
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Sleep == nil {
		cfg.Sleep = defaultSleep
	}
	return cfg
}

// defaultSleep waits for d or until ctx is cancelled.
func defaultSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Loop is the idle cron. Construct one with NewLoop, then call
// Run in its own goroutine. Cancel the context to stop.
//
// A Loop processes one entry at a time. Concurrency=1 is a hard
// requirement (single-machine, sequential solver) — a second entry
// is never dequeued while one is in flight. The Mutex serialises
// Tick calls and is the only reason the loop is safe to invoke
// concurrently (e.g., from a "drain now" admin endpoint).
type Loop struct {
	cfg     Config
	metrics Metrics

	// inflight guards the in-progress solve. Only one tick may
	// run at a time. Atomic CAS to avoid holding mu for the
	// entire solve duration.
	inflight atomic.Bool

	// done is closed when Run returns. Tests wait on this to
	// assert the loop actually stopped.
	done chan struct{}
}

// NewLoop returns a Loop bound to the given config. The config is
// normalised (defaults applied) before storage.
func NewLoop(cfg Config) *Loop {
	return &Loop{cfg: ResolveConfig(cfg), done: make(chan struct{})}
}

// Metrics returns a pointer to the loop's metrics. The pointer is
// stable for the loop's lifetime; callers can read the counters
// directly (with the atomic helpers) or call Snapshot() for a
// consistent view.
func (l *Loop) Metrics() *Metrics { return &l.metrics }

// Done returns a channel that is closed when Run exits. Useful in
// tests that need to wait for the loop to drain after cancelling.
func (l *Loop) Done() <-chan struct{} { return l.done }

// Run is the main loop. It blocks until ctx is cancelled, ticking
// once per Interval. Returns nil on a clean cancellation.
//
// On each tick:
//  0. Reap stale in_progress entries (runs even when the box is busy)
//  1. Check idle (skip if system is busy)
//  2. Dequeue one entry (skip if queue is empty)
//  3. Solve it via the Solver
//  4. Commit the result to the graph
//  5. Mark the queue entry complete or failed
//  6. Sleep until the next Interval
//
// Errors during steps 3-5 are logged and counted, but do NOT stop
// the loop. The only way Run returns is via ctx cancellation.
func (l *Loop) Run(ctx context.Context) error {
	defer close(l.done)
	for {
		// Check cancellation at the top of each iteration.
		// Custom Sleep implementations may not return early on
		// ctx.Done (some test stubs return nil instantly), so
		// we must inspect ctx here too.
		if err := ctx.Err(); err != nil {
			return nil
		}
		if err := l.Tick(ctx); err != nil && !errors.Is(err, context.Canceled) {
			l.cfg.Logger.Printf("cron: tick error: %v", err)
		}
		// Exit promptly when ctx is cancelled. Err here is
		// only the sleep error; context.Canceled returns from
		// the sleep so we don't double-log it.
		if err := l.cfg.Sleep(ctx, l.cfg.Interval); err != nil {
			return nil
		}
	}
}

// Tick is one iteration of the loop. It is exposed so tests and
// admin endpoints can drive the loop synchronously. Returns the
// last error encountered, or nil.
//
// Concurrency: only one Tick may run at a time across all callers.
// A second concurrent call returns ErrAlreadyRunning without
// doing any work — the goroutine that "won" the CAS does the
// solve.
func (l *Loop) Tick(ctx context.Context) error {
	if !l.inflight.CompareAndSwap(false, true) {
		return ErrAlreadyRunning
	}
	defer l.inflight.Store(false)

	// Reap stale in_progress entries before anything else. This runs
	// even when the system is busy — a wedged entry is stale no
	// matter the load, and skipping the reap on a busy box would let
	// Depth stay inflated indefinitely. Reaper errors are non-fatal:
	// a failed reap must not block a solve (same philosophy as the
	// idle probe).
	if n, err := l.cfg.Queue.ReapStale(ctx, l.cfg.ReapAfter); err != nil {
		l.cfg.Logger.Printf("cron: reap stale failed: %v (continuing)", err)
	} else if n > 0 {
		l.cfg.Logger.Printf("cron: reaped %d stale in_progress queue entries (older than %s)", n, l.cfg.ReapAfter)
	}

	if err := l.checkIdle(ctx); err != nil {
		return err
	}
	entry, err := l.cfg.Queue.Dequeue(ctx)
	if err != nil {
		return fmt.Errorf("dequeue: %w", err)
	}
	if entry == nil {
		l.metrics.recordEmptySkip()
		return nil
	}
	return l.processOne(ctx, entry)
}

// ErrAlreadyRunning signals that another Tick is already in
// flight. The loop is single-threaded by design; callers that want
// parallel solves should run multiple Loops with isolated queues.
var ErrAlreadyRunning = errors.New("cron: tick already in flight")

// checkIdle runs the configured IdleProbe. A negative threshold
// disables the check entirely (tests and the "always run" mode).
// Returns ErrNoIdle when the system is busy.
func (l *Loop) checkIdle(ctx context.Context) error {
	if l.cfg.LoadThreshold < 0 {
		return nil
	}
	load, err := l.cfg.IdleProbe()
	if err != nil {
		// If we can't probe, treat as idle — better to solve
		// a problem than to skip on a transient probe error.
		l.cfg.Logger.Printf("cron: idle probe error: %v (proceeding)", err)
		return nil
	}
	if load >= l.cfg.LoadThreshold {
		l.metrics.recordIdleSkip()
		return ErrNoIdle
	}
	return nil
}

// processOne solves a single dequeued entry and persists the result.
// All errors are returned to the caller (Tick / Run logs them) but
// the queue entry is always updated to a terminal state.
func (l *Loop) processOne(ctx context.Context, entry *ingest.Entry) error {
	start := l.cfg.Now()

	_ = l.cfg.Queue.SetStage(ctx, entry.ID, "solver_running")
	sol, err := l.cfg.Solver.Solve(ctx, entry)
	if err != nil {
		_ = l.cfg.Queue.SetStage(ctx, entry.ID, "solver_failed")
		_ = l.cfg.Queue.MarkFailed(ctx, entry.ID, err.Error())
		l.metrics.recordFailure(l.cfg.Now().Sub(start))
		l.cfg.Logger.Printf("cron: solve failed for %s: %v", entry.ID, err)
		return fmt.Errorf("solve: %w", err)
	}

	_ = l.cfg.Queue.SetStage(ctx, entry.ID, "committing")
	answerID, err := l.cfg.Solver.Commit(ctx, entry, sol)
	if err != nil {
		_ = l.cfg.Queue.SetStage(ctx, entry.ID, "commit_failed")
		_ = l.cfg.Queue.MarkFailed(ctx, entry.ID, err.Error())
		l.metrics.recordFailure(l.cfg.Now().Sub(start))
		l.cfg.Logger.Printf("cron: commit failed for %s: %v", entry.ID, err)
		return fmt.Errorf("commit: %w", err)
	}

	if err := l.cfg.Queue.MarkComplete(ctx, entry.ID, answerID); err != nil {
		l.metrics.recordFailure(l.cfg.Now().Sub(start))
		l.cfg.Logger.Printf("cron: mark-complete failed for %s: %v", entry.ID, err)
		return fmt.Errorf("mark complete: %w", err)
	}
	l.metrics.recordSuccess(l.cfg.Now().Sub(start))
	l.cfg.Logger.Printf("cron: solved %s → answer %d in %s", entry.ID, answerID, l.cfg.Now().Sub(start))
	return nil
}

// probeLoadavg returns the system's 1-minute load average by
// reading /proc/loadavg. Returns an error if /proc is not
// available (Windows tests, restricted containers). The loop
// treats probe errors as "idle" — see checkIdle.
func probeLoadavg() (float64, error) {
	data, err := readLoadavg()
	if err != nil {
		return 0, err
	}
	return parseLoadavg(data)
}
