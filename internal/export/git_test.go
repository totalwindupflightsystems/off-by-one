package export

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
)

// gitAvailable reports whether the git binary is on PATH.
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// skipIfNoGit skips the test if git isn't installed.
func skipIfNoGit(t *testing.T) {
	t.Helper()
	if !gitAvailable() {
		t.Skip("git not installed — skipping git integration test")
	}
}

// setGitIdentity makes engine-created commits hermetic. The export engine
// runs plain `git commit`, which falls back to ambient global config
// (~/.gitconfig) for author/committer identity; in CI/judge environments
// that config is absent and the commit dies with "Author identity unknown".
// t.Setenv pins the identity for the whole test process.
func setGitIdentity(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
}

// initBareRepo creates a bare git repository with HEAD pointing to the
// given branch, and returns its path. The export engine clones from this
// bare repo, so it serves as the "remote" in tests.
func initBareRepo(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	barePath := filepath.Join(dir, "remote.git")
	args := []string{"init", "--bare"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, barePath)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	return barePath
}

// seedRemote does an initial commit to the bare repo so that cloning
// with --branch works. It creates a temp working clone, commits a
// README, and pushes.
func seedRemote(t *testing.T, barePath, branch string) {
	t.Helper()
	dir := t.TempDir()
	// Clone empty bare repo
	if out, err := exec.Command("git", "clone", barePath, dir).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	// Set identity
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", args[0], err, out)
		}
	}
	// Write README
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Repo\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	// Add, commit, push
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "initial"},
		{"branch", "-M", branch},
		{"push", "origin", branch},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", args[0], err, out)
		}
	}
}

// gitIn runs git in dir and returns its trimmed stdout, failing the test
// if git exits non-zero. Used to snapshot clone state.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s (in %s): %v", strings.Join(args, " "), dir, err)
	}
	return strings.TrimSpace(string(out))
}

// advanceRemote pushes a new commit to the bare repo at barePath and
// returns the new commit SHA. Tests clone the bare repo first, advance it
// afterwards, then assert the clone's origin/<branch> ref did not move —
// which is only observable if a fetch really happened.
func advanceRemote(t *testing.T, barePath, branch, filename string) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "clone", barePath, dir).CombinedOutput(); err != nil {
		t.Fatalf("git clone (advance): %v\n%s", err, out)
	}
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", args[0], err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, filename), []byte("advanced\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "advance remote"},
		{"push", "origin", branch},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", args[0], err, out)
		}
	}
	return gitIn(t, dir, "rev-parse", "HEAD")
}

