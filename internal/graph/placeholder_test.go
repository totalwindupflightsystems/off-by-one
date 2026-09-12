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
		"self-test",
		"tick12-self-test",
		"Self-Test", // case-insensitive
		"test-self-dogfood",
		"dogfood-field-test-alpha",
		"docs-canary-001",
		"test",
		"TEST",
		"test-gap-sweep",
		"test-foreman-tick",
		"e2e-tick42",
		"foreman-tick82-e2e",
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
	}
	for _, title := range real {
		if IsPlaceholderClass(title) {
			t.Errorf("IsPlaceholderClass(%q) = true, want false", title)
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
