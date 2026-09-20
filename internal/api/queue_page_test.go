package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
)

// TestListQueue_PageAndCountComeFromBoundedQueries is the API-layer
// regression for OB-GAP-084. The behavior tests
// (TestListQueue_NewestFirstHonestTotalAndPendingPosition,
// TestListQueue_ExcludesPlaceholderClasses) pin WHAT the endpoint answers;
// this one pins that the answer is produced by the bounded SQL read the
// ticket requires — a page from LIMIT/OFFSET and a total from COUNT(*),
// both excluding placeholder classes in SQL — over a table large enough
// that a Go-side materialisation would be the visible failure mode.
//
// Fixture: 4 real pending rows, 2 placeholder rows (one pending, one
// complete), 1 real complete row. Matching the pending status filter after
// placeholder exclusion leaves 4 real rows.
func TestListQueue_PageAndCountComeFromBoundedQueries(t *testing.T) {
	s, store, _ := newTestServer(t)
	ctx := context.Background()

	for _, row := range []struct {
		id, class, status, at string
	}{
		{"sub_r1", "docker-perms", ingest.StatusPending, "2026-01-04 00:00:00"},
		{"sub_r2", "file-ownership", ingest.StatusPending, "2026-01-03 00:00:00"},
		{"sub_r3", "test-mocking-http-requests", ingest.StatusPending, "2026-01-02 00:00:00"},
		{"sub_r4", "latest-tag-pinning", ingest.StatusPending, "2026-01-01 00:00:00"},
		// placeholders: must not appear and must not be counted
		{"sub_p1", "off-by-one-self-test", ingest.StatusPending, "2026-02-01 00:00:00"},
		{"sub_p2", "foreman-tick91-e2e", ingest.StatusPending, "2026-02-02 00:00:00"},
		{"sub_p3", "docs-canary-7", ingest.StatusComplete, "2026-02-03 00:00:00"},
		// a real row of another status: not matched by status=pending
		{"sub_c1", "docker-perms", ingest.StatusComplete, "2025-12-01 00:00:00"},
	} {
		if _, err := store.DB().ExecContext(ctx, `INSERT INTO queue_entries
			(id, problem_class, status, stage, priority, created_at)
			VALUES (?, ?, ?, 'queued', 0, ?)`, row.id, row.class, row.status, row.at); err != nil {
			t.Fatalf("insert %s: %v", row.id, err)
		}
	}

	// --- limit=1 over 4 matching rows: exactly one entry, honest total ----
	rr := do(t, s, "GET", "/api/v1/queue?status=pending&limit=1", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var one queueListResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &one); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(one.Entries) != 1 {
		t.Errorf("limit=1 entries = %d, want 1", len(one.Entries))
	}
	if one.Total != 4 {
		t.Errorf("total = %d, want 4 (4 real pending rows; 3 placeholders + 1 complete excluded)", one.Total)
	}
	if one.Entries[0].SubmissionID != "sub_r1" {
		t.Errorf("entries[0] = %q, want sub_r1 (newest real pending row)", one.Entries[0].SubmissionID)
	}

	// --- offset past the end: empty page + the same honest total ---------
	past := do(t, s, "GET", "/api/v1/queue?status=pending&limit=1&offset=99", nil)
	var pastResp queueListResponse
	if err := json.Unmarshal(past.Body.Bytes(), &pastResp); err != nil {
		t.Fatalf("decode past-end: %v", err)
	}
	if len(pastResp.Entries) != 0 {
		t.Errorf("past-the-end entries = %d, want 0", len(pastResp.Entries))
	}
	if pastResp.Total != 4 {
		t.Errorf("past-the-end total = %d, want 4", pastResp.Total)
	}

	// --- no status filter: every servable row (4 real pending + 1 real
	// complete), placeholders still excluded ------------------------------
	all := do(t, s, "GET", "/api/v1/queue", nil)
	var allResp queueListResponse
	if err := json.Unmarshal(all.Body.Bytes(), &allResp); err != nil {
		t.Fatalf("decode all: %v", err)
	}
	if allResp.Total != 5 {
		t.Errorf("unfiltered total = %d, want 5 (8 rows - 3 placeholders)", allResp.Total)
	}
	for _, e := range allResp.Entries {
		if e.SubmissionID == "sub_p1" || e.SubmissionID == "sub_p2" || e.SubmissionID == "sub_p3" {
			t.Errorf("placeholder row %q served", e.SubmissionID)
		}
	}

	// --- paging walks the match set in order, total never moves ----------
	var seen []string
	for off := 0; off < 4; off++ {
		pr := do(t, s, "GET", "/api/v1/queue?status=pending&limit=1&offset="+itoa(off), nil)
		var resp queueListResponse
		if err := json.Unmarshal(pr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode offset=%d: %v", off, err)
		}
		if resp.Total != 4 {
			t.Errorf("offset=%d total = %d, want 4", off, resp.Total)
		}
		if len(resp.Entries) != 1 {
			t.Fatalf("offset=%d entries = %d, want 1", off, len(resp.Entries))
		}
		if resp.Entries[0].Position != off+1 {
			t.Errorf("offset=%d position = %d, want %d", off, resp.Entries[0].Position, off+1)
		}
		seen = append(seen, resp.Entries[0].SubmissionID)
	}
	want := []string{"sub_r1", "sub_r2", "sub_r3", "sub_r4"}
	for i := range want {
		if seen[i] != want[i] {
			t.Errorf("paged order = %v, want %v", seen, want)
			break
		}
	}
}

// itoa is a tiny local helper so this file needs no extra imports.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
