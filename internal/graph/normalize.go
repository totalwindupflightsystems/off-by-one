package graph

// REVIEW-OB-010: env/version are exact-match discovery filters (see
// discovery.go — bestAnswer ranks answers by (env, lang, version) tuple
// specificity, and empty values act as wildcards). Corpus answers were
// recorded with free-text prose in these fields ("linux host, go 1.26.6",
// "~/auger; DuckBrain live substrate; ...", "api.deepseek.com/v1, verified
// 2026-09-15T02:39Z"), which makes them unreachable via tuple-scoped
// discovery and leaks host context into the public catalog.
//
// NormalizeEnv and NormalizeVersion canonicalize the two fields at the
// store write funnel (CreateAnswerNode) and at export render time:
//
//  1. a value that is already token-shaped (no whitespace, ';' or '~')
//     passes through, lowercased and trimmed;
//  2. otherwise, if the leading comma/semicolon segment is itself a clean
//     token ("Linux, crier repo ~/crier" -> "linux"), that segment wins;
//  3. otherwise prose is mapped to a canonical token when a recognizable
//     pattern exists ("CI on GitHub Actions" -> "github-actions");
//  4. anything else is dropped to "" — the discovery wildcard — rather
//     than keeping prose in a filtered field.
//
// The rules are deliberately small and deterministic; the same mapping is
// mirrored in scripts/export-answers.py for the flat-file export.

import (
	"regexp"
	"strings"
)

// tokenForbidden matches the characters that must never appear in a
// canonical env/version filter token: whitespace, ';' and '~'.
var tokenForbidden = regexp.MustCompile(`[\s;~]`)

// envPatterns maps recognizable prose substrings to canonical environment
// tokens. Ordered — first match wins: CI/container platforms precede bare
// OS tokens (a GitHub Actions runner IS ubuntu; the CI context is the
// distinguishing one) and OS tokens precede language runtimes.
var envPatterns = []struct {
	re    *regexp.Regexp
	token string
}{
	{regexp.MustCompile(`github[- ]actions|actions runner`), "github-actions"},
	{regexp.MustCompile(`docker`), "docker"},
	{regexp.MustCompile(`kubernetes|k8s`), "kubernetes"},
	{regexp.MustCompile(`darwin|macos|mac os`), "darwin"},
	{regexp.MustCompile(`wsl|linux|ubuntu|debian|posix`), "linux"},
	{regexp.MustCompile(`windows`), "windows"},
	{regexp.MustCompile(`production`), "production"},
	{regexp.MustCompile(`vitest|nodejs|\bnode\b|nextjs|pnpm|\bnpm\b`), "node"},
	{regexp.MustCompile(`pytest|python|venv`), "python3"},
	{regexp.MustCompile(`\bgo ?1\.26|\bgo1\.26`), "go1.26"},
	{regexp.MustCompile(`golang|\bgo\b`), "go"},
	{regexp.MustCompile(`\bbash\b`), "bash"},
	{regexp.MustCompile(`\bshell\b|\bsh\b`), "shell"},
}

// versionToken is a canonical version shape: an optional v prefix followed
// by dotted numeric components ("1.26", "v1.0.0", "22").
var versionToken = regexp.MustCompile(`^v?\d+(\.\d+)*$`)

// versionBranchTokens are non-numeric version values worth preserving from
// prose ("main e50aea4" -> "main").
var versionBranchTokens = map[string]bool{"latest": true, "main": true, "master": true}

// cleanToken returns value lowercased/trimmed when it is already
// token-shaped (no whitespace, ';' or '~'), and ok=false otherwise.
func cleanToken(value string) (tok string, ok bool) {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" || tokenForbidden.MatchString(v) {
		return "", false
	}
	return v, true
}

// firstSegment returns the text before the first ',' or ';'.
func firstSegment(v string) string {
	if i := strings.IndexAny(v, ",;"); i >= 0 {
		return strings.TrimSpace(v[:i])
	}
	return v
}

// NormalizeEnv canonicalizes an environment filter token (REVIEW-OB-010).
// Prose with no recognizable pattern is dropped to "" (the discovery
// wildcard); it never survives into the filtered field.
func NormalizeEnv(value string) string {
	if tok, ok := cleanToken(value); ok {
		return tok
	}
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return ""
	}
	if first := firstSegment(v); first != "" && !tokenForbidden.MatchString(first) {
		return first
	}
	for _, p := range envPatterns {
		if p.re.MatchString(v) {
			return p.token
		}
	}
	return ""
}

// NormalizeVersion canonicalizes a version filter token (REVIEW-OB-010).
// Prose yields the first whitespace-delimited token that is version-shaped
// ("gitreins 0.14.0" -> "0.14.0"), then a branch token ("main e50aea4" ->
// "main"), and is dropped to "" when neither exists.
func NormalizeVersion(value string) string {
	if tok, ok := cleanToken(value); ok {
		return tok
	}
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return ""
	}
	for _, raw := range strings.Fields(v) {
		if t := strings.Trim(raw, ",;"); versionToken.MatchString(t) {
			return t
		}
	}
	for _, raw := range strings.Fields(v) {
		if t := strings.Trim(raw, ",;"); versionBranchTokens[t] {
			return t
		}
	}
	return ""
}
