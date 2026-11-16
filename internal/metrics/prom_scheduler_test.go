package metrics

import (
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

// readProcLoad1 returns /proc/loadavg field 1 (the 1-minute average the
// idle gate itself judges) and skips the test where /proc is absent.
func readProcLoad1(t *testing.T) float64 {
	t.Helper()
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		t.Skipf("/proc/loadavg unavailable: %v", err)
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		t.Skipf("/proc/loadavg is empty: %q", string(data))
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		t.Skipf("/proc/loadavg field 1 %q is not a float: %v", fields[0], err)
	}
	return v
}

// schedulerHealthFixture drives one Observer through every member of
// both closed vocabularies plus one value OUTSIDE each, so the closed-set
// assertions below have something to reject.
func schedulerHealthFixture(t *testing.T) *Observer {
	t.Helper()
	obs := New(0, nil)
	for _, outcome := range TickOutcomes {
		obs.RecordTick(outcome)
	}
	obs.RecordTick("definitely_not_an_outcome") // must be dropped

	for _, reason := range SkipReasons {
		obs.RecordSkip(HostSnapshot{Load1: 12.5, MemUsedFraction: 0.42, Reason: reason})
	}
	// An unrecognised reason must never create a series (spec §3.5:
	// closed enumerations only).
	obs.RecordSkip(HostSnapshot{Load1: 12.5, MemUsedFraction: 0.42, Reason: "disk"})
	// RecordHost without a skip is the healthy gate reading: it moves
	// the host gauges but no counter.
	obs.RecordHost(HostSnapshot{Load1: 11.25, MemUsedFraction: 0.5})
	return obs
}

// family returns the named family, failing the test (instead of
// panicking inside sampleValue) when the exposition does not carry it.
func family(t *testing.T, fams map[string]*dto.MetricFamily, name string) *dto.MetricFamily {
	t.Helper()
	f, ok := fams[name]
	if !ok || f == nil {
		t.Fatalf("exposition is missing family %q", name)
	}
	return f
}

// TestPromSchedulerHealthFamilies is the exposition contract for
// OBS-OB-002 (docs/observability-spec.md §3.2): all five families are
// on the wire with # HELP/# TYPE lines and the spec's label names, the
// counters carry every closed-vocabulary value exactly once, and the
// gauges mirror the last host reading + last tick stamp.
func TestPromSchedulerHealthFamilies(t *testing.T) {
	obs := schedulerHealthFixture(t)
	p := NewProm(PromConfig{Scheduler: obs.SchedulerHealth})
	body := serveProm(t, p)

	for _, want := range []string{
		"# HELP obo_cron_ticks_total",
		"# TYPE obo_cron_ticks_total counter",
		"# HELP obo_cron_skip_reason_total",
		"# TYPE obo_cron_skip_reason_total counter",
		"# HELP obo_host_load1",
		"# TYPE obo_host_load1 gauge",
		"# HELP obo_host_mem_used_fraction",
		"# TYPE obo_host_mem_used_fraction gauge",
		"# HELP obo_cron_last_tick_timestamp_seconds",
		"# TYPE obo_cron_last_tick_timestamp_seconds gauge",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("exposition missing %q", want)
		}
	}

	fams := parseProm(t, body)
	for _, name := range []string{
		"obo_cron_ticks_total",
		"obo_cron_skip_reason_total",
		"obo_host_load1",
		"obo_host_mem_used_fraction",
		"obo_cron_last_tick_timestamp_seconds",
	} {
		family(t, fams, name) // fail loudly, not with a nil deref below
	}
	// Each outcome counted once; the out-of-vocabulary value added no
	// series and no count.
	if got := len(fams["obo_cron_ticks_total"].GetMetric()); got != len(TickOutcomes) {
		t.Errorf("obo_cron_ticks_total has %d series, want %d (closed set: %v)",
			got, len(TickOutcomes), TickOutcomes)
	}
	for _, outcome := range TickOutcomes {
		if v := sampleValue(t, fams["obo_cron_ticks_total"], map[string]string{"outcome": outcome}); v != 1 {
			t.Errorf("obo_cron_ticks_total{outcome=%q} = %v, want 1", outcome, v)
		}
	}
	var tickSum float64
	for _, m := range fams["obo_cron_ticks_total"].GetMetric() {
		tickSum += m.GetCounter().GetValue()
	}
	if tickSum != float64(len(TickOutcomes)) {
		t.Errorf("sum(obo_cron_ticks_total) = %v, want %d — an unknown outcome created a series",
			tickSum, len(TickOutcomes))
	}

	if got := len(fams["obo_cron_skip_reason_total"].GetMetric()); got != len(SkipReasons) {
		t.Errorf("obo_cron_skip_reason_total has %d series, want %d (closed set: %v)",
			got, len(SkipReasons), SkipReasons)
	}
	for _, reason := range SkipReasons {
		if v := sampleValue(t, fams["obo_cron_skip_reason_total"], map[string]string{"reason": reason}); v != 1 {
			t.Errorf("obo_cron_skip_reason_total{reason=%q} = %v, want 1", reason, v)
		}
	}
	for _, m := range fams["obo_cron_skip_reason_total"].GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "reason" && !IsSkipReason(lp.GetValue()) {
				t.Errorf("exposition contains out-of-vocabulary reason %q", lp.GetValue())
			}
		}
	}

	// The last reading wins for the unattributed gauges (RecordHost
	// after the skips).
	if v := sampleValue(t, fams["obo_host_load1"], nil); v != 11.25 {
		t.Errorf("obo_host_load1 = %v, want 11.25 (last host reading)", v)
	}
	if v := sampleValue(t, fams["obo_host_mem_used_fraction"], nil); v != 0.5 {
		t.Errorf("obo_host_mem_used_fraction = %v, want 0.5 (last host reading)", v)
	}
	_, ok := obs.LastTick()
	if !ok {
		t.Fatal("LastTick reported no reading after five RecordTick calls")
	}
	ts := sampleValue(t, fams["obo_cron_last_tick_timestamp_seconds"], nil)
	if delta := math.Abs(ts - float64(time.Now().Unix())); delta > 120 {
		t.Errorf("obo_cron_last_tick_timestamp_seconds = %v, want unix seconds near now (delta %.0fs)", ts, delta)
	}
}

