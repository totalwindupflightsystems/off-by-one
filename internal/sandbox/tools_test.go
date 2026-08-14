package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTools_KnownTool(t *testing.T) {
	// "sh" is guaranteed to exist on any Linux/Unix system. Its
	// realpath (/usr/bin/sh) is covered by DefaultReadOnlyPaths
	// (/usr), so after OB-GAP-035 it is deduped: NOT missing and
	// NOT emitted as an extra bind — the defaults already provide it.
	resolved, missing := ResolveTools([]string{"sh"}, nil)
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty (sh resolves and is covered by defaults)", missing)
	}
	if len(resolved) != 0 {
		t.Errorf("resolved = %v, want empty (sh is under /usr in DefaultReadOnlyPaths — deduped)", resolved)
	}
	// Sanity: the binary does exist on PATH (i.e. we are not passing
	// vacuously because sh is absent).
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not on PATH — unexpected")
	}
}

func TestResolveTools_UnknownTool(t *testing.T) {
	resolved, missing := ResolveTools([]string{"nonexistent-tool-xyz123"}, nil)
	if len(resolved) != 0 {
		t.Errorf("resolved = %v, want empty for unknown tool", resolved)
	}
	if len(missing) != 1 || missing[0] != "nonexistent-tool-xyz123" {
		t.Errorf("missing = %v, want [nonexistent-tool-xyz123]", missing)
	}
}

func TestResolveTools_Mixed(t *testing.T) {
	// Mix of known and unknown. sh is covered by the defaults
	// (deduped, not missing); the unknown tool is reported missing.
	resolved, missing := ResolveTools([]string{"sh", "nonexistent-xyz"}, nil)
	if len(resolved) != 0 {
		t.Errorf("resolved = %v, want empty (sh deduped by /usr default mount)", resolved)
	}
	if len(missing) != 1 || missing[0] != "nonexistent-xyz" {
		t.Errorf("missing = %v, want [nonexistent-xyz]", missing)
	}
}

func TestResolveTools_DedupeAlreadyMounted(t *testing.T) {
	// If /usr is already mounted, any tool resolved under /usr/bin
	// should be skipped (deduped).
	path, _ := exec.LookPath("sh")
	if path == "" {
		t.Skip("sh not on PATH")
	}
	// If sh is under /usr or /bin (which is covered by /usr on most distros),
	// ResolveTools with DefaultReadOnlyPaths should return empty.
	resolved, missing := ResolveTools([]string{"sh"}, DefaultReadOnlyPaths)
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty (sh should resolve)", missing)
	}
	// On most distros sh is at /usr/bin/sh or /bin/sh, both covered.
	if len(resolved) != 0 {
		t.Logf("sh resolved to %v despite DefaultReadOnlyPaths — checking if path is under /usr or /bin", resolved)
		for _, p := range resolved {
			if strings.HasPrefix(p, "/usr/") || strings.HasPrefix(p, "/bin") {
				t.Errorf("path %s should have been deduped by /usr or /bin", p)
			}
		}
	}
}

func TestResolveTools_EmptyAndWhitespace(t *testing.T) {
	// Empty/whitespace entries are ignored; sh still resolves (and is
	// deduped by the default mounts, so NOT reported missing).
	resolved, missing := ResolveTools([]string{"", "  ", "sh"}, nil)
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty", missing)
	}
	if len(resolved) != 0 {
		t.Errorf("resolved = %v, want empty (sh deduped by default mounts)", resolved)
	}
}

func TestResolveTools_GitSpecialCase(t *testing.T) {
	// git is in DefaultReadOnlyPaths — all its paths should be deduped.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	resolved, missing := ResolveTools([]string{"git"}, DefaultReadOnlyPaths)
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty for git (should resolve)", missing)
	}
	// git's binary is at /usr/bin/git (under /usr) and git-core is at
	// /usr/lib/git-core (under /usr/lib → under /usr). Both should be
	// deduped by /usr in DefaultReadOnlyPaths.
	for _, p := range resolved {
		if strings.HasPrefix(p, "/usr") {
			t.Errorf("git path %s should have been deduped by /usr mount", p)
		}
	}
}

func TestResolveTools_NilTools(t *testing.T) {
	resolved, missing := ResolveTools(nil, nil)
	if resolved != nil && len(resolved) > 0 {
		t.Errorf("resolved = %v, want empty for nil tools", resolved)
	}
	if missing != nil && len(missing) > 0 {
		t.Errorf("missing = %v, want empty for nil tools", missing)
	}
}

func TestResolveTools_Sorted(t *testing.T) {
	// Whatever ResolveTools returns (possibly empty, since default-
	// covered tools are deduped after OB-GAP-035), it must be sorted.
	resolved, _ := ResolveTools([]string{"sh", "ls"}, []string{"/nonexistent"})
	for i := 1; i < len(resolved); i++ {
		if resolved[i-1] > resolved[i] {
			t.Errorf("resolved not sorted: %v", resolved)
			break
		}
	}
}

