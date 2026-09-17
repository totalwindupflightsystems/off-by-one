package graph

import "encoding/json"

// signatureNotFailedSQL is the SQL backstop predicate for the
// status-vs-signature divergence (OB-GAP-057 / OB-GAP-064). The status
// column is the primary signal, but an older binary could still have
// written a row whose status says 'verified' (or 'ci_passed') while its
// signatures JSON says {"result":"failed"} — the live graph carried 28
// such rows before OB-GAP-057 reclassified them, and any pre-fix DB can
// still carry them.
//
// The literal lives here ONCE and is interpolated into every query that
// needs it (bestAnswer, Stats, the derived best_status aggregates in
// ListProblemClassesWithCountsFiltered / CountProblemClasses /
// GetProblemClassStatus, and Search), so the SQL text cannot drift
// between call sites. The Go mirror is signatureFailed below — keep the
// two in sync. Sibling: Queue.hasVerifiedAnswer in internal/ingest.
const signatureNotFailedSQL = "COALESCE(json_extract(signatures, '$.result'), '') != 'failed'"

// signatureFailed reports whether a signatures JSON blob carries the
// result='failed' sentinel, i.e. whether the row is a failed solve that
// happens to be wearing a verified status. It is the Go mirror of
// signatureNotFailedSQL and is deliberately permissive: an empty,
// truncated, malformed or non-object blob, and a missing, null or
// non-string result, all report false — status stays the primary signal
// and only an explicit "failed" verdict is treated as one (same
// semantics as solver.answerStatusFromSignatures).
func signatureFailed(signatures string) bool {
	if signatures == "" {
		return false
	}
	var sig struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(signatures), &sig); err != nil {
		return false
	}
	return sig.Result == "failed"
}