// TestPromSchedulerHostLoadMatchesProcLoadavg pins the gauge against the
// host's real 1-minute load average (acceptance criterion AC2): the
// exposed value must be within 5% of /proc/loadavg field 1, so a unit
// conversion or truncation bug in the gauge cannot pass.
func TestPromSchedulerHostLoadMatchesProcLoadavg(t *testing.T) {
	before := readProcLoad1(t)
	obs := New(0, nil)
	obs.RecordHost(HostSnapshot{Load1: before, MemUsedFraction: 0.5})
	p := NewProm(PromConfig{Scheduler: obs.SchedulerHealth})
	fams := parseProm(t, serveProm(t, p))
	family(t, fams, "obo_host_load1")
	after := readProcLoad1(t)

	got := sampleValue(t, fams["obo_host_load1"], nil)
	if got != before {
		t.Errorf("obo_host_load1 = %v, want the recorded reading %v", got, before)
	}
	// The reading is taken between the two /proc samples; the 1-minute
	// average moves slowly, so 5% is a generous and stable bound.
	if delta := math.Abs(got - after); delta > 0.05*after {
		t.Errorf("obo_host_load1 = %v vs /proc/loadavg field 1 = %v (delta %.3f > 5%%)", got, after, delta)
	}
}

// TestPromSchedulerLastTickAdvances proves the freshness gauge moves
// between ticks — the property that makes "dead" distinguishable from
// "gated" (a starving loop still ticks).
func TestPromSchedulerLastTickAdvances(t *testing.T) {
	obs := New(0, nil)
	p := NewProm(PromConfig{Scheduler: obs.SchedulerHealth})

	// lastTick samples the freshness gauge, failing cleanly when the
	// exposition does not carry it.
	lastTick := func() float64 {
		t.Helper()
		fams := parseProm(t, serveProm(t, p))
		return sampleValue(t, family(t, fams, "obo_cron_last_tick_timestamp_seconds"), nil)
	}

	obs.RecordTick(TickOutcomeIdleGated)
	first := lastTick()

	time.Sleep(5 * time.Millisecond)
	obs.RecordTick(TickOutcomeIdleGated)
	second := lastTick()

	if !(second > first) {
		t.Fatalf("obo_cron_last_tick_timestamp_seconds did not advance: %v then %v", first, second)
	}
	// RecordTickStart (the loop's top-of-tick stamp) must move it too.
	time.Sleep(5 * time.Millisecond)
	obs.RecordTickStart()
	third := lastTick()
	if !(third > second) {
		t.Fatalf("RecordTickStart did not advance the stamp: %v then %v", second, third)
	}
	if h := obs.SchedulerHealth(); h.Ticks[TickOutcomeIdleGated] != 2 {
		t.Errorf("idle_gated ticks = %d, want 2 (RecordTickStart must not count an outcome)",
			h.Ticks[TickOutcomeIdleGated])
	}
}

