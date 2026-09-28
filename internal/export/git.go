// Package export implements the git subtree export engine that pushes
// verified Off-by-One answers into a target repository. The output layout
// follows spec §8.1:
//
//	pre-solve-answers/
//	  {problem-class}/
//	    {env}/
//	      {version}/
//	        solution.md
//	        evidence.md
//	        signatures.json
//
// The engine shells out to git (not git2go) to keep the dependency surface
// small and the logic auditable.
package export

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
)

// Config controls a single export operation. All paths are absolute or
// relative to the working directory of the calling process.
type Config struct {
	// RepoURL is the git remote to clone or pull. May be an SSH URL
	// (git@github.com:org/repo.git) or an HTTPS URL
	// (https://github.com/org/repo.git). For HTTPS with private repos,
	// embed a token: https://<token>@github.com/org/repo.git.
	RepoURL string

	// Branch is the target branch. Defaults to "main".
	Branch string

	// LocalDir is the working clone. If it already contains a .git
	// directory, the engine pulls instead of cloning. Otherwise it
	// creates the directory and clones into it.
	LocalDir string

	// SubtreePrefix is the directory inside the repo where answers are
	// written. Defaults to "pre-solve-answers" per spec §8.1.
	SubtreePrefix string

	// CommitAuthor overrides the git author for the export commit.
	// If empty, the repo's default identity is used.
	CommitAuthor string

	// CommitEmail overrides the git author email.
	CommitEmail string

	// CommitMessage overrides the export commit message verbatim. When
	// empty (or whitespace-only), the engine writes its default summary
	// message ("export: N pre-solve answers").
	CommitMessage string

	// Push controls whether the commit is pushed to the remote after
	// writing. Set to false for dry-run / preview mode.
	Push bool

	// GitPath is the path to the git binary. Defaults to "git".
	GitPath string
}

// ExportItem is one answer selected for export. The engine fetches the
// full ProblemClass and AnswerNode from the graph to build the file
// content, so callers only need to provide IDs.
type ExportItem struct {
	ClassID  int64
	AnswerID int64
}

// ExportResult summarises a completed (or dry-run) export.
type ExportResult struct {
	// CommitSHA is the short hash of the export commit. Empty in dry-run
	// mode or when there were no changes to commit.
	CommitSHA string

	// Branch is the branch the commit landed on.
	Branch string

	// FilesWritten lists every file path relative to the repo root.
	FilesWritten []string

	// ItemsExported is the number of answers written.
	ItemsExported int

	// ItemsSkipped lists answers that were skipped (e.g. already exported
	// with identical content) with a reason string.
	ItemsSkipped []SkipReason

	// Pushed is true if the push step ran and succeeded.
	Pushed bool

	// DryRun is true if no commit or push occurred.
	DryRun bool
}

// SkipReason records why a single answer was not written.
type SkipReason struct {
	AnswerID int64
	Reason   string
}

// Engine performs export operations against a git repository. It reads
// answers from a *graph.Store and writes formatted markdown + JSON files
// into a local clone, then commits (and optionally pushes).
type Engine struct {
	cfg   Config
	store *graph.Store

	// skipCommit is set by DryRun to suppress the commit step.
	skipCommit bool
}

// NewEngine returns an export Engine bound to the given config and graph
// store. The store is used to look up problem classes and answer nodes;
// the config controls git behaviour.
func NewEngine(cfg Config, store *graph.Store) *Engine {
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	if cfg.SubtreePrefix == "" {
		cfg.SubtreePrefix = "pre-solve-answers"
	}
	if cfg.GitPath == "" {
		cfg.GitPath = "git"
	}
	return &Engine{cfg: cfg, store: store}
}

// ErrNoItems is returned by Export when items is empty. Exporting zero
// answers is almost always a caller bug.
var ErrNoItems = errors.New("export: no items to export")

// ErrRepoMismatch is returned when LocalDir already contains a clone whose
// origin is a different repository than cfg.RepoURL. Reusing that clone
// would commit into (or push to) the wrong repository, so the engine
// refuses before any fetch, checkout or commit.
var ErrRepoMismatch = errors.New("export: RepoURL does not match the existing clone")

