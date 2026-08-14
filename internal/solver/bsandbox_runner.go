package solver

import (
	"context"
	"fmt"
	"log/slog"

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
// directory suffix. Per-solve options (e.g. WithRequiredTools) are
// applied to configure the sandbox's read-only mounts.
func (r *BSandboxRunner) Create(ctx context.Context, id string, opts ...CreateOption) (Handle, error) {
	oc := resolveCreateOptions(opts)

	cfg := sandbox.Config{}
	if len(oc.requiredTools) > 0 {
		// The full mount set matters for tool resolution coverage:
		// sandbox.Create merges the executor's ExtraReadOnlyPaths into
		// the config's effective mount set (see bwrap.go Create), so a
		// tool whose realpath lives under an executor extra (e.g.
		// $HOME/.local/bin, /tmp/pi, /etc) must be treated as already
		// covered — otherwise we would emit a duplicate (or, worse,
		// unmountable symlink) --ro-bind and hard-fail the whole solve
		// (OB-GAP-035, SBOX-002 never-fails contract).
		mountSet := make([]string, 0, len(sandbox.DefaultReadOnlyPaths)+len(r.exec.ExtraReadOnlyPaths))
		mountSet = append(mountSet, sandbox.DefaultReadOnlyPaths...)
		mountSet = append(mountSet, r.exec.ExtraReadOnlyPaths...)
		resolved, missing := sandbox.ResolveTools(oc.requiredTools, mountSet)
		if len(missing) > 0 {
			slog.Warn("sandbox: could not resolve declared tools on host; solve will proceed without them",
				"missing_tools", missing)
		}
		if len(resolved) > 0 {
			cfg.ExtraReadOnlyPaths = resolved
		}
	}

	s, err := r.exec.Create(ctx, id, cfg)
	if err != nil {
		return nil, fmt.Errorf("bwrap create: %w", err)
	}
	return &bwrapHandle{sandbox: s}, nil
}

// bwrapHandle adapts *sandbox.Sandbox to the solver's Handle
// interface. The Exec method calls sandbox.Run/RunWithEnv under
// the hood.
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
//
// Per-call env (e.g. DEEPSEEK_API_KEY) is delivered through
// RunWithEnv, which passes it via exec.Cmd.Env (envp) — never via
// the process argv — so secrets stay out of `ps` listings
// (OB-GAP-015). The old /usr/bin/env KEY=VAL shim is gone.
func (h *bwrapHandle) Exec(ctx context.Context, name string, args, env []string) ([]byte, error) {
	var stdout, stderr []byte
	var err error
	if len(env) == 0 {
		stdout, stderr, err = h.sandbox.Run(ctx, name, args...)
	} else {
		stdout, stderr, err = h.sandbox.RunWithEnv(ctx, name, args, env)
	}
	// Merge stderr into the returned stdout on error so the caller
	// gets the error plus stderr for diagnostics.
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
