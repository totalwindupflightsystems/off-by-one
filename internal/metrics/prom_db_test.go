package metrics

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// tier3Fixture is a full Tier-3 reading: every leg read, values chosen
// so a unit/scale mistake is visible (90s age, a 3/4 catalogue, a WAL
// smaller than the main file).
func tier3Reading() DBDerived {
	return DBDerived{
		QueuePending:     4,
		QueuePendingOK:   true,
		OldestPendingAge: 90 * time.Second,
		OldestPendingOK:  true,
		VerifiedAnswers:  3,
		TotalAnswers:     4,
		CatalogOK:        true,
		DBSizeMainBytes:  56 * 1024 * 1024,
		DBSizeMainOK:     true,
		DBSizeWALBytes:   1150 * 1024,
		DBSizeWALOK:      true,
	}
}

// TestPromTier3Families is the exposition contract for OBS-OB-004
// (docs/observability-spec.md §3.4): all six Tier-3 families are on the
// wire with the spec's names, types, units and label names, and the
// values are the refreshed readings — not defaults.
func TestPromTier3Families(t *testing.T) {
	obs := New(0, nil)
	obs.RecordDeferred("load")
	obs.RecordDeferred("memory")
	obs.RecordDeferred("memory")
	obs.RecordDiscovery(true)
	obs.RecordDiscovery(false)
	obs.RecordDiscovery(false)
	obs.RecordDiscovery(false)

	p := NewProm(PromConfig{
		DBDerived: func(ctx context.Context) DBDerived { return tier3Reading() },
		Deferrals: obs.Deferrals,
		Discovery: func() PromDiscoveryCounts {
			hits, misses := obs.DiscoveryCounts()
			return PromDiscoveryCounts{Hits: hits, Misses: misses}
		},
	})
	p.RefreshDB(context.Background())
	body := serveProm(t, p)

	for _, want := range []string{
		"# HELP obo_queue_oldest_pending_age_seconds",
		"# TYPE obo_queue_oldest_pending_age_seconds gauge",
		"# HELP obo_queue_deferred_total",
		"# TYPE obo_queue_deferred_total counter",
		"# HELP obo_catalog_verified_answers",
		"# TYPE obo_catalog_verified_answers gauge",
		"# HELP obo_catalog_hit_rate",
		"# TYPE obo_catalog_hit_rate gauge",
		"# HELP obo_discovery_requests_total",
		"# TYPE obo_discovery_requests_total counter",
		"# HELP obo_db_size_bytes",
		"# TYPE obo_db_size_bytes gauge",
		// Label order is alphabetical across the whole set, so the const
		// labels interleave with `reason` (product < reason < service).
		`obo_queue_deferred_total{product="off-by-one",reason="load",service="off-by-one"} 1`,
		`obo_queue_deferred_total{product="off-by-one",reason="memory",service="off-by-one"} 2`,
		`obo_queue_deferred_total{product="off-by-one",reason="load+memory",service="off-by-one"} 0`,
		`obo_discovery_requests_total{outcome="hit",` + wantConstLabels + `} 1`,
		`obo_discovery_requests_total{outcome="miss",` + wantConstLabels + `} 3`,
		`obo_db_size_bytes{kind="main",` + wantConstLabels + `}`,
		`obo_db_size_bytes{kind="wal",` + wantConstLabels + `}`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("exposition missing %q", want)
		}
	}

	fams := parseProm(t, body)

	// Seconds, not nanoseconds or minutes: 90s must read as 90.
	if v := sampleValue(t, family(t, fams, "obo_queue_oldest_pending_age_seconds"), nil); v != 90 {
		t.Errorf("obo_queue_oldest_pending_age_seconds = %v, want 90", v)
	}
	if v := sampleValue(t, family(t, fams, "obo_catalog_verified_answers"), nil); v != 3 {
		t.Errorf("obo_catalog_verified_answers = %v, want 3", v)
	}
	// The ratio is the SAME arithmetic /api/v1/stats serves: 3/4.
	if v := sampleValue(t, family(t, fams, "obo_catalog_hit_rate"), nil); v != 0.75 {
		t.Errorf("obo_catalog_hit_rate = %v, want 0.75 (3 verified / 4 answers)", v)
	}
	// queue_pending stays on the wire, now from the same cache.
	if v := sampleValue(t, family(t, fams, "obo_queue_pending"), nil); v != 4 {
		t.Errorf("obo_queue_pending = %v, want 4 (cached DB reading)", v)
	}
	// Bytes, not KiB/MiB: both parts match the stat() the wiring took.
	if v := sampleValue(t, family(t, fams, "obo_db_size_bytes"), map[string]string{"kind": "main"}); v != 56*1024*1024 {
		t.Errorf("obo_db_size_bytes{kind=\"main\"} = %v, want %d", v, 56*1024*1024)
	}
	if v := sampleValue(t, family(t, fams, "obo_db_size_bytes"), map[string]string{"kind": "wal"}); v != 1150*1024 {
		t.Errorf("obo_db_size_bytes{kind=\"wal\"} = %v, want %d", v, 1150*1024)
	}
}

