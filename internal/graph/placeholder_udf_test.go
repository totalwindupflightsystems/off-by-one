package graph

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	schemasql "github.com/totalwindupflightsystems/off-by-one/sql/schema"
)

// TestPlaceholderClassSQLFunc_MatchesGoPredicate is the SYNC GUARANTEE for
// the SQL surface added by OB-GAP-084: for every title, the SQL predicate
// graph.NotPlaceholderClassSQL must agree with the Go predicate
// IsPlaceholderClass — the authoritative list in placeholder.go, whose doc
// comment requires it to stay in sync with scripts/export-answers.py.
//
// The corpus is deliberately mixed: every family from the Go list, the
// real-class counter-examples that merely CONTAIN "test", case variants
// (the Go regexes are (?i), and a LIKE-based transcription would only be
// case-insensitive for ASCII), and titles that break a hand-written
// LIKE/GLOB transcription (`_` is a single-character wildcard in LIKE;
// `[0-9]` classes only exist in GLOB).
//
// A test that only asserted the placeholder direction would pass for a
// predicate that matches EVERYTHING; asserting both directions for every
// title is what makes the two definitions provably identical.
func TestPlaceholderClassSQLFunc_MatchesGoPredicate(t *testing.T) {
	store := udfTestStore(t)
	ctx := context.Background()
	db := store.DB()

	cases := []string{
		// Go-list families.
		"off-by-one-self-test",
		"self-test",
		"tick12-self-test",
		"Self-Test",
		"SELF-TEST",
		"test-self-dogfood",
		"self_dogfood",
		"dogfood-field-test-alpha",
		"dogfood",
		"docs-canary-001",
		"DOCS-CANARY",
		"canary",
		"test",
		"TEST",
		"test-gap-sweep",
		"test-foreman-tick",
		"e2e-tick42",
		"foreman-tick82-e2e",
		"tick89-e2e",
		"TICK89-E2E",
		"shell-script-e2e",
		"foreman-e2e-verification-pipeline",
		"tick88-foreman-audit",
		"ds-007",
		"ds-007-tick-106",
		"shell-say-hello-test",
		"shell-echo-hello-fix",
		// Real engineering classes — must NOT be excluded, including the
		// "test" substring counter-examples the Go list documents.
		"docker-perms",
		"file-ownership-after-container-transfer",
		"test-mocking-http-requests",
		"test-property-based-shrinking",
		"protest-signatures",
		"latest-tag-pinning",
		"docker_perms",
		"",
	}

	for _, title := range cases {
		goExcludes := IsPlaceholderClass(title)
		var kept int
		// The predicate is evaluated exactly as the queue reads evaluate
		// it: as a WHERE clause over a title value. `kept` counts the rows
		// that SURVIVE the exclusion, so kept == 0 means "this title is
		// treated as a placeholder".
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM (SELECT ? AS problem_class) WHERE `+NotPlaceholderClassSQL,
			title).Scan(&kept); err != nil {
			t.Fatalf("evaluate %s(%q): %v", PlaceholderClassSQLFunc, title, err)
		}
		sqlExcludes := kept == 0
		if sqlExcludes != goExcludes {
			t.Errorf("SQL exclusion(%q) = %v, Go IsPlaceholderClass = %v — the SQL surface drifted from the authoritative Go list",
				title, sqlExcludes, goExcludes)
		}
	}

	// A NULL title must not be excluded: problem_class is NOT NULL in the
	// schema, so this only pins the direction a malformed input resolves to
	// (list the row, never silently swallow it).
	var nullRows int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM (SELECT NULL AS problem_class) WHERE `+NotPlaceholderClassSQL).Scan(&nullRows); err != nil {
		t.Fatalf("evaluate over NULL: %v", err)
	}
	if nullRows != 1 {
		t.Errorf("NULL problem_class excluded from the predicate (rows = %d, want 1)", nullRows)
	}
}

// TestRegisterPlaceholderClassSQLFunc_Idempotent pins the memoised
// registration: the modernc.org/sqlite driver rejects a second registration
// of the same name, so opening a second Store (or calling the exported
// registration helper again) must not fail.
func TestRegisterPlaceholderClassSQLFunc_Idempotent(t *testing.T) {
	udfTestStore(t) // registers through graph.Open
	if err := RegisterPlaceholderClassSQLFunc(); err != nil {
		t.Fatalf("second registration: %v", err)
	}
	// The function is still callable from a connection opened after the
	// repeat registration.
	s := udfTestStore(t)
	var got string
	if err := s.DB().QueryRow(`SELECT ` + PlaceholderClassSQLFunc + `('docs-canary-1')`).Scan(&got); err != nil {
		t.Fatalf("call after re-registration: %v", err)
	}
	if got != "1" {
		t.Errorf("%s('docs-canary-1') = %q, want \"1\"", PlaceholderClassSQLFunc, got)
	}
}

