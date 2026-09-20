package ingest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
)

// --- helpers ---------------------------------------------------------------

// sqliteQueryPlan renders EXPLAIN QUERY PLAN for qry as one detail per line.
//
// Note what this can and cannot show: SQLite's planner omits the LIMIT step
// from the plan TEXT (the bound is carried by the program's counter, not by
// a plan node), so the plan proves which INDEX is used and whether the
// ordering needs a sort. "The read is bounded" is asserted on the prepared
// program instead — see sqliteVDBEProgram and assertBoundedRead.
func sqliteQueryPlan(t *testing.T, ctx context.Context, q *Queue, qry string, args ...any) string {
	t.Helper()
	rows, err := q.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+qry, args...)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()
	var out strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		out.WriteString(detail)
		out.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("plan rows: %v", err)
	}
	return out.String()
}

// sqliteVDBEProgram returns the opcode names of the PREPARED program for qry
// with the given bound arguments.
//
// EXPLAIN (without QUERY PLAN) is the only reliable way to assert on the
// LIMIT: SQLite lowers `LIMIT n` to a register holding n plus a
// DecrJumpZero opcode that halts the scan after n rows, and `LIMIT ?
// OFFSET ?` to OffsetLimit (which loads both counters and delegates to
// that same scheme). Asserting those opcodes — and the ABSENCE of Sort —
// is what proves the read is bounded by SQL rather than by a Go slice.
func sqliteVDBEProgram(t *testing.T, ctx context.Context, q *Queue, qry string, args ...any) []string {
	t.Helper()
	rows, err := q.db.QueryContext(ctx, "EXPLAIN "+qry, args...)
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

func programHas(ops []string, opcode string) bool {
	for _, op := range ops {
		if op == opcode {
			return true
		}
	}
	return false
}

// assertBoundedRead asserts that a prepared statement stops early after its
// LIMIT: the limit counter opcode must be present (OffsetLimit for
// LIMIT ? OFFSET ?, DecrJumpZero for the plain-LIMIT form) and the program
// must contain no Sort (a sort means the whole match set was gathered before
// the bound could apply).
func assertBoundedRead(t *testing.T, name string, ops []string, wantOffsetLimit bool) {
	t.Helper()
	if wantOffsetLimit && !programHas(ops, "OffsetLimit") {
		t.Errorf("%s: no OffsetLimit opcode — LIMIT/OFFSET are not bounding the scan:\n%s",
			name, strings.Join(ops, " "))
	}
	if !programHas(ops, "DecrJumpZero") {
		t.Errorf("%s: no DecrJumpZero opcode — there is no limit counter stopping the scan:\n%s",
			name, strings.Join(ops, " "))
	}
	if programHas(ops, "Sort") {
		t.Errorf("%s: program sorts its result set — the whole match set is materialised before LIMIT can apply:\n%s",
			name, strings.Join(ops, " "))
	}
	if !programHas(ops, "Function") {
		t.Errorf("%s: program does not call %s — the placeholder exclusion is not in the statement:\n%s",
			name, graph.PlaceholderClassSQLFunc, strings.Join(ops, " "))
	}
}

// seedQueueRows inserts n plain pending rows whose ids start with marker and
// all share a created_at, so ordering among them is decided by the id
// tiebreak.
func seedQueueRows(t *testing.T, q *Queue, n int, marker, class string) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		if _, err := q.db.ExecContext(ctx, `INSERT INTO queue_entries
			(id, problem_class, status, priority, created_at)
			VALUES (?, ?, 'pending', 0, ?)`,
			fmt.Sprintf("%s%03d", marker, i), class, "2026-01-01 00:00:00"); err != nil {
			t.Fatalf("insert %s%03d: %v", marker, i, err)
		}
	}
}

// --- mechanism guards (OB-GAP-084) ----------------------------------------