// ErrUnsafeExport is returned when a file the export is about to write
// holds content that is neither the fetched remote baseline nor identical
// to what this export would write — i.e. a community edit (an uncommitted
// modification or an unpushed commit in the clone) that the write would
// silently destroy. The engine refuses before any answer file is
// overwritten; only the exact target files are checked, never unrelated
// working-tree changes.
var ErrUnsafeExport = errors.New("export: refusing to overwrite community edits")

// Export runs the full export flow for the given items:
//
//  1. Prepare the local clone (clone or pull)
//  2. For each item: fetch class + answer, format files, write to disk
//  3. Stage all new/modified files
//  4. Commit (unless dry-run)
//  5. Push (unless dry-run or Push=false)
//
// The returned ExportResult contains the commit SHA, file list, and any
// skip reasons.
func (e *Engine) Export(ctx context.Context, items []ExportItem) (*ExportResult, error) {
	if len(items) == 0 {
		return nil, ErrNoItems
	}
	if e.cfg.RepoURL == "" {
		return nil, errors.New("export: RepoURL is required")
	}

	res := &ExportResult{Branch: e.cfg.Branch}

	// Step 1: ensure local clone exists and is up-to-date.
	if err := e.prepareClone(ctx); err != nil {
		return nil, fmt.Errorf("prepare clone: %w", err)
	}

	// Step 2: render each answer's files (no disk writes yet).
	var pending []renderedFile
	for _, item := range items {
		files, skip, err := e.renderItem(ctx, item)
		if err != nil {
			// Name the request field the caller controls: an item that
			// cannot be resolved is a request problem (a bogus
			// answer_ids entry), not an engine failure. Never leak the
			// ExportItem struct fields (class=/answer=) here — callers
			// cannot act on them.
			return nil, fmt.Errorf("invalid answer_ids: answer %d does not exist or its class is missing: %w", item.AnswerID, err)
		}
		if skip != nil {
			res.ItemsSkipped = append(res.ItemsSkipped, *skip)
			continue
		}
		for _, f := range files {
			res.FilesWritten = append(res.FilesWritten, f.relPath)
		}
		pending = append(pending, files...)
		res.ItemsExported++
	}

	// Step 3: nothing to do if no files were rendered.
	if len(pending) == 0 {
		return res, nil
	}

	// Step 4: fail closed. If any target file holds community edits the
	// write would destroy, refuse before touching disk (DF-OFF-BY-ONE-22).
	if err := e.checkNoForeignContent(ctx, pending); err != nil {
		return nil, err
	}

	// Step 5: write the rendered files.
	if err := e.writeRendered(pending); err != nil {
		return nil, err
	}

	// Step 6: stage, commit, push.
	if e.skipCommit {
		return res, nil
	}
	if err := e.stageAndCommit(ctx, res); err != nil {
		return nil, err
	}

	return res, nil
}

// DryRun performs steps 1–2 (clone/pull + file generation) but does NOT
// commit or push. The returned result has DryRun=true and FilesWritten
// populated so callers can preview what would be exported.
func (e *Engine) DryRun(ctx context.Context, items []ExportItem) (*ExportResult, error) {
	savedPush := e.cfg.Push
	e.cfg.Push = false
	e.skipCommit = true
	defer func() {
		e.cfg.Push = savedPush
		e.skipCommit = false
	}()

	res, err := e.Export(ctx, items)
	if err != nil {
		return nil, err
	}
	res.DryRun = true
	return res, nil
}

// --- Internal helpers ---------------------------------------------------

