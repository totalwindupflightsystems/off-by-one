package cron

import (
	"sync"
	"testing"
	"time"
)

// TestMetricsDurationObserverPins the OBS-OB-001 contract: each
// record* method emits exactly one duration observation, with the
// outcome label matching the partition the /metrics/prom collector
// serves (rejection → "rejected", not "failed").
func TestMetricsDurationObserver(t *testing.T) {
	var mu sync.Mutex
	type obs struct {
		outcome string
		d       time.Duration
	}
	var got []obs
	m := &Metrics{}
	m.SetDurationObserver(func(outcome string, d time.Duration) {
		mu.Lock()
		got = append(got, obs{outcome, d})
		mu.Unlock()
	})

	m.recordSuccess(100 * time.Millisecond)
	m.recordFailure(200 * time.Millisecond)
	m.recordRejection(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	want := []obs{
		{"succeeded", 100 * time.Millisecond},
		{"failed", 200 * time.Millisecond},
		{"rejected", 300 * time.Millisecond},
	}
	if len(got) != len(want) {
		t.Fatalf("observations = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("observation %d = %v, want %v", i, got[i], want[i])
		}
	}

	// The existing counter semantics are untouched: rejection moves
	// attempts+failed+rejected.
	snap := m.Snapshot()
	if snap.SolveAttempts != 3 || snap.SolveSuccess != 1 || snap.SolveFailed != 2 || snap.SolveRejected != 1 {
		t.Errorf("counters: %+v, want attempts=3 success=1 failed=2 rejected=1", snap)
	}
}

// TestMetricsDurationObserverNilAndUnset covers the two disabled
// states: never-installed (zero Metrics, as in most tests) and
// explicitly cleared. Neither may panic or observe anything.
func TestMetricsDurationObserverNilAndUnset(t *testing.T) {
	m := &Metrics{}
	m.recordSuccess(time.Millisecond) // unset: no-op

	called := false
	m.SetDurationObserver(func(string, time.Duration) { called = true })
	m.SetDurationObserver(nil)
	m.recordSuccess(time.Millisecond)
	m.recordFailure(time.Millisecond)
	m.recordRejection(time.Millisecond)
	if called {
		t.Error("observer called after SetDurationObserver(nil)")
	}
}
