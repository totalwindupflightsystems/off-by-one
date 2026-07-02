// Package importing implements the git subtree import engine that pulls
// verified Off-by-One answers from a source repository into the local
// graph. The expected directory layout mirrors the export format (spec §8.1):
//
//	pre-solve-answers/
//	  {problem-class}/
//	    {env}/
//	      {version}/
//	        solution.md
//	        evidence.md
//	        signatures.json
//
// The engine clones (or pulls) the source repo, walks the subtree, parses
// each answer directory, diffs against the local graph, and inserts or
// updates selected answers.
package importing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
)

// Config controls a single import operation.
type Config struct {
	// RepoURL is the git remote to clone or pull.
	RepoURL string

	// Branch is the source branch. Defaults to "main".
	Branch string

	// LocalDir is the working clone. If it already contains a .git
	// directory, the engine pulls instead of cloning.
	LocalDir string

	// SubtreePrefix is the directory inside the repo where answers
	// are stored. Defaults to "pre-solve-answers" per spec §8.1.
	SubtreePrefix string

	// GitPath is the path to the git binary. Defaults to "git".
	GitPath string
}

// ParsedAnswer is one answer parsed from the source repo's directory tree.
// The engine populates these from solution.md + evidence.md + signatures.json
// before inserting them into the local graph.
type ParsedAnswer struct {
	ClassTitle string
	Env        string
	Version    string
	Lang       string
	Status     string
	Solution   string
	Evidence   string
	Signatures string
	// DirPath is the directory path relative to the repo root.
	DirPath string
}

// Action classifies what happened to a single answer during import.
type Action string

const (
	ActionAdded      Action = "added"
	ActionUpdated    Action = "updated"
	ActionSkipped    Action = "skipped"
	ActionConflict   Action = "conflict"
	ActionParseError Action = "parse_error"
)

// ImportDetail records the outcome for a single parsed answer.
type ImportDetail struct {
	ClassTitle string
	Env        string
	Version    string
	Lang       string
	Action     Action
	AnswerID   int64
	Reason     string
}

// ImportResult summarises a completed import.
type ImportResult struct {
	Added      int
	Updated    int
	Skipped    int
	Conflicted int
	ParseErr   int
	Details    []ImportDetail
}

// Engine performs import operations against a git repository. It reads
// answers from a source repo and inserts/updates them in a *graph.Store.
type Engine struct {
	cfg   Config
	store *graph.Store
}

// NewEngine returns an import Engine bound to the given config and graph
// store.
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

// ErrNoRepoURL is returned when RepoURL is empty.
var ErrNoRepoURL = errors.New("import: RepoURL is required")

// ErrNoSubtree is returned when the subtree prefix directory doesn't exist
// in the cloned repo. This usually means the repo doesn't contain
// Off-by-One answers.
var ErrNoSubtree = errors.New("import: subtree directory not found in repo")

// Import runs the full import flow:
//
//  1. Clone or pull the source repo
//  2. Walk the subtree directory tree
//  3. Parse each answer directory
//  4. Diff each parsed answer against the local graph
//  5. Insert new answers or update existing ones
//
// The ImportResult contains per-answer details and aggregate counts.
func (e *Engine) Import(ctx context.Context) (*ImportResult, error) {
	if e.cfg.RepoURL == "" {
		return nil, ErrNoRepoURL
	}

	// Step 1: ensure local clone exists and is up-to-date.
	if err := e.prepareClone(ctx); err != nil {
		return nil, fmt.Errorf("prepare clone: %w", err)
	}

	// Step 2: walk the subtree.
	subtreeDir := filepath.Join(e.cfg.LocalDir, e.cfg.SubtreePrefix)
	if info, err := os.Stat(subtreeDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrNoSubtree, subtreeDir)
	}

	answers, err := e.walkSubtree(subtreeDir)
	if err != nil {
		return nil, fmt.Errorf("walk subtree: %w", err)
	}

	// Step 3 + 4: diff and insert.
	res := &ImportResult{}
	for _, ans := range answers {
		detail, err := e.importAnswer(ctx, ans)
		if err != nil {
			detail = ImportDetail{
				ClassTitle: ans.ClassTitle,
				Env:        ans.Env,
				Version:    ans.Version,
				Lang:       ans.Lang,
				Action:     ActionParseError,
				Reason:     err.Error(),
			}
		}
		res.Details = append(res.Details, detail)
		switch detail.Action {
		case ActionAdded:
			res.Added++
		case ActionUpdated:
			res.Updated++
		case ActionSkipped:
			res.Skipped++
		case ActionConflict:
			res.Conflicted++
		case ActionParseError:
			res.ParseErr++
		}
	}

	return res, nil
}

// --- Internal helpers ---------------------------------------------------

