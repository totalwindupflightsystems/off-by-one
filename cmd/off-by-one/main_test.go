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
	want := expectedMounts(localBin)
	if len(paths) != len(want) {
		t.Fatalf("paths: got %v, want %d entries (%v)", paths, len(want), want)
	}
	if paths[0] != localBin {
		t.Errorf("paths[0]: got %q, want %q", paths[0], localBin)
	}
	for i, w := range want {
		if paths[i] != w {
			t.Errorf("paths[%d]: got %q, want %q", i, paths[i], w)
		}
	}
}

// TestExtraReadOnlyPaths_SkipsMissingLocalBin asserts that a missing
// $HOME/.local/bin is omitted (a bind-mount of a nonexistent path would
// fail sandbox creation), while the static mounts remain.
func TestExtraReadOnlyPaths_SkipsMissingLocalBin(t *testing.T) {
	home := t.TempDir() // no .local/bin inside
	t.Setenv("HOME", home)

	paths := extraReadOnlyPaths()
	want := expectedMounts("")
	if len(paths) != len(want) {
		t.Fatalf("paths: got %v, want %d entries (%v)", paths, len(want), want)
	}
	for i, w := range want {
		if paths[i] != w {
			t.Errorf("paths[%d]: got %q, want %q", i, paths[i], w)
		}
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
	want := expectedMounts("")
	if len(paths) != len(want) {
		t.Fatalf("paths: got %v, want %d entries (file .local/bin skipped; %v)", paths, len(want), want)
	}
}

// TestExtraReadOnlyPaths_IncludesResolverWhenPresent asserts that
// /run/systemd/resolve (the target of /etc/resolv.conf on systemd-resolved
// hosts) is mounted when the directory exists — bwrap tmpfs's /run, so
// without the bind the sandbox has no DNS.
func TestExtraReadOnlyPaths_IncludesResolverWhenPresent(t *testing.T) {
	if _, err := os.Stat("/run/systemd/resolve"); err != nil || !isDir("/run/systemd/resolve") {
		t.Skip("/run/systemd/resolve not present on this host")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	paths := extraReadOnlyPaths()
	found := false
	for _, p := range paths {
		if p == "/run/systemd/resolve" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("paths: %v — want /run/systemd/resolve included when the dir exists", paths)
	}
}

// expectedMounts returns the mount set extraReadOnlyPaths should produce
// for the given (possibly empty) localBin path, mirroring the function's
// ordering: resolver (if present), localBin (if present), /tmp/pi, /etc.
func expectedMounts(localBin string) []string {
	paths := []string{"/tmp/pi", "/etc"}
	if isDir("/run/systemd/resolve") {
		paths = append([]string{"/run/systemd/resolve"}, paths...)
	}
	if localBin != "" {
		paths = append([]string{localBin}, paths...)
	}
	return paths
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
