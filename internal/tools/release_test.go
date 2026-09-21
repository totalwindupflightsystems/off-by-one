// Package tools hosts release-tooling regression tests (RELEASE-OB-002).
//
// There are no runtime .go files here on purpose: the package exists only so
// `go test ./...` can gate the `make release` target's logic. The tests parse
// the repository's real Makefile (located relative to this file) instead of
// invoking make, which keeps them hermetic, deterministic, and well under the
// 5s budget — no subprocesses, no temp clones, no network.
package tools

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// loadMakefile returns the content of the repo-root Makefile.
func loadMakefile(t *testing.T) string {
	t.Helper()
	// This file lives at <repo>/internal/tools/release_test.go.
	root := filepath.Join("..", "..")
	path := filepath.Join(root, "Makefile")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// recipeLines extracts the (tab-prefixed, backslash-continued) recipe lines of
// the named target, with the leading tab and line-continuation backslashes
// stripped. Non-recipe lines (including the target's own doc comments and
// other targets) are excluded.
func recipeLines(t *testing.T, makefile, target string) []string {
	t.Helper()
	lines := strings.Split(makefile, "\n")
	header := regexp.MustCompile(`^` + regexp.QuoteMeta(target) + `:`)
	var recipe []string
	inTarget := false
	for _, ln := range lines {
		switch {
		case header.MatchString(ln):
			inTarget = true
		case inTarget && ln != "" && !strings.HasPrefix(ln, "\t"):
			// First non-recipe line ends the target's recipe.
			inTarget = false
		case inTarget:
			recipe = append(recipe, strings.TrimSuffix(strings.TrimPrefix(ln, "\t"), "\\"))
		}
	}
	if len(recipe) == 0 {
		t.Fatalf("target %q has no recipe lines in Makefile", target)
	}
	return recipe
}

func joined(t *testing.T, makefile, target string) string {
	t.Helper()
	return strings.Join(recipeLines(t, makefile, target), "\n")
}

// TestReleaseTargetExists fails if the release target disappears.
func TestReleaseTargetExists(t *testing.T) {
	mk := loadMakefile(t)
	if !strings.Contains(mk, "\nrelease:") {
		t.Fatal("Makefile has no `release:` target")
	}
}

// TestReleaseRequiresTAG is the in-file encoding of AC1: a missing TAG must
// exit non-zero with a clear usage message BEFORE any other gate runs.
func TestReleaseRequiresTAG(t *testing.T) {
	mk := loadMakefile(t)
	rec := joined(t, mk, "release")

	first := recipeLines(t, mk, "release")[0]
	if !strings.Contains(first, `"${TAG:-}"`) && !strings.Contains(first, "$${TAG:-}") {
		t.Fatalf("first release recipe line must test TAG presence, got: %q", first)
	}
	if !strings.Contains(rec, "exit 1") {
		t.Fatal("missing TAG must exit non-zero (exit 1)")
	}
	if !strings.Contains(rec, "TAG is required") {
		t.Fatal("missing TAG must produce a clear message naming TAG")
	}
	// AC4 support: DRY_RUN=1 must be honored and must print the push command
	// without creating the tag.
	if !strings.Contains(rec, `DRY_RUN`) {
		t.Fatal("release target must support DRY_RUN mode for verification")
	}
	if !strings.Contains(rec, "git push origin") {
		t.Fatal("release target must print the push command for the human")
	}
	if strings.Contains(rec, "git push ") && !strings.Contains(rec, "echo") && !strings.Contains(rec, "Next:") {
		t.Fatal("push must be printed, not executed, by the release target")
	}
}

// TestReleaseValidatesSemver is the in-file encoding of AC2: TAG values that
// are not v<semver> must be rejected.
func TestReleaseValidatesSemver(t *testing.T) {
	mk := loadMakefile(t)
	rec := joined(t, mk, "release")

	if !strings.Contains(rec, "case") {
		t.Fatal("semver validation must use a case pattern so a malformed TAG cannot inject shell")
	}
	if !strings.Contains(rec, "not a valid release tag") {
		t.Fatal("invalid TAG must produce a clear rejection message")
	}
	if !strings.Contains(rec, "exit 1") {
		t.Fatal("invalid TAG must exit non-zero")
	}
	// A v-prefixed three-component digit shape must be the accepted form.
	pat := regexp.MustCompile(`v\[0-9\]\*`)
	if !pat.MatchString(rec) {
		t.Fatal("semver pattern must accept v<digit>... shapes (e.g. v0.1.0)")
	}
}

// TestReleaseDirtyTreeGuard is the in-file encoding of AC3: a non-empty
// `git status --porcelain` (i.e. anything outside .gitignore) must abort the
// release.
func TestReleaseDirtyTreeGuard(t *testing.T) {
	mk := loadMakefile(t)
	rec := joined(t, mk, "release")

	if !strings.Contains(rec, "git status --porcelain") {
		t.Fatal("release target must gate on `git status --porcelain`")
	}
	if !strings.Contains(rec, "dirty") {
		t.Fatal("dirty-tree failure must name the condition")
	}
}

// TestReleaseRejectsExistingTag: re-tagging an existing tag must fail.
func TestReleaseRejectsExistingTag(t *testing.T) {
	mk := loadMakefile(t)
	rec := joined(t, mk, "release")

	if !strings.Contains(rec, "refs/tags/") {
		t.Fatal("release target must check the tag does not already exist")
	}
	if !strings.Contains(rec, "already exists") {
		t.Fatal("existing-tag failure must be named")
	}
}

// TestReleaseGatesRunTestsBeforeTagging: the full (not -short) suite must run
// BEFORE `git tag`, and a test failure must abort (no tag).
func TestReleaseGatesRunTestsBeforeTagging(t *testing.T) {
	mk := loadMakefile(t)
	rec := joined(t, mk, "release")

	// LastIndex, not Index: the DRY_RUN preview lines merely mention "git tag
	// -a" and the test command before they run; the real invocations are the
	// last occurrences in the recipe.
	goTest := strings.LastIndex(rec, "go test -count=1 ./...")
	gitTag := strings.LastIndex(rec, "git tag -a")
	if goTest < 0 {
		t.Fatal("release target must run the full test suite (go test -count=1 ./...)")
	}
	if strings.Contains(rec, "go test") && strings.Contains(rec, "-short") {
		t.Fatal("release target must run the FULL suite, not -short")
	}
	if gitTag < 0 {
		t.Fatal("release target must create the annotated tag with git tag -a")
	}
	if gitTag < goTest {
		t.Fatal("the tag must be created only AFTER the test suite passes")
	}
	if !strings.Contains(rec, "exit 1") {
		t.Fatal("test/build failure must abort the release")
	}
}

// TestReleaseTagMessageFromChangelog: the annotated tag message must come from
// the matching `## [<TAG>]` CHANGELOG section, and a missing section must
// abort.
func TestReleaseTagMessageFromChangelog(t *testing.T) {
	mk := loadMakefile(t)
	rec := joined(t, mk, "release")

	if !strings.Contains(rec, "CHANGELOG.md") {
		t.Fatal("release target must read CHANGELOG.md for the tag message")
	}
	if !strings.Contains(rec, "no CHANGELOG.md section") {
		t.Fatal("missing CHANGELOG section must fail with a clear message")
	}
	// The section lookup must be anchored to the TAG value (`## [<TAG>]`),
	// not a fixed version string.
	if !strings.Contains(rec, "## ") || !strings.Contains(rec, "$TAG") && !strings.Contains(rec, "${TAG}") {
		t.Fatal("CHANGELOG section lookup must be driven by the TAG value")
	}
}