// makeStore creates an in-memory graph store with one problem class and
// one verified answer. Returns the store, class, and answer.
func makeStore(t *testing.T) (*graph.Store, *graph.ProblemClass, *graph.AnswerNode) {
	t.Helper()
	store, err := graph.OpenShared("export-test-" + t.Name())
	if err != nil {
		t.Fatalf("graph.OpenShared: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	pc, created, err := store.UpsertProblemClass(ctx, "docker-file-ownership", "Files owned by root after volume transfer")
	if err != nil {
		t.Fatalf("UpsertProblemClass: %v", err)
	}
	if !created {
		t.Fatal("expected class to be created")
	}

	answerID, err := store.CreateAnswerNode(ctx, pc.ID, 0,
		"docker", "go", "go-1.26",
		"Use `COPY --chown=appuser:appuser` in Dockerfile.",
		"Verified in Docker 24.0+. Validator ring: 2/2 passed.",
		`{"v1":{"model":"deepseek-v4-flash","passed":true},"v2":{"model":"minimax-m3","passed":true}}`,
	)
	if err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}
	if err := store.UpdateAnswerStatus(ctx, answerID, graph.AnswerVerified); err != nil {
		t.Fatalf("UpdateAnswerStatus: %v", err)
	}
	answer, err := store.GetAnswerNode(ctx, answerID)
	if err != nil {
		t.Fatalf("GetAnswerNode: %v", err)
	}
	return store, pc, answer
}

func TestNewEngine_Defaults(t *testing.T) {
	e := NewEngine(Config{
		RepoURL:  "https://example.com/repo.git",
		LocalDir: t.TempDir(),
	}, nil)
	if e.cfg.Branch != "main" {
		t.Errorf("default Branch = %q, want main", e.cfg.Branch)
	}
	if e.cfg.SubtreePrefix != "pre-solve-answers" {
		t.Errorf("default SubtreePrefix = %q, want pre-solve-answers", e.cfg.SubtreePrefix)
	}
	if e.cfg.GitPath != "git" {
		t.Errorf("default GitPath = %q, want git", e.cfg.GitPath)
	}
}

func TestExport_EmptyItems(t *testing.T) {
	store, _, _ := makeStore(t)
	e := NewEngine(Config{
		RepoURL:  "https://example.com/repo.git",
		LocalDir: t.TempDir(),
	}, store)
	_, err := e.Export(context.Background(), nil)
	if err != ErrNoItems {
		t.Errorf("Export(nil) err = %v, want ErrNoItems", err)
	}
}

func TestExport_NoRepoURL(t *testing.T) {
	store, pc, answer := makeStore(t)
	e := NewEngine(Config{
		LocalDir: t.TempDir(),
	}, store)
	_, err := e.Export(context.Background(), []ExportItem{
		{ClassID: pc.ID, AnswerID: answer.ID},
	})
	if err == nil || !strings.Contains(err.Error(), "RepoURL is required") {
		t.Errorf("Export without RepoURL err = %v, want 'RepoURL is required'", err)
	}
}

func TestExport_ClassMismatch(t *testing.T) {
	skipIfNoGit(t)
	setGitIdentity(t)
	store, pc, answer := makeStore(t)

	barePath := initBareRepo(t, "main")
	seedRemote(t, barePath, "main")

	localDir := filepath.Join(t.TempDir(), "clone")
	e := NewEngine(Config{
		RepoURL:  barePath,
		Branch:   "main",
		LocalDir: localDir,
	}, store)

	// Use a wrong class ID — should skip.
	_, err := e.Export(context.Background(), []ExportItem{
		{ClassID: pc.ID + 999, AnswerID: answer.ID},
	})
	// This will fail at GetProblemClass, which returns an error, not a skip.
	if err == nil {
		t.Error("expected error for non-existent class ID")
	}
}

func TestExport_FullFlow(t *testing.T) {
	skipIfNoGit(t)
	setGitIdentity(t)
	if runtime.GOOS == "windows" {
		t.Skip("file path separators differ on Windows")
	}
	store, pc, answer := makeStore(t)

	barePath := initBareRepo(t, "main")
	seedRemote(t, barePath, "main")

	localDir := filepath.Join(t.TempDir(), "clone")
	e := NewEngine(Config{
		RepoURL:  barePath,
		Branch:   "main",
		LocalDir: localDir,
		Push:     true,
	}, store)

	res, err := e.Export(context.Background(), []ExportItem{
		{ClassID: pc.ID, AnswerID: answer.ID},
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	if res.ItemsExported != 1 {
		t.Errorf("ItemsExported = %d, want 1", res.ItemsExported)
	}
	if len(res.FilesWritten) != 3 {
		t.Errorf("FilesWritten len = %d, want 3", len(res.FilesWritten))
	}
	if res.CommitSHA == "" {
		t.Error("CommitSHA is empty — commit was not created")
	}
	if !res.Pushed {
		t.Error("Pushed = false, want true")
	}

	// Verify file contents.
	expectedDir := filepath.Join(localDir, "pre-solve-answers", "docker-file-ownership", "docker", "go-1.26")
	for _, fname := range []string{"solution.md", "evidence.md", "signatures.json"} {
		path := filepath.Join(expectedDir, fname)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("ReadFile %s: %v", fname, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("%s is empty", fname)
		}
	}

	// Verify solution.md contains key fields.
	solData, _ := os.ReadFile(filepath.Join(expectedDir, "solution.md"))
	solStr := string(solData)
	if !strings.Contains(solStr, "docker-file-ownership") {
		t.Errorf("solution.md missing problem class title")
	}
	if !strings.Contains(solStr, "COPY --chown") {
		t.Errorf("solution.md missing solution text")
	}

	// Verify signatures.json is valid JSON.
	sigData, _ := os.ReadFile(filepath.Join(expectedDir, "signatures.json"))
	var sigs map[string]any
	if err := json.Unmarshal(sigData, &sigs); err != nil {
		t.Errorf("signatures.json is not valid JSON: %v", err)
	}

	// Verify the commit landed in the remote by doing a fresh clone.
	verifyDir := filepath.Join(t.TempDir(), "verify")
	if out, err := exec.Command("git", "clone", barePath, verifyDir).CombinedOutput(); err != nil {
		t.Fatalf("verify clone: %v\n%s", err, out)
	}
	verifySolution := filepath.Join(verifyDir, "pre-solve-answers", "docker-file-ownership", "docker", "go-1.26", "solution.md")
	if _, err := os.Stat(verifySolution); err != nil {
		t.Errorf("exported file not in remote repo: %v", err)
	}
}

func TestExport_DryRun(t *testing.T) {
	skipIfNoGit(t)
	setGitIdentity(t)
	store, pc, answer := makeStore(t)

	barePath := initBareRepo(t, "main")
	seedRemote(t, barePath, "main")

	localDir := filepath.Join(t.TempDir(), "clone")
	e := NewEngine(Config{
		RepoURL:  barePath,
		Branch:   "main",
		LocalDir: localDir,
		Push:     true, // should be ignored in dry run
	}, store)

	res, err := e.DryRun(context.Background(), []ExportItem{
		{ClassID: pc.ID, AnswerID: answer.ID},
	})
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if !res.DryRun {
		t.Error("DryRun = false, want true")
	}
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty in dry run", res.CommitSHA)
	}
	if res.Pushed {
		t.Error("Pushed = true in dry run, want false")
	}
	if len(res.FilesWritten) != 3 {
		t.Errorf("FilesWritten len = %d, want 3 (files should still be written to disk)", len(res.FilesWritten))
	}
}

func TestExport_IdempotentNoChanges(t *testing.T) {
	skipIfNoGit(t)
	setGitIdentity(t)
	store, pc, answer := makeStore(t)

	barePath := initBareRepo(t, "main")
	seedRemote(t, barePath, "main")

	localDir := filepath.Join(t.TempDir(), "clone")
	e := NewEngine(Config{
		RepoURL:  barePath,
		Branch:   "main",
		LocalDir: localDir,
		Push:     false, // don't push so we can re-run
	}, store)

	// First export — should produce a commit.
	res1, err := e.Export(context.Background(), []ExportItem{
		{ClassID: pc.ID, AnswerID: answer.ID},
	})
	if err != nil {
		t.Fatalf("first Export: %v", err)
	}
	if res1.CommitSHA == "" {
		t.Fatal("first export produced no commit")
	}

	// Second export — same content, should produce no new commit.
	res2, err := e.Export(context.Background(), []ExportItem{
		{ClassID: pc.ID, AnswerID: answer.ID},
	})
	if err != nil {
		t.Fatalf("second Export: %v", err)
	}
	if res2.CommitSHA != "" {
		t.Errorf("second export CommitSHA = %q, want empty (no changes)", res2.CommitSHA)
	}
}

func TestExport_PullExistingClone(t *testing.T) {
	skipIfNoGit(t)
	setGitIdentity(t)
	store, pc, answer := makeStore(t)

	barePath := initBareRepo(t, "main")
	seedRemote(t, barePath, "main")

	// Pre-clone the repo manually.
	localDir := filepath.Join(t.TempDir(), "clone")
	if out, err := exec.Command("git", "clone", barePath, localDir).CombinedOutput(); err != nil {
		t.Fatalf("manual clone: %v\n%s", err, out)
	}

	e := NewEngine(Config{
		RepoURL:  barePath,
		Branch:   "main",
		LocalDir: localDir,
		Push:     false,
	}, store)

	res, err := e.Export(context.Background(), []ExportItem{
		{ClassID: pc.ID, AnswerID: answer.ID},
	})
	if err != nil {
		t.Fatalf("Export with existing clone: %v", err)
	}
	if res.CommitSHA == "" {
		t.Error("expected a commit when exporting to existing clone")
	}
}

// TestExport_ExistingCloneMismatchedRepo_Errors covers the origin guard in
// prepareClone (ErrRepoMismatch): when LocalDir already holds a clone of
// repo A and the engine is asked to export into a different repo B, it must
// fail with ErrRepoMismatch and must NOT touch the clone — no fetch, no
// checkout, no pull-forward, no files written. Reusing that clone would
// silently commit into (or push to) the wrong repository.
//
// The no-fetch assertion is behavioural, not just an error-type check: repo A
// is advanced with a NEW commit after the clone is taken, so any fetch would
// move the clone's refs/remotes/origin/main. The test asserts that ref (HEAD,
// origin URL, working-tree cleanliness and the subtree dir too) is unchanged.
func TestExport_ExistingCloneMismatchedRepo_Errors(t *testing.T) {
	skipIfNoGit(t)
	setGitIdentity(t)
	store, pc, answer := makeStore(t)

	// Repo A — the repo the pre-existing clone belongs to.
	repoA := initBareRepo(t, "main")
	seedRemote(t, repoA, "main")

	// Repo B — a different, real repository: the silent-corruption case.
	repoB := initBareRepo(t, "main")
	seedRemote(t, repoB, "main")
	if repoA == repoB {
		t.Fatal("premise: repo A and repo B must be different repositories")
	}

	// Simulate a previous run against repo A.
	localDir := filepath.Join(t.TempDir(), "clone")
	if out, err := exec.Command("git", "clone", repoA, localDir).CombinedOutput(); err != nil {
		t.Fatalf("manual clone of repo A: %v\n%s", err, out)
	}

	// Snapshot the clone BEFORE the mismatched call.
	headBefore := gitIn(t, localDir, "rev-parse", "HEAD")
	originBefore := gitIn(t, localDir, "remote", "get-url", "origin")
	originMainBefore := gitIn(t, localDir, "rev-parse", "refs/remotes/origin/main")

	// Advance repo A after the clone: a fetch would now be observable.
	advancedSHA := advanceRemote(t, repoA, "main", "post-clone.txt")
	if advancedSHA == originMainBefore {
		t.Fatalf("premise failed: advancing repo A did not move it past %s", originMainBefore)
	}

	cases := []struct {
		name   string
		target string
		run    func(e *Engine) error
	}{
		{
			name:   "different-real-repo (dry-run)",
			target: repoB,
			run: func(e *Engine) error {
				_, err := e.DryRun(context.Background(), []ExportItem{{ClassID: pc.ID, AnswerID: answer.ID}})
				return err
			},
		},
		{
			name:   "nonexistent-path (export, Push=false)",
			target: filepath.Join(t.TempDir(), "not-a-repo.git"),
			run: func(e *Engine) error {
				_, err := e.Export(context.Background(), []ExportItem{{ClassID: pc.ID, AnswerID: answer.ID}})
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEngine(Config{
				RepoURL:  tc.target,
				Branch:   "main",
				LocalDir: localDir,
				Push:     false,
			}, store)

			err := tc.run(e)

			// 1. The mismatch must be reported as ErrRepoMismatch.
			if err == nil {
				t.Fatal("export with mismatched RepoURL: want error, got nil")
			}
			if !errors.Is(err, ErrRepoMismatch) {
				t.Errorf("error = %v, want errors.Is(err, ErrRepoMismatch)", err)
			}
			// 2. The message must name BOTH URLs so the operator can see the
			//    stale clone and the requested target.
			for _, want := range []string{originBefore, tc.target} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err.Error(), want)
				}
			}

			// 3. NO fetch: origin/main must not have moved to the commit repo A
			//    gained after the clone was taken.
			if got := gitIn(t, localDir, "rev-parse", "refs/remotes/origin/main"); got != originMainBefore {
				t.Errorf("clone origin/main moved: %s -> %s (guard fetched the clone)", originMainBefore, got)
			}
			// 4. NO pull-forward / checkout / commit.
			if got := gitIn(t, localDir, "rev-parse", "HEAD"); got != headBefore {
				t.Errorf("clone HEAD moved: %s -> %s (guard pulled the clone forward)", headBefore, got)
			}
			// 5. The clone still belongs to repo A, not the requested repo B.
			if got := gitIn(t, localDir, "remote", "get-url", "origin"); got != originBefore {
				t.Errorf("clone origin changed: %q -> %q", originBefore, got)
			}
			// 6. Working tree untouched (no checkout, no stray export files).
			if got := gitIn(t, localDir, "status", "--porcelain"); got != "" {
				t.Errorf("clone working tree dirty after mismatched export:\n%s", got)
			}
			if _, statErr := os.Stat(filepath.Join(localDir, "pre-solve-answers")); !os.IsNotExist(statErr) {
				t.Errorf("export wrote into the clone despite the mismatch (stat err = %v)", statErr)
			}
			// 7. The clone's HEAD is still the pre-advance commit of repo A.
			if headBefore == advancedSHA {
				t.Fatalf("premise failed: clone HEAD already at repo A's advanced commit")
			}
		})
	}
}

