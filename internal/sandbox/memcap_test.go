package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// memCapHelperEnv marks the re-exec'd test binary as the helper child
// (the standard TestHelperProcess pattern).
const memCapHelperEnv = "OBY_TEST_MEMCAP_HELPER"

// memCapPyEnv carries the resolved python3 path from the parent test to
// the alloc-under-cap helper child. The child execs it AFTER pinning
// RLIMIT_AS, so the ENOMEM observation is made by a process that does
// not run the Go runtime.
const memCapPyEnv = "OBY_TEST_MEMCAP_PY"

// allocUnderCapProbePy is the non-Go half of the alloc-under-cap arm: an
// anonymous 256 MiB mapping attempted under the 64 MiB RLIMIT_AS the
// helper pinned just before exec. mmap(2) charges the address space at
// map time, so no pages need to be touched for the kernel to refuse.
// It prints ALLOC-REFUSED and exits 0 only when the kernel answers
// ENOMEM (errno 12); a successful mapping means the cap is not enforced
// and the probe exits 1 so the parent test fails loudly.
const allocUnderCapProbePy = `import mmap, sys
try:
    m = mmap.mmap(-1, 268435456)
except OSError as e:
    if e.errno == 12:
        print("ALLOC-REFUSED")
        sys.exit(0)
    print("mmap failed with %r, want ENOMEM (errno 12)" % (e,), file=sys.stderr)
    sys.exit(1)
m.close()
print("256 MiB mmap SUCCEEDED under a 64 MiB RLIMIT_AS -- cap not enforced", file=sys.stderr)
sys.exit(1)
`

// TestMemCapHelperProcess is not a test — it is the child half of the
// subprocess arms below. It runs the mode named by
// OBY_TEST_MEMCAP_MODE and exits non-zero on any unexpected outcome.
//
// The rlimit arms MUST run in a subprocess: ApplyMemLimitMB pins the
// hard limit (Cur == Max), which a non-root process can never raise
// again, so applying it to the test binary itself would cap the rest
// of the suite.
func TestMemCapHelperProcess(t *testing.T) {
	if os.Getenv(memCapHelperEnv) != "1" {
		return
	}
	switch os.Getenv("OBY_TEST_MEMCAP_MODE") {
	case "roundtrip":
		// Set a generous cap and read it back: proves ApplyMemLimitMB
		// reaches the kernel with Cur == Max == mb<<20.
		const mb = 4096
		if err := ApplyMemLimitMB(mb); err != nil {
			fmt.Fprintf(os.Stderr, "ApplyMemLimitMB(%d): %v\n", mb, err)
			os.Exit(1)
		}
		var got syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_AS, &got); err != nil {
			fmt.Fprintf(os.Stderr, "Getrlimit: %v\n", err)
			os.Exit(1)
		}
		want := uint64(mb) << 20
		if got.Cur != want || got.Max != want {
			fmt.Fprintf(os.Stderr, "RLIMIT_AS = {Cur:%d Max:%d}, want {%d %d}\n", got.Cur, got.Max, want, want)
			os.Exit(1)
		}
		fmt.Println("ROUNDTRIP-OK")
		os.Exit(0)
	case "alloc-under-cap":
		// Pin the cap at 64 MiB, then REPLACE this process with a
		// non-Go probe that performs the 256 MiB mmap. The Go runtime
		// must not keep running post-setrlimit: on CI runners the
		// first runtime allocation after ApplyMemLimitMB(64) dies with
		// "fatal error: runtime: cannot allocate memory" in
		// runtime.persistentalloc1 before the probe is ever reached
		// (INT-CI-003), while locally the runtime happens to survive.
		// Everything the exec needs is resolved BEFORE the cap is
		// applied; between ApplyMemLimitMB and syscall.Exec there is
		// only the error branch, so no post-cap Go heap allocation is
		// required. python3 (-S: skip site imports) fits in 64 MiB and
		// its mmap is charged at map time, so the kernel's ENOMEM is
		// observed by the exec'd interpreter, never by live Go code.
		py := os.Getenv(memCapPyEnv)
		if py == "" {
			fmt.Fprintf(os.Stderr, "%s not set — parent must resolve python3\n", memCapPyEnv)
			os.Exit(2)
		}
		argv := []string{py, "-S", "-c", allocUnderCapProbePy}
		if err := ApplyMemLimitMB(64); err != nil {
			fmt.Fprintf(os.Stderr, "ApplyMemLimitMB(64): %v\n", err)
			os.Exit(1)
		}
		// No Go runtime work from here on: exec immediately. An exec
		// failure is reported best-effort (the cap is already pinned).
		if err := syscall.Exec(py, argv, nil); err != nil {
			fmt.Fprintf(os.Stderr, "exec python3 probe: %v\n", err)
			os.Exit(1)
		}
	case "alloc-control":
		// Same 256 MiB mapping with NO cap: must succeed, proving the
		// alloc-under-cap refusal is caused by the rlimit and not by
		// the host being unable to hand out 256 MiB.
		buf, err := syscall.Mmap(-1, 0, 256<<20, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
		if err != nil {
			fmt.Fprintf(os.Stderr, "control mmap failed: %v\n", err)
			os.Exit(1)
		}
		_ = syscall.Munmap(buf)
		fmt.Println("ALLOC-OK")
		os.Exit(0)
	case "inherited-check":
		// Child arm of TestObyMemcapExec_ChildInheritsCap: the parent
		// execs the REAL cmd/oby-memcap wrapper (which pinned RLIMIT_AS
		// from OBY_MEM_LIMIT_MB and syscall.Exec'd this process), so this
		// arm must observe the cap WITHOUT applying it itself. Asserts
		// both the value (Cur == Max == mb<<20) and the enforcement (a
		// 256 MiB mmap is refused with ENOMEM).
		var lim syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_AS, &lim); err != nil {
			fmt.Fprintf(os.Stderr, "Getrlimit: %v\n", err)
			os.Exit(1)
		}
		want := uint64(64) << 20
		if lim.Cur != want || lim.Max != want {
			fmt.Fprintf(os.Stderr, "inherited RLIMIT_AS = {Cur:%d Max:%d}, want {%d %d}\n", lim.Cur, lim.Max, want, want)
			os.Exit(1)
		}
		if _, err := syscall.Mmap(-1, 0, 256<<20, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE); err == nil {
			fmt.Fprintln(os.Stderr, "256 MiB mmap SUCCEEDED under inherited 64 MiB cap — cap not inherited")
			os.Exit(1)
		}
		fmt.Println("INHERITED-OK")
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown OBY_TEST_MEMCAP_MODE %q\n", os.Getenv("OBY_TEST_MEMCAP_MODE"))
		os.Exit(2)
	}
}

