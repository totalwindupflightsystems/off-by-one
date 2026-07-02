package importing

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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

// initBareRepo creates a bare git repository and returns its path.
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

// seedRemote creates an initial commit so cloning with --branch works.
func seedRemote(t *testing.T, barePath, branch string) {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "clone", barePath, dir).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
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
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Repo\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
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

// writeAnswerFiles writes solution.md, evidence.md, and signatures.json
// into dir following the export format from spec §8.1.
func writeAnswerFiles(t *testing.T, dir, classTitle, env, lang, version, status, solution, evidence, signatures string) {
	t.Helper()
	ansDir := filepath.Join(dir, "pre-solve-answers", classTitle, env, version)
	if err := os.MkdirAll(ansDir, 0o755); err != nil {
		t.Fatalf("mkdir answer dir: %v", err)
	}

	// solution.md
	solMD := "# Problem: " + classTitle + "\n\n"
	solMD += "**Environment:** " + env + "\n"
	solMD += "**Language:** " + lang + " " + version + "\n"
	solMD += "**Status:** " + status + "\n\n"
	solMD += "---\n\n"
	solMD += "## Problem Description\n\n"
	solMD += "Some description.\n\n"
	solMD += "---\n\n"
	solMD += "## Solution\n\n" + solution + "\n"
	if err := os.WriteFile(filepath.Join(ansDir, "solution.md"), []byte(solMD), 0o644); err != nil {
		t.Fatalf("write solution.md: %v", err)
	}

	// evidence.md
	evMD := "# Evidence\n\n"
	evMD += "**Status:** " + status + "\n"
	evMD += "**Created:** 2026-07-02T12:00:00Z\n\n"
	evMD += "---\n\n" + evidence + "\n"
	if err := os.WriteFile(filepath.Join(ansDir, "evidence.md"), []byte(evMD), 0o644); err != nil {
		t.Fatalf("write evidence.md: %v", err)
	}

	// signatures.json
	if err := os.WriteFile(filepath.Join(ansDir, "signatures.json"), []byte(signatures), 0o644); err != nil {
		t.Fatalf("write signatures.json: %v", err)
	}
}

// commitAndPush commits all changes and pushes to the bare repo.
func commitAndPush(t *testing.T, dir, branch string) {
	t.Helper()
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"add", "."},
		{"commit", "-m", "add answers"},
		{"push", "origin", branch},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", args[0], err, out)
		}
	}
}

