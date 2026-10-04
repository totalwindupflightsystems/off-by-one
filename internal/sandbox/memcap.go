// Per-solve memory cap for the bwrap sandbox (DF-OFF-BY-ONE-30).
//
// bwrap isolates filesystem/PID/session namespaces but NOT memory, and
// its cgroup-based limits only work when running as root — the daemon
// runs as an unprivileged user. The portable mechanism is RLIMIT_AS on
// the child: this package owns the rlimit application (ApplyMemLimitMB)
// and the argv rewrite (memCapCommand) that routes the bwrap invocation
// through the oby-memcap wrapper binary (cmd/oby-memcap), which calls
// ApplyMemLimitMB and then execs bwrap. A 0 limit leaves the argv
// byte-identical to the historical behaviour.
//
// Background: on 2026-10-03 a generated Go test (go-bbr-pacing class)
// reached 47 GB RSS and filled the host's 55 GiB swap twice in 20
// minutes while the unit ran MemoryMax=infinity. The unit-level bound
// lives in deploy/off-by-one.service (MemoryHigh/MemoryMax/
// MemorySwapMax); this per-solve bound stops one bad solve from eating
// the daemon's entire budget.
package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// MemCapEnvVar carries the per-solve memory cap (in MiB) to the
// oby-memcap wrapper. It travels in envp, never in argv, matching the
// sandbox's existing secret-handling rule (OB-GAP-015).
const MemCapEnvVar = "OBY_MEM_LIMIT_MB"

// memCapBinaryName is the wrapper binary's name; make build places it
// next to the off-by-one daemon binary in the repo root.
const memCapBinaryName = "oby-memcap"

// ApplyMemLimitMB sets RLIMIT_AS (the virtual address-space cap) of the
// calling process to mb MiB, pinned (Cur == Max) so the limit cannot be
// raised again by the sandboxed workload. TestObyMemcapExec_ChildInheritsCap
// proves the full chain: the cmd/oby-memcap wrapper calls this function and
// syscall.Exec's the real command, which inherits the pin; mb <= 0 is
// rejected so the unlimited case never reaches the kernel with a bogus value.
//
// Callers must apply this immediately before an exec — lowering the
// hard limit is irreversible for a non-root process, so applying it to
// a long-lived process would cap it forever.
func ApplyMemLimitMB(mb int) error {
	if mb <= 0 {
		return fmt.Errorf("sandbox: invalid memory limit %d MiB (must be > 0)", mb)
	}
	bytes := uint64(mb) << 20
	if err := syscall.Setrlimit(syscall.RLIMIT_AS, &syscall.Rlimit{Cur: bytes, Max: bytes}); err != nil {
		return fmt.Errorf("sandbox: setrlimit RLIMIT_AS=%d MiB: %w", mb, err)
	}
	return nil
}

// memCapCommand computes the binary path, argv, and extra env entries
// for a sandbox run. When cfg.MemLimitMB <= 0 the bwrap invocation is
// returned unchanged; when MemLimitMB > 0 the command becomes
// [oby-memcap, <bwrap path>, <bwrap args...>] with the limit delivered
// via MemCapEnvVar. Pure: no filesystem or process side effects.
func memCapCommand(cfg Config, bwrapArgs []string) (path string, argv []string, env []string) {
	if cfg.MemLimitMB <= 0 {
		return cfg.BwrapPath, bwrapArgs, nil
	}
	argv = append([]string{cfg.BwrapPath}, bwrapArgs...)
	return cfg.MemCapPath, argv, []string{fmt.Sprintf("%s=%d", MemCapEnvVar, cfg.MemLimitMB)}
}

// resolveMemCapPath finds the oby-memcap wrapper: first next to the
// running daemon binary (make build places both in the repo root, which
// is also the unit's WorkingDirectory), then on PATH. Fails loud when
// the wrapper is absent — a solve that runs without its configured cap
// is exactly the failure mode DF-OFF-BY-ONE-30 exists to prevent.
func resolveMemCapPath() (string, error) {
	if exe, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(exe), memCapBinaryName)
		if _, err := os.Stat(sibling); err == nil {
			return sibling, nil
		}
	}
	if p, err := exec.LookPath(memCapBinaryName); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("sandbox: %s wrapper not found (looked next to the daemon binary and on PATH) — run 'make build' or set Config.MemCapPath", memCapBinaryName)
}
