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
// Paths already present in alreadyMounted are skipped — this prevents
// duplicate --ro-bind entries when a resolved path is subsumed by a broader
// mount (e.g. /usr/bin/git is under /usr which is in DefaultReadOnlyPaths).
func ResolveTools(tools []string, alreadyMounted []string) (resolved []string, missing []string) {
	// Build a set of already-mounted paths for O(1) prefix checks.
	mounted := make(map[string]bool, len(alreadyMounted))
	for _, p := range alreadyMounted {
		mounted[p] = true
	}

	seen := make(map[string]bool) // dedupe across tools
	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool == "" {
			continue
		}
		paths := resolveToolPaths(tool)
		if len(paths) == 0 {
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
// inside the sandbox. Returns nil if the tool cannot be resolved.
func resolveToolPaths(tool string) []string {
	switch tool {
	case "git":
		// git is already in DefaultReadOnlyPaths (/usr/bin/git +
		// /usr/lib/git-core). We still resolve so the dedup logic
		// can skip them gracefully.
		return lookPathMulti("git", []string{"/usr/lib/git-core"})
	case "python3-venv":
		// The python3 binary plus venv support directories.
		paths := lookPathMulti("python3", nil)
		paths = append(paths, globDirs("/usr/lib/python3*", "venv")...)
		return paths
	case "jq", "parallel":
		return lookPathMulti(tool, nil)
	default:
		// Generic: just resolve the binary.
		return lookPathMulti(tool, nil)
	}
}

// lookPathMulti resolves the binary via exec.LookPath and optionally
// appends extra support directories that are verified to exist.
func lookPathMulti(binary string, extras []string) []string {
	path, err := exec.LookPath(binary)
	if err != nil {
		return nil
	}
	result := []string{path}
	for _, extra := range extras {
		if _, err := exec.LookPath(filepath.Join(extra, binary)); err == nil {
			result = append(result, extra)
		} else {
			// The extra path may be a directory, not a binary —
			// check if the directory itself exists.
			if dirExists(extra) {
				result = append(result, extra)
			}
		}
	}
	return result
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
