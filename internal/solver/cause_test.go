package solver

import (
	"strings"
	"testing"
)

// TestFailureCauseVocabularyIsClosed pins the label set itself: the
// exposition, the docs (observability-spec §3.3) and every alert built
// on obo_solve_failure_total depend on these exact seven values.
func TestFailureCauseVocabularyIsClosed(t *testing.T) {
	want := []string{
		"timeout",
		"solver_exit",
		"toolchain_missing",
		"invalid_problem",
		"commit",
		"stale_reaped",
		"other",
	}
	if len(FailureCauses) != len(want) {
		t.Fatalf("FailureCauses has %d values (%v), want %d", len(FailureCauses), FailureCauses, len(want))
	}
	for i, w := range want {
		if FailureCauses[i] != w {
			t.Errorf("FailureCauses[%d] = %q, want %q (spec order)", i, FailureCauses[i], w)
		}
		if !IsFailureCause(w) {
			t.Errorf("IsFailureCause(%q) = false", w)
		}
	}
	for _, not := range []string{"", "Timeout", "oom", "solver-error", "other\n"} {
		if IsFailureCause(not) {
			t.Errorf("IsFailureCause(%q) = true, want false", not)
		}
	}
}

// TestClassifyCause pins the classification of the failure classes that
// actually dominate the offline history (row OBS-OB-003 detail:
// timeout=112, exit1=40, bwrap_path=29, invalid_problem=18,
// stale_reaped=6) plus the stage-authoritative cases.
func TestClassifyCause(t *testing.T) {
	cases := []struct {
		name   string
		stage  string
		reason string
		want   string
	}{
		{
			name:   "solve-timeout kill (wraps the exec error)",
			stage:  StageSolverFailed,
			reason: "solve: solve killed by --solve-timeout 25s: pi-agent exec: signal: killed (stdout=)",
			want:   CauseTimeout,
		},
		{
			name:   "bwrap timeout kill",
			stage:  StageSolverFailed,
			reason: "solve killed by bwrap timeout 20s: pi-agent exec: exit status -1 (stdout=)",
			want:   CauseTimeout,
		},
		{
			name:   "legacy baseline text",
			stage:  StageSolverFailed,
			reason: "pi timed out after 25",
			want:   CauseTimeout,
		},
		{
			name:   "context deadline",
			stage:  StageSolverFailed,
			reason: "solve: pi-agent exec: context deadline exceeded",
			want:   CauseTimeout,
		},
		{
			name:   "plain non-zero exit",
			stage:  StageSolverFailed,
			reason: "pi exited with code 1",
			want:   CauseSolverExit,
		},
		{
			name:   "exec exit status",
			stage:  StageSolverFailed,
			reason: "solve: pi-agent exec: exit status 1 (stdout=boom)",
			want:   CauseSolverExit,
		},
		{
			name:   "memory cap kill is an abnormal solver exit",
			stage:  StageSolverFailed,
			reason: "solve killed by --solve-mem-mb 128 (RLIMIT_AS) after 1.2s: pi-agent exec: exit status 137: signal: killed",
			want:   CauseSolverExit,
		},
		{
			name:   "bwrap missing source path",
			stage:  StageSolverFailed,
			reason: "sandbox: bwrap: Can't find source path /tmp/obo-ws-1: No such file or directory",
			want:   CauseToolchainMissing,
		},
		{
			name:   "pi-agent binary missing",
			stage:  StageSolverFailed,
			reason: "solver: pi-agent binary not found",
			want:   CauseToolchainMissing,
		},
		{
			name:   "bwrap binary missing",
			stage:  StageSolverFailed,
			reason: "sandbox: bwrap not found at configured path",
			want:   CauseToolchainMissing,
		},
		{
			name:   "memcap wrapper missing",
			stage:  StageSolverFailed,
			reason: "sandbox: oby-memcap wrapper not found (looked next to the daemon binary and on PATH)",
			want:   CauseToolchainMissing,
		},
		{
			name:   "invalid problem file",
			stage:  StageSolverFailed,
			reason: "invalid problem file: required_tools is not a JSON array",
			want:   CauseInvalidProblem,
		},
		{
			name:   "solution artifact missing",
			stage:  StageSolverFailed,
			reason: "solver: solution.md missing after solve",
			want:   CauseInvalidProblem,
		},
		{
			name:   "provider guardrail block is not a known class",
			stage:  StageSolverFailed,
			reason: "spec: No endpoints available matching your guardrail restrictions and data policy.",
			want:   CauseOther,
		},
		{
			name:   "empty reason",
			stage:  StageSolverFailed,
			reason: "",
			want:   CauseOther,
		},
		{
			name:   "whitespace reason",
			stage:  StageSolverFailed,
			reason: "   \n\t ",
			want:   CauseOther,
		},
		{
			name:   "stage commit_failed wins over timeout text",
			stage:  StageCommitFailed,
			reason: "constraint failed: pi timed out after 25",
			want:   CauseCommit,
		},
		{
			name:   "stage stale_reaped wins over empty reason",
			stage:  StageStaleReaped,
			reason: "",
			want:   CauseStaleReaped,
		},
		{
			name:   "stage solver_rejected maps to other",
			stage:  StageSolverRejected,
			reason: "rejected: peak RSS 5.000 GiB exceeds reject threshold 4.000 GiB",
			want:   CauseOther,
		},
		{
			name:   "unknown stage falls through to text",
			stage:  "some_future_stage",
			reason: "pi exited with code 2",
			want:   CauseSolverExit,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyCause(tc.stage, tc.reason)
			if got != tc.want {
				t.Fatalf("ClassifyCause(%q, %q) = %q, want %q", tc.stage, tc.reason, got, tc.want)
			}
			if !IsFailureCause(got) {
				t.Fatalf("ClassifyCause returned %q, outside the closed set", got)
			}
		})
	}
}

