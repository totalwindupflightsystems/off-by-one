package cron

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
	"github.com/totalwindupflightsystems/off-by-one/internal/metrics"
	"github.com/totalwindupflightsystems/off-by-one/internal/solver"
)

// scrapeProm serves the exposition off p's handler and returns the body.
func scrapeProm(t *testing.T, p *metrics.Prom) string {
	t.Helper()
	rr := httptest.NewRecorder()
	p.Handler().ServeHTTP(rr, httptest.NewRequest("GET", "/metrics/prom", nil))
	if rr.Code != 200 {
		t.Fatalf("GET /metrics/prom: status %d, want 200", rr.Code)
	}
	return rr.Body.String()
}

// promSample reads the value of one exact series line
// (`name` or `name{labels}`) out of an exposition body. It fails when
// the series is absent: for OBS-OB-003 every family is always present,
// so a missing series is a contract break, not a zero.
func promSample(t *testing.T, body, series string) float64 {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, series+" ") {
			v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(line, series)), 64)
			if err != nil {
				t.Fatalf("parse %q from line %q: %v", series, line, err)
			}
			return v
		}
	}
	t.Fatalf("series %q absent from exposition", series)
	return 0
}

// causeLoop builds a loop with both host gates disabled (a deterministic
// open gate) and the given solver/queue/observer wired.
func causeLoop(q Queue, s Solver, obs *metrics.Observer) *Loop {
	return NewLoop(Config{
		Queue:           q,
		Solver:          s,
		Interval:        time.Millisecond,
		LoadThreshold:   -1, // disable the load gate
		MemoryThreshold: -1, // disable the memory gate
		Observer:        obs,
	})
}

// TestTimeoutKillRaisesCauseTimeoutByExactlyOne is the unit-level form
// of OBS-OB-003 acceptance criterion 2: promote ONE real failure whose
// error carries the --solve-timeout kill, read
// obo_solve_failure_total{cause="timeout"} before and after, and demand
// an increase of exactly 1 — no double counting, no misclassification
// into solver_exit (the timeout error also carries "pi-agent exec:").
func TestTimeoutKillRaisesCauseTimeoutByExactlyOne(t *testing.T) {
	var buf strings.Builder
	obs := metrics.New(metrics.DefaultRSSAlertBytes, nil)
	prom := metrics.NewProm(metrics.PromConfig{SolveResources: obs.SolveResources})

	const timeoutSeries = `obo_solve_failure_total{cause="timeout"}`
	const exitSeries = `obo_solve_failure_total{cause="solver_exit"}`
	beforeBody := scrapeProm(t, prom)
	beforeTimeout := promSample(t, beforeBody, timeoutSeries)
	beforeExit := promSample(t, beforeBody, exitSeries)

	q := newFakeQueue(makeEntry("timeout-1"))
	l := causeLoop(q, &fakeSolver{solveScript: func(*ingest.Entry) (*solver.Solution, error) {
		return nil, errors.New("solve killed by --solve-timeout 25s: pi-agent exec: signal: killed (stdout=)")
	}}, obs)
	l.cfg.Logger = newTestLogger(&buf)

	if err := l.Tick(context.Background()); err == nil {
		t.Fatal("Tick err = nil, want the solve failure")
	}

	afterBody := scrapeProm(t, prom)
	if delta := promSample(t, afterBody, timeoutSeries) - beforeTimeout; delta != 1 {
		t.Errorf("obo_solve_failure_total{cause=\"timeout\"} delta = %v, want exactly 1", delta)
	}
	if after := promSample(t, afterBody, exitSeries); after != beforeExit {
		t.Errorf("obo_solve_failure_total{cause=\"solver_exit\"} = %v, want unchanged (%v)", after, beforeExit)
	}
	// One solve, one cause sample: the failure series must not double count.
	var total float64
	for _, cause := range solver.FailureCauses {
		total += promSample(t, afterBody, `obo_solve_failure_total{cause="`+cause+`"}`)
	}
	if total != 1 {
		t.Errorf("sum(obo_solve_failure_total) = %v, want 1 (one failed solve)", total)
	}
	// The stored failure_reason is still the raw solver error: the metric
	// is additive, it does not replace the per-entry history.
	if got := q.failText["timeout-1"]; !strings.Contains(got, "--solve-timeout 25s") {
		t.Errorf("stored failure_reason = %q, want the raw solver error", got)
	}
	if got := q.stage["timeout-1"]; got != solver.StageSolverFailed {
		t.Errorf("stage = %q, want %q", got, solver.StageSolverFailed)
	}
	// The log line names the cause so an incident can be read from logs.
	if !strings.Contains(buf.String(), "cause=timeout") {
		t.Errorf("failure log %q missing cause=timeout", buf.String())
	}
}