// TestPromSchedulerClosedVocabulary feeds out-of-vocabulary labels and
// proves they are dropped rather than emitted (AC4's closed-set
// requirement): three reasons, four outcomes, nothing else.
func TestPromSchedulerClosedVocabulary(t *testing.T) {
	obs := New(0, nil)
	obs.RecordSkip(HostSnapshot{Load1: 40, MemUsedFraction: 0.99, Reason: "disk+io"})
	obs.RecordSkip(HostSnapshot{Load1: 40, MemUsedFraction: 0.99, Reason: ""})
	obs.RecordSkip(HostSnapshot{Load1: 40, MemUsedFraction: 0.99, Reason: "load"})
	obs.RecordTick("SOLVED") // case matters: not the closed value
	obs.RecordTick("")       // nothing to count
	obs.RecordTick(TickOutcomeSolved)

	p := NewProm(PromConfig{Scheduler: obs.SchedulerHealth})
	fams := parseProm(t, serveProm(t, p))
	ticks := family(t, fams, "obo_cron_ticks_total")
	reasons := family(t, fams, "obo_cron_skip_reason_total")

	if got := len(reasons.GetMetric()); got != len(SkipReasons) {
		t.Errorf("skip-reason series = %d, want %d (unknown reasons must be dropped)", got, len(SkipReasons))
	}
	if v := sampleValue(t, reasons, map[string]string{"reason": "load"}); v != 1 {
		t.Errorf("obo_cron_skip_reason_total{reason=\"load\"} = %v, want 1", v)
	}
	if got := len(ticks.GetMetric()); got != len(TickOutcomes) {
		t.Errorf("tick-outcome series = %d, want %d (unknown outcomes must be dropped)", got, len(TickOutcomes))
	}
	var sum float64
	for _, m := range ticks.GetMetric() {
		sum += m.GetCounter().GetValue()
	}
	if sum != 1 {
		t.Errorf("sum(obo_cron_ticks_total) = %v, want 1 (only TickOutcomeSolved counted)", sum)
	}
	// The dropped skip still counted the ordinary skip total and stored
	// the reading: only the LABEL is suppressed, never the liveness data.
	if snap := obs.Snapshot(); snap.SkippedHostPressure != 3 {
		t.Errorf("SkippedHostPressure = %d, want 3 (dropping a label must not drop the skip)", snap.SkippedHostPressure)
	}
}

// TestPromSchedulerNilSource proves the no-source exposition keeps its
// counter shape (all label values at 0, so rate() has a series) while
// omitting the gauges — a gauge with no reading must not claim 0.
func TestPromSchedulerNilSource(t *testing.T) {
	fams := parseProm(t, serveProm(t, NewProm(PromConfig{})))
	family(t, fams, "obo_cron_ticks_total")
	family(t, fams, "obo_cron_skip_reason_total")

	for _, outcome := range TickOutcomes {
		if v := sampleValue(t, fams["obo_cron_ticks_total"], map[string]string{"outcome": outcome}); v != 0 {
			t.Errorf("no-source obo_cron_ticks_total{outcome=%q} = %v, want 0", outcome, v)
		}
	}
	for _, reason := range SkipReasons {
		if v := sampleValue(t, fams["obo_cron_skip_reason_total"], map[string]string{"reason": reason}); v != 0 {
			t.Errorf("no-source obo_cron_skip_reason_total{reason=%q} = %v, want 0", reason, v)
		}
	}
	if fams["obo_host_load1"] != nil {
		t.Error("obo_host_load1 present with no reading — a gauge must be omitted, never fabricated as 0")
	}
	if fams["obo_host_mem_used_fraction"] != nil {
		t.Error("obo_host_mem_used_fraction present with no reading")
	}
	if fams["obo_cron_last_tick_timestamp_seconds"] != nil {
		t.Error("obo_cron_last_tick_timestamp_seconds present before the first tick")
	}

	// A host reading with no memory signal (memory gate disabled) emits
	// load but not a fabricated 0 memory fraction.
	obs := New(0, nil)
	obs.RecordHost(HostSnapshot{Load1: 3.5})
	fams = parseProm(t, serveProm(t, NewProm(PromConfig{Scheduler: obs.SchedulerHealth})))
	if v := sampleValue(t, fams["obo_host_load1"], nil); v != 3.5 {
		t.Errorf("obo_host_load1 = %v, want 3.5", v)
	}
	if fams["obo_host_mem_used_fraction"] != nil {
		t.Error("obo_host_mem_used_fraction emitted 0 for a snapshot that carried no memory reading")
	}
}
