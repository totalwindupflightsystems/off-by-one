package cron

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
	"github.com/totalwindupflightsystems/off-by-one/internal/solver"
)

// fakeQueue is an in-memory Queue for tests. The dequeueMu serialises
// Dequeue + MarkComplete/MarkFailed so a Tick that errors mid-flight
// still leaves the entry in a consistent state.
type fakeQueue struct {
	mu           sync.Mutex
	entries      []*ingest.Entry
	stage        map[string]string
	done         map[string]string
	answers      map[string]int64
	failText     map[string]string
	dequeueCalls atomic.Int64
	reapCalls    atomic.Int64
	reapTTL      time.Duration
	reapRows     int64
	reapErr      error
}

func newFakeQueue(entries ...*ingest.Entry) *fakeQueue {
	return &fakeQueue{
		entries:  entries,
		stage:    map[string]string{},
		done:     map[string]string{},
		answers:  map[string]int64{},
		failText: map[string]string{},
	}
}

func (q *fakeQueue) Dequeue(ctx context.Context) (*ingest.Entry, error) {
	q.dequeueCalls.Add(1)
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, e := range entries(q.entries) {
		if _, ok := q.done[e.ID]; ok {
			continue
		}
		return e, nil
	}
	return nil, nil
}

// entries returns a snapshot so the lock can be released before
// callers read the slice. The fake queue only adds / never mutates
// entries after construction, so this is safe.
func entries(in []*ingest.Entry) []*ingest.Entry {
	out := make([]*ingest.Entry, len(in))
	copy(out, in)
	return out
}

func (q *fakeQueue) MarkComplete(ctx context.Context, id string, answerID int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.done[id] = ingest.StatusComplete
	q.answers[id] = answerID
	return nil
}

func (q *fakeQueue) MarkFailed(ctx context.Context, id string, reason string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.done[id] = ingest.StatusFailed
	q.failText[id] = reason
	return nil
}

func (q *fakeQueue) SetStage(ctx context.Context, id, stage string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.stage[id] = stage
	return nil
}

// ReapStale records the call and returns the scripted (rows, err).
// The fake has no TTL logic of its own — staleness is exercised
// against the real SQLite queue in ingest's queue_test.go.
func (q *fakeQueue) ReapStale(ctx context.Context, olderThan time.Duration) (int64, error) {
	q.reapCalls.Add(1)
	q.mu.Lock()
	defer q.mu.Unlock()
	q.reapTTL = olderThan
	return q.reapRows, q.reapErr
}

// fakeSolver is an in-memory Solver. The script fields let tests
// drive every path: success, solve error, commit error.
type fakeSolver struct {
	mu           sync.Mutex
	solveScript  func(*ingest.Entry) (*solver.Solution, error)
	commitScript func(*solver.Solution) (int64, error)
	solveCalls   atomic.Int64
	commitCalls  atomic.Int64
	commitInputs []*solver.Solution
	commitDelay  time.Duration
}

func (s *fakeSolver) Solve(ctx context.Context, sub *ingest.Entry) (*solver.Solution, error) {
	s.solveCalls.Add(1)
	if s.commitDelay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(s.commitDelay):
		}
	}
	s.mu.Lock()
	fn := s.solveScript
	s.mu.Unlock()
	if fn == nil {
		return &solver.Solution{
			SolutionMarkdown: "default solution",
			EvidenceMarkdown: "default evidence",
			Model:            "fake",
		}, nil
	}
	return fn(sub)
}

func (s *fakeSolver) Commit(ctx context.Context, _ *ingest.Entry, sol *solver.Solution) (int64, error) {
	s.commitCalls.Add(1)
	s.mu.Lock()
	s.commitInputs = append(s.commitInputs, sol)
	fn := s.commitScript
	s.mu.Unlock()
	if fn == nil {
		return 42, nil
	}
	return fn(sol)
}

// makeEntry returns a queue entry with a unique ID. Used to
// construct test queue contents.
func makeEntry(id string) *ingest.Entry {
	return &ingest.Entry{
		ID:           id,
		ProblemClass: "class-" + id,
		Environment:  "docker",
		Language:     "go",
		Version:      "1.26",
		Status:       ingest.StatusPending,
	}
}