// TestMemoryKillRaisesKilledTotalAndHistogram is the unit-level form of
// OBS-OB-003 acceptance criterion 3: a solve killed by its memory cap
// increments obo_solve_killed_total and contributes exactly one
// observation to obo_solve_peak_rss_bytes (so the +Inf bucket matches
// the solve count). This is the arm that was dead code before the fix:
// killed was gated on a non-nil solution, which a killed solve never has.
func TestMemoryKillRaisesKilledTotalAndHistogram(t *testing.T) {
	const peak = uint64(768) << 20

	sampler := &stubSampler{byPid: map[int]uint64{os.Getpid(): peak}}
	saved := childPIDsFn
	childPIDsFn = func(int) []int { return nil }
	defer func() { childPIDsFn = saved }()

	obs := metrics.New(metrics.DefaultRSSAlertBytes, nil)
	prom := metrics.NewProm(metrics.PromConfig{SolveResources: obs.SolveResources})

	l := causeLoop(newFakeQueue(makeEntry("memkill-1")), &fakeSolver{
		solveScript: func(*ingest.Entry) (*solver.Solution, error) {
			// The exact shape classifyExecError produces under
			// -solve-mem-mb 128 (solver/piagent.go).
			return nil, errors.New("solve killed by --solve-mem-mb 128 (RLIMIT_AS) after 1.2s: pi-agent exec: exit status 137: signal: killed")
		},
	}, obs)
	l.cfg.RSSSampler = sampler

	if err := l.Tick(context.Background()); err == nil {
		t.Fatal("Tick err = nil, want the memory-kill failure")
	}

	res := obs.SolveResources()
	if res.Killed != 1 {
		t.Errorf("SolveResources().Killed = %d, want 1 (a killed solve returns no solution)", res.Killed)
	}
	if res.PeakRSSCount != 1 {
		t.Errorf("PeakRSSCount = %d, want 1", res.PeakRSSCount)
	}
	if inf := res.PeakRSSBucketCounts[len(res.PeakRSSBucketCounts)-1]; inf != 1 {
		t.Errorf("+Inf bucket = %d, want 1", inf)
	}
	if got := res.Failures[solver.CauseSolverExit]; got != 1 {
		t.Errorf("failure cause solver_exit = %d, want 1", got)
	}
	if got := res.Failures[solver.CauseTimeout]; got != 0 {
		t.Errorf("failure cause timeout = %d, want 0 — a memory kill is not a timeout", got)
	}

	body := scrapeProm(t, prom)
	if v := promSample(t, body, "obo_solve_killed_total"); v != 1 {
		t.Errorf("obo_solve_killed_total = %v, want 1", v)
	}
	// +Inf bucket count == solve count (acceptance criterion 3's invariant).
	if v := promSample(t, body, `obo_solve_peak_rss_bytes_bucket{le="+Inf"}`); v != 1 {
		t.Errorf(`obo_solve_peak_rss_bytes_bucket{le="+Inf"} = %v, want 1`, v)
	}
	if v := promSample(t, body, "obo_solve_peak_rss_bytes_count"); v != 1 {
		t.Errorf("obo_solve_peak_rss_bytes_count = %v, want 1", v)
	}
	if v := promSample(t, body, "obo_solve_peak_rss_bytes_sum"); v != float64(peak) {
		t.Errorf("obo_solve_peak_rss_bytes_sum = %v, want %v", v, float64(peak))
	}
}

