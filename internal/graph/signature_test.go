package graph

import "testing"

// TestSignatureFailed pins the Go mirror of signatureNotFailedSQL. The
// predicate is deliberately narrow: ONLY the exact "failed" sentinel is a
// failure. Empty, truncated, malformed, non-object and non-string-result
// blobs all report false so that status stays the primary signal — a
// legacy signature map without a result field must not start reading as a
// failed solve (same semantics as solver.answerStatusFromSignatures).
func TestSignatureFailed(t *testing.T) {
	cases := []struct {
		name string
		sigs string
		want bool
	}{
		{"empty string", "", false},
		{"empty object", "{}", false},
		{"absent result", `{"model":"m1"}`, false},
		{"result ok", `{"result":"ok"}`, false},
		{"result passed", `{"result":"passed"}`, false},
		{"result completed", `{"result":"completed"}`, false},
		{"result failed", `{"result":"failed"}`, true},
		{"failed with sibling fields", `{"result":"failed","model":"m2","tests":0}`, true},
		{"mixed case is not the sentinel", `{"result":"Failed"}`, false},
		{"non-string result", `{"result":0}`, false},
		{"null result", `{"result":null}`, false},
		{"nested result object", `{"result":{"state":"failed"}}`, false},
		{"truncated json", `{"result":"fail`, false},
		{"invalid json", `{`, false},
		{"bare json string", `"failed"`, false},
		{"json array", `["failed"]`, false},
		{"whitespace only", "   ", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := signatureFailed(tc.sigs); got != tc.want {
				t.Errorf("signatureFailed(%q) = %v, want %v", tc.sigs, got, tc.want)
			}
		})
	}
}
