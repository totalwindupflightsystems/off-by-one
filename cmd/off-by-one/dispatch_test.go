package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// serverFlagSet returns a fresh flag set carrying the REAL server flag
// declarations, so the dispatch tests exercise the same names and arity
// main() registers on flag.CommandLine.
func serverFlagSet(t *testing.T) *flag.FlagSet {
	t.Helper()
	fs := flag.NewFlagSet("off-by-one", flag.ContinueOnError)
	registerServerFlags(fs)
	return fs
}

// TestSeedDispatchRecognisesSubcommandForms covers the invocations that must
// reach the seed subcommand, including the OB-GAP-071 case where
// server-compatible flags precede `seed`. Any other pre-seed server flag is
// dropped (and named on stderr) rather than handed to the seed FlagSet, which
// would abort on an unknown flag.
func TestSeedDispatchRecognisesSubcommandForms(t *testing.T) {
	fs := serverFlagSet(t)
	cases := []struct {
		name string
		argv []string
		want []string
	}{
		{"subcommand only", []string{"seed"}, []string{}},
		{"historical form", []string{"seed", "-dir", "/corpus", "-db", "/db.sqlite"}, []string{"-dir", "/corpus", "-db", "/db.sqlite"}},
		{"seed then positional", []string{"seed", "extra"}, []string{"extra"}},
		{"db before subcommand", []string{"--db", "/db.sqlite", "seed"}, []string{"-db", "/db.sqlite"}},
		{"single-dash db", []string{"-db", "/db.sqlite", "seed"}, []string{"-db", "/db.sqlite"}},
		{"equals form", []string{"--db=/db.sqlite", "seed"}, []string{"-db", "/db.sqlite"}},
		{"server flags before subcommand", []string{"--port", "9000", "--db", "/db.sqlite", "seed"}, []string{"-db", "/db.sqlite"}},
		{"bool flag before subcommand", []string{"--readonly", "seed"}, []string{}},
		{"bool flag with value", []string{"--skip-sandbox=false", "seed"}, []string{}},
		{"flag terminator", []string{"--", "seed"}, []string{}},
		{
			"pre-seed db plus subcommand flags",
			[]string{"--db", "/db.sqlite", "seed", "-dir", "/corpus"},
			[]string{"-db", "/db.sqlite", "-dir", "/corpus"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := seedDispatch(tc.argv, fs)
			if !ok {
				t.Fatalf("seedDispatch(%v) = not-seed, want seed args %v", tc.argv, tc.want)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("seedDispatch(%v) args = %v, want %v", tc.argv, got, tc.want)
			}
		})
	}
}

// TestSeedDispatchLeavesServerStartupAlone covers the invocations that must
// keep booting the server: no subcommand, and the argv-vs-arity ambiguities
// where "seed" is a flag VALUE, not a subcommand. Help/version also stay with
// the server path so their output is unchanged.
func TestSeedDispatchLeavesServerStartupAlone(t *testing.T) {
	fs := serverFlagSet(t)
	cases := []struct {
		name string
		argv []string
	}{
		{"no args", nil},
		{"server flags only", []string{"--port", "9000", "--db", "/db.sqlite"}},
		{"port flag only", []string{"--port", "9000"}},
		{"value flag eats seed", []string{"--db", "seed"}},
		{"host flag eats seed", []string{"--host", "seed"}},
		{"prefix of subcommand", []string{"seedling"}},
		{"unknown positional", []string{"some-file.json"}},
		{"help before subcommand", []string{"--help", "seed"}},
		{"short help before subcommand", []string{"-h", "seed"}},
		{"unknown flag before subcommand", []string{"--nope", "seed"}},
		{"missing flag value", []string{"--db"}},
		{"version before subcommand", []string{"--version", "seed"}},
		{"version=false before subcommand", []string{"--version=false", "seed"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := seedDispatch(tc.argv, fs)
			if ok {
				t.Errorf("seedDispatch(%v) = seed %v, want server startup", tc.argv, got)
			}
		})
	}
}

// TestSeedDispatchMirrorsServerFlagArity walks the live server flag set: every
// value-taking flag must consume the following argument (so `--port seed`
// cannot be mistaken for a subcommand) while boolean flags must not, and the
// subcommand must still be found once the flag's value is present. This is the
// invariant that keeps the probe honest as server flags are added.
func TestSeedDispatchMirrorsServerFlagArity(t *testing.T) {
	fs := serverFlagSet(t)
	checked := 0
	fs.VisitAll(func(f *flag.Flag) {
		if f.Name == "version" {
			return // --version keeps server precedence (covered above)
		}
		isBool := false
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok {
			isBool = bf.IsBoolFlag()
		}
		checked++

		if isBool {
			if _, ok := seedDispatch([]string{"--" + f.Name, "seed"}, fs); !ok {
				t.Errorf("bool flag --%s: `--%s seed` must dispatch to seed", f.Name, f.Name)
			}
			return
		}

		if got, ok := seedDispatch([]string{"--" + f.Name, "seed"}, fs); ok {
			t.Errorf("value flag --%s: `--%s seed` must treat seed as the flag value, got seed args %v", f.Name, f.Name, got)
		}
		if _, ok := seedDispatch([]string{"--" + f.Name, "VALUE", "seed"}, fs); !ok {
			t.Errorf("value flag --%s: `--%s VALUE seed` must dispatch to seed", f.Name, f.Name)
		}
	})
	if checked == 0 {
		t.Fatal("no server flags registered — probe arity was not exercised")
	}
}

