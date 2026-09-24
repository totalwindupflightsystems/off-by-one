package graph

import (
	"context"
	"errors"
	"testing"
)

// TestIsPlaceholderClass locks the placeholder/probe predicate that guards
// the public serve surface (OB-GAP-061). It must match the same families as
// EXCLUDED_CLASS_PATTERNS in scripts/export-answers.py, and must NOT match
// real engineering classes whose titles merely contain "test".
func TestIsPlaceholderClass(t *testing.T) {
	placeholder := []string{
		"off-by-one-self-test",
		"off-by-one-self-test-2026-07-29-tick199",
		"off-by-one-self-test-tick155",
		"self-test",
		"tick12-self-test",
		"foreman-tick-132-self-test",
		"Self-Test", // case-insensitive
		"self-dogfood-tick23",
		"test-self-dogfood",
		"test-self-dogfood-1",
		"dogfood-field-test-alpha",
		"dogfood-field-test-2",
		"docs-canary-001",
		"docs-canary-x",
		"test",
		"TEST",
		"test-gap-sweep",
		"test-gap-sweep-x",
		"test-foreman-tick",
		"test-foreman-t50",
		"e2e-tick42",
		"e2e-tick-109",
		"foreman-tick82-e2e",
		"tick89-e2e",
		"shell-script-e2e",
		"foreman-e2e-verification-pipeline",
		"tick88-foreman-audit",
		"ds-007",
		"ds-007-tick-106",
		"shell-say-hello-test",
		"shell-echo-hello-fix",
	}
	for _, title := range placeholder {
		if !IsPlaceholderClass(title) {
			t.Errorf("IsPlaceholderClass(%q) = false, want true", title)
		}
	}

	real := []string{
		"docker-perms",
		"file-ownership",
		"test-mocking-http-requests",
		"test-property-based-shrinking",
		"latest-tag-pinning",
		"protest-signatures", // contains "test" substring only
		"",
		// DF-OFF-BY-ONE-15: titles that merely CONTAIN a protected word
		// mid-slug are real engineering classes, not probes. The unanchored
		// patterns 404ed these on discover despite stored, verified answers.
		"ob1-dogfood-fib-off-by-one-index",
		"ob1-ctx-canary-deploy-oom",
		"dogfood-stale-premise-filing",
		"canary-deliver-failure-recurrence",
		"python-canary-staleness-probe",
		"how-to-test-a-canary-deployment-strategy",
		// fused "selftest" (no separator) is a real word, not the probe token
		"bash-gate-selftest-asserts-repo-state-nonhermetic",
		// real E2E engineering classes (not e2e-tickNN probe rows)
		"bash-e2e-battery-hang-on-cli-subcommand",
		"go-e2e-battery",
		"python-sdk-e2e-battery",
		"e2e-battery-certifies-stale-binary",
	}
	for _, title := range real {
		if IsPlaceholderClass(title) {
			t.Errorf("IsPlaceholderClass(%q) = true, want false", title)
		}
	}
}

// TestIsPlaceholderClass_ContainsWordNotProbe is the DF-OFF-BY-ONE-15
// regression pin: anchoring must be driven by the probe families' real
// shapes, so a title whose slug merely CONTAINS "dogfood"/"canary"/
// "field-test"/"self-test" mid-way is served, while every documented probe
// family still quarantines.
func TestIsPlaceholderClass_ContainsWordNotProbe(t *testing.T) {
	notProbes := []string{
		"ob1-dogfood-fib-off-by-one-index",
		"ob1-ctx-canary-deploy-oom",
		"test-mocking-http-requests",
		"how-to-test-a-canary-deployment-strategy",
		"dogfood-stale-premise-filing",
		"canary-deliver-failure-recurrence",
		"python-canary-staleness-probe",
		"bash-gate-selftest-asserts-repo-state-nonhermetic",
	}
	for _, title := range notProbes {
		if IsPlaceholderClass(title) {
			t.Errorf("IsPlaceholderClass(%q) = true, want false (contains-word is not a probe)", title)
		}
	}

	probes := []string{
		// self-test family (separator-delimited token)
		"off-by-one-self-test-ping",
		"off-by-one-self-test",
		"self-test",
		"tick12-self-test",
		"foreman-tick-132-self-test",
		// dogfood probe families
		"self-dogfood-probe",
		"test-self-dogfood-1",
		"dogfood-field-test-2",
		// canary probe family
		"docs-canary-3",
		"docs-canary-x",
		// bare / prefix placeholders
		"test",
		"test-gap-sweep-x",
		"test-foreman-t50",
		// e2e probe families
		"e2e-tick7",
		"foreman-tick82-e2e",
		"tick89-e2e",
		"shell-script-e2e",
		"foreman-e2e-verification-pipeline",
		// one-off probe titles
		"tick88-foreman-audit",
		"ds-007",
		"ds-007-tick-106",
		"shell-say-hello-test",
		"shell-echo-hello-fix",
	}
	for _, title := range probes {
		if !IsPlaceholderClass(title) {
			t.Errorf("IsPlaceholderClass(%q) = false, want true (probe family)", title)
		}
	}
}

// TestStore_Discovery_PlaceholderClass verifies that a placeholder class
// with a verified answer in the DB is served exactly like an unknown class:
// ErrNotFound (which the discover handler maps to 404) (OB-GAP-061).
func TestStore_Discovery_PlaceholderClass(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.CreateProblemClass(ctx, "off-by-one-self-test", "probe"); err != nil {
		t.Fatalf("create: %v", err)
	}
	cid := mustGetClassID(t, s, ctx, "off-by-one-self-test")
	aid, err := s.CreateAnswerNode(ctx, cid, 0, "", "", "", "placeholder solution", "evidence", `{}`)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if err := s.UpdateAnswerStatus(ctx, aid, AnswerVerified); err != nil {
		t.Fatalf("status: %v", err)
	}

	_, err = s.Discovery(ctx, "off-by-one-self-test", "", "", "", false)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Discovery err = %v, want ErrNotFound", err)
	}

	// Regression: a normal class with a verified answer still discovers.
	if _, err := s.CreateProblemClass(ctx, "file-ownership", "files"); err != nil {
		t.Fatalf("create: %v", err)
	}
	cid2 := mustGetClassID(t, s, ctx, "file-ownership")
	aid2, err := s.CreateAnswerNode(ctx, cid2, 0, "docker", "go", "go-1.25", "use --chown", "verified", `{}`)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if err := s.UpdateAnswerStatus(ctx, aid2, AnswerVerified); err != nil {
		t.Fatalf("status: %v", err)
	}
	res, err := s.Discovery(ctx, "file-ownership", "", "", "", false)
	if err != nil {
		t.Fatalf("Discovery normal class: %v", err)
	}
	if res.Exact == nil {
		t.Fatal("normal class: no exact match")
	}
}