// TestClassifyCauseArbitraryStringsStayClosed is the acceptance-criterion
// test: for ARBITRARY solver error strings — including mixed-case,
// unicode, near-miss signatures, and every single-signature prefix — the
// returned cause never falls outside the seven-value set. Without this,
// one creative provider error would mint an unbounded label.
func TestClassifyCauseArbitraryStringsStayClosed(t *testing.T) {
	fragments := []string{
		"", " ", "\x00", "\n", "\t", "TIMEOUT", "Timeout:", "timed  out",
		"deadline", "exit", "exit status", "exit status 0", "signal:",
		"signal: killed", "signal: segmentation fault", "can't find source path",
		"Can't find source path", "not found", "invalid", "invalid problem",
		"rejected:", "commit failed", "stale", "pi-agent", "bwrap",
		"javascript: alert(1)", "../etc/passwd", "π≈3.14159", "错误",
		"a" + strings.Repeat("b", 4096),
	}
	prefixes := []string{"", "solve: ", "pi-agent exec: ", "sandbox: ", "solver: ", "PANIC "}
	stages := []string{"", StageSolverFailed, StageSolverRejected, StageCommitFailed, StageStaleReaped, "unknown"}
	for _, stage := range stages {
		for _, prefix := range prefixes {
			for _, frag := range fragments {
				reason := prefix + frag
				got := ClassifyCause(stage, reason)
				if !IsFailureCause(got) {
					t.Fatalf("ClassifyCause(%q, %q) = %q, outside the closed set", stage, reason, got)
				}
			}
		}
	}
}

// FuzzClassifyCause is the property arm of the same acceptance
// criterion. `go test` runs the seed corpus; `go test -fuzz` explores
// further. A failure here means a cause label escaped the closed set.
func FuzzClassifyCause(f *testing.F) {
	seeds := []struct{ stage, reason string }{
		{"", ""},
		{StageSolverFailed, "solve killed by --solve-timeout 25s: pi-agent exec: signal: killed"},
		{StageSolverFailed, "bwrap: Can't find source path /tmp/x"},
		{StageCommitFailed, "UNIQUE constraint failed"},
		{StageSolverRejected, "rejected: peak RSS 5 GiB"},
		{StageStaleReaped, ""},
		{"weird", "exit status 137: signal: killed"},
	}
	for _, s := range seeds {
		f.Add(s.stage, s.reason)
	}
	f.Fuzz(func(t *testing.T, stage, reason string) {
		got := ClassifyCause(stage, reason)
		if !IsFailureCause(got) {
			t.Fatalf("ClassifyCause(%q, %q) = %q, outside the closed set %v", stage, reason, got, FailureCauses)
		}
		// Idempotence: classifying twice yields the same label.
		if again := ClassifyCause(stage, reason); again != got {
			t.Fatalf("ClassifyCause is not deterministic: %q then %q", got, again)
		}
		// The label must be usable as a Prometheus label value: no
		// newline can ever be smuggled in to forge a second series.
		if strings.ContainsAny(got, "\n\r") {
			t.Fatalf("cause %q contains a line break", got)
		}
	})
}

// TestClassifyCauseAllSolverErrorsCovered keeps the signature table
// honest: every documented solver/sandbox error value in this package
// must classify to something precise (never silently "other") EXCEPT
// the documented catch-alls.
func TestClassifyCauseAllSolverErrorsCovered(t *testing.T) {
	for _, err := range []error{
		ErrPiAgentNotFound,
		ErrSolutionMissing,
	} {
		got := ClassifyCause(StageSolverFailed, err.Error())
		if !IsFailureCause(got) {
			t.Errorf("ClassifyCause(%q) = %q, outside the closed set", err.Error(), got)
		}
		if got == CauseOther {
			t.Errorf("ClassifyCause(%q) = other, want a precise class", err.Error())
		}
	}
	// Sanity: the classifier is not vacuous — distinct classes differ.
	if ClassifyCause(StageSolverFailed, ErrPiAgentNotFound.Error()) ==
		ClassifyCause(StageSolverFailed, "pi exited with code 1") {
		t.Error("toolchain and solver_exit classes collapse to the same label")
	}
}
