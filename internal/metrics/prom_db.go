// prom_db.go — Tier-3 DB-derived gauges (OBS-OB-004,
// docs/observability-spec.md §3.4) and their refresh cache (§4.4).
//
// The queue age, catalogue and storage gauges each need a SQLite query
// or a stat(). §4.4 is explicit that they must NOT be computed inside
// the scrape: the queue's paged reads were re-indexed precisely because
// query cost on this store matters (17.3ms → 0.11ms), and a scrape
// storm must never become a SQL storm. So the readings live in a
// package-private cache that a ticker refreshes every 15s; Collect only
// copies cached values and never touches SQLite or the filesystem.
//
// Honesty rules inherited from the Tier-2 collectors: a reading that
// has not been taken is OMITTED, never fabricated as 0. Each leg of
// DBDerived carries its own OK flag for exactly that reason — one dead
// leg (a failed stat, a locked store) must not blank the others, and it
// must not silently report zero either.
package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// DefaultDBRefreshInterval is the DB-gauge refresh cadence: 15s, the
// interval §4.4 specifies.
const DefaultDBRefreshInterval = 15 * time.Second

// dbRefreshTimeout bounds ONE refresh so a wedged or locked store
// cannot stall the refresher goroutine (and, through it, every later
// refresh and the shutdown path).
const dbRefreshTimeout = 2 * time.Second

// DBKinds is the complete `kind` label set for obo_db_size_bytes. A
// kind outside this set is never emitted; the collector iterates this
// slice, not whatever the source reports.
var DBKinds = []string{"main", "wal"}

// DBDerived is one refresh of the DB-derived Tier-3 gauge readings
// (spec §3.4). It is data, not a handle: the metrics package never
// imports the graph or queue stores, so the wiring (and the SQL truth
// behind it) stays in main/api and only the numbers cross the boundary.
//
// Every leg has an OK flag. False means "no reading this refresh" and
// the corresponding series is omitted — the same rule the scheduler
// gauges follow.
type DBDerived struct {
	// QueuePending is the count of pending/deferred/in_progress
	// entries — the queue_depth /api/v1/stats serves.
	QueuePending   int64
	QueuePendingOK bool
	// OldestPendingAge is now - MIN(created_at) over the same status
	// set: how long the oldest waiting submission has waited. An empty
	// queue is a real 0 (nothing is waiting) — the same zero
	// AvgSolveTime returns with no completed solves — so the family is
	// always present; only a failed query leaves OldestPendingOK false
	// and omits it.
	OldestPendingAge time.Duration
	OldestPendingOK  bool
	// VerifiedAnswers / TotalAnswers mirror /api/v1/stats; the hit
	// rate is derived from them by the collector so the two cannot
	// disagree arithmetically.
	VerifiedAnswers int64
	TotalAnswers    int64
	CatalogOK       bool
	// DBSizeMainBytes is the SQLite main file; DBSizeWALBytes is its
	// -wal sibling. A missing -wal file is a REAL zero (the store is
	// not in WAL mode), so the wiring sets DBSizeWALOK with 0 bytes
	// rather than leaving the series out.
	DBSizeMainBytes int64
	DBSizeMainOK    bool
	DBSizeWALBytes  int64
	DBSizeWALOK     bool
}

// dbGaugeCache holds the most recent refresh. Readers (Collect) take
// the read lock for a struct copy and never block on the source.
//
// The zero snapshot is not usable: obo_queue_pending has been present
// on every exposition since OBS-OB-001, including a source-less
// registry, so newDBGaugeCache seeds that one reading as a known zero
// and leaves every other leg unread (omitted until the first refresh).
type dbGaugeCache struct {
	mu   sync.RWMutex
	snap DBDerived
}

func newDBGaugeCache() *dbGaugeCache {
	return &dbGaugeCache{snap: DBDerived{QueuePendingOK: true}}
}

func (c *dbGaugeCache) store(d DBDerived) {
	c.mu.Lock()
	c.snap = d
	c.mu.Unlock()
}

func (c *dbGaugeCache) snapshot() DBDerived {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snap
}