// TestQueue_ListPage_SQLPushdown_NoGoPostFilter is the MECHANISM guard for
// OB-GAP-084. The honesty semantics of the listing (match count after
// placeholder exclusion, newest-first page, empty page with an honest total
// past the end) are pinned by TestQueue_List_NewestFirstAndHonestTotal and
// TestQueue_ListPage_TotalExcludesPlaceholdersAndCountsMatches; those tests
// stay green under either implementation, so they cannot detect a
// regression that moves the filter — and therefore the count and the page
// slice — back into Go.
//
// This test pins the implementation the row asks for: the placeholder
// exclusion is a SQL predicate on BOTH the page and the count query, the
// page carries LIMIT/OFFSET as bound arguments, and neither builder
// interpolates caller input into the SQL text.
func TestQueue_ListPage_SQLPushdown_NoGoPostFilter(t *testing.T) {
	pageQry, pageArgs := listQueuePageQuery(StatusPending, 25, 50)
	if !strings.Contains(pageQry, graph.NotPlaceholderClassSQL) {
		t.Errorf("page query does not carry the placeholder exclusion in SQL:\n%s", pageQry)
	}
	if got := strings.Count(pageQry, graph.PlaceholderClassSQLFunc); got != 1 {
		t.Errorf("page query calls %s %d times, want exactly 1:\n%s", graph.PlaceholderClassSQLFunc, got, pageQry)
	}
	if !strings.Contains(strings.ToUpper(pageQry), "LIMIT ?") {
		t.Errorf("page query has no LIMIT placeholder — the page would be sliced in Go:\n%s", pageQry)
	}
	if !strings.Contains(strings.ToUpper(pageQry), "OFFSET ?") {
		t.Errorf("page query has no OFFSET placeholder — the page would be sliced in Go:\n%s", pageQry)
	}
	// status + limit + offset, all bound as arguments.
	if len(pageArgs) != 3 {
		t.Fatalf("page args = %d (%v), want 3 (status, limit, offset)", len(pageArgs), pageArgs)
	}
	if pageArgs[0] != StatusPending || pageArgs[1] != 25 || pageArgs[2] != 50 {
		t.Errorf("page args = %v, want [%s 25 50]", pageArgs, StatusPending)
	}
	if strings.Contains(pageQry, "25") || strings.Contains(pageQry, "50") {
		t.Errorf("page limit/offset were interpolated into the SQL instead of bound:\n%s", pageQry)
	}

	countQry, countArgs := listQueueCountQuery(StatusPending)
	if !strings.Contains(countQry, "COUNT(*)") {
		t.Errorf("count query is not a COUNT(*):\n%s", countQry)
	}
	if !strings.Contains(countQry, graph.NotPlaceholderClassSQL) {
		t.Errorf("count query does not carry the placeholder exclusion in SQL — the count would be taken over unfiltered rows:\n%s", countQry)
	}
	if strings.Contains(strings.ToUpper(countQry), "LIMIT") || strings.Contains(strings.ToUpper(countQry), "OFFSET") {
		t.Errorf("count query is paginated — total must describe the whole match set:\n%s", countQry)
	}
	if len(countArgs) != 1 || countArgs[0] != StatusPending {
		t.Errorf("count args = %v, want [%s]", countArgs, StatusPending)
	}
	// A count query must not fetch rows: no shared column list.
	if strings.Contains(countQry, "SELECT id,") {
		t.Errorf("count query selects rows instead of counting them:\n%s", countQry)
	}

	// The unfiltered (status == "") shape: same exclusion, no status bind.
	allPage, allArgs := listQueuePageQuery("", 10, 0)
	if !strings.Contains(allPage, graph.NotPlaceholderClassSQL) || strings.Contains(allPage, "status = ?") {
		t.Errorf("unfiltered page query wrong:\n%s", allPage)
	}
	if len(allArgs) != 2 {
		t.Errorf("unfiltered page args = %v, want [limit offset]", allArgs)
	}
	allCount, allCountArgs := listQueueCountQuery("")
	if strings.Contains(allCount, "status = ?") || len(allCountArgs) != 0 {
		t.Errorf("unfiltered count query wrong: %s (args %v)", allCount, allCountArgs)
	}
}

// TestQueue_ListPage_ReadIsBoundedBySQL asserts the page and count reads are
// bounded by the STATEMENT rather than by a Go slice: the prepared programs
// must carry the limit counter, must not sort, and must call the
// placeholder predicate. See assertBoundedRead for why the plan text alone
// cannot prove this.
func TestQueue_ListPage_ReadIsBoundedBySQL(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()
	seedQueueRows(t, q, 30, "b", "real-class")

	for _, spec := range []struct {
		name string
		qry  string
		args []any
	}{
		{"status-filtered page", func() string { s, _ := listQueuePageQuery(StatusPending, 10, 5); return s }(), []any{StatusPending, 10, 5}},
		{"unfiltered page", func() string { s, _ := listQueuePageQuery("", 10, 5); return s }(), []any{10, 5}},
	} {
		ops := sqliteVDBEProgram(t, ctx, q, spec.qry, spec.args...)
		assertBoundedRead(t, spec.name, ops, true)
	}

	// The count is computed by SQLite (AggStep/AggFinal over the index) and
	// carries the exclusion; it fetches no rows for Go to count.
	countQry, countArgs := listQueueCountQuery(StatusPending)
	countOps := sqliteVDBEProgram(t, ctx, q, countQry, countArgs...)
	if !programHas(countOps, "AggStep") || !programHas(countOps, "AggFinal") {
		t.Errorf("count program is not an aggregate over the index:\n%s", strings.Join(countOps, " "))
	}
	if !programHas(countOps, "Function") {
		t.Errorf("count program does not call %s:\n%s", graph.PlaceholderClassSQLFunc, strings.Join(countOps, " "))
	}
	if programHas(countOps, "Sort") {
		t.Errorf("count program sorts:\n%s", strings.Join(countOps, " "))
	}
}

