package solver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
	"github.com/totalwindupflightsystems/off-by-one/internal/sandbox"
)

// TestBSandboxRunner_RoundTrip exercises the real bwrap
// integration. We skip when bwrap is not installed so CI without
// bwrap can still run the unit tests. The test verifies the
// full bwrap plumbing: workspace bind-mount, command exec, file
// read/write through the BSandboxRunner wrapper.
func TestBSandboxRunner_RoundTrip(t *testing.T) {
	if !sandbox.BwrapAvailable() {
		t.Skip("bwrap not installed; skipping real-bwrap integration test")
	}

	bwrapPath, err := exec.LookPath("bwrap")
	if err != nil {
		t.Skipf("bwrap not on PATH: %v", err)
	}

	runner := NewBSandboxRunner(&sandbox.Executor{
		BwrapPath: bwrapPath,
		WorkDir:   t.TempDir(),
		Timeout:   30 * time.Second,
	})
	store, err := graph.OpenShared("bwrap-roundtrip-" + t.Name())
	if err != nil {
		t.Fatalf("graph.OpenShared: %v", err)
	}
	defer store.Close()

	// Build a small helper script and place it in the workspace
	// so the bwrap can exec /bin/sh with it as an argument.
	script := `#!/bin/sh
cat > /workspace/solution.md << 'EOF'
# Solution
Real bwrap integration test.
EOF
cat > /workspace/evidence.md << 'EOF'
# Evidence
Real bwrap integration test.
EOF
cat > /workspace/signatures.json << 'EOF'
{"problem_class":"bwrap-test","environment":"docker","language":"go","version":"1.26","model":"test"}
EOF
exit 0
`

	handle, err := runner.Create(context.Background(), "bwrap-test-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer func() { _ = handle.Destroy() }()

	// Write a problem file via the wrapper.
	if err := handle.WriteFile("problem.json", []byte(`{"problem_class":"bwrap-test"}`)); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Copy our helper script into the workspace so bwrap can
	// exec /bin/sh /workspace/script.sh.
	if err := handle.WriteFile("script.sh", []byte(script)); err != nil {
		t.Fatalf("WriteFile script: %v", err)
	}

	stdout, err := handle.Exec(context.Background(), "/bin/sh",
		[]string{"/workspace/script.sh"}, nil)
	if err != nil {
		t.Fatalf("Exec: %v (stdout=%s)", err, stdout)
	}

	// Read the three output files back via the wrapper.
	solMD, err := handle.ReadFile("solution.md")
	if err != nil {
		t.Fatalf("ReadFile solution.md: %v", err)
	}
	if !strings.Contains(string(solMD), "Real bwrap integration test") {
		t.Errorf("solution.md = %q, want it to contain 'Real bwrap integration test'", solMD)
	}
	evMD, err := handle.ReadFile("evidence.md")
	if err != nil {
		t.Fatalf("ReadFile evidence.md: %v", err)
	}
	if !strings.Contains(string(evMD), "Real bwrap integration test") {
		t.Errorf("evidence.md = %q, want it to contain 'Real bwrap integration test'", evMD)
	}
	sigMD, err := handle.ReadFile("signatures.json")
	if err != nil {
		t.Fatalf("ReadFile signatures.json: %v", err)
	}
	if !strings.Contains(string(sigMD), `"problem_class":"bwrap-test"`) {
		t.Errorf("signatures.json = %q, want problem_class=bwrap-test", sigMD)
	}
}

// makeRecordingBwrap writes a fake bwrap that records its full
// argv (what `ps` would show) and its own process environment
// (envp) into files, then exits 0. Used by the OB-GAP-015
// regression test below.
func makeRecordingBwrap(t *testing.T) (bwrapPath, argvFile, envFile string) {
	t.Helper()
	dir := t.TempDir()
	argvFile = filepath.Join(dir, "argv.txt")
	envFile = filepath.Join(dir, "env.txt")
	bwrapPath = filepath.Join(dir, "bwrap")
	script := "#!/bin/sh\n" +
		"printf '%s\n' \"$@\" > " + argvFile + "\n" +
		"env > " + envFile + "\n" +
		"exit 0\n"
	if err := os.WriteFile(bwrapPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write recording fake bwrap: %v", err)
	}
	return bwrapPath, argvFile, envFile
}

// TestExecEnvNotInArgv is the OB-GAP-015 regression test: env
// passed to bwrapHandle.Exec (DEEPSEEK_API_KEY / LLM_API_KEY from
// the pi-agent solve path) must reach the sandboxed process via
// envp, never via argv. argv is visible in `ps` listings; envp is
// not. The old /usr/bin/env KEY=VAL shim leaked both names and
// values into argv — this test pins the fix.
func TestExecEnvNotInArgv(t *testing.T) {
	t.Run("env delivered via envp not argv", func(t *testing.T) {
		bwrapPath, argvFile, envFile := makeRecordingBwrap(t)
		runner := NewBSandboxRunner(&sandbox.Executor{
			BwrapPath: bwrapPath,
			WorkDir:   t.TempDir(),
			Timeout:   5 * time.Second,
		})
		handle, err := runner.Create(context.Background(), "exec-env-leak")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		defer func() { _ = handle.Destroy() }()

		const secret = "sk-obgap015-test-secret"
		env := []string{"DEEPSEEK_API_KEY=" + secret, "LLM_API_KEY=" + secret}
		if _, err := handle.Exec(context.Background(), "/bin/echo", []string{"hello"}, env); err != nil {
			t.Fatalf("Exec: %v", err)
		}

		// argv must carry NO key names and NO key values.
		argvDump, err := os.ReadFile(argvFile)
		if err != nil {
			t.Fatalf("read argv dump: %v", err)
		}
		for _, leak := range []string{"DEEPSEEK_API_KEY", "LLM_API_KEY", secret, "sk-"} {
			if strings.Contains(string(argvDump), leak) {
				t.Errorf("argv contains %q — secret would be visible in ps:\n%s", leak, argvDump)
			}
		}

		// envp MUST carry the vars — delivery still works.
		envDump, err := os.ReadFile(envFile)
		if err != nil {
			t.Fatalf("read env dump: %v", err)
		}
		for _, want := range []string{"DEEPSEEK_API_KEY=" + secret, "LLM_API_KEY=" + secret} {
			if !strings.Contains(string(envDump), want) {
				t.Errorf("envp missing %q — per-call env did not reach the sandboxed process:\n%s", want, envDump)
			}
		}
	})

	t.Run("empty env runs command directly", func(t *testing.T) {
		bwrapPath, argvFile, _ := makeRecordingBwrap(t)
		runner := NewBSandboxRunner(&sandbox.Executor{
			BwrapPath: bwrapPath,
			WorkDir:   t.TempDir(),
			Timeout:   5 * time.Second,
		})
		handle, err := runner.Create(context.Background(), "exec-no-env")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		defer func() { _ = handle.Destroy() }()

		if _, err := handle.Exec(context.Background(), "/bin/echo", []string{"hello"}, nil); err != nil {
			t.Fatalf("Exec: %v", err)
		}

		argvDump, err := os.ReadFile(argvFile)
		if err != nil {
			t.Fatalf("read argv dump: %v", err)
		}
		if strings.Contains(string(argvDump), "/usr/bin/env") {
			t.Errorf("argv contains /usr/bin/env shim — command not run directly:\n%s", argvDump)
		}
		if !strings.Contains(string(argvDump), "/bin/echo") {
			t.Errorf("argv missing the command itself:\n%s", argvDump)
		}
	})
}