// TestResolveTools_SymlinkOutsideMountSet is the OB-GAP-035 regression
// test: a tool whose exec.LookPath result is a symlink pointing OUTSIDE
// the mount set must be reported missing (degrade gracefully), never
// returned in resolved — previously the raw symlink path was passed to
// bwrap --ro-bind, which failed with "Can't create file ... No such file
// or directory" and instant-failed the whole solve.
func TestResolveTools_SymlinkOutsideMountSet(t *testing.T) {
	// Real symlink fixture: a fake bin dir containing a symlink whose
	// target lives outside any mounted path.
	binDir := t.TempDir()
	outside := t.TempDir() // target dir — under /tmp, NOT in DefaultReadOnlyPaths and not in our mount set
	target := filepath.Join(outside, "ob1-fake-tool")
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(target, []byte(script), 0o755); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(binDir, "ob1-fake-tool")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	t.Setenv("PATH", binDir)

	resolved, missing := ResolveTools([]string{"ob1-fake-tool"}, DefaultReadOnlyPaths)
	if len(resolved) != 0 {
		t.Errorf("resolved = %v, want empty (symlink target outside mount set must never be resolved)", resolved)
	}
	if len(missing) != 1 || missing[0] != "ob1-fake-tool" {
		t.Errorf("missing = %v, want [ob1-fake-tool]", missing)
	}
}

// TestResolveTools_SymlinkTargetCovered is the OB-GAP-035 positive case:
// a symlink whose realpath IS under an already-mounted path is deduped —
// neither resolved (no duplicate --ro-bind) nor reported missing.
func TestResolveTools_SymlinkTargetCovered(t *testing.T) {
	// Point a symlink from a non-mounted dir at /usr/bin/sh, whose
	// realpath is covered by DefaultReadOnlyPaths (/usr, /bin).
	realSh, err := filepath.EvalSymlinks("/usr/bin/sh")
	if err != nil {
		// Some minimal systems only have /bin/sh.
		var err2 error
		realSh, err2 = filepath.EvalSymlinks("/bin/sh")
		if err2 != nil {
			t.Skipf("neither /usr/bin/sh nor /bin/sh resolvable: %v / %v", err, err2)
		}
	}
	binDir := t.TempDir()
	link := filepath.Join(binDir, "ob1-sh-alias")
	if err := os.Symlink(realSh, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	t.Setenv("PATH", binDir)

	resolved, missing := ResolveTools([]string{"ob1-sh-alias"}, DefaultReadOnlyPaths)
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty (covered symlink target is not missing)", missing)
	}
	if len(resolved) != 0 {
		t.Errorf("resolved = %v, want empty (covered symlink target must be deduped)", resolved)
	}
}

// TestResolveTools_ExecutorExtrasCoverSymlink verifies the bsandbox_runner
// scenario from OB-GAP-035: a tool in $HOME/.local/bin symlinking into a
// project venv. The venv realpath is outside every plausible mount set, so
// the tool must be reported missing (WARN + degrade) whether or not the
// symlink's own directory is mounted — binding the symlink dir alone can
// never deliver the target.
func TestResolveTools_ExecutorExtrasCoverSymlink(t *testing.T) {
	home := t.TempDir() // stands in for $HOME
	venv := t.TempDir() // stands in for the project venv (outside mounts)
	target := filepath.Join(venv, "gitreins")
	if err := os.WriteFile(target, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write target: %v", err)
	}
	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0o755); err != nil {
		t.Fatalf("mkdir localBin: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(localBin, "gitreins")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	t.Setenv("PATH", localBin)

	// Without extras: target not covered -> missing (this used to be the
	// bwrap hard-fail before OB-GAP-035).
	resolved, missing := ResolveTools([]string{"gitreins"}, DefaultReadOnlyPaths)
	if len(resolved) != 0 || len(missing) != 1 || missing[0] != "gitreins" {
		t.Errorf("no-extras: resolved = %v, missing = %v; want empty / [gitreins]", resolved, missing)
	}

	// With executor extras covering the symlink dir: the realpath
	// (venv target) is still NOT covered, so it must STILL be dropped —
	// mounting the symlink dir alone cannot deliver the target.
	mountSet := append(append([]string{}, DefaultReadOnlyPaths...), localBin)
	resolved, missing = ResolveTools([]string{"gitreins"}, mountSet)
	if len(resolved) != 0 || len(missing) != 1 || missing[0] != "gitreins" {
		t.Errorf("extras-cover-link-dir: resolved = %v, missing = %v; want empty / [gitreins] (realpath uncovered)", resolved, missing)
	}
}

func TestIsPathCovered(t *testing.T) {
	mounted := map[string]bool{
		"/usr": true,
		"/lib": true,
		"/bin": true,
		"/etc": true,
	}
	tests := []struct {
		path string
		want bool
	}{
		{"/usr/bin/git", true},      // under /usr
		{"/usr/lib/git-core", true}, // under /usr
		{"/usr", true},              // exact match
		{"/opt/jq", false},          // not covered
		{"/etc/resolv.conf", true},  // exact match with /etc... wait, /etc is a dir
		{"/home/user/.local/bin/jq", false},
	}
	for _, tt := range tests {
		got := isPathCovered(tt.path, mounted)
		if got != tt.want {
			t.Errorf("isPathCovered(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