// TestPendingQueueOrder_LimitIsSQLBounded guards the second read path
// OB-GAP-084 moved: PendingQueueOrder used to materialise every pending row
// and cut the slice in Go. The cap now travels to SQL as a LIMIT, and the
// placeholder exclusion is the same SQL predicate the listing uses.
func TestPendingQueueOrder_LimitIsSQLBounded(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()

	seedQueueRows(t, q, 12, "r", "real-class")
	seedQueueRows(t, q, 3, "ph", "e2e-tick99") // placeholders

	order, err := q.PendingQueueOrder(ctx, 5)
	if err != nil {
		t.Fatalf("PendingQueueOrder: %v", err)
	}
	if len(order) != 5 {
		t.Fatalf("len = %d, want 5 (the requested cap)", len(order))
	}
	for _, e := range order {
		if graph.IsPlaceholderClass(e.ProblemClass) {
			t.Errorf("placeholder %q in the pending order", e.ProblemClass)
		}
	}
	// A cap larger than the match set returns everything, never more and
	// never a placeholder.
	all, err := q.PendingQueueOrder(ctx, 1000)
	if err != nil {
		t.Fatalf("PendingQueueOrder(1000): %v", err)
	}
	if len(all) != 12 {
		t.Errorf("len = %d, want 12 (12 real pending rows; 3 placeholders excluded)", len(all))
	}

	// The statement the store issues must be bounded by SQL. It is rebuilt
	// here from the same constants so the assertion tracks the store's query
	// text.
	ops := sqliteVDBEProgram(t, ctx, q, pendingOrderQuery(1000), StatusPending, 1000)
	assertBoundedRead(t, "pending order", ops, false)
}

// TestPlaceholderSQL_GoOracleOverTableRows is the ingest-side half of the
// sync guarantee: over rows stored in a REAL queue table, the rows SQL keeps
// (and the count SQL reports) must equal the rows the authoritative Go
// predicate keeps. A SQL predicate that drifted from the Go list would make
// the listing serve probe classes — or hide real ones — while both sides
// stayed self-consistent.
func TestPlaceholderSQL_GoOracleOverTableRows(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()

	titles := []string{
		"off-by-one-self-test", "self-test", "tick12-self-test", "Self-Test",
		"test-self-dogfood", "dogfood-field-test-alpha", "docs-canary-001",
		"test", "test-gap-sweep", "test-foreman-tick", "e2e-tick42",
		"foreman-tick82-e2e", "shell-script-e2e",
		"foreman-e2e-verification-pipeline", "tick88-foreman-audit",
		"ds-007", "ds-007-tick-106", "shell-say-hello-test",
		"shell-echo-hello-fix",
		// real classes, including the "test"-substring counter-examples
		"docker-perms", "file-ownership",
		"test-mocking-http-requests", "test-property-based-shrinking",
		"protest-signatures",
	}
	wantKept := 0
	for i, title := range titles {
		if _, err := q.db.ExecContext(ctx, `INSERT INTO queue_entries
			(id, problem_class, status, created_at) VALUES (?, ?, 'pending', ?)`,
			fmt.Sprintf("oracle_%02d", i), title,
			fmt.Sprintf("2026-01-%02d 00:00:00", 1+i%28)); err != nil {
			t.Fatalf("insert %q: %v", title, err)
		}
		if !graph.IsPlaceholderClass(title) {
			wantKept++
		}
	}

	page, total, err := q.ListPage(ctx, StatusPending, 1000, 0)
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if total != wantKept {
		t.Errorf("total = %d, want %d (Go IsPlaceholderClass over the same %d titles)",
			total, wantKept, len(titles))
	}
	if len(page) != wantKept {
		t.Fatalf("page len = %d, want %d", len(page), wantKept)
	}
	for _, e := range page {
		if graph.IsPlaceholderClass(e.ProblemClass) {
			t.Errorf("placeholder %q served — SQL exclusion disagrees with the Go list", e.ProblemClass)
		}
	}
	// Every non-placeholder title must be present: the exclusion must not
	// over-match (a LIKE transcription with an unescaped `_`, or a GLOB
	// class, is the realistic way to lose a real row).
	got := map[string]bool{}
	for _, e := range page {
		got[e.ProblemClass] = true
	}
	for _, title := range titles {
		if !graph.IsPlaceholderClass(title) && !got[title] {
			t.Errorf("real class %q missing from the listing — the SQL exclusion over-matched", title)
		}
	}

	// The pending order is the same filtered set, so the counts agree.
	order, err := q.PendingQueueOrder(ctx, 1000)
	if err != nil {
		t.Fatalf("PendingQueueOrder: %v", err)
	}
	if len(order) != wantKept {
		t.Errorf("pending order len = %d, want %d", len(order), wantKept)
	}
}

