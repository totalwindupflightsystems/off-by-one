package solver

import (
	"context"
	"fmt"

	"github.com/totalwindupflightsystems/off-by-one/internal/sandbox"
)

// BSandboxRunner wraps the internal/sandbox Executor. Each
// Solve() call gets its own bwrap workspace.
type BSandboxRunner struct {
	exec *sandbox.Executor
}

// NewBSandboxRunner returns a Runner that creates one bwrap
// sandbox per Create. Pass an Executor built with
// sandbox.NewExecutor() in production; tests can construct one
// with a custom BwrapPath.
func NewBSandboxRunner(exec *sandbox.Executor) *BSandboxRunner {
	return &BSandboxRunner{exec: exec}
}

// Create allocates a bwrap sandbox. The id becomes the workspace
// directory suffix.
func (r *BSandboxRunner) Create(ctx context.Context, id string) (Handle, error) {
	s, err := r.exec.Create(ctx, id, sandbox.Config{})
	if err != nil {
		return nil, fmt.Errorf("bwrap create: %w", err)
	}
	return &bwrapHandle{sandbox: s}, nil
}

// bwrapHandle adapts *sandbox.Sandbox to the solver's Handle
// interface. The Exec method calls sandbox.Run under the hood.
type bwrapHandle struct {
	sandbox *sandbox.Sandbox
}

// WriteFile writes data to relPath in the workspace.
func (h *bwrapHandle) WriteFile(relPath string, data []byte) error {
	return h.sandbox.CopyIn(relPath, data)
}

// ReadFile reads relPath from the workspace.
func (h *bwrapHandle) ReadFile(relPath string) ([]byte, error) {
	return h.sandbox.CopyOut(relPath)
}

// Exec runs name with args + env inside the bwrap sandbox. stdout
// and stderr are merged into the returned bytes. The sandbox's
// configured Timeout caps the run; pass a shorter context to
// tighten it further.
func (h *bwrapHandle) Exec(ctx context.Context, name string, args, env []string) ([]byte, error) {
	// The sandbox.Config.ExtraEnv is set at Create time and can't
	// be changed per-Run. We work around this by passing the
	// per-call env through a tiny shim command: when env is
	// non-empty, we exec /usr/bin/env with the env vars and the
	// real command. When env is empty, we exec the command
	// directly.
	if len(env) == 0 {
		stdout, _, err := h.sandbox.Run(ctx, name, args...)
		return stdout, err
	}
	// Build a /usr/bin/env command line: env KEY=VAL ... name args...
	envArgs := append([]string{}, env...)
	envArgs = append(envArgs, name)
	envArgs = append(envArgs, args...)
	stdout, stderr, err := h.sandbox.Run(ctx, "/usr/bin/env", envArgs...)
	// Merge stderr into the returned stdout on success so callers
	// see pi-agent's full output. On error, the caller already
	// gets the error — we just include stderr for diagnostics.
	if err != nil && len(stderr) > 0 {
		merged := append([]byte{}, stdout...)
		merged = append(merged, '\n')
		merged = append(merged, stderr...)
		return merged, err
	}
	return stdout, err
}

// Destroy releases the sandbox and removes the workspace.
func (h *bwrapHandle) Destroy() error {
	return h.sandbox.Destroy()
}

// Ensure BSandboxRunner satisfies Runner at compile time.
var _ Runner = (*BSandboxRunner)(nil)

// ensure *bwrapHandle satisfies Handle at compile time.
var _ Handle = (*bwrapHandle)(nil)