// prepareClone ensures LocalDir contains an up-to-date clone of RepoURL
// on the target branch. Mirrors the export engine's prepareClone.
func (e *Engine) prepareClone(ctx context.Context) error {
	gitDir := filepath.Join(e.cfg.LocalDir, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		// Existing clone — fetch + checkout + pull.
		if err := e.runGit(ctx, e.cfg.LocalDir, "fetch", "origin"); err != nil {
			return fmt.Errorf("fetch origin: %w", err)
		}
		if err := e.runGit(ctx, e.cfg.LocalDir, "checkout", e.cfg.Branch); err != nil {
			if err2 := e.runGit(ctx, e.cfg.LocalDir, "checkout", "-b", e.cfg.Branch,
				"origin/"+e.cfg.Branch); err2 != nil {
				return fmt.Errorf("checkout branch %s: %w (fallback also failed: %v)", e.cfg.Branch, err, err2)
			}
		}
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

// walkSubtree walks the subtree directory tree and returns parsed answers.
// Expected layout: {subtree}/{class-title}/{env}/{version}/{files}
func (e *Engine) walkSubtree(subtreeDir string) ([]ParsedAnswer, error) {
	var answers []ParsedAnswer

	classEntries, err := os.ReadDir(subtreeDir)
	if err != nil {
		return nil, fmt.Errorf("read subtree dir: %w", err)
	}
	for _, classEntry := range classEntries {
		if !classEntry.IsDir() || strings.HasPrefix(classEntry.Name(), ".") {
			continue
		}
		classTitle := classEntry.Name()
		classDir := filepath.Join(subtreeDir, classTitle)

		envEntries, err := os.ReadDir(classDir)
		if err != nil {
			continue
		}
		for _, envEntry := range envEntries {
			if !envEntry.IsDir() || strings.HasPrefix(envEntry.Name(), ".") {
				continue
			}
			env := envEntry.Name()
			envDir := filepath.Join(classDir, env)

			versionEntries, err := os.ReadDir(envDir)
			if err != nil {
				continue
			}
			for _, versionEntry := range versionEntries {
				if !versionEntry.IsDir() || strings.HasPrefix(versionEntry.Name(), ".") {
					continue
				}
				version := versionEntry.Name()
				versionDir := filepath.Join(envDir, version)

				ans, err := e.parseAnswerDir(versionDir, classTitle, env, version)
				if err != nil {
					// Record as a parse-error entry with minimal info.
					answers = append(answers, ParsedAnswer{
						ClassTitle: classTitle,
						Env:        env,
						Version:    version,
						Status:     "parse_error",
					})
					continue
				}
				answers = append(answers, ans)
			}
		}
	}

	return answers, nil
}

// parseAnswerDir reads solution.md, evidence.md, and signatures.json from
// the given directory and returns a ParsedAnswer.
func (e *Engine) parseAnswerDir(dir, classTitle, env, version string) (ParsedAnswer, error) {
	ans := ParsedAnswer{
		ClassTitle: classTitle,
		Env:        env,
		Version:    version,
		DirPath:    relSubtreePath(e.cfg.SubtreePrefix, classTitle, env, version),
	}

	// Read solution.md — required.
	solutionPath := filepath.Join(dir, "solution.md")
	solBytes, err := os.ReadFile(solutionPath)
	if err != nil {
		return ans, fmt.Errorf("read solution.md: %w", err)
	}
	solution, lang, status := parseSolutionMD(string(solBytes))
	ans.Solution = solution
	if lang != "" {
		ans.Lang = lang
	}
	if status != "" {
		ans.Status = status
	}

	// Read evidence.md — optional but expected.
	evidencePath := filepath.Join(dir, "evidence.md")
	if evBytes, err := os.ReadFile(evidencePath); err == nil {
		ans.Evidence = parseEvidenceMD(string(evBytes))
	}

	// Read signatures.json — optional.
	sigPath := filepath.Join(dir, "signatures.json")
	if sigBytes, err := os.ReadFile(sigPath); err == nil {
		ans.Signatures = strings.TrimSpace(string(sigBytes))
	}

	return ans, nil
}

// importAnswer diffs a parsed answer against the local graph and inserts
// or updates it. Returns the ImportDetail describing the action taken.
func (e *Engine) importAnswer(ctx context.Context, ans ParsedAnswer) (ImportDetail, error) {
	detail := ImportDetail{
		ClassTitle: ans.ClassTitle,
		Env:        ans.Env,
		Version:    ans.Version,
		Lang:       ans.Lang,
	}

	// Upsert the problem class.
	pc, _, err := e.store.UpsertProblemClass(ctx, ans.ClassTitle, "")
	if err != nil {
		return detail, fmt.Errorf("upsert problem class: %w", err)
	}

	// Check for existing answer with same (class, env, lang, version).
	existing, err := e.findAnswer(ctx, pc.ID, ans.Env, ans.Lang, ans.Version)
	if err != nil {
		return detail, fmt.Errorf("find existing answer: %w", err)
	}

	if existing == nil {
		// No existing answer — insert as new.
		answerID, err := e.store.CreateAnswerNode(ctx, pc.ID, 0,
			ans.Env, ans.Lang, ans.Version,
			ans.Solution, ans.Evidence, ans.Signatures)
		if err != nil {
			return detail, fmt.Errorf("create answer node: %w", err)
		}
		// Set status if parsed and valid.
		if ans.Status != "" {
			_ = e.store.UpdateAnswerStatus(ctx, answerID, ans.Status)
		}
		detail.Action = ActionAdded
		detail.AnswerID = answerID
		return detail, nil
	}

	// Existing answer found — compare content.
	if existing.Solution == ans.Solution &&
		existing.Evidence == ans.Evidence &&
		existing.Signatures == ans.Signatures {
		// Identical content — skip.
		detail.Action = ActionSkipped
		detail.AnswerID = existing.ID
		detail.Reason = "identical content"
		return detail, nil
	}

	// Content differs — update the existing answer.
	_, err = e.store.DB().ExecContext(ctx,
		`UPDATE answer_nodes SET solution = ?, evidence = ?, signatures = ? WHERE id = ?`,
		ans.Solution, ans.Evidence, ans.Signatures, existing.ID)
	if err != nil {
		return detail, fmt.Errorf("update answer node: %w", err)
	}
	if ans.Status != "" {
		_ = e.store.UpdateAnswerStatus(ctx, existing.ID, ans.Status)
	}
	detail.Action = ActionUpdated
	detail.AnswerID = existing.ID
	detail.Reason = "content updated"
	return detail, nil
}

// findAnswer looks for an existing answer matching (classID, env, lang, version).
// Returns nil if no match exists.
func (e *Engine) findAnswer(ctx context.Context, classID int64, env, lang, version string) (*graph.AnswerNode, error) {
	answers, err := e.store.ListAnswers(ctx, classID)
	if err != nil {
		return nil, err
	}
	for i := range answers {
		a := &answers[i]
		if a.Env == env && a.Lang == lang && a.Version == version {
			return a, nil
		}
	}
	return nil, nil
}

// runGit executes a git command in the given directory.
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

// --- Parsing helpers ---------------------------------------------------

// parseSolutionMD extracts the solution text, language, and status from a
// solution.md file. The format produced by the export engine is:
//
//	# Problem: {title}
//
//	**Environment:** {env}
//	**Language:** {lang} {version}
//	**Status:** {status}
//
//	---
//
//	## Problem Description
//
//	{description}
//
//	---
//
//	## Solution
//
//	{solution text}
//
// Returns (solution, lang, status).
func parseSolutionMD(content string) (solution, lang, status string) {
	// Extract Language from header: **Language:** go go-1.26
	if m := langRe.FindStringSubmatch(content); len(m) > 1 {
		lang = m[1]
	}

	// Extract Status from header: **Status:** verified
	if m := statusRe.FindStringSubmatch(content); len(m) > 1 {
		status = m[1]
	}

	// Extract Solution text: everything after "## Solution\n\n"
	solution = extractSection(content, "## Solution")
	return
}

// parseEvidenceMD extracts the evidence text from evidence.md. The format is:
//
//	# Evidence
//
//	**Status:** {status}
//	**Created:** {timestamp}
//
//	---
//
//	{evidence text}
func parseEvidenceMD(content string) string {
	// Everything after the last "---" separator.
	parts := strings.SplitN(content, "---", 2)
	if len(parts) < 2 {
		return strings.TrimSpace(content)
	}
	return strings.TrimSpace(parts[1])
}

// extractSection returns the text after a section header marker (e.g. "## Solution").
// It finds the first occurrence of the marker and returns everything after it,
// stripped of leading/trailing whitespace. If the marker is not found, returns "".
func extractSection(content, marker string) string {
	idx := strings.Index(content, marker)
	if idx < 0 {
		return ""
	}
	rest := content[idx+len(marker):]
	// Skip the trailing newline(s) after the marker.
	rest = strings.TrimLeft(rest, "\n\r ")
	// If there's another section after this one, cut at it.
	for _, nextMarker := range []string{"\n---\n", "\n## ", "\n# "} {
		if cutIdx := strings.Index(rest, nextMarker); cutIdx >= 0 && cutIdx < len(rest) {
			candidate := strings.TrimSpace(rest[:cutIdx])
			if candidate != "" {
				return candidate
			}
		}
	}
	return strings.TrimSpace(rest)
}

// relSubtreePath builds the relative path for a parsed answer within the repo.
func relSubtreePath(prefix, classTitle, env, version string) string {
	return filepath.Join(prefix, classTitle, env, version)
}

var (
	langRe   = regexp.MustCompile(`\*\*Language:\*\*\s+(\S+)`)
	statusRe = regexp.MustCompile(`\*\*Status:\*\*\s+(\S+)`)
)

// ParseSignaturesJSON validates and normalises a signatures JSON string.
// Returns "{}" for empty input. Returns the compact form if valid.
func ParseSignaturesJSON(raw string) string {
	if raw == "" {
		return "{}"
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		// Not valid JSON — wrap in raw key.
		return fmt.Sprintf(`{"raw": %q}`, raw)
	}
	compact, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(compact)
}
