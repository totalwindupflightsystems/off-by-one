package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// statusFixture is the documented /proc/<pid>/status shape (proc(5)),
// values chosen so the expected VmHWM is unambiguous (4762 kB).
const statusFixture = `Name:	pi-agent
Umask:	0022
State:	S (sleeping)
Tgid:	4321
Pid:	4321
PPid:	1234
TracerPid:	0
Uid:	1000	1000	1000	1000
Gid:	1000	1000	1000	1000
VmPeak:	  103500 kB
VmSize:	  103500 kB
VmLck:	       0 kB
VmPin:	       0 kB
VmHWM:	    4762 kB
VmRSS:	    3108 kB
RssAnon:	    1024 kB
RssFile:	    2084 kB
RssShmem:	       0 kB
VmData:	   12345 kB
VmStk:	     132 kB
VmExe:	      16 kB
VmLib:	   20480 kB
Threads:	1
`

// TestParseVmHWM_Fixture pins the parser against the documented
// status-body shape: value taken from the VmHWM line, converted
// kB → bytes (4762 kB = 4762 × 1024).
func TestParseVmHWM_Fixture(t *testing.T) {
	got, ok := ParseVmHWM([]byte(statusFixture))
	if !ok {
		t.Fatal("ParseVmHWM(fixture) = not found, want the VmHWM line")
	}
	want := uint64(4762) << 10
	if got != want {
		t.Fatalf("ParseVmHWM(fixture) = %d, want %d", got, want)
	}
}

// TestParseVmHWM_MissingLine — a body with no VmHWM line reports
// not-found rather than a zero sample.
func TestParseVmHWM_MissingLine(t *testing.T) {
	body := strings.ReplaceAll(statusFixture, "VmHWM:", "VmHWMX:")
	if _, ok := ParseVmHWM([]byte(body)); ok {
		t.Fatal("ParseVmHWM(body without VmHWM) = ok, want not found")
	}
}

// TestParseVmHWM_Empty — an empty read is not-found, not 0.
func TestParseVmHWM_Empty(t *testing.T) {
	if _, ok := ParseVmHWM(nil); ok {
		t.Fatal("ParseVmHWM(nil) = ok, want not found")
	}
}

// TestParseVmHWM_MalformedValue — a VmHWM line whose value does not
// parse is not-found (and never a silently-truncated number).
func TestParseVmHWM_MalformedValue(t *testing.T) {
	body := strings.Replace(statusFixture, "VmHWM:	    4762 kB", "VmHWM:	 47x2 kB", 1)
	if _, ok := ParseVmHWM([]byte(body)); ok {
		t.Fatal("ParseVmHWM(malformed value) = ok, want not found")
	}
}

// TestParseVmHWM_LargeValuesSurvive — a multi-GiB peak (the 2026-10-03
// incident scale) parses without overflow: 50331648 kB = 48 GiB.
func TestParseVmHWM_LargeValuesSurvive(t *testing.T) {
	body := strings.Replace(statusFixture, "VmHWM:	    4762 kB", "VmHWM:	 50331648 kB", 1)
	got, ok := ParseVmHWM([]byte(body))
	if !ok {
		t.Fatal("ParseVmHWM(48GiB line) = not found")
	}
	if want := uint64(48) << 30; got != want {
		t.Fatalf("ParseVmHWM(48GiB line) = %d, want %d", got, want)
	}
}

// TestParseVmHWM_AccidentalPrefixImmunity — a hypothetical "VmHWM2:"
// line must not satisfy the parser: field 0 is matched exactly.
func TestParseVmHWM_AccidentalPrefixImmunity(t *testing.T) {
	body := "VmHWM2:\t999 kB\n"
	if _, ok := ParseVmHWM([]byte(body)); ok {
		t.Fatal("ParseVmHWM('VmHWM2:' line) = ok, want not found (exact-field match)")
	}
}

// --- ProcSampler against real /proc -------------------------------------

// TestProcSampler_Self samples this test process itself: /proc/self
// exists on Linux, so the round trip (pid → status file → VmHWM →
// bytes) is exercised against the real kernel interface. The golang
// runtime always has a non-zero RSS, so a 0 sample is a defect.
func TestProcSampler_Self(t *testing.T) {
	pid := os.Getpid()
	got, err := ProcSampler{}.PeakRSSBytes(context.Background(), pid)
	if err != nil {
		t.Fatalf("PeakRSSBytes(self) = %v, want a live sample", err)
	}
	if got == 0 {
		t.Fatal("PeakRSSBytes(self) = 0, want > 0 (the Go runtime has an RSS)")
	}
	const oneTiB = uint64(1) << 40
	if got >= oneTiB { // 1 TiB — would be unit confusion
		t.Fatalf("PeakRSSBytes(self) = %d, implausibly large (unit error?)", got)
	}
}

// TestProcSampler_DeadPid pins the ErrProcessGone contract: a pid that
// cannot exist has no status file, and the error must match the
// sentinel (callers classify skip-vs-fail with errors.Is).
func TestProcSampler_DeadPid(t *testing.T) {
	pid := 1 << 22 // 4194304 — above pid_max defaults of 4194304 on 64-bit; +1 to be safe
	for pid > 0 {
		if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); err != nil {
			break
		}
		pid++
	}
	_, err := ProcSampler{}.PeakRSSBytes(context.Background(), pid)
	if err == nil {
		t.Skipf("pid %d unexpectedly exists on this host", pid)
	}
	if !errors.Is(err, ErrProcessGone) {
		t.Fatalf("PeakRSSBytes(dead pid %d) err = %v, want ErrProcessGone", pid, err)
	}
}

// TestProcSampler_InvalidPid — pid <= 0 is rejected before touching
// /proc (no "status unavailable for pid 0" ambiguity).
func TestProcSampler_InvalidPid(t *testing.T) {
	for _, pid := range []int{0, -1} {
		s := ProcSampler{}
		_, err := s.PeakRSSBytes(context.Background(), pid)
		if err == nil || errors.Is(err, ErrProcessGone) {
			t.Fatalf("PeakRSSBytes(pid=%d) err = %v, want invalid-pid error", pid, err)
		}
	}
}

// TestProcStatusPath_Shape pins the documented path construction once:
// the Sampler and any debug logging must agree on it.
func TestProcStatusPath_Shape(t *testing.T) {
	if got := procStatusPath(4321); got != "/proc/4321/status" {
		t.Fatalf("procStatusPath(4321) = %q, want /proc/4321/status", got)
	}
	if _, err := os.Stat(procStatusPath(os.Getpid())); err != nil {
		t.Fatalf("procStatusPath(self) = %q does not stat: %v", procStatusPath(os.Getpid()), err)
	}
}

// TestSamplerInterface_HeldByProcSampler keeps the interface wired.
func TestSamplerInterface_HeldByProcSampler(t *testing.T) {
	var s Sampler = ProcSampler{}
	if _, err := s.PeakRSSBytes(context.Background(), os.Getpid()); err != nil {
		t.Fatalf("interface dispatch to ProcSampler failed: %v", err)
	}
}

var _ = fmt.Sprintf // keep fmt imported for fixture edits
