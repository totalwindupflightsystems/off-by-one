// Package solver integrates the Pi Agent (TypeScript CLI) into the
// off-by-one pre-solve lab. The solver prepares a structured
// problem.json, hands it to Pi Agent inside a bwrap sandbox, and
// parses the solution.md + evidence.md + signatures.json output back
// into the graph store.
//
// The interface between this package and the sandbox is the
// Runner abstraction — concrete implementation BSandboxRunner wraps
// a *sandbox.Sandbox and *sandbox.Executor; tests use a fake
// filesystem-based runner that doesn't need bwrap or pi-agent
// installed. See piagent_test.go for the fake.
package solver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
)

// Default model is the spec-recommended fast DeepSeek tier.
const DefaultModel = "deepseek-v4-flash"

// Default timeout for a single solve. Pi Agent can take a while
// (search → fetch → reflect → answer). Complex problems (Raft, B-tree,
// distributed systems) need more time. 10 minutes matches the wrapper's
// execSync limit and gives models room to work through hard problems.
const DefaultSolveTimeout = 30 * time.Minute

// Config controls how the solver invokes Pi Agent. Zero values are
// filled in by ResolveConfig before Solve is called.
type Config struct {
	// PiAgentPath is the absolute path to the pi-agent binary.
	// Required — empty triggers ErrPiAgentNotFound at Solve time.
	PiAgentPath string

	// Model is the model identifier passed via --model. Defaults
	// to DefaultModel when empty.
	Model string

	// APIKey is the LLM API key used to authenticate with the
	// upstream model provider. The solver propagates it as
	// DEEPSEEK_API_KEY (and the generic LLM_API_KEY) for Pi Agent's
	// environment.
	APIKey string

	// Timeout is the wall-clock cap for a single solve. Defaults
	// to DefaultSolveTimeout when zero.
	Timeout time.Duration

	// MemLimitMB is the per-solve memory cap in MiB applied inside
	// the sandbox as RLIMIT_AS (the daemon wires it from
	// --solve-mem-mb → sandbox.Executor.MemLimitMB). 0 means
	// unlimited. The solver does not enforce it — the sandbox's
	// oby-memcap wrapper does — but Solve uses the value to
	// classify a SIGKILL/SIGSEGV death as "killed by the memory
	// cap" instead of leaving the operator with a bare
	// "signal: killed" (DF-OFF-BY-ONE-46).
	MemLimitMB int

	// SandboxTimeout is the bwrap outer wall-clock cap configured
	// on the sandbox executor (OB1_BWRAP_TIMEOUT). 0 means unknown
	// — in that case Solve never attributes a kill to the bwrap
	// timeout. Used only for kill-reason classification; the cap
	// itself is enforced by the sandbox.
	SandboxTimeout time.Duration

	// ExtraEnv is appended to the sandbox's environment.
	ExtraEnv []string
}

// ResolveConfig returns a copy of cfg with zero values replaced by
// defaults.
func ResolveConfig(cfg Config) Config {
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultSolveTimeout
	}
	return cfg
}

// Problem is the structured input the solver hands to Pi Agent.
// Field tags match the JSON shape written to problem.json.
type Problem struct {
	ProblemClass  string         `json:"problem_class"`
	Environment   string         `json:"environment"`
	Language      string         `json:"language"`
	Version       string         `json:"version"`
	Description   string         `json:"description"`
	ErrorMessage  string         `json:"error_message"`
	StackTrace    string         `json:"stack_trace"`
	Context       map[string]any `json:"context"`
	RequiredTools []string       `json:"required_tools,omitempty"`
}

// Solution is the structured output Pi Agent produces. Pi Agent
// writes solution.md (free-form markdown), evidence.md (free-form
// markdown), and signatures.json (structured metadata) to the
// sandbox workspace; ParseResult reads them back into this type.
type Solution struct {
	SolutionMarkdown string            `json:"solution"`
	EvidenceMarkdown string            `json:"evidence"`
	Signatures       map[string]any    `json:"signatures"`
	Model            string            `json:"model"`
	DurationMS       int64             `json:"duration_ms"`
	Raw              map[string]string `json:"raw,omitempty"`
}

// Result is the stored form of a Solution — what the graph and
// queue accept. It is also returned from Solve so callers don't
// have to re-build it.
type Result struct {
	AnswerID int64
	Solution Solution
}

// ErrPiAgentNotFound is returned when Config.PiAgentPath is empty or
// the binary does not exist.
var ErrPiAgentNotFound = errors.New("solver: pi-agent binary not found")

// ErrSolutionMissing is returned when Pi Agent didn't write the
// expected output files.
var ErrSolutionMissing = errors.New("solver: solution.md missing after solve")