// TestQueue_PageQueryPlanIsIndexOrdered is the MEASURED half of the bound
// proof: the indexes added with the pushdown (OB-GAP-084) must be the ones
// the paged reads walk, with no temp B-tree sort. A plan that regressed to
// `USE TEMP B-TREE FOR ORDER BY` would still return the right rows while
// reading (and sorting) the entire status partition per poll.
func TestQueue_PageQueryPlanIsIndexOrdered(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()
	seedQueueRows(t, q, 30, "x", "real-class")

	for _, spec := range []struct {
		name    string
		qry     string
		args    []any
		wantIdx string
	}{
		{
			"status-filtered page",
			func() string { s, _ := listQueuePageQuery(StatusPending, 5, 0); return s }(),
			[]any{StatusPending, 5, 0},
			"idx_queue_entries_status_created",
		},
		{
			"unfiltered page",
			func() string { s, _ := listQueuePageQuery("", 5, 0); return s }(),
			[]any{5, 0},
			"idx_queue_entries_created",
		},
	} {
		plan := sqliteQueryPlan(t, ctx, q, spec.qry, spec.args...)
		if !strings.Contains(plan, spec.wantIdx) {
			t.Errorf("%s: plan does not use %s:\n%s", spec.name, spec.wantIdx, plan)
		}
		if strings.Contains(plan, "USE TEMP B-TREE FOR ORDER BY") {
			t.Errorf("%s: ORDER BY is sorted in a temp B-tree instead of walked in index order:\n%s", spec.name, plan)
		}
	}

	// The solver order gets its own index for the same reason.
	orderPlan := sqliteQueryPlan(t, ctx, q, pendingOrderQuery(1000), StatusPending, 1000)
	if !strings.Contains(orderPlan, "idx_queue_entries_status_priority") {
		t.Errorf("pending order does not use idx_queue_entries_status_priority:\n%s", orderPlan)
	}
	if strings.Contains(orderPlan, "USE TEMP B-TREE FOR ORDER BY") {
		t.Errorf("pending order sorts in a temp B-tree:\n%s", orderPlan)
	}
}

// TestQueue_ReadPathHasNoGoPlaceholderPostFilter covers the other half of
// "no Go-side post-filter on the counted set": the read path must not call
// the Go predicate at all. The VDBE assertions above prove the SQL carries
// the exclusion; this scan proves nothing re-applies it (or counts a
// different set) after the rows come back — the pre-OB-GAP-084 shape.
func TestQueue_ReadPathHasNoGoPlaceholderPostFilter(t *testing.T) {
	src, err := os.ReadFile("queue.go")
	if err != nil {
		t.Fatalf("read queue.go: %v", err)
	}
	if strings.Contains(string(src), "IsPlaceholderClass") {
		t.Errorf("internal/ingest/queue.go still calls the Go placeholder predicate — the filter belongs in SQL (OB-GAP-084)")
	}
	// And the count must not be derived from a fetched slice.
	if strings.Contains(string(src), "total := len(") {
		t.Errorf("internal/ingest/queue.go derives the match count from a Go slice instead of COUNT(*) (OB-GAP-084)")
	}
}

// TestQueue_ListPage_ConcurrentReadsAreConsistent is a light guard that the
// split count+page read did not introduce a shape that falls over under
// concurrent readers (the API serves this path on every poll).
func TestQueue_ListPage_ConcurrentReadsAreConsistent(t *testing.T) {
	q, _ := newTestQueue(t)
	seedQueueRows(t, q, 40, "c", "real-class")

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				page, total, err := q.ListPage(context.Background(), StatusPending, 7, 3)
				if err != nil {
					errs <- err
					return
				}
				if total != 40 || len(page) != 7 {
					errs <- fmt.Errorf("concurrent read: total/page = %d/%d, want 40/7", total, len(page))
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
