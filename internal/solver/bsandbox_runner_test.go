package solver

import (
	"context"
	"os/exec"
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