// fastTick returns a Loop with a tiny default interval and a
// deterministic, always-idle probe.
func fastLoop(q Queue, s Solver) *Loop {
	return NewLoop(Config{
		Queue:         q,
		Solver:        s,
		Interval:      10 * time.Millisecond,
		LoadThreshold: -1, // disable
		IdleProbe:     func() (float64, error) { return 0, nil },
		Sleep:         func(ctx context.Context, d time.Duration) error { return nil },
	})
}

// -----------------------------------------------------------------------------
// TestNewLoopDefaults — loop applies spec defaults.
func TestNewLoopDefaults(t *testing.T) {
	l := NewLoop(Config{Queue: newFakeQueue(), Solver: &fakeSolver{}})
	if l.cfg.Interval != DefaultInterval {
		t.Errorf("default interval = %s, want %s", l.cfg.Interval, DefaultInterval)
	}
	if l.cfg.LoadThreshold != DefaultLoadThreshold {
		t.Errorf("default load threshold = %f, want %f", l.cfg.LoadThreshold, DefaultLoadThreshold)
	}
	if l.cfg.IdleProbe == nil {
		t.Error("IdleProbe default not applied")
	}
	if l.cfg.Logger == nil {
		t.Error("Logger default not applied")
	}
	if l.cfg.Now == nil {
		t.Error("Now default not applied")
	}
	if l.cfg.Sleep == nil {
		t.Error("Sleep default not applied")
	}
}

// -----------------------------------------------------------------------------
// TestResolveConfigKeepsExplicitValues — explicit config wins over defaults.
func TestResolveConfigKeepsExplicitValues(t *testing.T) {
	custom := Config{
		Interval:      2 * time.Second,
		LoadThreshold: 4.2,
		IdleProbe:     func() (float64, error) { return 0, nil },
	}
	got := ResolveConfig(custom)
	if got.Interval != 2*time.Second {
		t.Errorf("Interval = %s, want 2s", got.Interval)
	}
	if got.LoadThreshold != 4.2 {
		t.Errorf("LoadThreshold = %f, want 4.2", got.LoadThreshold)
	}
}

// -----------------------------------------------------------------------------
// TestTickEmptyQueue — Tick on an empty queue records SkippedEmpty and
// returns nil. No Dequeue error, no Solve call.
func TestTickEmptyQueue(t *testing.T) {
	q := newFakeQueue()
	s := &fakeSolver{}
	l := fastLoop(q, s)

	if err := l.Tick(context.Background()); err != nil {
		t.Fatalf("Tick err: %v", err)
	}
	m := l.Metrics().Snapshot()
	if m.SkippedEmpty != 1 {
		t.Errorf("SkippedEmpty = %d, want 1", m.SkippedEmpty)
	}
	if m.SolveAttempts != 0 {
		t.Errorf("SolveAttempts = %d, want 0", m.SolveAttempts)
	}
	if s.solveCalls.Load() != 0 {
		t.Errorf("Solve calls = %d, want 0", s.solveCalls.Load())
	}
}

// -----------------------------------------------------------------------------
// TestTickIdleSkip — when the load probe returns a value above the
// threshold, the loop records SkippedIdle and never dequeues.
func TestTickIdleSkip(t *testing.T) {
	q := newFakeQueue(makeEntry("a"))
	s := &fakeSolver{}
	l := NewLoop(Config{
		Queue:         q,
		Solver:        s,
		Interval:      time.Millisecond,
		LoadThreshold: 0.5,
		IdleProbe:     func() (float64, error) { return 1.0, nil },
	})

	if err := l.Tick(context.Background()); !errors.Is(err, ErrNoIdle) {
		t.Fatalf("Tick err = %v, want ErrNoIdle", err)
	}
	m := l.Metrics().Snapshot()
	if m.SkippedIdle != 1 {
		t.Errorf("SkippedIdle = %d, want 1", m.SkippedIdle)
	}
	if q.dequeueCalls.Load() != 0 {
		t.Errorf("Dequeue called %d times, want 0", q.dequeueCalls.Load())
	}
}