// TestIsMemoryKillAttribution pins the precedence between timeout and
// memory kills: both die by SIGKILL, but only a solve the memory cap
// killed may increment obo_solve_killed_total (OBS-OB-003). A live
// probe caught the blunt "signal: killed" proxy counting a forced
// --solve-timeout failure as a memory kill.
func TestIsMemoryKillAttribution(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{
			"solve timeout kill is NOT a memory kill",
			errors.New("solve killed by --solve-timeout 4s: pi-agent exec: signal: killed (stdout=)"),
			false,
		},
		{
			"bwrap timeout kill is NOT a memory kill",
			errors.New("solve killed by bwrap timeout 20s: pi-agent exec: exit status -1 (stdout=)"),
			false,
		},
		{
			"deadline exceeded is NOT a memory kill",
			errors.New("solve: pi-agent exec: context deadline exceeded"),
			false,
		},
		{
			"memory cap attribution IS a memory kill",
			errors.New("solve killed by --solve-mem-mb 128 (RLIMIT_AS) after 1.2s: pi-agent exec: exit status 137: signal: killed"),
			true,
		},
		{
			"memory cap attribution wins over a non-signal exit shape",
			errors.New("solve killed by --solve-mem-mb 128 (RLIMIT_AS) after 1.2s: pi-agent exec: exit status 1"),
			true,
		},
		{"bare SIGKILL (kernel OOM killer)", errors.New("pi-agent exec: signal: killed"), true},
		{"bare SIGSEGV", errors.New("pi-agent exec: signal: segmentation fault"), true},
		{"ordinary exit status 1", errors.New("pi-agent exec: exit status 1"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isMemoryKill(tc.err); got != tc.want {
				t.Fatalf("isMemoryKill(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestTimeoutKillDoesNotCountAsMemoryKill closes the loop between the
// two counters: a forced timeout raises cause="timeout" by exactly 1 and
// leaves obo_solve_killed_total untouched.
func TestTimeoutKillDoesNotCountAsMemoryKill(t *testing.T) {
	obs := metrics.New(metrics.DefaultRSSAlertBytes, nil)
	prom := metrics.NewProm(metrics.PromConfig{SolveResources: obs.SolveResources})

	l := causeLoop(newFakeQueue(makeEntry("timeout-2")), &fakeSolver{
		solveScript: func(*ingest.Entry) (*solver.Solution, error) {
			return nil, errors.New("solve killed by --solve-timeout 4s: pi-agent exec: signal: killed (stdout=)")
		},
	}, obs)
	if err := l.Tick(context.Background()); err == nil {
		t.Fatal("Tick err = nil, want the solve failure")
	}

	body := scrapeProm(t, prom)
	if v := promSample(t, body, `obo_solve_failure_total{cause="timeout"}`); v != 1 {
		t.Errorf("failure_total{cause=timeout} = %v, want 1", v)
	}
	if v := promSample(t, body, "obo_solve_killed_total"); v != 0 {
		t.Errorf("obo_solve_killed_total = %v, want 0 — a timeout kill is not a memory kill", v)
	}
	if got := obs.SolveResources().Killed; got != 0 {
		t.Errorf("Killed = %d, want 0", got)
	}
	// The peak is still observed: the histogram counts every solve.
	if v := promSample(t, body, "obo_solve_peak_rss_bytes_count"); v != 1 {
		t.Errorf("obo_solve_peak_rss_bytes_count = %v, want 1", v)
	}
}

// TestCommitFailureRaisesCauseCommit pins the stage-authoritative
// classification: any Commit error is cause="commit", whatever its text.
func TestCommitFailureRaisesCauseCommit(t *testing.T) {
	obs := metrics.New(metrics.DefaultRSSAlertBytes, nil)
	l := causeLoop(newFakeQueue(makeEntry("commit-1")), &fakeSolver{
		commitScript: func(*solver.Solution) (int64, error) {
			return 0, errors.New("graph: constraint failed: pi timed out after 25")
		},
	}, obs)

	if err := l.Tick(context.Background()); err == nil {
		t.Fatal("Tick err = nil, want the commit failure")
	}
	res := obs.SolveResources()
	if got := res.Failures[solver.CauseCommit]; got != 1 {
		t.Errorf("failure cause commit = %d, want 1 (stage wins over the misleading text)", got)
	}
	if got := res.Failures[solver.CauseTimeout]; got != 0 {
		t.Errorf("failure cause timeout = %d, want 0", got)
	}
}

// TestStaleReapCountsStaleReaped proves the reaper's batch accounting:
// reaped entries never reach processOne, so without this arm the
// stale_reaped cause would sit at zero forever.
func TestStaleReapCountsStaleReaped(t *testing.T) {
	obs := metrics.New(metrics.DefaultRSSAlertBytes, nil)
	q := newFakeQueue() // empty queue: the tick reaps and idles
	q.reapRows = 2
	l := causeLoop(q, &fakeSolver{}, obs)

	if err := l.Tick(context.Background()); err != nil {
		t.Fatalf("Tick err = %v, want nil (empty queue is not an error)", err)
	}
	if got := obs.SolveResources().Failures[solver.CauseStaleReaped]; got != 2 {
		t.Errorf("failure cause stale_reaped = %d, want 2 (one per reaped entry)", got)
	}

	// A second tick with nothing to reap must not double count.
	q.reapRows = 0
	if err := l.Tick(context.Background()); err != nil {
		t.Fatalf("second Tick err = %v", err)
	}
	if got := obs.SolveResources().Failures[solver.CauseStaleReaped]; got != 2 {
		t.Errorf("failure cause stale_reaped = %d after a zero reap, want 2", got)
	}
}

// TestRejectionCountsRejectedAndCauseOther pins the two-sided accounting
// of a harness rejection: the dedicated rejection counter moves AND the
// closed cause set records the failure as "other" (the spec's seven
// values have no rejection label, and the set must stay closed).
func TestRejectionCountsRejectedAndCauseOther(t *testing.T) {
	const peak = uint64(1) << 30

	sampler := &stubSampler{byPid: map[int]uint64{os.Getpid(): peak}}
	saved := childPIDsFn
	childPIDsFn = func(int) []int { return nil }
	defer func() { childPIDsFn = saved }()

	obs := metrics.New(metrics.DefaultRSSAlertBytes, nil)
	l := causeLoop(newFakeQueue(makeEntry("rej-1")), &fakeSolver{}, obs)
	l.cfg.RSSSampler = sampler
	l.cfg.SolveRejectRSSBytes = peak / 2 // 512 MiB threshold

	if err := l.Tick(context.Background()); !errors.Is(err, ErrSolveRejected) {
		t.Fatalf("Tick err = %v, want ErrSolveRejected", err)
	}
	res := obs.SolveResources()
	if res.Rejected != 1 {
		t.Errorf("Rejected = %d, want 1", res.Rejected)
	}
	if got := res.Failures[solver.CauseOther]; got != 1 {
		t.Errorf("failure cause other = %d, want 1", got)
	}
	if res.Killed != 0 {
		t.Errorf("Killed = %d, want 0 (rejected, not killed)", res.Killed)
	}
}
