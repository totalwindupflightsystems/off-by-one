package graph

import "regexp"

// Placeholder (self-test / canary / probe) problem classes must never be
// served as pre-verified answers: they are synthetic probe rows, not real
// engineering knowledge. This predicate mirrors EXCLUDED_CLASS_PATTERNS in
// scripts/export-answers.py (the sibling list for the flat export corpus) —
// keep the two lists in sync.
//
// Matching is ANCHORED, not a bare substring search: every pattern is
// pinned to the shape the probe family actually takes (a ^ prefix for
// prefix families, an exact ^...$ for single probe titles, or a
// separator-delimited token for token-shaped families like self-test).
// Unanchored searches 404ed real engineering classes whose titles merely
// CONTAIN a protected word mid-slug (e.g. "ob1-dogfood-fib-off-by-one-index"
// and "ob1-ctx-canary-deploy-oom" were stored, solved and verified but
// discover returned 404 for both — DF-OFF-BY-ONE-15). The separator form
// matters too: in a hyphen/underscore-joined slug, - and _ are word
// boundaries, so \b anchors do not distinguish "self-test" (probe token)
// from the fused real word "selftest" ("bash-gate-selftest-..." is real
// engineering knowledge); probe tokens keep the internal separator and the
// fused real forms do not.
//
// This list is the AUTHORITATIVE definition, and it is the only one: the SQL
// predicate the queue read paths use (graph.NotPlaceholderClassSQL, backed by
// the ob1_is_placeholder_class scalar function in placeholder_udf.go)
// evaluates this slice rather than restating it in LIKE/GLOB, so adding a
// family here changes both surfaces at once. The parity is pinned by
// TestPlaceholderClassSQLFunc_MatchesGoPredicate and by the row-level oracle
// in internal/ingest (OB-GAP-084).
//
// Keep these specific: real engineering classes whose titles merely contain
// "test" (e.g. "test-mocking-http-requests", "test-property-based-shrinking")
// must NOT match.
var placeholderClassPatterns = []*regexp.Regexp{
	// off-by-one-self-test-* family: every live member starts with the prefix.
	regexp.MustCompile(`(?i)^off-by-one-self-test`),
	// self-test family, separator-delimited: bare "self-test", tick12-self-test,
	// foreman-tick-132-self-test, e2e-tick99-self-test. Fused "selftest" (no
	// separator) is a real engineering word and must NOT match.
	regexp.MustCompile(`(?i)(?:^|[-_])self[-_]test(?:[-_]|$)`),
	// self-dogfood probes (self-dogfood-tick23).
	regexp.MustCompile(`(?i)^self[-_]dogfood`),
	// test-self-dogfood probe.
	regexp.MustCompile(`(?i)^test-self-dogfood`),
	// dogfood-field-test-* probe prefix.
	regexp.MustCompile(`(?i)^dogfood-field-test`),
	// the bare placeholder row "dogfood" (exact).
	regexp.MustCompile(`(?i)^dogfood$`),
	// docs-canary-* probe prefix.
	regexp.MustCompile(`(?i)^docs-canary`),
	// bare "canary" placeholder row (exact) — mid-slug "canary" is real.
	regexp.MustCompile(`(?i)^canary$`),
	// bare "test" placeholder class (exact).
	regexp.MustCompile(`(?i)^test$`),
	// test-gap-sweep probe family (prefix: test-gap-sweep, test-gap-sweep-x).
	regexp.MustCompile(`(?i)^test-gap-sweep`),
	// test-foreman-* probe prefix family.
	regexp.MustCompile(`(?i)^test-foreman-`),
	// e2e-tickNN probe classes (also the hyphenated e2e-tick-109 form).
	regexp.MustCompile(`(?i)^e2e-tick`),
	// reversed-form probes appearing mid-slug: foreman-tick82-e2e, tick89-e2e.
	// Genuinely token/substring-shaped, so the tail is a word boundary.
	regexp.MustCompile(`(?i)tick\d+-e2e\b`),
	// 0001-0017-era probe class (exact title).
	regexp.MustCompile(`(?i)^shell-script-e2e$`),
	// e2e-verification-pipeline probe (exact titles, both stored forms).
	regexp.MustCompile(`(?i)^e2e-verification-pipeline$`),
	regexp.MustCompile(`(?i)^foreman-e2e-verification-pipeline$`),
	// tick88-foreman-audit probe: the token sits mid-slug after "tick88-", so
	// it is delimiter-anchored rather than ^-prefixed.
	regexp.MustCompile(`(?i)(?:^|[-_])foreman[-_]audit(?:[-_]|$)`),
	// DS-007 probe family (ds-007, ds-007-tick-106).
	regexp.MustCompile(`(?i)^ds-007`),
	// one-off probe class names (exact titles).
	regexp.MustCompile(`(?i)^shell-say-hello-test$`),
	regexp.MustCompile(`(?i)^shell-echo-hello-fix$`),
}

// IsPlaceholderClass reports whether a problem-class title is a
// self-test/canary/probe placeholder that must be excluded from the public
// serve surface (discover, queue listing). Sibling of
// is_excluded_class() in scripts/export-answers.py.
func IsPlaceholderClass(title string) bool {
	for _, p := range placeholderClassPatterns {
		if p.MatchString(title) {
			return true
		}
	}
	return false
}
