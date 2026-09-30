package graph

import (
	"context"
	"testing"
)

// REVIEW-OB-010: env/version are exact-match discovery filters (see
// discovery.go — bestAnswer ranks by (env, lang, version) tuple). A
// free-text prose value makes the answer unreachable via tuple-scoped
// discovery and leaks host context into the public catalog. NormalizeEnv
// and NormalizeVersion map prose to a canonical token when a recognizable
// pattern exists and drop it to "" (the discovery wildcard) otherwise;
// token-shaped values (no whitespace, ';' or '~') pass through lowercased.

func TestNormalizeEnv(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// already-canonical tokens pass through (lowercased/trimmed)
		{"clean token", "docker", "docker"},
		{"clean token uppercase", "Linux", "linux"},
		{"clean token padded", "  node20  ", "node20"},
		{"compound token", "linux/amd64", "linux/amd64"},
		{"hyphenated token", "postgresql-16", "postgresql-16"},
		{"fused token", "node20-typescript-vitest-sqlite", "node20-typescript-vitest-sqlite"},
		{"empty", "", ""},

		// leading segment is itself a clean token
		{"comma prose after token", "Linux, crier repo ~/crier", "linux"},
		{"semicolon prose after token", "linux; go1.26; GNU coreutils", "linux"},
		{"comma list of tokens", "postgres-16, pgx, chi, zerolog/hlog", "postgres-16"},

		// recognizable prose maps to a canonical token
		{"github actions prose", "~/auger; DuckBrain live substrate; GitHub Actions exact implementation SHA passed", "github-actions"},
		{"ci on github actions", "chimera-v2 integration tests, pytest + httpx, CI on GitHub Actions", "github-actions"},
		{"actions runner", "self-hosted GitHub Actions runner on a shared Linux host", "github-actions"},
		{"docker prose", "docker compose stack on a linux host", "docker"},
		{"linux host prose", "linux host, go 1.26.6", "linux"},
		{"wsl maps to linux", "n100 wsl2; scheduler dispatch via bash", "linux"},
		{"ubuntu maps to linux", "ubuntu linux host, python 3.11 client", "linux"},
		{"darwin prose", "macOS 15 arm64 laptop", "darwin"},
		{"windows prose", "windows 11 pro workstation", "windows"},
		{"node prose", "node 22, vitest, TS 7 strict", "node"},
		{"vitest prose", "vitest 4 + node 22, repo @ 18c45f7", "node"},
		{"python prose", "python 3.11, uv, pytest", "python3"},
		{"go 1.26 prose", "repo with go 1.26 toolchain and make", "go1.26"},
		{"go prose", "Go incident-response service with PUT semantics", "go"},
		{"module path segment wins", "modernc.org/sqlite, Go service, PUT semantics", "modernc.org/sqlite"},
		{"bash prose", "set -uo pipefail bash hermetic test", "bash"},
		{"shell prose", "posix-sh repo guard script", "linux"},

		// prose with no recognizable pattern drops to the wildcard
		{"unrecognizable prose", "warpfs (hilo) Rust workspace", ""},
		{"host path only", "~/auger", ""},

		// REVIEW-OB-009: a token carrying a redaction placeholder is not a
		// meaningful filter value — drop to the wildcard.
		{"placeholder token", "hilo/<project>", ""},
		{"placeholder leading segment", "hilo/<project>, Rust workspace, tracing-subscriber 0.3.23", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeEnv(tc.in); got != tc.want {
				t.Errorf("NormalizeEnv(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		// clean tokens pass through
		{"latest", "latest", "latest"},
		{"semver", "0.14.0", "0.14.0"},
		{"v-prefixed", "v0.1.1", "v0.1.1"},
		{"fused tool token", "go1.26", "go1.26"},
		{"bare major", "20", "20"},
		{"empty", "", ""},

		// prose: first version-shaped whitespace token wins
		{"tool then version", "gitreins 0.14.0", "0.14.0"},
		{"runtime then major", "node 22", "22"},
		{"v-prefixed in prose", "h3-test v1.0.0", "v1.0.0"},
		{"go prose version", "go 1.26.6 toolchain", "1.26.6"},

		// branch tokens survive prose
		{"branch with sha", "main e50aea4", "main"},

		// host-context prose drops entirely
		{"api endpoint with verified stamp", "api.deepseek.com/v1, verified 2026-09-15T02:39Z", ""},
		{"pure prose", "tick 520 follow-up notes", "520"}, // first numeric token wins — deterministic
		{"no version token", "verified on the shared host", ""},

		// REVIEW-OB-009: placeholders are not filter tokens — a real
		// version-shaped field still wins, a bare placeholder drops.
		{"placeholder then version", "<tool> 0.14.0", "0.14.0"},
		{"placeholder token only", "hilo/<project>", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeVersion(tc.in); got != tc.want {
				t.Errorf("NormalizeVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// The store is the write funnel for every producer (solver, seed, import):
// a prose env/version submitted through CreateAnswerNode must be stored
// normalized so new solves cannot regress the corpus (REVIEW-OB-010).
func TestStore_CreateAnswerNode_NormalizesEnvVersion(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, err := s.CreateProblemClass(ctx, "normalize-funnel", "")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}

	id, err := s.CreateAnswerNode(ctx, cid, 0,
		"Linux, crier repo ~/crier", "go", "api.deepseek.com/v1, verified 2026-09-15T02:39Z",
		"sol", "ev", "{}")
	if err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}
	a, err := s.GetAnswerNode(ctx, id)
	if err != nil {
		t.Fatalf("GetAnswerNode: %v", err)
	}
	if a.Env != "linux" {
		t.Errorf("env = %q, want %q", a.Env, "linux")
	}
	if a.Version != "" {
		t.Errorf("version = %q, want dropped to empty wildcard", a.Version)
	}
}