// ErrEvidenceMissing is returned when Pi Agent didn't write
// evidence.md. We treat this as a soft failure — the solution may
// still be useful, but the caller's Commit helper will refuse to
// store a Solution with empty Evidence unless the caller overrides.
var ErrEvidenceMissing = errors.New("solver: evidence.md missing after solve")

// Runner abstracts the sandbox so the solver can be unit-tested
// without a real bwrap binary. The contract:
//
//   - Create returns a Handle whose workspace is empty. Per-solve
//     options (e.g. required tools) may be passed as variadic
//     CreateOption values.
//   - WriteFile writes data to relPath inside the workspace.
//   - Exec runs cmd with the given env and returns combined
//     stdout+stderr in Output. A non-zero exit becomes an error
//     wrapping *exec.ExitError when possible.
//   - ReadFile returns the contents of relPath or an error.
//   - Destroy releases all resources.
type Runner interface {
	Create(ctx context.Context, id string, opts ...CreateOption) (Handle, error)
}

// CreateOption configures per-sandbox behaviour at Create time.
type CreateOption func(*createConfig)

// createConfig holds the resolved options for a single Create call.
type createConfig struct {
	requiredTools []string
}

// WithRequiredTools declares tools that must be available inside the
// sandbox workspace. The concrete runner (BSandboxRunner) resolves
// each tool name to host paths and mounts them read-only; test fakes
// may ignore the option entirely.
func WithRequiredTools(tools []string) CreateOption {
	return func(c *createConfig) {
		c.requiredTools = tools
	}
}

