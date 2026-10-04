// Per-solve peak RSS sampling (DF-OFF-BY-ONE-32 rework).
//
// The 2026-10-03 RAM incident (a generated go-bbr-pacing solve reaching
// 47 GB RSS) was invisible to the daemon: bwrap isolates namespaces but
// not memory, the RLIMIT_AS cap (DF-OFF-BY-ONE-30) bounds a single
// address space, and nothing measured what the sandboxed process TREE
// actually reserved. This file owns the measurement primitive: read the
// kernel's own high-water mark (VmHWM in /proc/<pid>/status) for a pid.
//
// Why VmHWM and not VmRSS: VmRSS is an instantaneous sample — a 47 GB
// spike between two polls is invisible. VmHWM is the kernel-maintained
// peak for the process's lifetime, so any poll that lands while the
// process is still alive observes the true maximum up to that moment.
// The cron loop exploits this by polling the solve's process tree
// during the solve and keeping the max across polls (see internal/cron).
//
// Linux-only by design: the daemon is a Linux-only systemd service, and
// /proc is the documented interface (matches internal/cron/loadavg.go).

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Sampler reads one process's peak resident set. PeakRSSBytes returns
// the VmHWM (high-water mark RSS) of pid in bytes. A pid that no longer
// exists (the normal case once a solve has been reaped) returns an
// error — callers decide whether that is fatal or a skip.
type Sampler interface {
	PeakRSSBytes(ctx context.Context, pid int) (uint64, error)
}

// ErrProcessGone is returned when /proc/<pid>/status cannot be read —
// the process exited (or never existed). Sentinel so callers can
// classify "no sample available" without string matching.
var ErrProcessGone = errors.New("sandbox: /proc status unavailable for pid")

// ProcSampler is the Linux Sampler implementation. It has no state and
// is safe for concurrent use.
type ProcSampler struct{}

// compile-time proof ProcSampler satisfies Sampler.
var _ Sampler = ProcSampler{}

// PeakRSSBytes reads /proc/<pid>/status and returns VmHWM in bytes.
// The kernel reports VmHWM in kB (kilobytes == 1024 bytes in proc(5));
// this converts to bytes so downstream thresholds (GiB maths, alert
// lines) never mix units.
func (ProcSampler) PeakRSSBytes(ctx context.Context, pid int) (uint64, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("sandbox: invalid pid %d", pid)
	}
	// ctx is part of the Sampler contract for future samplers that
	// cross a process boundary; the procfs read is a single syscall
	// and not cancellable, so ctx is deliberately unused here.
	_ = ctx
	data, err := os.ReadFile(procStatusPath(pid))
	if err != nil {
		return 0, fmt.Errorf("%w %d: %v", ErrProcessGone, pid, err)
	}
	hwm, ok := ParseVmHWM(data)
	if !ok {
		return 0, fmt.Errorf("sandbox: no VmHWM line in /proc/%d/status", pid)
	}
	return hwm, nil
}

// procStatusPath returns the procfs status path for pid. A function
// (not an inline literal) so tests can pin the documented shape.
func procStatusPath(pid int) string {
	return fmt.Sprintf("/proc/%d/status", pid)
}

// ParseVmHWM extracts the VmHWM value in BYTES from a
// /proc/<pid>/status body. ok is false when the body carries no
// parseable VmHWM line (kernel too old, truncated read, wrong file).
//
// The line's documented shape (proc(5)):
//
//	VmHWM:	     4762 kB
//
// Fields are whitespace-separated; the value is the second field and
// the unit the third. Tolerates extra whitespace and a missing unit
// (treated as kB, the kernel's only unit for this line).
func ParseVmHWM(data []byte) (uint64, bool) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "VmHWM:" {
			continue
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, false
		}
		return kb << 10, true
	}
	return 0, false
}
