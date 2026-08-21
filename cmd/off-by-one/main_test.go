package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/sandbox"
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
// $HOME/.local/bin is mounted read-only when it exists on the host, and
// /etc is always mounted (issue #1 pt 9: /tmp/pi is conditional, covered
// by the file-bin test below).
func TestExtraReadOnlyPaths_IncludesExistingLocalBin(t *testing.T) {
	home := t.TempDir()
	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("HOME", home)

	paths := extraReadOnlyPaths()
	if !slices.Contains(paths, localBin) {
		t.Fatalf("paths: existing .local/bin must be mounted, got %v", paths)
	}
	if !slices.Contains(paths, "/etc") {
		t.Fatalf("paths: /etc must always be mounted, got %v", paths)
	}
}

// TestExtraReadOnlyPaths_SkipsMissingLocalBin asserts that a missing
// $HOME/.local/bin is omitted (a bind-mount of a nonexistent path would
// fail sandbox creation), while /etc remains.
func TestExtraReadOnlyPaths_SkipsMissingLocalBin(t *testing.T) {
	home := t.TempDir() // no .local/bin inside
	t.Setenv("HOME", home)

	paths := extraReadOnlyPaths()
	if slices.Contains(paths, filepath.Join(home, ".local", "bin")) {
		t.Fatalf("paths: missing .local/bin must be omitted, got %v", paths)
	}
	if !slices.Contains(paths, "/etc") {
		t.Fatalf("paths: /etc must always be mounted, got %v", paths)
	}
}

// TestExtraReadOnlyPaths_SkipsFileLocalBin asserts a regular file at
// $HOME/.local/bin (not a directory) is not mounted, and that /etc is
// always mounted while /tmp/pi is conditional on its presence (issue #1
// pt 9 — a wiped /tmp/pi must never fail sandbox creation).
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
	if !slices.Contains(paths, "/etc") {
		t.Fatalf("paths: /etc must always be mounted, got %v", paths)
	}
	for _, p := range paths {
		if p == filepath.Join(home, ".local", "bin") {
			t.Fatalf("paths: file .local/bin must be skipped, got %v", paths)
		}
	}
	_, err := os.Stat("/tmp/pi")
	hasTmpPi := slices.Contains(paths, "/tmp/pi")
	if err == nil && !hasTmpPi {
		t.Fatalf("paths: /tmp/pi exists on host but was not mounted: %v", paths)
	}
	if err != nil && hasTmpPi {
		t.Fatalf("paths: /tmp/pi absent on host but still mounted: %v", paths)
	}
}

// TestSandboxTimeout covers OB1_BWRAP_TIMEOUT parsing: unset → default,
// valid seconds → override, invalid → default with warning (issue #1 pt 8).
func TestSandboxTimeout(t *testing.T) {
	t.Setenv("OB1_BWRAP_TIMEOUT", "")
	if got := sandboxTimeout(); got != sandbox.DefaultBwrapTimeout {
		t.Fatalf("unset: got %s, want default %s", got, sandbox.DefaultBwrapTimeout)
	}
	t.Setenv("OB1_BWRAP_TIMEOUT", "900")
	if got := sandboxTimeout(); got != 900*time.Second {
		t.Fatalf("900: got %s, want 15m", got)
	}
	t.Setenv("OB1_BWRAP_TIMEOUT", "abc")
	if got := sandboxTimeout(); got != sandbox.DefaultBwrapTimeout {
		t.Fatalf("invalid: got %s, want default %s", got, sandbox.DefaultBwrapTimeout)
	}
	t.Setenv("OB1_BWRAP_TIMEOUT", "-5")
	if got := sandboxTimeout(); got != sandbox.DefaultBwrapTimeout {
		t.Fatalf("negative: got %s, want default %s", got, sandbox.DefaultBwrapTimeout)
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

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// TestPrintUsage_ShowsSeedSubcommand asserts that the --help output
// lists a Commands section naming the `seed` subcommand before the
// server flags — the default flag-package usage printed only the flags,
// so users never learned seed existed (OB-GAP-053).
func TestPrintUsage_ShowsSeedSubcommand(t *testing.T) {
	var buf bytes.Buffer
	flag.CommandLine.SetOutput(&buf)
	defer flag.CommandLine.SetOutput(os.Stderr)

	printUsage()

	out := buf.String()
	for _, want := range []string{"Usage of off-by-one:", "Commands:", "seed", "Flags:"} {
		if !strings.Contains(out, want) {
			t.Errorf("usage output missing %q, got:\n%s", want, out)
		}
	}
	// The Commands section must come before the flag dump.
	if strings.Index(out, "Commands:") > strings.Index(out, "Flags:") {
		t.Errorf("usage output: Commands section must precede Flags, got:\n%s", out)
	}
}