// resolveCreateOptions applies all opts to a fresh createConfig and
// returns the result. Used by concrete Runner implementations.
func resolveCreateOptions(opts []CreateOption) createConfig {
	var cfg createConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// Handle is one running sandbox instance. The solver writes the
// problem in, runs pi-agent, and reads the solution out.
type Handle interface {
	WriteFile(relPath string, data []byte) error
	ReadFile(relPath string) ([]byte, error)
	Exec(ctx context.Context, name string, args, env []string) (stdout []byte, err error)
	Destroy() error
}

// Executor wires the solver to a Runner. The zero value is not
// usable — construct one with NewExecutor.
type Executor struct {
	cfg    Config
	runner Runner
	store  *graph.Store
}

// NewExecutor returns an Executor bound to the given graph store
// and sandbox runner. The graph is used to persist the Solution on
// success; the runner is the sandbox abstraction.
func NewExecutor(cfg Config, runner Runner, store *graph.Store) *Executor {
	return &Executor{cfg: ResolveConfig(cfg), runner: runner, store: store}
}

// Solve runs one full solve cycle:
//
//  1. Create a sandbox workspace
//  2. Write problem.json into it
//  3. Spawn pi-agent inside the sandbox
//  4. Read solution.md / evidence.md / signatures.json back
//  5. Return the structured Solution (caller decides whether to
//     persist it via Commit)
//
// On any failure the sandbox is destroyed before returning. The
// caller can choose to skip Destroy when re-using the handle.
func (e *Executor) Solve(ctx context.Context, sub *ingest.Entry) (*Solution, error) {
	if e.cfg.PiAgentPath == "" {
		return nil, ErrPiAgentNotFound
	}
	resolvedPath, err := exec.LookPath(e.cfg.PiAgentPath)
	if err != nil {
		return nil, ErrPiAgentNotFound
	}
	if sub == nil {
		return nil, errors.New("solver: nil queue entry")
	}

	handle, err := e.runner.Create(ctx, sub.ID, WithRequiredTools(sub.RequiredTools))
	if err != nil {
		return nil, fmt.Errorf("create sandbox: %w", err)
	}
	defer func() { _ = handle.Destroy() }()

	// Translate the queue entry into the structured problem that
	// pi-agent understands.
	problem := Problem{
		ProblemClass:  sub.ProblemClass,
		Environment:   sub.Environment,
		Language:      sub.Language,
		Version:       sub.Version,
		Description:   sub.Description,
		ErrorMessage:  sub.ErrorMessage,
		StackTrace:    sub.StackTrace,
		Context:       sub.Context,
		RequiredTools: sub.RequiredTools,
	}
	problemBytes, err := json.MarshalIndent(problem, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal problem: %w", err)
	}
	if err := handle.WriteFile("problem.json", problemBytes); err != nil {
		return nil, fmt.Errorf("write problem.json: %w", err)
	}

	// Build the command and the env. Authentication is supplied via
	// DEEPSEEK_API_KEY / LLM_API_KEY env vars (pi-agent 0.84.0
	// reads them); we intentionally do NOT pass it as a CLI flag so
	// it is not visible in process listings. Extra env vars come last so
	// they win over our defaults when both define the same key.
	args := []string{
		"solve",
		"--problem-file", "/workspace/problem.json",
		"--output", "/workspace/solution.md",
		"--evidence", "/workspace/evidence.md",
		"--signatures", "/workspace/signatures.json",
		"--model", e.cfg.Model,
	}
	env := append([]string{}, e.cfg.ExtraEnv...)
	if e.cfg.APIKey != "" {
		env = append(env,
			"DEEPSEEK_API_KEY="+e.cfg.APIKey,
			"LLM_API_KEY="+e.cfg.APIKey,
		)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, e.cfg.Timeout)
	defer cancel()

	start := time.Now()
	stdout, err := handle.Exec(timeoutCtx, resolvedPath, args, env)
	duration := time.Since(start)

	if err != nil {
		// Return the stdout too so the caller can log it — pi-agent
		// tends to be verbose on failure and the raw output is the
		// most useful diagnostic. classifyExecError names the
		// configured limit when one fired (DF-OFF-BY-ONE-46).
		return nil, e.classifyExecError(err, stdout, timeoutCtx, duration)
	}

	// Parse the result. Missing files are errors — we want the
	// caller to mark the queue entry failed rather than store an
	// empty Solution that pollutes the graph.
	solMD, err := handle.ReadFile("solution.md")
	if err != nil {
		return nil, ErrSolutionMissing
	}
	evMD, err := handle.ReadFile("evidence.md")
	if err != nil {
		// Evidence is informative, not load-bearing. We log via
		// the returned error and continue with empty evidence —
		// callers can decide to fail or commit anyway.
		evMD = nil
	}
	sigJSON, _ := handle.ReadFile("signatures.json")

	sol := Solution{
		SolutionMarkdown: string(solMD),
		EvidenceMarkdown: string(evMD),
		DurationMS:       duration.Milliseconds(),
		Model:            e.cfg.Model,
	}
	if len(sigJSON) > 0 {
		var sigs map[string]any
		if err := json.Unmarshal(sigJSON, &sigs); err == nil {
			sol.Signatures = sigs
		}
	}
	sol.Raw = map[string]string{
		"stdout": truncate(string(stdout), 8192),
	}
	return &sol, nil
}

// killClassifyGrace is the tolerance added when comparing the measured
// solve duration against the bwrap outer timeout: the sandbox starts its
// own clock a few milliseconds after Solve starts ours, and timer
// scheduling can shave off a few more.
const killClassifyGrace = 250 * time.Millisecond

// classifyExecError wraps a failed pi-agent Exec in the standard
// "pi-agent exec" error and, when the death matches one of the
// configured limits, prefixes the limit's name so the queue entry's
// failure_reason tells the operator WHICH cap fired instead of a bare
// "signal: killed" / "exit status 139" (DF-OFF-BY-ONE-46).
//
// Precedence matters:
//
//  1. --solve-timeout: our own context fired. CommandContext kills the
//     child with SIGKILL on deadline, so this check must come before
//     any signal-based classification.
//  2. bwrap timeout: the child died by SIGKILL/SIGSEGV at or after the
//     sandbox's outer cap while our context stayed live.
//  3. --solve-mem-mb: the child died by SIGKILL/SIGSEGV well before
//     every configured timeout with RLIMIT_AS armed. A genuine
//     pi-agent segfault under a configured cap is attributed to the
//     cap too — acceptable: the hint names the knob to check, and
//     under RLIMIT_AS SIGSEGV is the common OOM manifestation.
//
// Anything else returns the historical unclassified error unchanged.
func (e *Executor) classifyExecError(execErr error, stdout []byte, timeoutCtx context.Context, duration time.Duration) error {
	orig := fmt.Errorf("pi-agent exec: %w (stdout=%s)", execErr, truncate(string(stdout), 4096))
	if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("solve killed by --solve-timeout %s: %w", e.cfg.Timeout, orig)
	}
	if killedByLimitSignal(execErr) {
		if e.cfg.SandboxTimeout > 0 && duration+killClassifyGrace >= e.cfg.SandboxTimeout {
			return fmt.Errorf("solve killed by bwrap timeout %s: %w", e.cfg.SandboxTimeout, orig)
		}
		if e.cfg.MemLimitMB > 0 {
			return fmt.Errorf("solve killed by --solve-mem-mb %d (RLIMIT_AS) after %s: %w",
				e.cfg.MemLimitMB, duration.Round(time.Millisecond), orig)
		}
	}
	return orig
}

// killedByLimitSignal reports whether err is a process death by SIGKILL
// or SIGSEGV — the two signals a configured limit produces (RLIMIT_AS
// enforcement, kernel OOM-kill, or a CommandContext deadline cancel).
// *exec.ExitError is the primary shape; a string fallback covers
// runners that return pre-rendered errors.
func killedByLimitSignal(err error) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			if ws.Signaled() {
				sig := ws.Signal()
				return sig == syscall.SIGKILL || sig == syscall.SIGSEGV
			}
			// Shell-shaped exit codes: 137 = 128+SIGKILL, 139 = 128+SIGSEGV.
			code := ws.ExitStatus()
			return code == 137 || code == 139
		}
	}
	msg := err.Error()
	return strings.Contains(msg, "signal: killed") ||
		strings.Contains(msg, "signal: segmentation fault") ||
		strings.Contains(msg, "exit status 137") ||
		strings.Contains(msg, "exit status 139")
}

