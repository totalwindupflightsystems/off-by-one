package sandbox

import (
	"os/exec"
	"strings"
	"testing"
)

func TestResolveTools_KnownTool(t *testing.T) {
	// "sh" is guaranteed to exist on any Linux/Unix system.
	resolved, missing := ResolveTools([]string{"sh"}, nil)
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty", missing)
	}
	if len(resolved) == 0 {
		t.Fatal("expected at least one resolved path for 'sh'")
	}
	// The resolved path should be an absolute path (from exec.LookPath).
	path, _ := exec.LookPath("sh")
	if path == "" {
		t.Skip("sh not on PATH — unexpected")
	}
	found := false
	for _, p := range resolved {
		if p == path {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("resolved = %v, want to contain %s", resolved, path)
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
	// Mix of known and unknown.
	resolved, missing := ResolveTools([]string{"sh", "nonexistent-xyz"}, nil)
	if len(resolved) == 0 {
		t.Error("expected sh to resolve")
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
	resolved, missing := ResolveTools([]string{"", "  ", "sh"}, nil)
	if len(missing) != 0 {
		t.Errorf("missing = %v, want empty", missing)
	}
	if len(resolved) == 0 {
		t.Error("expected sh to resolve despite empty/whitespace entries")
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
	// Multiple tools should produce sorted output.
	resolved, _ := ResolveTools([]string{"sh", "ls"}, []string{"/nonexistent"})
	if len(resolved) < 1 {
		t.Skip("neither sh nor ls resolved")
	}
	for i := 1; i < len(resolved); i++ {
		if resolved[i-1] > resolved[i] {
			t.Errorf("resolved not sorted: %v", resolved)
			break
		}
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
