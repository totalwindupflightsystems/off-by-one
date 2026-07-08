// Package sandbox wraps the bwrap (bubblewrap) command-line tool
// for unprivileged containerization of Pi Agent solves. The sandbox
// owns a per-solve workspace directory, copies the problem file in,
// runs an arbitrary command, and exposes the workspace for the
// caller to extract results.
//
// Why bwrap: it gives unprivileged filesystem + PID + session
// isolation in ~10ms startup with no daemon. See specs/system-spec.md
// §6 for the full rationale.
package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Config controls a single sandbox. The defaults are tuned for the
// off-by-one Pi Agent solve path: 5-minute wall clock, 1GB tmpfs on
// /tmp, a private workspace under the caller's chosen WorkDir.
type Config struct {
	// BwrapPath is the absolute path to the bwrap binary. Required.
	// An empty string causes Create to return ErrBwrapNotFound.
	BwrapPath string

	// WorkDir is the parent directory for per-solve workspaces. Each
	// Create() call creates a new subdirectory under here. Defaults
	// to os.TempDir() if empty.
	WorkDir string

	// Timeout is the wall-clock cap for the sandboxed command.
	// Defaults to 5 minutes when zero.
	Timeout time.Duration

	// ExtraEnv is appended to the child process's environment. The
	// PATH is preserved by default (bwrap --bind passes the host
	// path) — extra vars override.
	ExtraEnv []string

	// ReadOnlyPaths are paths to bind read-only into the sandbox.
	// Defaults to the standard library set (/usr, /lib, /lib64, /bin)
	// when empty. Add /etc for tools that need resolver config.
	ReadOnlyPaths []string

	// ExtraReadOnlyPaths are appended to the default ReadOnlyPaths
	// (or the explicit ReadOnlyPaths if set). Use this to add
	// tool-specific paths without overriding the defaults.
	ExtraReadOnlyPaths []string
}

// DefaultBwrapTimeout is the spec-recommended 5-minute cap.
const DefaultBwrapTimeout = 5 * time.Minute

// ErrBwrapNotFound is returned when Config.BwrapPath is empty or
// points to a non-existent binary.
var ErrBwrapNotFound = errors.New("sandbox: bwrap not found at configured path")

// ErrAlreadyDestroyed is returned by operations on a sandbox that
// has already been destroyed.
var ErrAlreadyDestroyed = errors.New("sandbox: already destroyed")

// Sandbox is a running bwrap container bound to a workspace dir.
// The zero value is not usable — call Executor.Create().
type Sandbox struct {
	id      string
	cfg     Config
	workDir string
	mu      sync.Mutex
	cmd     *exec.Cmd
	done    chan struct{}
}

// ID returns the unique identifier for this sandbox. The same value
// is used for the workspace subdirectory name.
func (s *Sandbox) ID() string { return s.id }

// WorkDir returns the absolute path to the per-solve workspace
// directory. The caller can write problem files here before Run()
// and read results out after.
func (s *Sandbox) WorkDir() string { return s.workDir }

// Executor creates sandboxes. The zero value uses defaults: a
// /usr/bin/bwrap path, os.TempDir() as the work parent, and the
// spec's 5-minute timeout.
type Executor struct {
	// BwrapPath is the default bwrap path for Create() calls that
	// don't override Config.BwrapPath. When both are empty, returns
	// ErrBwrapNotFound.
	BwrapPath string

	// WorkDir is the default parent for sandbox workspaces.
	WorkDir string

	// Timeout is the default per-Run timeout.
	Timeout time.Duration

	// ExtraReadOnlyPaths are appended to the sandbox's default
	// read-only bind mounts. Use this for tool-specific paths
	// (e.g. /home/user/.local/bin for pi-agent).
	ExtraReadOnlyPaths []string
}

// NewExecutor returns an Executor using the OS-detected bwrap path
// (or /usr/bin/bwrap as a fallback) and the system temp dir for
// workspaces. Use this in production code paths.
func NewExecutor() *Executor {
	return &Executor{
		BwrapPath: lookupBwrap(),
		WorkDir:   os.TempDir(),
		Timeout:   DefaultBwrapTimeout,
	}
}

// Create allocates a new workspace directory and returns a Sandbox
// bound to it. The bwrap process is NOT started yet — call Run() to
// execute a command inside the sandbox.
//
// The workspace lives at <WorkDir>/<id>/. Use CopyIn() to populate
// it before Run, and CopyOut() to harvest results after.
func (e *Executor) Create(ctx context.Context, id string, cfg Config) (*Sandbox, error) {
	// Merge defaults from the Executor with the per-call Config.
	if cfg.BwrapPath == "" {
		cfg.BwrapPath = e.BwrapPath
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = e.WorkDir
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = e.Timeout
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultBwrapTimeout
	}
	if cfg.WorkDir == "" {
		cfg.WorkDir = os.TempDir()
	}
	if cfg.BwrapPath == "" {
		cfg.BwrapPath = "/usr/bin/bwrap"
	}
	if _, err := os.Stat(cfg.BwrapPath); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrBwrapNotFound
		}
		return nil, fmt.Errorf("stat bwrap: %w", err)
	}
	if cfg.ReadOnlyPaths == nil {
		cfg.ReadOnlyPaths = []string{"/usr", "/lib", "/lib64", "/bin"}
	}
	if cfg.ExtraReadOnlyPaths == nil {
		cfg.ExtraReadOnlyPaths = e.ExtraReadOnlyPaths
	} else if len(e.ExtraReadOnlyPaths) > 0 {
		// Merge: per-call extras come after executor defaults.
		cfg.ExtraReadOnlyPaths = append(
			append([]string{}, e.ExtraReadOnlyPaths...),
			cfg.ExtraReadOnlyPaths...,
		)
	}

	workDir := filepath.Join(cfg.WorkDir, "off-by-one-sandbox-"+id)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	return &Sandbox{
		id:      id,
		cfg:     cfg,
		workDir: workDir,
		done:    make(chan struct{}),
	}, nil
}