// RefreshDB performs one synchronous refresh of the DB-derived gauges.
// It is the ONLY caller of PromConfig.DBDerived, and it is meant to be
// called from a ticker (StartDBRefresh) or once at startup so the
// gauges exist on the first scrape. Nil-receiver- and nil-source-safe;
// a Prom with no DBDerived source simply keeps the default snapshot.
func (p *Prom) RefreshDB(ctx context.Context) {
	if p == nil || p.dbSrc == nil {
		return
	}
	// Serialise refreshes: a source slower than the tick must not
	// stack up goroutines against a wedged store.
	p.dbRefreshMu.Lock()
	defer p.dbRefreshMu.Unlock()
	rctx, cancel := context.WithTimeout(ctx, dbRefreshTimeout)
	defer cancel()
	p.db.store(p.dbSrc(rctx))
}

// StartDBRefresh refreshes once immediately (so a fresh process serves
// real values rather than an empty gauges set) and then every interval
// until ctx is cancelled. A non-positive interval falls back to
// DefaultDBRefreshInterval. It returns immediately; the refresh runs in
// its own goroutine and a Prom with no DBDerived source starts nothing.
func (p *Prom) StartDBRefresh(ctx context.Context, interval time.Duration) {
	if p == nil || p.dbSrc == nil {
		return
	}
	if interval <= 0 {
		interval = DefaultDBRefreshInterval
	}
	p.RefreshDB(ctx)
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				p.RefreshDB(ctx)
			}
		}
	}()
}

// dbGaugeCollector emits the Tier-3 DB-derived gauges from the cache.
//
// Why the cache is the whole story here: Collect must be O(1) — the
// spec's 50ms scrape budget is a promise to the monitoring stack, and a
// DB-derived gauge computed per scrape breaks it on a 200k-row queue at
// exactly the moment (a scrape storm) when it matters most.
type dbGaugeCollector struct {
	cache                *dbGaugeCache
	pendingDesc          *prometheus.Desc
	oldestPendingAgeDesc *prometheus.Desc
	verifiedAnswersDesc  *prometheus.Desc
	hitRateDesc          *prometheus.Desc
	sizeDesc             *prometheus.Desc
}

func (c *dbGaugeCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.pendingDesc
	ch <- c.oldestPendingAgeDesc
	ch <- c.verifiedAnswersDesc
	ch <- c.hitRateDesc
	ch <- c.sizeDesc
}

func (c *dbGaugeCollector) Collect(ch chan<- prometheus.Metric) {
	var d DBDerived
	if c.cache != nil {
		d = c.cache.snapshot()
	}

	if d.QueuePendingOK {
		n := d.QueuePending
		if n < 0 {
			n = 0
		}
		ch <- prometheus.MustNewConstMetric(c.pendingDesc, prometheus.GaugeValue, float64(n))
	}
	if d.OldestPendingOK {
		secs := d.OldestPendingAge.Seconds()
		if secs < 0 {
			secs = 0
		}
		ch <- prometheus.MustNewConstMetric(c.oldestPendingAgeDesc, prometheus.GaugeValue, secs)
	}
	if d.CatalogOK {
		verified := d.VerifiedAnswers
		if verified < 0 {
			verified = 0
		}
		total := d.TotalAnswers
		if total < 0 {
			total = 0
		}
		ch <- prometheus.MustNewConstMetric(c.verifiedAnswersDesc, prometheus.GaugeValue, float64(verified))
		// Same arithmetic as graph.Stats: verified/total, and 0 on an
		// empty catalogue (a defined ratio, not a missing reading).
		hitRate := 0.0
		if total > 0 {
			hitRate = float64(verified) / float64(total)
		}
		ch <- prometheus.MustNewConstMetric(c.hitRateDesc, prometheus.GaugeValue, hitRate)
	}

	sizes := map[string]struct {
		bytes int64
		ok    bool
	}{
		DBKinds[0]: {d.DBSizeMainBytes, d.DBSizeMainOK},
		DBKinds[1]: {d.DBSizeWALBytes, d.DBSizeWALOK},
	}
	for _, kind := range DBKinds {
		s := sizes[kind]
		if !s.ok {
			continue
		}
		n := s.bytes
		if n < 0 {
			n = 0
		}
		ch <- prometheus.MustNewConstMetric(c.sizeDesc, prometheus.GaugeValue, float64(n), kind)
	}
}