// makeStore creates a fresh in-memory graph store.
func makeStore(t *testing.T) *graph.Store {
	t.Helper()
	store, err := graph.OpenShared("import-test-" + t.Name())
	if err != nil {
		t.Fatalf("graph.OpenShared: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// setupSourceRepo creates a bare repo, seeds it with answer files, and returns
// the bare path. Answers are written to the bare repo via a working clone.
func setupSourceRepo(t *testing.T, branch string) string {
	t.Helper()
	barePath := initBareRepo(t, branch)
	seedRemote(t, barePath, branch)

	// Clone, add answers, push.
	workDir := filepath.Join(t.TempDir(), "source-work")
	if out, err := exec.Command("git", "clone", barePath, workDir).CombinedOutput(); err != nil {
		t.Fatalf("git clone source: %v\n%s", err, out)
	}
	writeAnswerFiles(t, workDir,
		"docker-permission-denied", "docker", "go", "go-1.26", "verified",
		"Use `RUN chmod +x /app` in the Dockerfile.",
		"Tested in Docker 24.0+.",
		`{"v1":{"model":"deepseek-v4-flash","passed":true}}`,
	)
	commitAndPush(t, workDir, branch)
	return barePath
}

// --- Engine config tests ---------------------------------------------------

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

func TestImport_NoRepoURL(t *testing.T) {
	store := makeStore(t)
	e := NewEngine(Config{
		LocalDir: t.TempDir(),
	}, store)
	_, err := e.Import(context.Background())
	if err != ErrNoRepoURL {
		t.Errorf("Import without RepoURL err = %v, want ErrNoRepoURL", err)
	}
}

func TestImport_NoSubtree(t *testing.T) {
	skipIfNoGit(t)
	store := makeStore(t)

	barePath := initBareRepo(t, "main")
	seedRemote(t, barePath, "main")

	localDir := filepath.Join(t.TempDir(), "clone")
	e := NewEngine(Config{
		RepoURL:  barePath,
		Branch:   "main",
		LocalDir: localDir,
	}, store)

	_, err := e.Import(context.Background())
	if err == nil || !strings.Contains(err.Error(), "subtree directory not found") {
		t.Errorf("Import without subtree err = %v, want 'subtree directory not found'", err)
	}
}

// --- Full import flow tests ------------------------------------------------

func TestImport_NewAnswer(t *testing.T) {
	skipIfNoGit(t)
	if runtime.GOOS == "windows" {
		t.Skip("file path separators differ on Windows")
	}
	store := makeStore(t)
	barePath := setupSourceRepo(t, "main")

	localDir := filepath.Join(t.TempDir(), "clone")
	e := NewEngine(Config{
		RepoURL:  barePath,
		Branch:   "main",
		LocalDir: localDir,
	}, store)

	res, err := e.Import(context.Background())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if res.Added != 1 {
		t.Errorf("Added = %d, want 1", res.Added)
	}
	if res.Updated != 0 {
		t.Errorf("Updated = %d, want 0", res.Updated)
	}
	if res.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", res.Skipped)
	}

	// Verify the answer was inserted into the graph.
	ctx := context.Background()
	pc, err := store.GetProblemClassByTitle(ctx, "docker-permission-denied")
	if err != nil {
		t.Fatalf("GetProblemClassByTitle: %v", err)
	}
	answers, err := store.ListAnswers(ctx, pc.ID)
	if err != nil {
		t.Fatalf("ListAnswers: %v", err)
	}
	if len(answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(answers))
	}
	a := &answers[0]
	if a.Env != "docker" {
		t.Errorf("answer Env = %q, want docker", a.Env)
	}
	if a.Lang != "go" {
		t.Errorf("answer Lang = %q, want go", a.Lang)
	}
	if a.Version != "go-1.26" {
		t.Errorf("answer Version = %q, want go-1.26", a.Version)
	}
	if !strings.Contains(a.Solution, "chmod +x") {
		t.Errorf("answer Solution = %q, expected to contain 'chmod +x'", a.Solution)
	}
}

func TestImport_Idempotent(t *testing.T) {
	skipIfNoGit(t)
	store := makeStore(t)
	barePath := setupSourceRepo(t, "main")

	localDir := filepath.Join(t.TempDir(), "clone")
	e := NewEngine(Config{
		RepoURL:  barePath,
		Branch:   "main",
		LocalDir: localDir,
	}, store)

	// First import — should add the answer.
	res1, err := e.Import(context.Background())
	if err != nil {
		t.Fatalf("first Import: %v", err)
	}
	if res1.Added != 1 {
		t.Fatalf("first import Added = %d, want 1", res1.Added)
	}

	// Second import — same content, should skip.
	res2, err := e.Import(context.Background())
	if err != nil {
		t.Fatalf("second Import: %v", err)
	}
	if res2.Skipped != 1 {
		t.Errorf("second import Skipped = %d, want 1", res2.Skipped)
	}
	if res2.Added != 0 {
		t.Errorf("second import Added = %d, want 0", res2.Added)
	}
}

func TestImport_UpdateAnswer(t *testing.T) {
	skipIfNoGit(t)
	store := makeStore(t)

	// Pre-populate the graph with an existing answer that will be updated.
	ctx := context.Background()
	pc, _, err := store.UpsertProblemClass(ctx, "docker-permission-denied", "")
	if err != nil {
		t.Fatalf("UpsertProblemClass: %v", err)
	}
	oldAnswerID, err := store.CreateAnswerNode(ctx, pc.ID, 0,
		"docker", "go", "go-1.26",
		"OLD solution text.", "OLD evidence.", "{}")
	if err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}

	barePath := setupSourceRepo(t, "main")

	localDir := filepath.Join(t.TempDir(), "clone")
	e := NewEngine(Config{
		RepoURL:  barePath,
		Branch:   "main",
		LocalDir: localDir,
	}, store)

	res, err := e.Import(context.Background())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Updated != 1 {
		t.Errorf("Updated = %d, want 1", res.Updated)
	}

	// Verify the answer was updated in-place.
	updated, err := store.GetAnswerNode(ctx, oldAnswerID)
	if err != nil {
		t.Fatalf("GetAnswerNode: %v", err)
	}
	if updated.Solution == "OLD solution text." {
		t.Error("answer Solution was not updated")
	}
	if !strings.Contains(updated.Solution, "chmod +x") {
		t.Errorf("updated Solution = %q, expected new content", updated.Solution)
	}
}

