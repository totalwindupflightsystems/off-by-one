// oby-memcap is the per-solve memory-cap exec wrapper for the off-by-one
// bwrap sandbox (DF-OFF-BY-ONE-30).
//
// bwrap has no rlimit flag and its cgroup limits require root, so the
// sandbox applies the cap one level up: when Config.MemLimitMB > 0 the
// daemon spawns [oby-memcap, bwrap, ...args] with OBY_MEM_LIMIT_MB set.
// This binary pins RLIMIT_AS (soft == hard) on itself and then execs the
// real command, which inherits the limit. See internal/sandbox/memcap.go
// for the full rationale and the 2026-10-03 incident that motivated it.
//
// Usage: OBY_MEM_LIMIT_MB=6144 oby-memcap <command> [args...]
// An unset OBY_MEM_LIMIT_MB means no cap — the wrapper is a plain exec.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/totalwindupflightsystems/off-by-one/internal/sandbox"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: oby-memcap <command> [args...] (cap via %s, MiB; unset = unlimited)\n", sandbox.MemCapEnvVar)
		os.Exit(2)
	}
	// Resolve EVERYTHING the exec needs BEFORE the cap is applied.
	// RLIMIT_AS pins the virtual address space of this process, and the
	// Go runtime allocates on paths like exec.LookPath (PATH walking,
	// error construction) and os.Environ (slice copy); under a tight cap
	// (e.g. 64 MiB in the sandbox inheritance test) the first such
	// post-setrlimit allocation can kill the runtime with
	// "fatal error: runtime: cannot allocate memory" before the exec is
	// ever reached — runner-sensitive, seen on CI (INT-CI-005).
	path, err := exec.LookPath(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "oby-memcap: %v\n", err)
		os.Exit(127)
	}
	argv := os.Args[1:]
	envp := os.Environ()
	// Pre-build the execve(2) argument vectors BEFORE the cap too.
	// Resolving the Go values early (above) is not sufficient:
	// syscall.Exec converts argv/envp to C pointer arrays LAZILY, inside
	// the call, via syscall.SlicePtrFromStrings — that mallocgc is a
	// post-setrlimit heap allocation, and on the Go 1.26.9 CI runner it
	// required a fresh 4 MiB address-space reservation that the 64 MiB
	// RLIMIT_AS refused, killing the wrapper with
	// "fatal error: runtime: out of memory" at the exec call itself
	// (INT-CI-006, run 40569450107) before the child ever started.
	// sandbox.RawExecve performs no heap allocation; between
	// ApplyMemLimitMB and RawExecve there is only the exec error branch,
	// so no post-cap Go heap allocation is required.
	pathPtr, err := syscall.BytePtrFromString(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oby-memcap: exec path %q: %v\n", path, err)
		os.Exit(127)
	}
	argvPtr, err := syscall.SlicePtrFromStrings(argv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oby-memcap: argv: %v\n", err)
		os.Exit(127)
	}
	envpPtr, err := syscall.SlicePtrFromStrings(envp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oby-memcap: envp: %v\n", err)
		os.Exit(127)
	}
	if raw := os.Getenv(sandbox.MemCapEnvVar); raw != "" {
		mb, err := strconv.Atoi(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "oby-memcap: %s=%q is not an integer: %v\n", sandbox.MemCapEnvVar, raw, err)
			os.Exit(2)
		}
		if err := sandbox.ApplyMemLimitMB(mb); err != nil {
			fmt.Fprintf(os.Stderr, "oby-memcap: %v\n", err)
			os.Exit(2)
		}
	}
	if errno := sandbox.RawExecve(pathPtr, argvPtr, envpPtr); errno != 0 {
		fmt.Fprintf(os.Stderr, "oby-memcap: exec %s: %v\n", path, errno)
		os.Exit(127)
	}
}