func TestFormatSolutionMD(t *testing.T) {
	pc := &graph.ProblemClass{
		ID:          1,
		Title:       "test-problem",
		Description: "A test problem description.",
	}
	a := &graph.AnswerNode{
		ID:       2,
		ClassID:  1,
		Env:      "docker",
		Lang:     "go",
		Version:  "go-1.26",
		Solution: "Do the thing.",
		Status:   "verified",
	}
	md := formatSolutionMD(pc, a)
	if !strings.Contains(md, "# Problem: test-problem") {
		t.Errorf("solution.md missing title header")
	}
	if !strings.Contains(md, "**Environment:** docker") {
		t.Errorf("solution.md missing environment")
	}
	if !strings.Contains(md, "**Language:** go go-1.26") {
		t.Errorf("solution.md missing language+version")
	}
	if !strings.Contains(md, "**Status:** verified") {
		t.Errorf("solution.md missing status")
	}
	if !strings.Contains(md, "Do the thing.") {
		t.Errorf("solution.md missing solution text")
	}
}

func TestFormatEvidenceMD(t *testing.T) {
	a := &graph.AnswerNode{
		Evidence:  "All tests pass.",
		Status:    "verified",
		CreatedAt: time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC),
	}
	md := formatEvidenceMD(a)
	if !strings.Contains(md, "# Evidence") {
		t.Errorf("evidence.md missing header")
	}
	if !strings.Contains(md, "All tests pass.") {
		t.Errorf("evidence.md missing evidence text")
	}
}