func TestImport_ConflictDifferentClass(t *testing.T) {
	skipIfNoGit(t)
	store := makeStore(t)

	// Pre-populate with an answer under a different version so the import adds a new one.
	ctx := context.Background()
	pc, _, err := store.UpsertProblemClass(ctx, "docker-permission-denied", "")
	if err != nil {
		t.Fatalf("UpsertProblemClass: %v", err)
	}
	_, err = store.CreateAnswerNode(ctx, pc.ID, 0,
		"docker", "go", "go-1.20", // different version
		"Different version answer.", "", "{}")
	if err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}

	barePath := setupSourceRepo(t, "main")

	localDir := filepath.Join(t.TempDir(), "clone")
	e := NewEngine(Config{
		RepoURL:  barePath,
		Branch:   "main",
		LocalDir: localDir,
	}, store)

	res, err := e.Import(context.Background())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	// The go-1.26 answer should be added as new (no conflict with go-1.20).
	if res.Added != 1 {
		t.Errorf("Added = %d, want 1", res.Added)
	}

	// Verify both answers exist.
	answers, err := store.ListAnswers(ctx, pc.ID)
	if err != nil {
		t.Fatalf("ListAnswers: %v", err)
	}
	if len(answers) != 2 {
		t.Errorf("expected 2 answers (go-1.20 + go-1.26), got %d", len(answers))
	}
}

func TestImport_PullExistingClone(t *testing.T) {
	skipIfNoGit(t)
	store := makeStore(t)
	barePath := setupSourceRepo(t, "main")

	// Pre-clone the repo manually.
	localDir := filepath.Join(t.TempDir(), "clone")
	if out, err := exec.Command("git", "clone", barePath, localDir).CombinedOutput(); err != nil {
		t.Fatalf("manual clone: %v\n%s", err, out)
	}

	e := NewEngine(Config{
		RepoURL:  barePath,
		Branch:   "main",
		LocalDir: localDir,
	}, store)

	res, err := e.Import(context.Background())
	if err != nil {
		t.Fatalf("Import with existing clone: %v", err)
	}
	if res.Added != 1 {
		t.Errorf("Added = %d, want 1", res.Added)
	}
}

func TestImport_MultipleAnswers(t *testing.T) {
	skipIfNoGit(t)
	store := makeStore(t)

	barePath := initBareRepo(t, "main")
	seedRemote(t, barePath, "main")

	// Set up source repo with multiple answers.
	workDir := filepath.Join(t.TempDir(), "source-work")
	if out, err := exec.Command("git", "clone", barePath, workDir).CombinedOutput(); err != nil {
		t.Fatalf("git clone source: %v\n%s", err, out)
	}
	writeAnswerFiles(t, workDir,
		"docker-permission-denied", "docker", "go", "go-1.26", "verified",
		"Use chmod.", "Tested.", `{}`)
	writeAnswerFiles(t, workDir,
		"k8s-crashloop", "k8s", "python", "py-3.11", "verified",
		"Check resource limits.", "Tested in minikube.", `{}`)
	writeAnswerFiles(t, workDir,
		"segfault-on-start", "bare-metal", "rust", "rust-1.75", "verified",
		"Recompile with debug symbols.", "GDB confirmed.", `{}`)
	commitAndPush(t, workDir, "main")

	localDir := filepath.Join(t.TempDir(), "clone")
	e := NewEngine(Config{
		RepoURL:  barePath,
		Branch:   "main",
		LocalDir: localDir,
	}, store)

	res, err := e.Import(context.Background())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Added != 3 {
		t.Errorf("Added = %d, want 3", res.Added)
	}

	// Verify all three classes were created.
	ctx := context.Background()
	for _, title := range []string{"docker-permission-denied", "k8s-crashloop", "segfault-on-start"} {
		if _, err := store.GetProblemClassByTitle(ctx, title); err != nil {
			t.Errorf("problem class %q not found after import: %v", title, err)
		}
	}
}