// prepareClone ensures LocalDir contains an up-to-date clone of RepoURL
// on the target branch. If LocalDir/.git exists, it pulls; otherwise it
// clones fresh.
func (e *Engine) prepareClone(ctx context.Context) error {
	gitDir := filepath.Join(e.cfg.LocalDir, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		// Existing clone — it must belong to the repo we were asked for.
		// Committing into a stale clone would push the wrong repo.
		origin, err := e.gitOutput(ctx, e.cfg.LocalDir, "remote", "get-url", "origin")
		if err != nil {
			return fmt.Errorf("read clone origin: %w", err)
		}
		if normalizeRepoURL(origin) != normalizeRepoURL(e.cfg.RepoURL) {
			return fmt.Errorf("%w: existing clone origin %q, requested RepoURL %q",
				ErrRepoMismatch, strings.TrimSpace(origin), e.cfg.RepoURL)
		}

		// Existing clone — fetch + checkout + pull.
		if err := e.runGit(ctx, e.cfg.LocalDir, "fetch", "origin"); err != nil {
			return fmt.Errorf("fetch origin: %w", err)
		}
		if err := e.runGit(ctx, e.cfg.LocalDir, "checkout", e.cfg.Branch); err != nil {
			// Branch may not exist yet — create it tracking origin.
			if err2 := e.runGit(ctx, e.cfg.LocalDir, "checkout", "-b", e.cfg.Branch,
				"origin/"+e.cfg.Branch); err2 != nil {
				return fmt.Errorf("checkout branch %s: %w (fallback also failed: %v)", e.cfg.Branch, err, err2)
			}
		}
		// Pull may fail if the branch is brand new locally; that's OK — ignore.
		_ = e.runGit(ctx, e.cfg.LocalDir, "pull", "origin", e.cfg.Branch)
		return nil
	}

	// Fresh clone.
	if err := os.MkdirAll(e.cfg.LocalDir, 0o755); err != nil {
		return fmt.Errorf("mkdir clone dir: %w", err)
	}
	if err := e.runGit(ctx, "", "clone", "--branch", e.cfg.Branch, e.cfg.RepoURL, e.cfg.LocalDir); err != nil {
		// Branch may not exist on remote — clone default then checkout.
		if err := e.runGit(ctx, "", "clone", e.cfg.RepoURL, e.cfg.LocalDir); err != nil {
			return fmt.Errorf("clone %s: %w", e.cfg.RepoURL, err)
		}
		_ = e.runGit(ctx, e.cfg.LocalDir, "checkout", "-b", e.cfg.Branch)
	}
	return nil
}

// renderedFile is one file the export will write: a path relative to the
// repo root plus the exact content to write. Files are rendered in memory
// first so the fail-closed community-edit check can compare before any
// bytes touch disk.
type renderedFile struct {
	relPath string
	content []byte
}

// renderItem fetches the class and answer from the store and formats the
// output files WITHOUT writing them to disk. Returns the rendered files
// or a SkipReason if the answer was skipped.
func (e *Engine) renderItem(ctx context.Context, item ExportItem) ([]renderedFile, *SkipReason, error) {
	pc, err := e.store.GetProblemClass(ctx, item.ClassID)
	if err != nil {
		return nil, nil, fmt.Errorf("get problem class: %w", err)
	}
	answer, err := e.store.GetAnswerNode(ctx, item.AnswerID)
	if err != nil {
		return nil, nil, fmt.Errorf("get answer node: %w", err)
	}

	if answer.ClassID != item.ClassID {
		return nil, &SkipReason{
			AnswerID: item.AnswerID,
			Reason:   fmt.Sprintf("answer class_id=%d does not match item class_id=%d", answer.ClassID, item.ClassID),
		}, nil
	}

	solutionMD := sanitizeHostPaths(formatSolutionMD(pc, answer))
	evidenceMD := sanitizeHostPaths(formatEvidenceMD(answer))
	signaturesJSON := sanitizeHostPaths(formatSignatures(answer))

	dir := filepath.Join(e.cfg.SubtreePrefix, pc.Title, answer.Env, answer.Version)
	files := []renderedFile{
		{filepath.Join(dir, "solution.md"), []byte(solutionMD)},
		{filepath.Join(dir, "evidence.md"), []byte(evidenceMD)},
		{filepath.Join(dir, "signatures.json"), []byte(signaturesJSON)},
	}
	return files, nil, nil
}

// writeRendered writes each rendered file to disk under LocalDir,
// creating parent directories as needed.
func (e *Engine) writeRendered(files []renderedFile) error {
	for _, f := range files {
		full := filepath.Join(e.cfg.LocalDir, f.relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return fmt.Errorf("mkdir export dir for %s: %w", f.relPath, err)
		}
		if err := os.WriteFile(full, f.content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.relPath, err)
		}
	}
	return nil
}