// TestNotPlaceholderClassSQL_PagesInSQL guards the MECHANISM the queue read
// paths depend on (OB-GAP-084): the exclusion must be part of the SQL that
// SQLite plans, so the page is bounded by LIMIT instead of by a Go slice
// over a fully materialised result set.
//
// Two assertions, because neither alone is conclusive:
//   - the PLAN must show the newest-first ordering walked in index order
//     (a temp B-tree means the whole match set is gathered and sorted
//     before any bound could apply);
//   - the prepared PROGRAM must carry the limit counter (DecrJumpZero) and
//     no Sort opcode. SQLite omits the LIMIT step from the plan TEXT, so
//     the opcode is the only reliable evidence that the read stops early.
func TestNotPlaceholderClassSQL_PagesInSQL(t *testing.T) {
	store := udfTestStore(t)
	ctx := context.Background()
	// The canonical queue table (same SQL ingest.Open applies), so the plan
	// this asserts is the plan the store's read paths actually get.
	if err := store.ApplyExtra(schemasql.QueueSchema); err != nil {
		t.Fatalf("apply queue schema: %v", err)
	}

	qry := `SELECT id FROM queue_entries
		WHERE status = 'pending' AND ` + NotPlaceholderClassSQL + `
		ORDER BY created_at DESC, id ASC
		LIMIT 1 OFFSET 0`
	plan := udfQueryPlan(t, ctx, store.DB(), "EXPLAIN QUERY PLAN "+qry)
	if !strings.Contains(plan, "idx_queue_entries_status_created") {
		t.Errorf("page query does not use idx_queue_entries_status_created:\n%s", plan)
	}
	if strings.Contains(plan, "USE TEMP B-TREE FOR ORDER BY") {
		t.Errorf("page query sorts in a temp B-tree — the ordering is not walked in index order:\n%s", plan)
	}

	ops := udfProgram(t, ctx, store.DB(), qry)
	if !programOpcode(ops, "DecrJumpZero") {
		t.Errorf("page program has no limit counter (DecrJumpZero) — the read is not bounded by SQL:\n%s", strings.Join(ops, " "))
	}
	if programOpcode(ops, "Sort") {
		t.Errorf("page program sorts its result set — the whole match set is materialised:\n%s", strings.Join(ops, " "))
	}
	if !programOpcode(ops, "Function") {
		t.Errorf("page program does not call %s:\n%s", PlaceholderClassSQLFunc, strings.Join(ops, " "))
	}
}

// udfTestStore returns a Store backed by a fresh temp-file database, named
// to avoid colliding with store_test.go's newTestStore helper.
func udfTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("graph.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// udfQueryPlan renders EXPLAIN QUERY PLAN for qry as the detail column
// joined with newlines.
func udfQueryPlan(t *testing.T, ctx context.Context, db *sql.DB, qry string) string {
	t.Helper()
	rows, err := db.QueryContext(ctx, qry)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()
	var out strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan plan row: %v", err)
		}
		out.WriteString(detail)
		out.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("plan rows: %v", err)
	}
	return out.String()
}

// udfProgram returns the opcode names of the prepared program for qry, which
// must NOT be prefixed with EXPLAIN here.
func udfProgram(t *testing.T, ctx context.Context, db *sql.DB, qry string, args ...any) []string {
	t.Helper()
	rows, err := db.QueryContext(ctx, "EXPLAIN "+qry, args...)
	if err != nil {
		t.Fatalf("explain program: %v", err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("program columns: %v", err)
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	var ops []string
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatalf("scan program: %v", err)
		}
		var op string
		switch v := vals[1].(type) {
		case string:
			op = v
		case []byte:
			op = string(v)
		}
		ops = append(ops, op)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("program rows: %v", err)
	}
	return ops
}

func programOpcode(ops []string, opcode string) bool {
	for _, op := range ops {
		if op == opcode {
			return true
		}
	}
	return false
}