// --- Parsing tests ---------------------------------------------------------

func TestParseSolutionMD(t *testing.T) {
	content := `# Problem: test-problem

**Environment:** docker
**Language:** go go-1.26
**Status:** verified

---

## Problem Description

A description here.

---

## Solution

Do the thing.
`
	solution, lang, status := parseSolutionMD(content)
	if lang != "go" {
		t.Errorf("lang = %q, want go", lang)
	}
	if status != "verified" {
		t.Errorf("status = %q, want verified", status)
	}
	if !strings.Contains(solution, "Do the thing.") {
		t.Errorf("solution = %q, want to contain 'Do the thing.'", solution)
	}
}

func TestParseSolutionMD_EmptySolution(t *testing.T) {
	content := `# Problem: test

**Environment:** docker
**Language:** go go-1.26
**Status:** verified

---

## Solution

`
	solution, _, _ := parseSolutionMD(content)
	if solution != "" {
		t.Errorf("expected empty solution, got %q", solution)
	}
}

func TestParseEvidenceMD(t *testing.T) {
	content := `# Evidence

**Status:** verified
**Created:** 2026-07-02T12:00:00Z

---

All tests pass.
`
	ev := parseEvidenceMD(content)
	if !strings.Contains(ev, "All tests pass.") {
		t.Errorf("evidence = %q, want to contain 'All tests pass.'", ev)
	}
}

func TestParseEvidenceMD_NoSeparator(t *testing.T) {
	content := "Just some evidence text."
	ev := parseEvidenceMD(content)
	if !strings.Contains(ev, "Just some evidence text.") {
		t.Errorf("evidence without separator = %q", ev)
	}
}

func TestParseSignaturesJSON_Valid(t *testing.T) {
	raw := `{"v1":{"passed":true}}`
	out := ParseSignaturesJSON(raw)
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Errorf("ParseSignaturesJSON output is not valid JSON: %v", err)
	}
}

func TestParseSignaturesJSON_Empty(t *testing.T) {
	out := ParseSignaturesJSON("")
	if out != "{}" {
		t.Errorf("ParseSignaturesJSON('') = %q, want {}", out)
	}
}

func TestParseSignaturesJSON_Invalid(t *testing.T) {
	out := ParseSignaturesJSON("not-json")
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Errorf("ParseSignaturesJSON with invalid input should produce valid JSON: %v", err)
	}
}

func TestExtractSection(t *testing.T) {
	content := "Header\n\n## Solution\n\nThe answer.\n\n## Next\n\nMore."
	got := extractSection(content, "## Solution")
	if !strings.Contains(got, "The answer.") {
		t.Errorf("extractSection = %q, want to contain 'The answer.'", got)
	}
}

func TestExtractSection_NotFound(t *testing.T) {
	got := extractSection("no markers here", "## Solution")
	if got != "" {
		t.Errorf("extractSection without marker = %q, want empty", got)
	}
}

// --- Import detail count tests --------------------------------------------

func TestImportResult_Details(t *testing.T) {
	skipIfNoGit(t)
	store := makeStore(t)
	barePath := setupSourceRepo(t, "main")

	localDir := filepath.Join(t.TempDir(), "clone")
	e := NewEngine(Config{
		RepoURL:  barePath,
		Branch:   "main",
		LocalDir: localDir,
	}, store)

	res, err := e.Import(context.Background())
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(res.Details) != 1 {
		t.Fatalf("expected 1 detail entry, got %d", len(res.Details))
	}
	d := res.Details[0]
	if d.Action != ActionAdded {
		t.Errorf("detail Action = %q, want %q", d.Action, ActionAdded)
	}
	if d.ClassTitle != "docker-permission-denied" {
		t.Errorf("detail ClassTitle = %q, want docker-permission-denied", d.ClassTitle)
	}
	if d.AnswerID == 0 {
		t.Error("detail AnswerID = 0, want non-zero")
	}
}
