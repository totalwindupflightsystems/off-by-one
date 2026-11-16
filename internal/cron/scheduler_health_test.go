package cron

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
	"github.com/totalwindupflightsystems/off-by-one/internal/metrics"
	"github.com/totalwindupflightsystems/off-by-one/internal/solver"
)

// schedulerHealth returns the observer's scheduler view, asserting the
// observer is wired (a nil observer would silently record nothing and
// make every assertion below vacuous).
func schedulerHealth(t *testing.T, obs *metrics.Observer) metrics.SchedulerHealth {
	t.Helper()
	if obs == nil {
		t.Fatal("Observer is nil — the tick outcome would be recorded nowhere")
	}
	return obs.SchedulerHealth()
}

// sumTicks returns the total number of recorded ticks across outcomes.
func sumTicks(h metrics.SchedulerHealth) int64 {
	var n int64
	for _, v := range h.Ticks {
		n += v
	}
	return n
}

// TestTickGateClosedRecordsIdleGatedOnce is the OBS-OB-002 core case
// (acceptance criterion AC3's unit half): a gate-closed tick is ONE
// idle_gated outcome, its skip reason is tallied, and the last-tick
// stamp still advances — a starving loop is alive and must look alive.
func TestTickGateClosedRecordsIdleGatedOnce(t *testing.T) {
	q := newFakeQueue(makeEntry("a"))
	obs := metrics.New(0, nil)
	l := NewLoop(Config{
		Queue:         q,
		Solver:        &fakeSolver{},
		Interval:      time.Millisecond,
		LoadThreshold: 0.5,
		IdleProbe:     func() (float64, error) { return 12.0, nil },
		Observer:      obs,
	})

	before := time.Now()
	if err := l.Tick(context.Background()); !errors.Is(err, ErrNoIdle) {
		t.Fatalf("Tick err = %v, want ErrNoIdle", err)
	}
	h := schedulerHealth(t, obs)
	if got := h.Ticks[metrics.TickOutcomeIdleGated]; got != 1 {
		t.Errorf("idle_gated ticks = %d, want 1", got)
	}
	if got := sumTicks(h); got != 1 {
		t.Errorf("total ticks = %d, want 1 (exactly one outcome per tick, got %v)", got, h.Ticks)
	}
	if got := h.SkipReasons["load"]; got != 1 {
		t.Errorf("skip reason load = %d, want 1 (%v)", got, h.SkipReasons)
	}
	if !h.HostOK {
		t.Fatal("no host reading recorded on the gate path")
	}
	if h.Host.Load1 != 12.0 {
		t.Errorf("host load1 = %v, want the probe's 12.0", h.Host.Load1)
	}
	if !h.LastTickOK {
		t.Fatal("last tick not stamped on the gate path — a starving loop would look dead")
	}
	if h.LastTick.Before(before.Add(-2 * time.Second)) {
		t.Errorf("last tick %v is not the tick's wall clock (tick started %v)", h.LastTick, before)
	}

	// A second gated tick advances the stamp and adds exactly one more
	// outcome: the counters are per-tick, the gauge is freshness.
	time.Sleep(5 * time.Millisecond)
	if err := l.Tick(context.Background()); !errors.Is(err, ErrNoIdle) {
		t.Fatalf("second Tick err = %v, want ErrNoIdle", err)
	}
	h2 := schedulerHealth(t, obs)
	if got := h2.Ticks[metrics.TickOutcomeIdleGated]; got != 2 {
		t.Errorf("idle_gated ticks after two gated ticks = %d, want 2", got)
	}
	if got := sumTicks(h2); got != 2 {
		t.Errorf("total ticks after two gated ticks = %d, want 2 (%v)", got, h2.Ticks)
	}
	if !h2.LastTick.After(h.LastTick) {
		t.Errorf("last tick did not advance: %v then %v", h.LastTick, h2.LastTick)
	}
}

