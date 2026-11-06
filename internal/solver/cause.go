package solver

import "strings"

// Failure-cause vocabulary for obo_solve_failure_total (OBS-OB-003,
// docs/observability-spec.md §3.3).
//
// This is a CLOSED set of seven values: ClassifyCause never returns
// anything outside it, so no amount of free-text solver output can grow
// the label cardinality of the failure series. Everything the lab cannot
// attribute to a known class lands in CauseOther — which is honest and,
// unlike an unlabelled counter, still alertable ("failures with an
// unknown cause are climbing").
const (
	// CauseTimeout — the solve was killed by --solve-timeout or by the
	// bwrap sandbox timeout. Largest historical class ("pi timed out
	// after 25", 112 rows).
	CauseTimeout = "timeout"
	// CauseSolverExit — the solver process ended abnormally without a
	// more specific class: a non-zero exit, SIGKILL from the per-solve
	// memory cap, SIGSEGV, etc. ("pi exited with code 1", 40 rows).
	CauseSolverExit = "solver_exit"
	// CauseToolchainMissing — the host could not provide the sandbox /
	// solver toolchain: bwrap source paths, the memcap wrapper, or the
	// pi-agent binary itself ("bwrap: Can't find source path", 29 rows).
	CauseToolchainMissing = "toolchain_missing"
	// CauseInvalidProblem — the submission or the solver's output did not
	// satisfy the problem/solution contract ("invalid problem file",
	// 18 rows).
	CauseInvalidProblem = "invalid_problem"
	// CauseCommit — persisting a successful solve into the graph failed
	// (Solver.Commit). Classified from the queue stage, never from text:
	// Commit only touches the graph, so no provider error can originate
	// there.
	CauseCommit = "commit"
	// CauseStaleReaped — an in_progress entry outlived its solve window
	// and was reaped as orphaned (server restart / crash mid-solve).
	CauseStaleReaped = "stale_reaped"
	// CauseOther — attributed to no known class. Also the home of the
	// harness-side rejection (peak RSS above the reject threshold): that
	// solve has dedicated counters (obo_solve_rejected_total and
	// obo_solves_total{outcome="rejected"}), and the cause set is closed,
	// so it must not invent an eighth label.
	CauseOther = "other"
)

// Queue stage names the loop stamps before/while a solve is classified
// (sql/schema/queue.sql stage column). Only the outcomes that produce a
// failure cause are named here; the progress stages (solver_running,
// committing) are not part of the failure vocabulary.
const (
	StageSolverFailed   = "solver_failed"
	StageSolverRejected = "solver_rejected"
	StageCommitFailed   = "commit_failed"
	StageStaleReaped    = "stale_reaped"
)

// FailureCauses is the closed cause vocabulary in spec order. Exported so
// the exposition can pre-create every series at zero (a missing series is
// invisible to rate(); a zero series is not) and so tests can assert the
// set itself, not just individual values.
var FailureCauses = []string{
	CauseTimeout,
	CauseSolverExit,
	CauseToolchainMissing,
	CauseInvalidProblem,
	CauseCommit,
	CauseStaleReaped,
	CauseOther,
}

// IsFailureCause reports whether s is one of the seven closed values.
func IsFailureCause(s string) bool {
	for _, c := range FailureCauses {
		if s == c {
			return true
		}
	}
	return false
}

// timeoutSignatures are matched before anything else: a solve killed by
// --solve-timeout also carries "pi-agent exec:" text (it wraps the kill
// error), so ordering is what keeps it out of solver_exit.
var timeoutSignatures = []string{
	"solve killed by --solve-timeout",
	"solve killed by bwrap timeout",
	"timed out",
	"timeout",
	"deadline exceeded",
}

// toolchainSignatures name a host-side provisioning failure, not a solve
// failure: the sandbox or the agent binary the host was supposed to
// provide is absent (ErrBwrapNotFound, ErrPiAgentNotFound, the missing
// oby-memcap wrapper, and bwrap's own missing-source-path error).
var toolchainSignatures = []string{
	"can't find source path",
	"not found at configured path",
	"pi-agent binary not found",
	"wrapper not found",
	"executable file not found",
}

// invalidProblemSignatures name a contract violation on the problem or
// solution side (bad problem payload, missing solution.md).
var invalidProblemSignatures = []string{
	"invalid problem",
	"solution.md missing",
}

// solverExitSignatures are the abnormal-termination shapes left over
// once the specific classes above are excluded.
var solverExitSignatures = []string{
	"pi-agent exec",
	"exited with code",
	"exit status",
	"signal: killed",
	"signal: segmentation fault",
	"signal: aborted",
}

// ClassifyCause maps a failed queue entry's stage + failure_reason onto
// the closed seven-value cause vocabulary.
//
// Classification happens at the failure site (cron/loop.go) with the raw
// error in hand, not at scrape time by re-parsing stored text, so a
// scrape can neither change history nor cost a scan of queue_entries.
//
// Precedence: stage first (it is structured and authoritative — a
// commit_failed row is a commit failure whatever its message says), then
// the reason text in specificity order. Unknown or empty text is
// CauseOther; the return value is always a member of FailureCauses.
func ClassifyCause(stage, reason string) string {
	switch stage {
	case StageCommitFailed:
		return CauseCommit
	case StageStaleReaped:
		return CauseStaleReaped
	case StageSolverRejected:
		return CauseOther
	}

	text := strings.ToLower(reason)
	if strings.TrimSpace(text) == "" {
		return CauseOther
	}
	for _, group := range []struct {
		cause      string
		signatures []string
	}{
		{CauseTimeout, timeoutSignatures},
		{CauseToolchainMissing, toolchainSignatures},
		{CauseInvalidProblem, invalidProblemSignatures},
		{CauseSolverExit, solverExitSignatures},
	} {
		for _, sig := range group.signatures {
			if strings.Contains(text, sig) {
				return group.cause
			}
		}
	}
	return CauseOther
}
