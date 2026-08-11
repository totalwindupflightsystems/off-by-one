package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLooksPlaceholderAPIKey covers the startup key-validation
// heuristic: empty, short, and placeholder-marked keys must be flagged;
// a realistic long sk- key must pass without a warning.
func TestLooksPlaceholderAPIKey(t *testing.T) {
	realistic := "sk-" + strings.Repeat("a1", 24) // 51 chars, no markers
	cases := []struct {
		name string
		key  string
		want bool
	}{
		{"empty", "", true},
		{"whitespace", "   ", true},
		{"bare prefix", "sk-", true},
		{"short", "sk-abc123", true},
		{"your-key", "your-key", true},
		{"changeme", "changeme", true},
		{"long your marker", "sk-your-key-here-" + strings.Repeat("x", 20), true},
		{"long changeme marker", "sk-changeme-" + strings.Repeat("x", 20), true},
		{"placeholder marker", "sk-placeholder-" + strings.Repeat("x", 20), true},
		{"realistic", realistic, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksPlaceholderAPIKey(tc.key); got != tc.want {
				t.Errorf("looksPlaceholderAPIKey(%q): got %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

// TestExtraReadOnlyPaths_IncludesExistingLocalBin asserts that
// $HOME/.local/bin is mounted read-only when it exists on the host.
func TestExtraReadOnlyPaths_IncludesExistingLocalBin(t *testing.T) {
	home := t.TempDir()
	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("HOME", home)

	paths := extraReadOnlyPaths()
	if len(paths) != 3 {
		t.Fatalf("paths: got %v, want 3 entries", paths)
	}
	if paths[0] != localBin {
		t.Errorf("paths[0]: got %q, want %q", paths[0], localBin)
	}
	if paths[1] != "/tmp/pi" || paths[2] != "/etc" {
		t.Errorf("static mounts: got %v, want [/tmp/pi /etc] after localBin", paths[1:])
	}
}

// TestExtraReadOnlyPaths_SkipsMissingLocalBin asserts that a missing
// $HOME/.local/bin is omitted (a bind-mount of a nonexistent path would
// fail sandbox creation), while the static mounts remain.
func TestExtraReadOnlyPaths_SkipsMissingLocalBin(t *testing.T) {
	home := t.TempDir() // no .local/bin inside
	t.Setenv("HOME", home)

	paths := extraReadOnlyPaths()
	if len(paths) != 2 {
		t.Fatalf("paths: got %v, want 2 entries", paths)
	}
	if paths[0] != "/tmp/pi" || paths[1] != "/etc" {
		t.Errorf("paths: got %v, want [/tmp/pi /etc]", paths)
	}
}

// TestExtraReadOnlyPaths_SkipsFileLocalBin asserts a regular file at
// $HOME/.local/bin (not a directory) is not mounted.
func TestExtraReadOnlyPaths_SkipsFileLocalBin(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".local"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".local", "bin"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("HOME", home)

	paths := extraReadOnlyPaths()
	if len(paths) != 2 {
		t.Fatalf("paths: got %v, want 2 entries (file .local/bin skipped)", paths)
	}
}