// TestSeedDispatchForwardsDBIntoSeedRun proves the forwarded -db is honoured
// end to end: the dispatched argument list is fed to seedRun and the database
// lands at the pre-subcommand --db path. It also pins the precedence rule — a
// -db after the subcommand is applied later by the seed FlagSet and therefore
// wins — and proves the corpus really was loaded into that database.
func TestSeedDispatchForwardsDBIntoSeedRun(t *testing.T) {
	corpus := writeCorpusRootNamed(t, filepath.Join(t.TempDir(), "data"), "forwarded db class")
	dbBefore := filepath.Join(t.TempDir(), "before.db")
	dbAfter := filepath.Join(t.TempDir(), "after.db")

	args, ok := seedDispatch([]string{"--db", dbBefore, "seed", "-dir", corpus, "-db", dbAfter}, serverFlagSet(t))
	if !ok {
		t.Fatalf("seedDispatch did not select seed for --db ... seed")
	}
	if err := seedRun(args); err != nil {
		t.Fatalf("seedRun(%v): %v", args, err)
	}

	if _, err := os.Stat(dbBefore); err == nil {
		t.Errorf("database %s was created — the forwarded -db should lose to the subcommand's own -db", dbBefore)
	}
	if !classExists(t, dbAfter, "forwarded db class") {
		t.Errorf("seeded db %s does not hold the fixture corpus", dbAfter)
	}
}

// --- real-binary acceptance ------------------------------------------------

var (
	ob1BuildOnce sync.Once
	ob1BuildPath string
	ob1BuildErr  error
	ob1BuildDir  string
)

// TestMain removes the directory holding the CLI built for the subprocess
// tests below.
func TestMain(m *testing.M) {
	code := m.Run()
	if ob1BuildDir != "" {
		_ = os.RemoveAll(ob1BuildDir)
	}
	os.Exit(code)
}

// offByOneBinary builds the real CLI once for this test binary and returns its
// path. The acceptance criterion is about the real argv, so these tests exec
// the built command rather than calling a helper.
func offByOneBinary(t *testing.T) string {
	t.Helper()
	ob1BuildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "ob1-cli-")
		if err != nil {
			ob1BuildErr = fmt.Errorf("mkdtemp: %w", err)
			return
		}
		ob1BuildDir = dir
		bin := filepath.Join(dir, "off-by-one")
		if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
			ob1BuildErr = fmt.Errorf("go build: %v\n%s", err, out)
			return
		}
		ob1BuildPath = bin
	})
	if ob1BuildErr != nil {
		t.Fatalf("%v", ob1BuildErr)
	}
	return ob1BuildPath
}

// startCLI starts the built binary in dir with its combined output streamed to
// a log file. A FILE (not a bytes.Buffer) keeps the concurrently reading test
// goroutine from racing with os/exec's writer.
func startCLI(t *testing.T, bin, dir string, args ...string) (*exec.Cmd, string) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "cli.log")
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create log file: %v", err)
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = f, f
	if err := cmd.Start(); err != nil {
		_ = f.Close()
		t.Fatalf("start %s %v: %v", bin, args, err)
	}
	// The child holds its own descriptor; dropping the parent's prevents a
	// leaked handle per test.
	_ = f.Close()
	return cmd, logPath
}

