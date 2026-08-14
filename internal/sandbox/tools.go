package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveTools maps tool names (e.g. "jq", "parallel", "python3-venv") to
// host filesystem paths that should be bind-mounted read-only into the
// sandbox. It returns the deduplicated list of resolved paths and the list
// of tool names that could not be resolved on the host.
//
// Degrade-gracefully contract: a tool that cannot be resolved NEVER causes
// a failure. The missing names are returned so the caller can log a warning,
// and the solve proceeds without those tools. The pi-agent may still produce
// a useful answer using whatever tools are available.
//
// Symlink handling (OB-GAP-035): every path returned by exec.LookPath is
// resolved to its REALPATH via filepath.EvalSymlinks before any coverage
// decision. A symlink whose realpath is NOT under the effective mount set
// cannot be safely bind-mounted (bwrap fails with "Can't create file ...
// No such file or directory" when the symlink target is outside the mount
// set), so the tool is treated as missing — WARN + degrade — instead of
// hard-failing the whole solve.
//
// The effective mount set is DefaultReadOnlyPaths (bwrap always mounts
// those) united with alreadyMounted — callers pass the executor's
// ExtraReadOnlyPaths so tool realpaths under executor extras are
// correctly treated as covered.
//
// Paths already covered by the effective mount set are skipped — this
// prevents duplicate --ro-bind entries when a resolved path is subsumed
// by a broader mount (e.g. /usr/bin/git is under /usr).
func ResolveTools(tools []string, alreadyMounted []string) (resolved []string, missing []string) {
	// Build the effective mount set for O(1) prefix checks: the
	// caller-supplied mounts (executor extras) plus the bwrap defaults.
	mounted := make(map[string]bool, len(alreadyMounted)+len(DefaultReadOnlyPaths))
	for _, p := range alreadyMounted {
		mounted[p] = true
	}
	for _, p := range DefaultReadOnlyPaths {
		mounted[p] = true
	}

	seen := make(map[string]bool) // dedupe across tools
	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool == "" {
			continue
		}
		paths, ok := resolveToolPaths(tool, mounted)
		if !ok {
			missing = append(missing, tool)
			continue
		}
		for _, p := range paths {
			if isPathCovered(p, mounted) {
				continue
			}
			if seen[p] {
				continue
			}
			seen[p] = true
			resolved = append(resolved, p)
			mounted[p] = true
		}
	}
	sort.Strings(resolved)
	return resolved, missing
}

// resolveToolPaths maps a single tool name to the host paths needed
// inside the sandbox. The second return value is false when the tool
// cannot be resolved for sandbox use — not on PATH, a broken symlink,
// or (OB-GAP-035) its realpath resolves outside the effective mount
// set — in which case the caller reports the tool as missing and the
// solve degrades gracefully instead of failing on an unmountable
// bwrap --ro-bind.
func resolveToolPaths(tool string, mounted map[string]bool) ([]string, bool) {
	switch tool {
	case "git":
		// git is already in DefaultReadOnlyPaths (/usr/bin/git +
		// /usr/lib/git-core); the realpath coverage check dedups it.
		ok, err := lookPathUsable("git", mounted)
		return nil, err == nil && ok
	case "python3-venv":
		// The python3 binary plus venv support directories.
		ok, err := lookPathUsable("python3", mounted)
		if !ok || err != nil {
			return nil, false
		}
		paths := globDirs("/usr/lib/python3*", "venv")
		return paths, true
	case "jq", "parallel":
		ok, err := lookPathUsable(tool, mounted)
		return nil, err == nil && ok
	default:
		// Generic: just resolve the binary.
		ok, err := lookPathUsable(tool, mounted)
		return nil, err == nil && ok
	}
}

// lookPathUsable resolves the binary via exec.LookPath and reports
// whether it can be made available inside the sandbox.
//
// If the resolved binary is (or lives behind) a symlink,
// filepath.EvalSymlinks resolves it to its realpath. The realpath is
// what matters:
//   - if the realpath is under an already-mounted path, the tool is
//     already accessible inside the sandbox — the path is deduped
//     (skipped) and the tool is NOT reported missing;
//   - if the realpath is NOT under any mount, the tool cannot be
//     bind-mounted safely (bwrap would have to bind the symlink,
//     which fails because its target is outside the mount set), so
//     the tool is treated as missing — degrade gracefully, never
//     hard-fail the solve (OB-GAP-035).
//
// If EvalSymlinks fails (broken symlink, permission, etc.), the tool
// is likewise treated as missing.
func lookPathUsable(binary string, mounted map[string]bool) (bool, error) {
	path, err := exec.LookPath(binary)
	if err != nil {
		return false, err
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		// Broken symlink or unreadable target — cannot be mounted.
		return false, err
	}
	return isPathCovered(real, mounted), nil
}

// globDirs expands a glob pattern like /usr/lib/python3* and filters
// to directories that contain the given subdirectory (e.g. "venv").
func globDirs(pattern, subdir string) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	var result []string
	for _, m := range matches {
		// Only include if the subdir exists inside.
		if subdir != "" {
			check := filepath.Join(m, subdir)
			if !dirExists(check) {
				continue
			}
		}
		if dirExists(m) {
			result = append(result, m)
		}
	}
	return result
}

// isPathCovered reports whether path p is already accessible via an
// existing mount. A path is covered if it exactly matches a mounted
// path or is a subdirectory of one.
func isPathCovered(p string, mounted map[string]bool) bool {
	if mounted[p] {
		return true
	}
	// Check if p is under any already-mounted directory.
	current := p
	for {
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		if mounted[parent] {
			return true
		}
		current = parent
	}
	return false
}

// dirExists reports whether path is a directory.
func dirExists(path string) bool {
	// Use exec.LookPath's underlying stat via a helper; we can't
	// import os here without adding it, but filepath.Glob already
	// filters non-existent paths. Use os.Stat via a small wrapper.
	return statIsDir(path)
}

// statIsDir is split out for testability — it uses os.Stat to check
// whether the path is a directory.
func statIsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