func TestFormatEvidenceMD_Empty(t *testing.T) {
	a := &graph.AnswerNode{
		Evidence: "",
	}
	md := formatEvidenceMD(a)
	if !strings.Contains(md, "_No evidence recorded._") {
		t.Errorf("evidence.md should contain placeholder for empty evidence")
	}
}

func TestFormatSignatures_ValidJSON(t *testing.T) {
	a := &graph.AnswerNode{
		Signatures: `{"v1":{"passed":true}}`,
	}
	out := formatSignatures(a)
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Errorf("formatSignatures output is not valid JSON: %v\n%s", err, out)
	}
	if _, ok := raw["v1"]; !ok {
		t.Errorf("formatSignatures lost the v1 key")
	}
}

func TestFormatSignatures_Empty(t *testing.T) {
	a := &graph.AnswerNode{
		Signatures: "",
	}
	out := formatSignatures(a)
	if out != "{}\n" {
		t.Errorf("formatSignatures('') = %q, want %q", out, "{}\n")
	}
}

func TestFormatSignatures_InvalidJSON(t *testing.T) {
	a := &graph.AnswerNode{
		Signatures: "not-json",
	}
	out := formatSignatures(a)
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Errorf("formatSignatures with invalid JSON should produce valid JSON, got: %v\n%s", err, out)
	}
	if _, ok := raw["raw"]; !ok {
		t.Errorf("formatSignatures should wrap invalid JSON in a 'raw' key")
	}
}