// awaitSeedExit waits for a started invocation to exit, failing fast when it
// binds port — proof the HTTP server started — or when it outlives timeout.
// The pre-fix `off-by-one --db PATH seed` argv serves forever in
// ListenAndServe, so "exits on its own" is the acceptance signal.
func awaitSeedExit(t *testing.T, cmd *exec.Cmd, port int, logPath string, timeout time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	deadline := time.Now().Add(timeout)
	for {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("invocation exited with error: %v\noutput:\n%s", err, readFileString(logPath))
			}
			return
		default:
		}
		if portListening(port) {
			_ = cmd.Process.Kill()
			<-done
			t.Fatalf("the invocation bound 127.0.0.1:%d — the HTTP server started instead of seeding (OB-GAP-071)\noutput:\n%s",
				port, readFileString(logPath))
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			<-done
			t.Fatalf("the invocation did not exit within %s — it is serving instead of seeding\noutput:\n%s",
				timeout, readFileString(logPath))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// assertSeeded proves the invocation seeded dbPath from the cwd corpus, never
// started the server, and ignored the OFF_BY_ONE_DB fallback.
func assertSeeded(t *testing.T, logPath, dbPath, envDB, title string, port int) {
	t.Helper()
	output := readFileString(logPath)
	if !strings.Contains(output, "seed complete:") {
		t.Errorf("seed did not run — output:\n%s", output)
	}
	if strings.Contains(output, "listening on") {
		t.Errorf("server startup log line found — the seed invocation served HTTP:\n%s", output)
	}
	if portListening(port) {
		t.Errorf("something is still listening on 127.0.0.1:%d after the invocation exited", port)
	}
	st, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("--db path %s was not created: %v\noutput:\n%s", dbPath, err, output)
	}
	if st.Size() == 0 {
		t.Fatalf("--db path %s was created but is empty", dbPath)
	}
	if _, err := os.Stat(envDB); err == nil {
		t.Errorf("OFF_BY_ONE_DB path %s was created — the --db flag was ignored", envDB)
	}
	if !classExists(t, dbPath, title) {
		t.Errorf("seeded db %s does not hold the cwd corpus %q\noutput:\n%s", dbPath, title, output)
	}
}

// TestSeedAfterLeadingServerFlagsSeedsAndNeverServes is the OB-GAP-071
// acceptance test: `off-by-one --db PATH --port N seed` must honour PATH,
// finish seeding, and never bind the port. Against the pre-fix source the
// binary boots the server here and this test fails.
func TestSeedAfterLeadingServerFlagsSeedsAndNeverServes(t *testing.T) {
	bin := offByOneBinary(t)

	work := t.TempDir()
	const title = "leading-flag corpus class"
	writeCorpusRootNamed(t, filepath.Join(work, "data"), title)

	dbPath := filepath.Join(t.TempDir(), "leading.db")
	envDB := filepath.Join(t.TempDir(), "env.db")
	t.Setenv("OFF_BY_ONE_DB", envDB) // must lose to the --db flag
	port := freePort(t)

	cmd, logPath := startCLI(t, bin, work, "--db", dbPath, "--port", strconv.Itoa(port), "seed")
	awaitSeedExit(t, cmd, port, logPath, 60*time.Second)

	assertSeeded(t, logPath, dbPath, envDB, title, port)
}

// TestSeedSubcommandFirstStillSeeds proves the historical invocation still
// works as a real process: `off-by-one seed -db PATH` seeds and exits.
func TestSeedSubcommandFirstStillSeeds(t *testing.T) {
	bin := offByOneBinary(t)

	work := t.TempDir()
	const title = "leading-subcommand corpus class"
	writeCorpusRootNamed(t, filepath.Join(work, "data"), title)

	dbPath := filepath.Join(t.TempDir(), "first.db")
	envDB := filepath.Join(t.TempDir(), "env.db")
	t.Setenv("OFF_BY_ONE_DB", envDB)
	port := freePort(t)

	cmd, logPath := startCLI(t, bin, work, "seed", "-db", dbPath)
	awaitSeedExit(t, cmd, port, logPath, 60*time.Second)

	assertSeeded(t, logPath, dbPath, envDB, title, port)
}

// TestServerStartupStillParsesFlags guards the reordered registration: with no
// subcommand the binary must still parse server flags, log the requested port
// and database, and bind that port.
func TestServerStartupStillParsesFlags(t *testing.T) {
	bin := offByOneBinary(t)

	work := t.TempDir()
	dbPath := filepath.Join(work, "server.db")
	port := freePort(t)

	cmd, logPath := startCLI(t, bin, work, "--port", strconv.Itoa(port), "--db", dbPath, "--skip-sandbox")
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	output := waitForFileLine(t, logPath, fmt.Sprintf("listening on :%d", port), 60*time.Second)
	if !strings.Contains(output, "db="+dbPath) {
		t.Errorf("server ignored --db: want db=%s\noutput:\n%s", dbPath, output)
	}
	deadline := time.Now().Add(10 * time.Second)
	for !portListening(port) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !portListening(port) {
		t.Errorf("server did not bind 127.0.0.1:%d\noutput:\n%s", port, readFileString(logPath))
	}
}

// --- helpers ---------------------------------------------------------------

// freePort returns a port the OS just handed out and released, so the tests
// know exactly which port must stay unbound.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}
	return port
}

// portListening reports whether anything accepts TCP connections on port.
func portListening(port int) bool {
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func readFileString(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("<no output: %v>", err)
	}
	return string(b)
}

// waitForFileLine polls the child's log file until it contains want.
func waitForFileLine(t *testing.T, path, want string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	last := ""
	for {
		last = readFileString(path)
		if strings.Contains(last, want) {
			return last
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %q in %s; output:\n%s", want, path, last)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