// checkNoForeignContent refuses the export when any target file holds
// content that is neither the fetched remote baseline (origin/Branch) nor
// identical to the content this export is about to write. Such content is
// a community edit — an uncommitted modification or an unpushed commit in
// the clone — and overwriting it would silently destroy work the export
// does not own.
//
// Only the exact files this export will write are examined; unrelated
// working-tree changes (inside or outside the subtree) never block an
// export. Three states are safe:
//
//   - the file is absent locally and absent from the baseline (new answer);
//   - the local content equals the baseline (clean fast-forward state —
//     this also covers community edits that were PUSHED to the remote,
//     whose resolution belongs to a conflict strategy, not this guard);
//   - the local content already equals the rendered content (idempotent
//     re-export over the producer's own unpushed export commit).
func (e *Engine) checkNoForeignContent(ctx context.Context, files []renderedFile) error {
	var conflicts []string
	seen := make(map[string]bool, len(files))
	for _, f := range files {
		if seen[f.relPath] {
			continue
		}
		seen[f.relPath] = true

		full := filepath.Join(e.cfg.LocalDir, f.relPath)
		current, curErr := os.ReadFile(full)
		if curErr == nil && bytes.Equal(current, f.content) {
			// No-op write — already exactly what we would write.
			continue
		}

		// Read the baseline blob. A failure means the file is absent
		// from origin/Branch (or the ref does not exist yet) — both
		// read as "no baseline", which is fail-closed here: any local
		// content then counts as foreign.
		baseline, baseErr := e.gitOutput(ctx, e.cfg.LocalDir,
			"show", "origin/"+e.cfg.Branch+":"+filepath.ToSlash(f.relPath))
		if curErr == nil && baseErr == nil && bytes.Equal(current, []byte(baseline)) {
			// Clean state: local content matches the fetched baseline.
			continue
		}
		if curErr != nil && baseErr != nil {
			// New file, absent locally and from the baseline.
			continue
		}
		conflicts = append(conflicts, f.relPath)
	}

	if len(conflicts) > 0 {
		return fmt.Errorf("%w: %s in %s differ from both origin/%s and the export content; "+
			"push or revert those changes, or point LocalDir at a fresh clone",
			ErrUnsafeExport, strings.Join(conflicts, ", "), e.cfg.LocalDir, e.cfg.Branch)
	}
	return nil
}

// stageAndCommit stages all files in FilesWritten, creates a commit,
// and pushes if configured.
func (e *Engine) stageAndCommit(ctx context.Context, res *ExportResult) error {
	// Stage files.
	args := append([]string{"add"}, res.FilesWritten...)
	if err := e.runGit(ctx, e.cfg.LocalDir, args...); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Check if there are staged changes.
	out, err := e.gitOutput(ctx, e.cfg.LocalDir, "diff", "--cached", "--name-only")
	if err != nil {
		return fmt.Errorf("git diff --cached: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		// Nothing to commit — all files were identical to existing content.
		return nil
	}

	// Commit message: a caller-supplied message is used verbatim (no
	// wrapping, no timestamp appended); otherwise fall back to the
	// engine's default summary.
	msg := e.cfg.CommitMessage
	if strings.TrimSpace(msg) == "" {
		msg = fmt.Sprintf("export: %d pre-solve answers\n\nExported via Off-by-One at %s",
			res.ItemsExported, time.Now().UTC().Format(time.RFC3339))
	}

	commitArgs := []string{"commit", "-m", msg}
	if e.cfg.CommitAuthor != "" && e.cfg.CommitEmail != "" {
		commitArgs = append(commitArgs,
			"--author", fmt.Sprintf("%s <%s>", e.cfg.CommitAuthor, e.cfg.CommitEmail))
	}
	if err := e.runGit(ctx, e.cfg.LocalDir, commitArgs...); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	// Get the commit SHA.
	sha, err := e.gitOutput(ctx, e.cfg.LocalDir, "rev-parse", "--short", "HEAD")
	if err != nil {
		return fmt.Errorf("rev-parse HEAD: %w", err)
	}
	res.CommitSHA = strings.TrimSpace(sha)

	// Push.
	if e.cfg.Push {
		if err := e.runGit(ctx, e.cfg.LocalDir, "push", "-u", "origin", e.cfg.Branch); err != nil {
			return fmt.Errorf("git push: %w", err)
		}
		res.Pushed = true
	}
	return nil
}

// runGit executes a git command in the given directory (or the current
// directory if dir is empty). stderr is captured for error messages.
func (e *Engine) runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, e.cfg.GitPath, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w\n%s", e.cfg.GitPath, strings.Join(args, " "), err, string(out))
	}
	return nil
}

