package export

import (
	"context"
	"encoding/json"
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