// TestPromTier3ClosedVocabulary feeds out-of-vocabulary labels and
// proves they are dropped rather than emitted: three deferral reasons,
// two discovery outcomes, two DB kinds — nothing else (AC6).
func TestPromTier3ClosedVocabulary(t *testing.T) {
	obs := New(0, nil)
	// Out-of-vocabulary reasons must never create a series. "disk" and
	// "" are the same rejections RecordSkip makes.
	obs.RecordDeferred("disk")
	obs.RecordDeferred("")
	obs.RecordDeferred("LOAD") // case matters: not the closed value
	obs.RecordDeferred("load")
	// A gate DECISION without displaced work must not move the
	// deferral counter: RecordSkip counts the decision, obo_queue_
	// deferred_total counts the work it postponed.
	obs.RecordSkip(HostSnapshot{Load1: 30, MemUsedFraction: 0.99, Reason: "load"})

	p := NewProm(PromConfig{
		DBDerived: func(ctx context.Context) DBDerived { return tier3Reading() },
		Deferrals: obs.Deferrals,
		Discovery: func() PromDiscoveryCounts {
			return PromDiscoveryCounts{Hits: 2, Misses: 0}
		},
	})
	p.RefreshDB(context.Background())
	fams := parseProm(t, serveProm(t, p))

	deferred := family(t, fams, "obo_queue_deferred_total")
	if got := len(deferred.GetMetric()); got != len(SkipReasons) {
		t.Errorf("deferral series = %d, want %d (closed set: %v)", got, len(SkipReasons), SkipReasons)
	}
	for _, m := range deferred.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "reason" && !IsSkipReason(lp.GetValue()) {
				t.Errorf("exposition contains out-of-vocabulary deferral reason %q", lp.GetValue())
			}
		}
	}
	if v := sampleValue(t, deferred, map[string]string{"reason": "load"}); v != 1 {
		t.Errorf("obo_queue_deferred_total{reason=\"load\"} = %v, want 1 (three unknown reasons dropped, the skip not counted)", v)
	}

	discovery := family(t, fams, "obo_discovery_requests_total")
	if got := len(discovery.GetMetric()); got != len(DiscoveryOutcomes) {
		t.Errorf("discovery series = %d, want %d (closed set: %v)", got, len(DiscoveryOutcomes), DiscoveryOutcomes)
	}
	for _, m := range discovery.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "outcome" && !IsDiscoveryOutcome(lp.GetValue()) {
				t.Errorf("exposition contains out-of-vocabulary discovery outcome %q", lp.GetValue())
			}
		}
	}
	if v := sampleValue(t, discovery, map[string]string{"outcome": "miss"}); v != 0 {
		t.Errorf("obo_discovery_requests_total{outcome=\"miss\"} = %v, want 0 (present at zero, not absent)", v)
	}

	sizes := family(t, fams, "obo_db_size_bytes")
	if got := len(sizes.GetMetric()); got != len(DBKinds) {
		t.Errorf("db_size series = %d, want %d (closed set: %v)", got, len(DBKinds), DBKinds)
	}
	for _, m := range sizes.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "kind" {
				if lp.GetValue() != DBKinds[0] && lp.GetValue() != DBKinds[1] {
					t.Errorf("exposition contains out-of-vocabulary db kind %q", lp.GetValue())
				}
			}
		}
	}
}