// CopyIn writes a file into the workspace at relPath. The path is
// relative to WorkDir() — subdirectories are created as needed.
// Returns ErrAlreadyDestroyed if Destroy() has already been called.
func (s *Sandbox) CopyIn(relPath string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workDir == "" {
		return ErrAlreadyDestroyed
	}
	full := filepath.Join(s.workDir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("mkdir for copy_in: %w", err)
	}
	return os.WriteFile(full, data, 0o644)
}

// CopyOut reads a file from the workspace at relPath and returns its
// contents. Returns an error if the file doesn't exist.
func (s *Sandbox) CopyOut(relPath string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workDir == "" {
		return nil, ErrAlreadyDestroyed
	}
	full := filepath.Join(s.workDir, relPath)
	return os.ReadFile(full)
}

// CopyInFile copies an existing host file into the workspace.
func (s *Sandbox) CopyInFile(relPath, srcPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workDir == "" {
		return ErrAlreadyDestroyed
	}
	full := filepath.Join(s.workDir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("mkdir for copy_in_file: %w", err)
	}
	in, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer in.Close()
	out, err := os.Create(full)
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

// Run executes cmd inside the sandbox and waits for it to finish
// (or until the configured timeout elapses). stdout and stderr are
// captured and returned.
//
// The bwrap invocation follows the spec §6.2 profile:
//   - unshare-all, die-with-parent, new-session for isolation
//   - tmpfs on /tmp, /var, /run
//   - ro-bind on /usr /lib /lib64 /bin (plus extras)
//   - bind on the workspace at /workspace
//   - proc /dev for minimal runtime
func (s *Sandbox) Run(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error) {
	s.mu.Lock()
	if s.workDir == "" {
		s.mu.Unlock()
		return nil, nil, ErrAlreadyDestroyed
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	bwrapArgs := buildBwrapArgs(s.cfg.ReadOnlyPaths, s.cfg.ExtraReadOnlyPaths, s.workDir)
	bwrapArgs = append(bwrapArgs, "--", name)
	bwrapArgs = append(bwrapArgs, args...)

	cmd := exec.CommandContext(timeoutCtx, s.cfg.BwrapPath, bwrapArgs...)
	cmd.Env = append(os.Environ(), s.cfg.ExtraEnv...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	s.cmd = cmd
	s.mu.Unlock()

	runErr := cmd.Run()
	s.mu.Lock()
	s.cmd = nil
	close(s.done)
	s.mu.Unlock()
	return outBuf.Bytes(), errBuf.Bytes(), runErr
}

// Kill terminates the running bwrap process if any. Safe to call
// from any goroutine and idempotent. Does NOT acquire s.mu — callers
// that hold the lock should use killLocked() instead.
func (s *Sandbox) Kill() {
	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		// Kill the whole process group — bwrap spawns the child
		// under the same session, and we want the child to die
		// too when the parent dies.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// killLocked is the inner Kill for callers that already hold s.mu.
// It avoids the self-deadlock when Destroy (which holds the lock)
// needs to kill the running process.
func (s *Sandbox) killLocked() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
	}
}

// Destroy stops the running command (if any) and removes the
// workspace directory. After Destroy, the Sandbox cannot be reused.
func (s *Sandbox) Destroy() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.workDir == "" {
		return nil
	}
	s.killLocked()
	workDir := s.workDir
	s.workDir = ""
	return os.RemoveAll(workDir)
}

// buildBwrapArgs constructs the bwrap argument list. The workspace
// is bind-mounted at /workspace inside the sandbox; commands are
// expected to live there (or in the read-only host paths).
func buildBwrapArgs(readOnlyPaths, extraReadOnlyPaths []string, workDir string) []string {
	args := []string{
		"--unshare-all",
		"--share-net",
		"--die-with-parent",
		"--new-session",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--tmpfs", "/var",
		"--tmpfs", "/run",
	}
	for _, p := range readOnlyPaths {
		args = append(args, "--ro-bind", p, p)
	}
	for _, p := range extraReadOnlyPaths {
		args = append(args, "--ro-bind", p, p)
	}
	args = append(args, "--bind", workDir, "/workspace")
	return args
}

// lookupBwrap returns the path to bwrap or a sensible default.
func lookupBwrap() string {
	if path, err := exec.LookPath("bwrap"); err == nil {
		return path
	}
	return "/usr/bin/bwrap"
}

// BwrapAvailable reports whether the system has bwrap installed.
// Used by tests to skip cleanly when bwrap is not present.
func BwrapAvailable() bool {
	path, err := exec.LookPath("bwrap")
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}
