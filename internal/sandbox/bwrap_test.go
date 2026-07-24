package sandbox

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeFakeBwrap writes a shell script that mimics bwrap's behavior
// for the args we care about: bind the workspace at /workspace and
// execute the command. Returns the path to the script.
//
// The fake "bwrap" sets BWRAP_WORKSPACE=<workspace> in the child's
// environment and execs the command. Tests use this to assert
// bwrap's args are correct and to capture the child command's
// stdout/stderr.
func makeFakeBwrap(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "bwrap")
	// The fake bwrap emulates the flag-parse part: scan for the
	// last --bind SRC DST pair (workspace) and for the separator
	// before the real command. Then exec the real command with
	// BWRAP_WORKSPACE set.
	script := "#!/bin/sh\n" +
		`WS=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "--bind" ]; then
    WS="$arg"
  fi
  prev="$arg"
done
export BWRAP_WORKSPACE="$WS"
# Strip everything up to and including the separator, then exec.
while [ "$#" -gt 0 ] && [ "$1" != "--" ]; do
  shift
done
if [ "$#" -gt 0 ]; then
  shift
fi
if [ "$#" -gt 0 ]; then
  exec "$@"
fi
echo "fake bwrap: no command after separator" >&2
exit 127
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake bwrap: %v", err)
	}
	return path
}

func TestExecutor_Create_Workspace(t *testing.T) {
	x := &Executor{BwrapPath: "/bin/true"}
	s, err := x.Create(context.Background(), "test-ws", Config{BwrapPath: "/bin/true"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.WorkDir() == "" {
		t.Error("WorkDir is empty")
	}
	if !strings.Contains(s.WorkDir(), "off-by-one-sandbox-test-ws") {
		t.Errorf("WorkDir = %q, want off-by-one-sandbox-test-ws", s.WorkDir())
	}
	if _, err := os.Stat(s.WorkDir()); err != nil {
		t.Errorf("workspace dir not created: %v", err)
	}
	if err := s.Destroy(); err != nil {
		t.Errorf("destroy: %v", err)
	}
	if _, err := os.Stat(s.WorkDir()); !os.IsNotExist(err) {
		t.Errorf("workspace still exists after destroy: %v", err)
	}
}

func TestExecutor_Create_BwrapNotFound(t *testing.T) {
	x := &Executor{}
	_, err := x.Create(context.Background(), "x", Config{BwrapPath: "/nonexistent/bwrap-binary"})
	if err != ErrBwrapNotFound {
		t.Errorf("err = %v, want ErrBwrapNotFound", err)
	}
}

func TestExecutor_Create_DefaultBwrapPath(t *testing.T) {
	// Empty config + executor that has no bwrap path: should fall
	// back to /usr/bin/bwrap and fail with ErrBwrapNotFound only
	// if bwrap is not installed. In CI / dev, bwrap is installed.
	x := &Executor{}
	_, err := x.Create(context.Background(), "x", Config{})
	if err != nil {
		// Either ErrBwrapNotFound (bwrap missing) or success — both
		// acceptable. We only assert no panic.
		t.Logf("create with default path: %v (acceptable if bwrap installed)", err)
	}
}

func TestSandbox_CopyIn_CopyOut(t *testing.T) {
	x := &Executor{BwrapPath: "/bin/true"}
	s, err := x.Create(context.Background(), "copy-test", Config{BwrapPath: "/bin/true"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = s.Destroy() }()

	// CopyIn writes to workspace.
	want := []byte("hello world")
	if err := s.CopyIn("problem.json", want); err != nil {
		t.Fatalf("copy in: %v", err)
	}
	got, err := s.CopyOut("problem.json")
	if err != nil {
		t.Fatalf("copy out: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSandbox_CopyIn_NestedPath(t *testing.T) {
	x := &Executor{BwrapPath: "/bin/true"}
	s, err := x.Create(context.Background(), "nested", Config{BwrapPath: "/bin/true"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = s.Destroy() }()

	if err := s.CopyIn("sub/dir/file.txt", []byte("nested")); err != nil {
		t.Errorf("nested copy_in: %v", err)
	}
	got, err := s.CopyOut("sub/dir/file.txt")
	if err != nil {
		t.Errorf("nested copy_out: %v", err)
	}
	if string(got) != "nested" {
		t.Errorf("got %q, want nested", got)
	}
}

func TestSandbox_CopyIn_AfterDestroyFails(t *testing.T) {
	x := &Executor{BwrapPath: "/bin/true"}
	s, err := x.Create(context.Background(), "destroyed", Config{BwrapPath: "/bin/true"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Destroy(); err != nil {
		t.Fatalf("destroy: %v", err)
	}
	if err := s.CopyIn("a.txt", []byte("x")); err != ErrAlreadyDestroyed {
		t.Errorf("CopyIn after destroy: err = %v, want ErrAlreadyDestroyed", err)
	}
	if _, err := s.CopyOut("a.txt"); err != ErrAlreadyDestroyed {
		t.Errorf("CopyOut after destroy: err = %v, want ErrAlreadyDestroyed", err)
	}
}

func TestSandbox_CopyInFile(t *testing.T) {
	x := &Executor{BwrapPath: "/bin/true"}
	s, err := x.Create(context.Background(), "copyfile", Config{BwrapPath: "/bin/true"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = s.Destroy() }()

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "source.txt")
	if err := os.WriteFile(srcPath, []byte("from src"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := s.CopyInFile("imported.txt", srcPath); err != nil {
		t.Fatalf("copy in file: %v", err)
	}
	got, err := s.CopyOut("imported.txt")
	if err != nil {
		t.Fatalf("copy out: %v", err)
	}
	if string(got) != "from src" {
		t.Errorf("got %q, want from src", got)
	}
}

func TestSandbox_Run_RealBwrap(t *testing.T) {
	if !BwrapAvailable() {
		t.Skip("bwrap not installed on this system")
	}
	x := &Executor{BwrapPath: lookupBwrap(), WorkDir: t.TempDir(), Timeout: 30 * time.Second}
	s, err := x.Create(context.Background(), "real-bwrap", Config{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = s.Destroy() }()

	// Run a simple command. `/bin/echo` is in the ro-bind set
	// (/bin). The output should be "hello from bwrap".
	stdout, _, err := s.Run(context.Background(), "/bin/echo", "hello from bwrap")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if strings.TrimSpace(string(stdout)) != "hello from bwrap" {
		t.Errorf("stdout = %q, want 'hello from bwrap'", string(stdout))
	}
}

func TestSandbox_Run_Timeout(t *testing.T) {
	if !BwrapAvailable() {
		t.Skip("bwrap not installed on this system")
	}
	x := &Executor{BwrapPath: lookupBwrap(), WorkDir: t.TempDir(), Timeout: 1 * time.Second}
	s, err := x.Create(context.Background(), "timeout", Config{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = s.Destroy() }()

	// `/bin/sleep 5` should be killed by the 1s context timeout.
	_, _, err = s.Run(context.Background(), "/bin/sleep", "5")
	if err == nil {
		t.Error("run: expected timeout error, got nil")
	}
}

func TestSandbox_Run_CommandFailure(t *testing.T) {
	if !BwrapAvailable() {
		t.Skip("bwrap not installed on this system")
	}
	x := &Executor{BwrapPath: lookupBwrap(), WorkDir: t.TempDir(), Timeout: 10 * time.Second}
	s, err := x.Create(context.Background(), "fail", Config{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = s.Destroy() }()

	// /bin/false always returns non-zero.
	_, _, err = s.Run(context.Background(), "/bin/false")
	if err == nil {
		t.Error("run: expected non-zero exit, got nil")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("err = %T, want *exec.ExitError", err)
	} else if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
	}
}

func TestSandbox_Run_FakeBwrap(t *testing.T) {
	// Use the fake bwrap script to assert the args we pass.
	fakeBwrap := makeFakeBwrap(t)
	x := &Executor{BwrapPath: fakeBwrap, WorkDir: t.TempDir(), Timeout: 5 * time.Second}
	s, err := x.Create(context.Background(), "fake", Config{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = s.Destroy() }()

	// /bin/sh -c 'echo $BWRAP_WORKSPACE' should print the workspace path.
	stdout, stderr, err := s.Run(context.Background(), "/bin/sh", "-c", "echo $BWRAP_WORKSPACE")
	if err != nil {
		t.Fatalf("run: %v, stderr = %s", err, stderr)
	}
	got := strings.TrimSpace(string(stdout))
	if !strings.Contains(got, "off-by-one-sandbox-fake") {
		t.Errorf("BWRAP_WORKSPACE = %q, want to contain off-by-one-sandbox-fake", got)
	}
}

func TestBuildBwrapArgs(t *testing.T) {
	workDir := "/tmp/work"
	args := buildBwrapArgs([]string{"/usr", "/bin", "/etc"}, nil, workDir)
	// Required flags in any order.
	checks := map[string]bool{
		"--unshare-all":     false,
		"--die-with-parent": false,
		"--new-session":     false,
		"--proc":            false,
		"--dev":             false,
	}
	for _, a := range args {
		if _, ok := checks[a]; ok {
			checks[a] = true
		}
	}
	for k, seen := range checks {
		if !seen {
			t.Errorf("missing required flag: %s", k)
		}
	}
	// Workspace should be bound at /workspace.
	wsBound := false
	for i, a := range args {
		if a == "--bind" && i+2 < len(args) && args[i+1] == workDir && args[i+2] == "/workspace" {
			wsBound = true
		}
	}
	if !wsBound {
		t.Errorf("--bind %s /workspace not found in args: %v", workDir, args)
	}
	// All ReadOnlyPaths should produce --ro-bind SRC SRC.
	for _, ro := range []string{"/usr", "/bin", "/etc"} {
		ok := false
		for i, a := range args {
			if a == "--ro-bind" && i+2 < len(args) && args[i+1] == ro && args[i+2] == ro {
				ok = true
			}
		}
		if !ok {
			t.Errorf("--ro-bind %s %s not found", ro, ro)
		}
	}
}

// TestBuildBwrapArgs_DefaultReadOnlyPaths verifies the default mount
// set (used when Config.ReadOnlyPaths is left empty) includes git's
// binary and helper directory so shell problems can run git clone /
// log / bisect inside the sandbox (SBOX-001).
func TestBuildBwrapArgs_DefaultReadOnlyPaths(t *testing.T) {
	workDir := "/tmp/work"
	args := buildBwrapArgs(DefaultReadOnlyPaths, nil, workDir)
	// Git binary and git-core helper directory must both be ro-bound.
	for _, want := range []string{"/usr/bin/git", "/usr/lib/git-core"} {
		bound := false
		for i, a := range args {
			if a == "--ro-bind" && i+2 < len(args) && args[i+1] == want && args[i+2] == want {
				bound = true
				break
			}
		}
		if !bound {
			t.Errorf("--ro-bind %s %s not found in default mount set: %v", want, want, args)
		}
	}
}

// TestSandbox_Run_GitAvailable drives a real bwrap sandbox and
// confirms `git --version` succeeds — the end-to-end check for
// SBOX-001. Skipped if bwrap or git isn't installed.
func TestSandbox_Run_GitAvailable(t *testing.T) {
	if !BwrapAvailable() {
		t.Skip("bwrap not installed on this system")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed on this system")
	}
	x := &Executor{BwrapPath: lookupBwrap(), WorkDir: t.TempDir(), Timeout: 30 * time.Second}
	s, err := x.Create(context.Background(), "git-test", Config{})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = s.Destroy() }()

	stdout, stderr, err := s.Run(context.Background(), "git", "--version")
	if err != nil {
		t.Fatalf("git --version: %v (stderr=%s)", err, stderr)
	}
	got := strings.TrimSpace(string(stdout))
	if !strings.HasPrefix(got, "git version ") {
		t.Errorf("git --version output = %q, want prefix 'git version '", got)
	}
}

func TestBwrapAvailable(t *testing.T) {
	// Just exercises the path lookup; the result depends on the system.
	got := BwrapAvailable()
	t.Logf("BwrapAvailable = %v", got)
}

func TestNewExecutor(t *testing.T) {
	e := NewExecutor()
	if e.Timeout != DefaultBwrapTimeout {
		t.Errorf("Timeout = %v, want %v", e.Timeout, DefaultBwrapTimeout)
	}
	if e.WorkDir != os.TempDir() {
		t.Errorf("WorkDir = %q, want %q", e.WorkDir, os.TempDir())
	}
}

func TestSandbox_Destroy_Idempotent(t *testing.T) {
	exec := &Executor{BwrapPath: "/bin/true"}
	s, err := exec.Create(context.Background(), "idem", Config{BwrapPath: "/bin/true"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Destroy(); err != nil {
		t.Fatalf("first destroy: %v", err)
	}
	if err := s.Destroy(); err != nil {
		t.Errorf("second destroy should be no-op, got %v", err)
	}
}

func TestSandbox_Kill_NoProcess(t *testing.T) {
	exec := &Executor{BwrapPath: "/bin/true"}
	s, err := exec.Create(context.Background(), "kill", Config{BwrapPath: "/bin/true"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = s.Destroy() }()
	// Kill without a running process should not panic.
	s.Kill()
}

func TestSandbox_ID(t *testing.T) {
	x := &Executor{BwrapPath: "/bin/true"}
	s, err := x.Create(context.Background(), "my-id-test", Config{BwrapPath: "/bin/true"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = s.Destroy() }()
	id := s.ID()
	if id == "" {
		t.Fatal("ID() returned empty string")
	}
	if !strings.Contains(id, "my-id-test") {
		t.Errorf("ID() = %q, want it to contain 'my-id-test'", id)
	}
}