// answerStatusFromSignatures derives the status to store for a newly
// committed answer from the solve's own signature map.
//
// Invariant: a solve the validator marked failed must never be stored
// as verified. The lab advertises pre-verified answers, so a failed
// solve stored as verified is served by discovery as an answer and
// suppresses re-submission of that class via queue dedup (OB-GAP-057).
//
// Every other verdict keeps the historical verified status: "passed",
// "completed", a non-string result, and legacy signature maps with no
// result key at all.
func answerStatusFromSignatures(sigs map[string]any) string {
	if res, ok := sigs["result"].(string); ok && res == graph.AnswerFailed {
		return graph.AnswerFailed
	}
	return graph.AnswerVerified
}

// Commit persists a Solution into the graph: upsert the problem
// class by title, then create a new answer node. Returns the new
// answer ID.
//
// Signatures are stored as a JSON blob (the graph column is TEXT).
// Empty signature maps become "{}" so the column is never NULL.
//
// The stored status is the solve's own verdict (see
// answerStatusFromSignatures) — never a blanket verified stamp.
func (e *Executor) Commit(ctx context.Context, sub *ingest.Entry, sol *Solution) (int64, error) {
	if sol == nil {
		return 0, errors.New("solver: nil solution")
	}
	if sol.SolutionMarkdown == "" {
		return 0, errors.New("solver: empty solution.md")
	}
	// Prefer the queue entry's problem class over the extracted one
	// so discover queries match what was submitted.
	title := sub.ProblemClass
	if title == "" {
		title, _ = extractProblemClass(sol)
	}
	description, _ := extractProblemClass(sol)
	class, _, err := e.store.UpsertProblemClass(ctx, title, description)
	if err != nil {
		return 0, fmt.Errorf("upsert problem_class: %w", err)
	}

	sigs := "{}"
	if len(sol.Signatures) > 0 {
		b, err := json.Marshal(sol.Signatures)
		if err != nil {
			return 0, fmt.Errorf("marshal signatures: %w", err)
		}
		sigs = string(b)
	}

	// env/lang/version are owned by the submission entry.
	// Use the submission's metadata first; fall back to
	// signatures only when the submission doesn't provide them
	// (legacy entries or admin-injected problems).
	env, lang, version := sub.Environment, sub.Language, sub.Version
	if env == "" {
		env, lang, version = extractEnvFromSigs(sol)
	}
	answerID, err := e.store.CreateAnswerNode(ctx,
		class.ID, 0, env, lang, version,
		sol.SolutionMarkdown, sol.EvidenceMarkdown, sigs)
	if err != nil {
		return 0, fmt.Errorf("create answer_node: %w", err)
	}
	if err := e.store.UpdateAnswerStatus(ctx, answerID, answerStatusFromSignatures(sol.Signatures)); err != nil {
		return 0, fmt.Errorf("update answer status: %w", err)
	}
	return answerID, nil
}

// extractProblemClass reads the problem_class from signatures, or
// falls back to "unknown" when missing. Pi Agent is expected to
// include {"problem_class": "...", "description": "..."} in its
// signatures.json output.
func extractProblemClass(sol *Solution) (string, string) {
	if sol == nil {
		return "unknown", ""
	}
	if pc, ok := sol.Signatures["problem_class"].(string); ok && pc != "" {
		desc, _ := sol.Signatures["description"].(string)
		return pc, desc
	}
	return "unknown", firstParagraph(sol.SolutionMarkdown)
}

// extractEnvFromSigs returns the env/lang/version triple from the
// signatures. Defaults to "docker"/"go"/"latest" when missing so
// the answer is always addressable.
func extractEnvFromSigs(sol *Solution) (string, string, string) {
	env, lang, version := "docker", "go", "latest"
	if sol == nil {
		return env, lang, version
	}
	if v, ok := sol.Signatures["environment"].(string); ok && v != "" {
		env = v
	}
	if v, ok := sol.Signatures["language"].(string); ok && v != "" {
		lang = v
	}
	if v, ok := sol.Signatures["version"].(string); ok && v != "" {
		version = v
	}
	return env, lang, version
}

// firstParagraph returns the first non-empty line of markdown,
// trimmed. Used to seed a description when signatures don't
// provide one.
func firstParagraph(md string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) > 200 {
			return line[:200]
		}
		return line
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…[truncated]"
}