// gitOutput runs a git command and returns stdout as a string.
func (e *Engine) gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, e.cfg.GitPath, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", e.cfg.GitPath, strings.Join(args, " "), err)
	}
	return string(out), nil
}

// normalizeRepoURL canonicalises a git remote URL for comparison: trims
// surrounding whitespace and strips one trailing "/" plus one trailing
// ".git", so "/src/repo.git/", "/src/repo.git" and "/src/repo" all compare
// equal. The comparison stays strict otherwise — two different
// repositories must never match.
func normalizeRepoURL(u string) string {
	s := strings.TrimSpace(u)
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")
	return s
}

// --- Host-path hygiene (REVIEW-OB-006) -----------------------------------

// hostPathPattern matches an operator-machine home prefix: /home/<user>
// where <user> is any plausible account name (kara, bunker, bunker-*,
// runner, user, ...). The subtree export is a public surface, so these
// must never be written into it.
var hostPathPattern = regexp.MustCompile(`/home/[A-Za-z0-9_-]+`)

// sanitizeHostPaths rewrites every /home/<user> prefix in s to "~"
// (e.g. "/home/kara/.local/bin/pi-agent" -> "~/.local/bin/pi-agent").
// Non-path text ("/usr/bin", a bare "/home", "home/kara" without the
// leading slash) is untouched. writeItem applies it to every rendered
// file before it reaches disk.
func sanitizeHostPaths(s string) string {
	return hostPathPattern.ReplaceAllString(s, "~")
}

// --- Formatting (spec §5.1) ---------------------------------------------

// formatSolutionMD renders the answer into the solution.md template from
// spec §5.1. Only the fields available in the AnswerNode + ProblemClass
// are filled in; callers that want richer templates can post-process.
func formatSolutionMD(pc *graph.ProblemClass, a *graph.AnswerNode) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Problem: %s\n\n", pc.Title)
	fmt.Fprintf(&b, "**Environment:** %s\n", a.Env)
	fmt.Fprintf(&b, "**Language:** %s %s\n", a.Lang, a.Version)
	fmt.Fprintf(&b, "**Status:** %s\n\n", a.Status)
	fmt.Fprintf(&b, "---\n\n")
	fmt.Fprintf(&b, "## Problem Description\n\n%s\n\n", pc.Description)
	fmt.Fprintf(&b, "---\n\n")
	fmt.Fprintf(&b, "## Solution\n\n%s\n", a.Solution)
	return b.String()
}

// formatEvidenceMD renders the evidence.md companion file.
func formatEvidenceMD(a *graph.AnswerNode) string {
	var b strings.Builder
	b.WriteString("# Evidence\n\n")
	fmt.Fprintf(&b, "**Status:** %s\n", a.Status)
	fmt.Fprintf(&b, "**Created:** %s\n\n", a.CreatedAt.Format(time.RFC3339))
	b.WriteString("---\n\n")
	if a.Evidence != "" {
		fmt.Fprintf(&b, "%s\n", a.Evidence)
	} else {
		b.WriteString("_No evidence recorded._\n")
	}
	return b.String()
}

// formatSignatures renders the signatures.json file. The AnswerNode's
// Signatures field is already a JSON string (from SQLite); if it's valid
// JSON we pretty-print it, otherwise we wrap it in an object.
func formatSignatures(a *graph.AnswerNode) string {
	if a.Signatures == "" {
		return "{}\n"
	}
	var raw any
	if err := json.Unmarshal([]byte(a.Signatures), &raw); err == nil {
		pretty, err := json.MarshalIndent(raw, "", "  ")
		if err == nil {
			return string(pretty) + "\n"
		}
	}
	// Fall back to wrapping the raw string.
	return fmt.Sprintf(`{"raw": %q}`+"\n", a.Signatures)
}