// TestPromCatalogHitRateEmptyCatalogue pins the ratio's zero case: an
// empty catalogue is 0 (the arithmetic graph.Stats serves), not a
// missing series.
func TestPromCatalogHitRateEmptyCatalogue(t *testing.T) {
	p := NewProm(PromConfig{
		DBDerived: func(ctx context.Context) DBDerived {
			return DBDerived{VerifiedAnswers: 0, TotalAnswers: 0, CatalogOK: true}
		},
	})
	p.RefreshDB(context.Background())
	fams := parseProm(t, serveProm(t, p))

	if v := sampleValue(t, family(t, fams, "obo_catalog_verified_answers"), nil); v != 0 {
		t.Errorf("obo_catalog_verified_answers = %v, want 0", v)
	}
	if v := sampleValue(t, family(t, fams, "obo_catalog_hit_rate"), nil); v != 0 {
		t.Errorf("obo_catalog_hit_rate = %v, want 0 on an empty catalogue", v)
	}
}

// TestPromDBGaugesCachedNotQueriedPerScrape is AC5's proof: the DB
// source runs on the refresher only. The source sleeps 250ms and counts
// its calls, so a single scrape that queried it would take at least a
// quarter second and bump the counter — either failure is unambiguous.
func TestPromDBGaugesCachedNotQueriedPerScrape(t *testing.T) {
	var calls int64
	p := NewProm(PromConfig{
		DBDerived: func(ctx context.Context) DBDerived {
			atomic.AddInt64(&calls, 1)
			time.Sleep(250 * time.Millisecond)
			d := tier3Reading()
			d.QueuePending = 42
			return d
		},
	})
	p.RefreshDB(context.Background())
	if got := atomic.LoadInt64(&calls); got != 1 {
		t.Fatalf("DB source called %d times by one RefreshDB, want 1", got)
	}

	// Every scrape must be served from the cache: no source call, and
	// the reading is the cached one even though the source would block.
	best := time.Hour
	for i := 0; i < 10; i++ {
		start := time.Now()
		body := serveProm(t, p)
		elapsed := time.Since(start)
		if elapsed < best {
			best = elapsed
		}
		if v := sampleValue(t, parseProm(t, body)["obo_queue_pending"], nil); v != 42 {
			t.Fatalf("scrape %d: obo_queue_pending = %v, want the cached 42", i, v)
		}
	}
	if got := atomic.LoadInt64(&calls); got != 1 {
		t.Errorf("DB source called %d times across 10 scrapes, want 1 — Collect must never query the DB (§4.4)", got)
	}
	// The budget is 50ms; a scrape that hit the 250ms source cannot pass
	// this even on a busy host (we take the best of ten to keep the
	// assertion about the code, not about scheduler noise).
	if best >= 50*time.Millisecond {
		t.Errorf("fastest of 10 scrapes took %v, want < 50ms", best)
	}
}