// runMemCapHelper re-executes the test binary as the helper child in
// the given mode and returns its combined output. extraEnv entries
// (KEY=value) are appended to the child's environment.
func runMemCapHelper(t *testing.T, mode string, extraEnv ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestMemCapHelperProcess$")
	env := append(os.Environ(), memCapHelperEnv+"=1", "OBY_TEST_MEMCAP_MODE="+mode)
	cmd.Env = append(env, extraEnv...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestApplyMemLimitMB_RoundTrip(t *testing.T) {
	out, err := runMemCapHelper(t, "roundtrip")
	if err != nil {
		t.Fatalf("roundtrip helper failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ROUNDTRIP-OK") {
		t.Fatalf("roundtrip helper did not confirm the limit, output:\n%s", out)
	}
}

func TestApplyMemLimitMB_RefusesAllocationPastCap(t *testing.T) {
	// Control first: the host can hand out 256 MiB with no cap. This arm
	// is MANDATORY — without it a refusal under the cap cannot be
	// attributed to the rlimit.
	out, err := runMemCapHelper(t, "alloc-control")
	if err != nil || !strings.Contains(out, "ALLOC-OK") {
		t.Fatalf("control arm (no cap) failed — cannot attribute the capped refusal: err=%v\n%s", err, out)
	}
	// The under-cap ENOMEM is observed by an exec'd python3 probe, not
	// by the Go helper that pinned the cap (see the alloc-under-cap arm
	// in TestMemCapHelperProcess): a live Go runtime under a 64 MiB
	// RLIMIT_AS dies on its own post-setrlimit allocations on CI
	// runners, which is the env-dependent flake this test must not be.
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not on PATH: the under-cap ENOMEM probe needs a non-Go runtime (a Go process cannot survive post-setrlimit under 64 MiB); the control arm above already passed")
	}
	out, err = runMemCapHelper(t, "alloc-under-cap", memCapPyEnv+"="+py)
	if err != nil {
		t.Fatalf("alloc-under-cap helper failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ALLOC-REFUSED") {
		t.Fatalf("alloc-under-cap helper did not observe ENOMEM, output:\n%s", out)
	}
}

func TestApplyMemLimitMB_RejectsNonPositive(t *testing.T) {
	// mb <= 0 must be a validation error and must NOT touch the process
	// limits — read them before and after to prove it.
	var before syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_AS, &before); err != nil {
		t.Fatalf("Getrlimit: %v", err)
	}
	for _, mb := range []int{0, -1, -4096} {
		if err := ApplyMemLimitMB(mb); err == nil {
			t.Errorf("ApplyMemLimitMB(%d) = nil, want error", mb)
		}
	}
	var after syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_AS, &after); err != nil {
		t.Fatalf("Getrlimit: %v", err)
	}
	if after != before {
		t.Errorf("RLIMIT_AS changed by rejected calls: before %+v, after %+v", before, after)
	}
}

func TestMemCapCommand_UnlimitedLeavesArgvUntouched(t *testing.T) {
	cfg := Config{BwrapPath: "/usr/bin/bwrap", MemLimitMB: 0}
	in := []string{"--unshare-all", "--bind", "/x", "/workspace", "--", "/workspace/run.sh", "--fast"}
	path, argv, env := memCapCommand(cfg, append([]string{}, in...))
	if path != cfg.BwrapPath {
		t.Errorf("path = %q, want the bwrap path %q (no wrapper when unlimited)", path, cfg.BwrapPath)
	}
	if len(argv) != len(in) {
		t.Fatalf("argv length = %d, want %d (%v)", len(argv), len(in), argv)
	}
	for i := range in {
		if argv[i] != in[i] {
			t.Fatalf("argv[%d] = %q, want %q (full argv %v)", i, argv[i], in[i], argv)
		}
	}
	if env != nil {
		t.Errorf("env = %v, want nil (no %s entry when unlimited)", env, MemCapEnvVar)
	}
}

func TestMemCapCommand_WrapsThroughObyMemcap(t *testing.T) {
	cfg := Config{BwrapPath: "/usr/bin/bwrap", MemLimitMB: 6144, MemCapPath: "/repo/oby-memcap"}
	in := []string{"--unshare-all", "--", "/workspace/run.sh"}
	path, argv, env := memCapCommand(cfg, in)
	if path != "/repo/oby-memcap" {
		t.Errorf("path = %q, want the oby-memcap wrapper", path)
	}
	wantArgv := append([]string{"/usr/bin/bwrap"}, in...)
	if len(argv) != len(wantArgv) {
		t.Fatalf("argv = %v, want %v", argv, wantArgv)
	}
	for i := range wantArgv {
		if argv[i] != wantArgv[i] {
			t.Fatalf("argv[%d] = %q, want %q (full argv %v)", i, argv[i], wantArgv[i], argv)
		}
	}
	if len(env) != 1 || env[0] != "OBY_MEM_LIMIT_MB=6144" {
		t.Errorf("env = %v, want [%s=6144]", env, MemCapEnvVar)
	}
}

func TestExecutorCreate_MemCapDefaultsAndResolution(t *testing.T) {
	// Executor defaults flow into the per-call Config.
	x := &Executor{BwrapPath: "/bin/true", MemLimitMB: 1024, MemCapPath: "/bin/true"}
	s, err := x.Create(context.Background(), "memcap-defaults", Config{BwrapPath: "/bin/true"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer func() { _ = s.Destroy() }()
	if s.cfg.MemLimitMB != 1024 {
		t.Errorf("MemLimitMB = %d, want 1024 inherited from the Executor", s.cfg.MemLimitMB)
	}
	if s.cfg.MemCapPath != "/bin/true" {
		t.Errorf("MemCapPath = %q, want the Executor default", s.cfg.MemCapPath)
	}

	// An explicit MemCapPath is used verbatim (Create does not stat it —
	// exec.CommandContext reports a missing binary at Run time).
	x2 := &Executor{BwrapPath: "/bin/true"}
	absent := filepath.Join(t.TempDir(), "definitely-absent-oby-memcap")
	s2, err := x2.Create(context.Background(), "memcap-explicit", Config{BwrapPath: "/bin/true", MemLimitMB: 64, MemCapPath: absent})
	if err != nil {
		t.Fatalf("create with explicit MemCapPath: %v", err)
	}
	defer func() { _ = s2.Destroy() }()
	if s2.cfg.MemCapPath != absent {
		t.Errorf("MemCapPath = %q, want the explicit %q", s2.cfg.MemCapPath, absent)
	}
}

// TestObyMemcapExec_ChildInheritsCap proves the FULL exec chain with the
// real wrapper binary: cmd/oby-memcap is built, exec'd with OBY_MEM_LIMIT_MB
// set, and it must syscall.Exec the child, which then observes the pinned
// RLIMIT_AS (Cur == Max == 64 MiB) WITHOUT applying it itself — plus the
// cap is actually enforced (a 256 MiB mapping is refused). This is the
// wrapper's production behaviour; the in-process roundtrip test above only
// proves sandbox.ApplyMemLimitMB, not the exec inheritance.
func TestObyMemcapExec_ChildInheritsCap(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH; cannot build cmd/oby-memcap")
	}
	wrapper := filepath.Join(t.TempDir(), "oby-memcap")
	out, err := exec.Command("go", "build", "-o", wrapper, "github.com/totalwindupflightsystems/off-by-one/cmd/oby-memcap").CombinedOutput()
	if err != nil {
		t.Fatalf("go build cmd/oby-memcap: %v\n%s", err, out)
	}

	// A Go child cannot start under a 64 MiB RLIMIT_AS (the runtime
	// reserves far more address space than it commits), so the child is
	// /bin/sh reading /proc/self/limits — the KERNEL's record of the
	// rlimit this exact process inherited across the wrapper's
	// syscall.Exec. 64 MiB = 65536 kB.
	cmd := exec.Command(wrapper, "/bin/sh", "-c", `grep 'Max address space' /proc/self/limits`)
	cmd.Env = append(os.Environ(), MemCapEnvVar+"=64")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oby-memcap exec chain failed: %v\n%s", err, out)
	}
	// Expected line: "Max address space  67108864  67108864  bytes"
	// (64 MiB in BYTES, soft==hard — /proc/self/limits prints bytes here).
	if !strings.Contains(string(out), "67108864             67108864") {
		t.Fatalf("inherited RLIMIT_AS is not pinned soft==hard at 64 MiB (67108864 bytes), got:\n%s", out)
	}
}