// TestTickEmptyQueueRecordsEmptyOnce: an empty queue is its own
// outcome, counted once, with the stamp still advancing.
func TestTickEmptyQueueRecordsEmptyOnce(t *testing.T) {
	obs := metrics.New(0, nil)
	l := fastLoop(newFakeQueue(), &fakeSolver{})
	l.cfg.Observer = obs

	if err := l.Tick(context.Background()); err != nil {
		t.Fatalf("Tick err: %v", err)
	}
	h := schedulerHealth(t, obs)
	if got := h.Ticks[metrics.TickOutcomeEmpty]; got != 1 {
		t.Errorf("empty ticks = %d, want 1", got)
	}
	if got := sumTicks(h); got != 1 {
		t.Errorf("total ticks = %d, want 1 (got %v)", got, h.Ticks)
	}
	if !h.LastTickOK {
		t.Fatal("last tick not stamped on the empty path")
	}
	if h.SkipReasons["load"]+h.SkipReasons["memory"]+h.SkipReasons["load+memory"] != 0 {
		t.Errorf("empty tick recorded a gate skip: %v", h.SkipReasons)
	}
}

// TestTickSolvedRecordsSolvedOnce: the happy path is exactly one
// solved outcome.
func TestTickSolvedRecordsSolvedOnce(t *testing.T) {
	obs := metrics.New(0, nil)
	l := fastLoop(newFakeQueue(makeEntry("s1")), &fakeSolver{})
	l.cfg.Observer = obs

	if err := l.Tick(context.Background()); err != nil {
		t.Fatalf("Tick err: %v", err)
	}
	h := schedulerHealth(t, obs)
	if got := h.Ticks[metrics.TickOutcomeSolved]; got != 1 {
		t.Errorf("solved ticks = %d, want 1", got)
	}
	if got := sumTicks(h); got != 1 {
		t.Errorf("total ticks = %d, want 1 (got %v)", got, h.Ticks)
	}
	if !h.LastTickOK {
		t.Fatal("last tick not stamped on the solved path")
	}
}

// TestTickErrorRecordsErrorOnce: every other failure is one error
// outcome, never zero (a failing tick that vanishes from the counter is
// exactly the blindness this tier exists to close).
func TestTickErrorRecordsErrorOnce(t *testing.T) {
	obs := metrics.New(0, nil)
	s := &fakeSolver{}
	s.solveScript = func(*ingest.Entry) (*solver.Solution, error) {
		return nil, errors.New("solver exploded")
	}
	l := fastLoop(newFakeQueue(makeEntry("e1")), s)
	l.cfg.Observer = obs

	if err := l.Tick(context.Background()); err == nil {
		t.Fatal("Tick err = nil, want the solver failure")
	}
	h := schedulerHealth(t, obs)
	if got := h.Ticks[metrics.TickOutcomeError]; got != 1 {
		t.Errorf("error ticks = %d, want 1", got)
	}
	if got := sumTicks(h); got != 1 {
		t.Errorf("total ticks = %d, want 1 (got %v)", got, h.Ticks)
	}
	if !h.LastTickOK {
		t.Fatal("last tick not stamped on the error path")
	}
}

// TestTickConcurrentCallIsNotATick: a Tick rejected by the in-flight
// CAS did no work, so it must not count an outcome nor stamp liveness.
func TestTickConcurrentCallIsNotATick(t *testing.T) {
	obs := metrics.New(0, nil)
	l := fastLoop(newFakeQueue(), &fakeSolver{})
	l.cfg.Observer = obs
	l.inflight.Store(true) // simulate a tick already running
	defer l.inflight.Store(false)

	if err := l.Tick(context.Background()); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("Tick err = %v, want ErrAlreadyRunning", err)
	}
	h := schedulerHealth(t, obs)
	if got := sumTicks(h); got != 0 {
		t.Errorf("rejected concurrent call counted %d ticks, want 0 (%v)", got, h.Ticks)
	}
	if h.LastTickOK {
		t.Errorf("rejected concurrent call stamped last tick at %v — it did no work", h.LastTick)
	}
}