// TestPromDBRefreshTicker proves StartDBRefresh actually refreshes on
// its interval (the mechanism §4.4 specifies) and stops when its
// context is cancelled.
func TestPromDBRefreshTicker(t *testing.T) {
	var calls int64
	p := NewProm(PromConfig{
		DBDerived: func(ctx context.Context) DBDerived {
			atomic.AddInt64(&calls, 1)
			return tier3Reading()
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.StartDBRefresh(ctx, 10*time.Millisecond)

	// StartDBRefresh refreshes immediately, so the gauges exist straight
	// away rather than after one interval.
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt64(&calls) < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := atomic.LoadInt64(&calls); got < 2 {
		t.Fatalf("refresher ran %d times in 2s at a 10ms interval, want repeated refreshes", got)
	}

	cancel()
	// Let any in-flight refresh land, then require the count to freeze.
	time.Sleep(50 * time.Millisecond)
	after := atomic.LoadInt64(&calls)
	time.Sleep(100 * time.Millisecond)
	if got := atomic.LoadInt64(&calls); got != after {
		t.Errorf("refresher kept running after cancel: %d then %d calls", after, got)
	}
}

// TestPromTier3NilSource proves the source-less exposition still carries
// every Tier-3 family that has a defined zero (the counters, and
// obo_queue_pending whose pre-OBS-OB-004 contract was a constant 0) and
// omits the gauges that would otherwise claim a reading nobody took.
func TestPromTier3NilSource(t *testing.T) {
	fams := parseProm(t, serveProm(t, NewProm(PromConfig{})))

	if v := sampleValue(t, family(t, fams, "obo_queue_pending"), nil); v != 0 {
		t.Errorf("no-source obo_queue_pending = %v, want 0", v)
	}
	for _, reason := range SkipReasons {
		if v := sampleValue(t, family(t, fams, "obo_queue_deferred_total"), map[string]string{"reason": reason}); v != 0 {
			t.Errorf("no-source obo_queue_deferred_total{reason=%q} = %v, want 0", reason, v)
		}
	}
	for _, outcome := range DiscoveryOutcomes {
		if v := sampleValue(t, family(t, fams, "obo_discovery_requests_total"), map[string]string{"outcome": outcome}); v != 0 {
			t.Errorf("no-source obo_discovery_requests_total{outcome=%q} = %v, want 0", outcome, v)
		}
	}
	for _, name := range []string{
		"obo_queue_oldest_pending_age_seconds",
		"obo_catalog_verified_answers",
		"obo_catalog_hit_rate",
		"obo_db_size_bytes",
	} {
		if fams[name] != nil {
			t.Errorf("%s present with no DB reading — a gauge must be omitted, never fabricated as 0", name)
		}
	}
}

// TestPromDBUnreadLegsAreOmitted pins the per-leg honesty rule: one
// failed leg (the store is locked, the stat failed) must not blank or
// fabricate the others.
func TestPromDBUnreadLegsAreOmitted(t *testing.T) {
	p := NewProm(PromConfig{
		DBDerived: func(ctx context.Context) DBDerived {
			// Queue read fine, the catalogue query and the stat failed.
			return DBDerived{QueuePending: 9, QueuePendingOK: true}
		},
	})
	p.RefreshDB(context.Background())
	fams := parseProm(t, serveProm(t, p))

	if v := sampleValue(t, family(t, fams, "obo_queue_pending"), nil); v != 9 {
		t.Errorf("obo_queue_pending = %v, want 9", v)
	}
	for _, name := range []string{
		"obo_queue_oldest_pending_age_seconds",
		"obo_catalog_verified_answers",
		"obo_catalog_hit_rate",
		"obo_db_size_bytes",
	} {
		if fams[name] != nil {
			t.Errorf("%s emitted despite an unread source leg", name)
		}
	}
}

// TestPromDBEmptyQueueAgeIsZero pins the "nothing is waiting" case: 0,
// present, because an empty queue is a real reading — the same zero
// AvgSolveTime returns — and AC1 requires the family on every scrape.
func TestPromDBEmptyQueueAgeIsZero(t *testing.T) {
	p := NewProm(PromConfig{
		DBDerived: func(ctx context.Context) DBDerived {
			return DBDerived{QueuePending: 0, QueuePendingOK: true, OldestPendingOK: true}
		},
	})
	p.RefreshDB(context.Background())
	fams := parseProm(t, serveProm(t, p))

	if v := sampleValue(t, family(t, fams, "obo_queue_pending"), nil); v != 0 {
		t.Errorf("obo_queue_pending = %v, want 0 on an empty queue", v)
	}
	if v := sampleValue(t, family(t, fams, "obo_queue_oldest_pending_age_seconds"), nil); v != 0 {
		t.Errorf("obo_queue_oldest_pending_age_seconds = %v, want 0 on an empty queue", v)
	}
}

// TestPromDBRefreshNilSafe proves a source-less Prom (and a nil one)
// tolerates the refresher API: no goroutine, no panic.
func TestPromDBRefreshNilSafe(t *testing.T) {
	var p *Prom
	p.RefreshDB(context.Background())
	p.StartDBRefresh(context.Background(), time.Millisecond)

	empty := NewProm(PromConfig{})
	empty.RefreshDB(context.Background())
	empty.StartDBRefresh(context.Background(), time.Millisecond)
	if v := sampleValue(t, parseProm(t, serveProm(t, empty))["obo_queue_pending"], nil); v != 0 {
		t.Errorf("obo_queue_pending = %v, want 0", v)
	}
}
