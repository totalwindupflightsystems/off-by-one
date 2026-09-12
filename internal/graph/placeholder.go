package graph

import "regexp"

// Placeholder (self-test / canary / probe) problem classes must never be
// served as pre-verified answers: they are synthetic probe rows, not real
// engineering knowledge. This predicate mirrors EXCLUDED_CLASS_PATTERNS in
// scripts/export-answers.py (the sibling list for the flat export corpus) —
// keep the two lists in sync. Matching is a case-insensitive regex SEARCH
// against the class title, same semantics as the Python `re.search` version.
//
// Keep these specific: real engineering classes whose titles merely contain
// "test" (e.g. "test-mocking-http-requests", "test-property-based-shrinking")
// must NOT match.
var placeholderClassPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)self-test`),       // off-by-one-self-test family, bare "self-test"
	regexp.MustCompile(`(?i)self[-_]dogfood`), // self-dogfood probes
	regexp.MustCompile(`(?i)dogfood`),         // test-self-dogfood, dogfood-field-test-*
	regexp.MustCompile(`(?i)canary`),          // docs-canary-*
	regexp.MustCompile(`(?i)field-test`),      // dogfood-field-test-*
	regexp.MustCompile(`(?i)^test$`),          // bare "test" placeholder class
	regexp.MustCompile(`(?i)test-gap-sweep`),
	regexp.MustCompile(`(?i)test-foreman-`),
	regexp.MustCompile(`(?i)e2e-tick`),                  // e2e-tickNN probe classes
	regexp.MustCompile(`(?i)tick\d+-e2e`),               // reversed-form probes: foreman-tick82-e2e, tick89-e2e
	regexp.MustCompile(`(?i)shell-script-e2e`),          // 0001-0017-era probe class
	regexp.MustCompile(`(?i)e2e-verification-pipeline`), // foreman-e2e-verification-pipeline probe
	regexp.MustCompile(`(?i)foreman-audit`),             // tick88-foreman-audit probe
	regexp.MustCompile(`(?i)ds-007`),                    // DS-007 probe family (ds-007, ds-007-tick-106)
	regexp.MustCompile(`(?i)shell-say-hello-test`),
	regexp.MustCompile(`(?i)shell-echo-hello-fix`),
	regexp.MustCompile(`(?i)tick\d+-self-test`), // tickN-self-test variants
	regexp.MustCompile(`(?i)docs-canary`),
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
