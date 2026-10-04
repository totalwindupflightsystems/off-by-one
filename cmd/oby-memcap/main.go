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
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: oby-memcap <command> [args...] (cap via %s, MiB; unset = unlimited)\n", sandbox.MemCapEnvVar)
		os.Exit(2)
	}
	path, err := exec.LookPath(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "oby-memcap: %v\n", err)
		os.Exit(127)
	}
	if err := syscall.Exec(path, os.Args[1:], os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "oby-memcap: exec %s: %v\n", path, err)
		os.Exit(127)
	}
}
