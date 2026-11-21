package metrics

import (
	"context"
	"strings"
	"testing"
	"time"
)

// labelledConfig returns a Prom wired to live sources for every family
// shape (plain gauge, GaugeFunc, const-metric collector, histogram
// children) so the label assertions below cannot be satisfied by a
// near-empty exposition.
func labelledConfig() PromConfig {
	return PromConfig{
		LoopRunning: func() bool { return true },
		SolveCounts: func() PromSolveCounts {
			return PromSolveCounts{Success: 5, Failed: 3, Rejected: 1}
		},
		SolveResources: func() SolveResources {
			return SolveResources{
				PeakRSSBucketCounts: []uint64{0, 1, 1, 1},
				PeakRSSSum:          300 << 20,
				PeakRSSCount:        1,
				Killed:              1,
			}
		},
		Deferrals: func() map[string]int64 {
			return map[string]int64{"load": 2, "memory": 1}
		},
		Discovery: func() PromDiscoveryCounts {
			return PromDiscoveryCounts{Hits: 1, Misses: 3}
		},
		DBDerived: func(context.Context) DBDerived {
			return DBDerived{
				QueuePending:     7,
				QueuePendingOK:   true,
				OldestPendingAge: 5 * time.Minute,
				OldestPendingOK:  true,
				VerifiedAnswers:  2,
				TotalAnswers:     4,
				CatalogOK:        true,
				DBSizeMainBytes:  1024,
				DBSizeMainOK:     true,
				DBSizeWALBytes:   512,
				DBSizeWALOK:      true,
			}
		},
	}
}

// TestPromConstLabelsAreStable pins the constants themselves: the wire
// requirement (OBS-OB-005) is the literal value off-by-one on both
// labels, so a refactor that changes ProductValue must fail here.
func TestPromConstLabelsAreStable(t *testing.T) {
	if ProductValue != "off-by-one" {
		t.Fatalf("ProductValue = %q, want %q", ProductValue, "off-by-one")
	}
	if got := PromConstLabels["product"]; got != "off-by-one" {
		t.Errorf(`PromConstLabels["product"] = %q, want "off-by-one"`, got)
	}
	if got := PromConstLabels["service"]; got != "off-by-one" {
		t.Errorf(`PromConstLabels["service"] = %q, want "off-by-one"`, got)
	}
}

// TestPromEverySeriesCarriesProductLabels is the OBS-OB-005 gate: on a
// populated exposition, EVERY obo_* sample — gauge, counter, histogram
// child and histogram bucket — carries product="off-by-one" and
// service="off-by-one". It asserts three ways: parsed label sets (parsing
// proves the wire format), the literal line count the task measures, and
// a per-line scan that no sample slipped through.
func TestPromEverySeriesCarriesProductLabels(t *testing.T) {
	p := NewProm(labelledConfig())
	p.RefreshDB(context.Background())
	p.ObserveSolveDuration("succeeded", 95*time.Second)

	body := serveProm(t, p)
	fams := parseProm(t, body)

	oboFamilies, samples := 0, 0
	for name, fam := range fams {
		if !strings.HasPrefix(name, "obo_") {
			t.Errorf("unexpected non-obo_ family %q in the product exposition", name)
			continue
		}
		oboFamilies++
		if len(fam.Metric) == 0 {
			t.Errorf("family %s exposes no samples — a family that never renders cannot be labelled", name)
		}
		for _, m := range fam.Metric {
			samples++
			got := map[string]string{}
			for _, lp := range m.Label {
				got[lp.GetName()] = lp.GetValue()
			}
			if got["product"] != ProductValue {
				t.Errorf("%s series %v: product = %q, want %q", name, got, got["product"], ProductValue)
			}
			if got["service"] != ProductValue {
				t.Errorf("%s series %v: service = %q, want %q", name, got, got["service"], ProductValue)
			}
		}
	}
	if oboFamilies < 12 {
		t.Fatalf("exposition has %d obo_* families, want >= 12 (a thin exposition makes this gate vacuous)", oboFamilies)
	}
	if samples < 12 {
		t.Fatalf("exposition has %d obo_* samples, want >= 12", samples)
	}

	// The requirement is stated on the wire, so count the wire too.
	if got := strings.Count(body, `product="off-by-one"`); got < 12 {
		t.Errorf(`exposition carries product="off-by-one" on %d lines, want >= 12`, got)
	}
	if got := strings.Count(body, `service="off-by-one"`); got < 12 {
		t.Errorf(`exposition carries service="off-by-one" on %d lines, want >= 12`, got)
	}

	// And no sample line at all may be missing it (comments excepted).
	for _, line := range strings.Split(body, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "obo_") {
			t.Errorf("sample line outside the obo_ namespace: %q", line)
			continue
		}
		if !strings.Contains(line, `product="off-by-one"`) {
			t.Errorf("sample line without the product label: %q", line)
		}
	}
}

// TestPromNilSafeExpositionCarriesProductLabels covers the second
// construction path: a registry with no sources wired still serves its
// zero-valued families, and those must be labelled too (a missing series
// in an install-time scrape is exactly when an operator looks).
func TestPromNilSafeExpositionCarriesProductLabels(t *testing.T) {
	var nilProm *Prom
	body := serveProm(t, nilProm)

	samples := 0
	for _, line := range strings.Split(body, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		samples++
		if !strings.Contains(line, `product="off-by-one"`) {
			t.Errorf("nil-safe sample line without the product label: %q", line)
		}
	}
	if samples < 12 {
		t.Fatalf("nil-safe exposition has %d samples, want >= 12", samples)
	}
}