// -----------------------------------------------------------------------------
// TestTickIdleProbeError — a probe error is treated as "idle" (the loop
// solves anyway). The error is logged, not returned.
func TestTickIdleProbeError(t *testing.T) {
	q := newFakeQueue(makeEntry("a"))
	s := &fakeSolver{}
	l := NewLoop(Config{
		Queue:         q,
		Solver:        s,
		Interval:      time.Millisecond,
		LoadThreshold: 0.5,
		IdleProbe:     func() (float64, error) { return 0, errors.New("probe fail") },
	})

	if err := l.Tick(context.Background()); err != nil {
		t.Fatalf("Tick err: %v (probe errors should be treated as idle)", err)
	}
	m := l.Metrics().Snapshot()
	if m.SkippedIdle != 0 {
		t.Errorf("SkippedIdle = %d, want 0 (probe error → idle)", m.SkippedIdle)
	}
	if m.SolveSuccess != 1 {
		t.Errorf("SolveSuccess = %d, want 1 (probe error → solved)", m.SolveSuccess)
	}
}

// -----------------------------------------------------------------------------
// TestTickSuccess — the happy path: dequeue, solve, commit, mark complete.
// Metrics and queue state must reflect the full success.
func TestTickSuccess(t *testing.T) {
	q := newFakeQueue(makeEntry("e1"))
	s := &fakeSolver{}
	l := fastLoop(q, s)

	if err := l.Tick(context.Background()); err != nil {
		t.Fatalf("Tick err: %v", err)
	}
	m := l.Metrics().Snapshot()
	if m.SolveAttempts != 1 {
		t.Errorf("SolveAttempts = %d, want 1", m.SolveAttempts)
	}
	if m.SolveSuccess != 1 {
		t.Errorf("SolveSuccess = %d, want 1", m.SolveSuccess)
	}
	if m.SolveFailed != 0 {
		t.Errorf("SolveFailed = %d, want 0", m.SolveFailed)
	}
	if m.AvgSolveMS < 0 {
		t.Errorf("AvgSolveMS = %f, want >= 0", m.AvgSolveMS)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.done["e1"] != ingest.StatusComplete {
		t.Errorf("e1 done = %v, want complete", q.done["e1"])
	}
	if q.answers["e1"] != 42 {
		t.Errorf("e1 answer = %d, want 42", q.answers["e1"])
	}
	if q.stage["e1"] != "committing" {
		// The last stage write before MarkComplete is "committing";
		// MarkComplete does not touch stage. We assert the highest
		// stage the loop set.
		t.Errorf("e1 stage = %q, want committing", q.stage["e1"])
	}
}

// -----------------------------------------------------------------------------
// TestTickSolveError — when Solver.Solve returns an error, the entry
// is marked failed and the metrics reflect SolveFailed.
func TestTickSolveError(t *testing.T) {
	q := newFakeQueue(makeEntry("e1"))
	s := &fakeSolver{
		solveScript: func(*ingest.Entry) (*solver.Solution, error) {
			return nil, errors.New("pi-agent crashed")
		},
	}
	l := fastLoop(q, s)

	err := l.Tick(context.Background())
	if err == nil {
		t.Fatal("expected Tick to return the solve error")
	}
	if !strings.Contains(err.Error(), "pi-agent crashed") {
		t.Errorf("err = %v, expected to wrap solve error", err)
	}
	m := l.Metrics().Snapshot()
	if m.SolveFailed != 1 {
		t.Errorf("SolveFailed = %d, want 1", m.SolveFailed)
	}
	if m.SolveSuccess != 0 {
		t.Errorf("SolveSuccess = %d, want 0", m.SolveSuccess)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.done["e1"] != ingest.StatusFailed {
		t.Errorf("e1 done = %v, want failed", q.done["e1"])
	}
	if q.stage["e1"] != "solver_failed" {
		t.Errorf("e1 stage = %q, want solver_failed", q.stage["e1"])
	}
	if !strings.Contains(q.failText["e1"], "pi-agent crashed") {
		t.Errorf("fail reason = %q, want contains 'pi-agent crashed'", q.failText["e1"])
	}
	if s.commitCalls.Load() != 0 {
		t.Errorf("Commit calls = %d, want 0 (Solve failed before commit)", s.commitCalls.Load())
	}
}

// -----------------------------------------------------------------------------
// TestTickCommitError — Solve succeeds but Commit fails. Entry is
// marked failed, metrics record the failure.
func TestTickCommitError(t *testing.T) {
	q := newFakeQueue(makeEntry("e1"))
	s := &fakeSolver{
		commitScript: func(*solver.Solution) (int64, error) {
			return 0, errors.New("db locked")
		},
	}
	l := fastLoop(q, s)

	err := l.Tick(context.Background())
	if err == nil {
		t.Fatal("expected Tick to return the commit error")
	}
	if !strings.Contains(err.Error(), "db locked") {
		t.Errorf("err = %v, expected to wrap commit error", err)
	}
	m := l.Metrics().Snapshot()
	if m.SolveFailed != 1 {
		t.Errorf("SolveFailed = %d, want 1", m.SolveFailed)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.done["e1"] != ingest.StatusFailed {
		t.Errorf("e1 done = %v, want failed", q.done["e1"])
	}
	if q.stage["e1"] != "commit_failed" {
		t.Errorf("e1 stage = %q, want commit_failed", q.stage["e1"])
	}
}

// -----------------------------------------------------------------------------
// TestTickSerialises — two concurrent Tick calls produce only one
// solve (the second hits ErrAlreadyRunning and exits).
func TestTickSerialises(t *testing.T) {
	q := newFakeQueue(makeEntry("e1"))
	s := &fakeSolver{
		commitDelay: 50 * time.Millisecond,
	}
	l := fastLoop(q, s)

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	results := make([]error, 2)
	go func() {
		defer wg.Done()
		close(start)
		results[0] = l.Tick(context.Background())
	}()
	go func() {
		defer wg.Done()
		<-start
		// Tiny gap so the first tick wins the CAS.
		time.Sleep(2 * time.Millisecond)
		results[1] = l.Tick(context.Background())
	}()
	wg.Wait()

	// Exactly one tick did real work; the other returned ErrAlreadyRunning.
	good, busy := 0, 0
	for _, e := range results {
		switch {
		case e == nil:
			good++
		case errors.Is(e, ErrAlreadyRunning):
			busy++
		default:
			t.Errorf("unexpected Tick result: %v", e)
		}
	}
	if good != 1 || busy != 1 {
		t.Errorf("good=%d busy=%d, want 1/1", good, busy)
	}
	if s.solveCalls.Load() != 1 {
		t.Errorf("Solve calls = %d, want 1 (only the winning tick)", s.solveCalls.Load())
	}
}

// -----------------------------------------------------------------------------
// TestRunStopsOnCancel — Run is the long-lived loop; cancelling
// the context must cause it to return promptly.
func TestRunStopsOnCancel(t *testing.T) {
	q := newFakeQueue()
	s := &fakeSolver{}

	// Sleep yields immediately; the loop will burn CPU unless
	// the context cancellation short-circuits it. We use a sleep
	// that returns ErrNoMore so the loop can be interrupted.
	sleepCalls := atomic.Int64{}
	l := NewLoop(Config{
		Queue:         q,
		Solver:        s,
		Interval:      time.Millisecond,
		LoadThreshold: -1,
		IdleProbe:     func() (float64, error) { return 0, nil },
		Sleep: func(ctx context.Context, d time.Duration) error {
			sleepCalls.Add(1)
			// Block until cancellation. Returning ctx.Err()
			// signals the loop to exit.
			<-ctx.Done()
			return ctx.Err()
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	doneCh := l.Done()
	go func() {
		_ = l.Run(ctx)
	}()
	// Give the loop a chance to enter the first Sleep, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-doneCh:
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after cancel")
	}
	if sleepCalls.Load() == 0 {
		t.Error("Sleep was never called — Run did not enter the loop body")
	}
}

// -----------------------------------------------------------------------------
// TestRunDrainsQueueAcrossTicks — Run with a populated queue
// processes every entry across multiple ticks.
func TestRunDrainsQueueAcrossTicks(t *testing.T) {
	q := newFakeQueue(makeEntry("a"), makeEntry("b"), makeEntry("c"))
	s := &fakeSolver{}

	ticks := atomic.Int64{}
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	l := NewLoop(Config{
		Queue:         q,
		Solver:        s,
		Interval:      0, // every Tick returns immediately
		LoadThreshold: -1,
		IdleProbe:     func() (float64, error) { return 0, nil },
		Sleep: func(ctx context.Context, d time.Duration) error {
			n := ticks.Add(1)
			// Stop after 5 ticks — covers the 3 entries plus
			// a couple of empty-queue ticks.
			if n >= 5 {
				cancelRun()
			}
			return nil
		},
	})

	go func() { _ = l.Run(runCtx) }()

	// Wait for the loop to finish.
	<-l.Done()

	m := l.Metrics().Snapshot()
	if m.SolveSuccess != 3 {
		t.Errorf("SolveSuccess = %d, want 3", m.SolveSuccess)
	}
	if m.SolveAttempts != 3 {
		t.Errorf("SolveAttempts = %d, want 3", m.SolveAttempts)
	}
}

// -----------------------------------------------------------------------------
// TestMetricsAvg — after N solves, AvgSolveMS = totalRuntime / N.
func TestMetricsAvg(t *testing.T) {
	q := newFakeQueue(makeEntry("a"), makeEntry("b"))
	s := &fakeSolver{}
	l := NewLoop(Config{
		Queue:         q,
		Solver:        s,
		Interval:      0,
		LoadThreshold: -1,
		IdleProbe:     func() (float64, error) { return 0, nil },
		Sleep:         func(ctx context.Context, d time.Duration) error { return nil },
	})
	// Manually record a deterministic runtime to verify the
	// averaging math.
	l.metrics.recordSuccess(100 * time.Millisecond)
	l.metrics.recordSuccess(300 * time.Millisecond)

	m := l.Metrics().Snapshot()
	if m.SolveSuccess != 2 {
		t.Errorf("SolveSuccess = %d, want 2", m.SolveSuccess)
	}
	want := float64(400) / float64(2) // ms
	if m.AvgSolveMS != want {
		t.Errorf("AvgSolveMS = %f, want %f", m.AvgSolveMS, want)
	}
	if m.TotalRuntimeMS != 400 {
		t.Errorf("TotalRuntimeMS = %d, want 400", m.TotalRuntimeMS)
	}
}

// -----------------------------------------------------------------------------
// TestParseLoadavg — exercises the loadavg parser on canonical
// input and malformed input.
func TestParseLoadavg(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"typical", "0.42 0.31 0.28 1/123 4567\n", 0.42, false},
		{"zero", "0.00 0.00 0.00 0/0 0\n", 0.0, false},
		{"high", "12.50 8.0 4.0 5/200 9999\n", 12.5, false},
		{"empty", "", 0, true},
		{"whitespace", "   \n", 0, true},
		{"non-numeric", "abc 0.0 0.0 0/0 0\n", 0, true},
		{"just_first_field", "1.5\n", 1.5, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseLoadavg([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("parseLoadavg = %f, want %f", got, tc.want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// TestProbeLoadavgLive — only run when /proc/loadavg exists.
// Skips silently on non-Linux CI runners.
func TestProbeLoadavgLive(t *testing.T) {
	load, err := probeLoadavg()
	if err != nil {
		// /proc/loadavg missing is acceptable (Windows, etc.).
		if errors.Is(err, ErrMalformedLoadavg) {
			t.Skipf("loadavg probe unavailable: %v", err)
		}
		t.Skipf("loadavg probe unavailable: %v", err)
	}
	if load < 0 {
		t.Errorf("load = %f, want >= 0", load)
	}
}

// -----------------------------------------------------------------------------
// TestResolveConfigReapAfter — ReapAfter defaults to twice the solver
// timeout (a solve that runs past 2x its own timeout is definitionally
// wedged); an explicit value is never overridden.
func TestResolveConfigReapAfter(t *testing.T) {
	got := ResolveConfig(Config{})
	want := 2 * solver.DefaultSolveTimeout
	if got.ReapAfter != want {
		t.Errorf("default ReapAfter = %s, want %s", got.ReapAfter, want)
	}
	custom := ResolveConfig(Config{ReapAfter: 5 * time.Minute})
	if custom.ReapAfter != 5*time.Minute {
		t.Errorf("explicit ReapAfter = %s, want 5m", custom.ReapAfter)
	}
}

// -----------------------------------------------------------------------------
// TestTickReapsBeforeIdle — the reaper runs at the start of the tick,
// BEFORE the idle check: even on a busy box the queue is swept, with
// the configured TTL, and the dequeue path is never reached.
func TestTickReapsBeforeIdle(t *testing.T) {
	q := newFakeQueue(makeEntry("a"))
	q.reapRows = 2
	s := &fakeSolver{}
	l := NewLoop(Config{
		Queue:         q,
		Solver:        s,
		Interval:      time.Millisecond,
		LoadThreshold: 0.5,
		IdleProbe:     func() (float64, error) { return 1.0, nil }, // busy
		ReapAfter:     42 * time.Minute,
	})

	if err := l.Tick(context.Background()); !errors.Is(err, ErrNoIdle) {
		t.Fatalf("Tick err = %v, want ErrNoIdle", err)
	}
	if q.reapCalls.Load() != 1 {
		t.Errorf("ReapStale calls = %d, want 1 (reaper runs even when busy)", q.reapCalls.Load())
	}
	q.mu.Lock()
	ttl := q.reapTTL
	q.mu.Unlock()
	if ttl != 42*time.Minute {
		t.Errorf("ReapStale TTL = %s, want 42m (configured ReapAfter)", ttl)
	}
	if q.dequeueCalls.Load() != 0 {
		t.Errorf("Dequeue called %d times, want 0 (idle skip still applies after reap)", q.dequeueCalls.Load())
	}
}

// -----------------------------------------------------------------------------
// TestTickReapErrorNonFatal — a reaper failure is logged and the tick
// proceeds; a broken reaper must never wedge the solver.
func TestTickReapErrorNonFatal(t *testing.T) {
	q := newFakeQueue(makeEntry("e1"))
	q.reapErr = errors.New("db locked")
	s := &fakeSolver{}
	l := fastLoop(q, s)

	if err := l.Tick(context.Background()); err != nil {
		t.Fatalf("Tick err: %v (reaper error must be non-fatal)", err)
	}
	if q.reapCalls.Load() != 1 {
		t.Errorf("ReapStale calls = %d, want 1", q.reapCalls.Load())
	}
	m := l.Metrics().Snapshot()
	if m.SolveSuccess != 1 {
		t.Errorf("SolveSuccess = %d, want 1 (tick continued after reap error)", m.SolveSuccess)
	}
}

// -----------------------------------------------------------------------------
// TestTickReapUsesDefaultTTL — with no explicit ReapAfter the tick
// passes the resolved default (2x solver timeout) to the queue.
func TestTickReapUsesDefaultTTL(t *testing.T) {
	q := newFakeQueue()
	s := &fakeSolver{}
	l := fastLoop(q, s)

	if err := l.Tick(context.Background()); err != nil {
		t.Fatalf("Tick err: %v", err)
	}
	if q.reapCalls.Load() != 1 {
		t.Fatalf("ReapStale calls = %d, want 1", q.reapCalls.Load())
	}
	q.mu.Lock()
	ttl := q.reapTTL
	q.mu.Unlock()
	if want := 2 * solver.DefaultSolveTimeout; ttl != want {
		t.Errorf("ReapStale TTL = %s, want default %s", ttl, want)
	}
}
