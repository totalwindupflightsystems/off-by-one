package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLoopbackOnlyBind pins the classifier behind the documented security
// posture (REVIEW-OB-023): loopback spellings must classify as loopback,
// and everything an operator could set that widens exposure — the empty
// string (all interfaces), wildcards, routable addresses, unresolvable
// hostnames — must classify as NOT loopback so main warns at startup.
func TestLoopbackOnlyBind(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		// Loopback: the shipped default and its equivalents.
		{"127.0.0.1", true},
		{"127.0.0.2", true}, // the whole 127.0.0.0/8 block is loopback
		{"127.255.255.254", true},
		{"::1", true},
		{"localhost", true},
		{"LOCALHOST", true},
		{"Localhost", true},
		{"  127.0.0.1\t", true}, // whitespace-padded env values still classify

		// Exposed: must warn.
		{"", false},        // empty host binds all interfaces (":port")
		{"0.0.0.0", false}, // IPv4 wildcard
		{"::", false},      // IPv6 wildcard
		{"192.168.1.10", false},
		{"203.0.113.7", false},
		{"example.com", false},           // hostname: not provably loopback
		{"localhost.localdomain", false}, // fail-closed: only exact "localhost"
		{"127.0.0.1:8766", false},        // host:port is not a bare host
	}
	for _, tc := range cases {
		if got := LoopbackOnlyBind(tc.host); got != tc.want {
			t.Errorf("LoopbackOnlyBind(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

// TestNoAuthMiddlewareOnWriteRoutes pins the honesty half of the REVIEW-OB-023
// posture: while the documented stance is "loopback-only, no HTTP
// authentication", the mux must NOT silently grow an auth boundary that the
// docs deny — and, conversely, must not pretend to have one. A write request
// carrying no Authorization header, Bearer token, or X-API-Key reaches the
// handler and fails on configuration (501 export-not-configured), not on
// authentication (401/403). If token auth is ever implemented, this test
// must be deleted together with the README/docs posture text in the same
// change.
func TestNoAuthMiddlewareOnWriteRoutes(t *testing.T) {
	s := &Server{} // no ExportLocalDir: handleExport fails fast with 501
	req := httptest.NewRequest(http.MethodPost, "/api/v1/export",
		strings.NewReader(`{"target_repo":"x","answer_ids":["a"]}`))
	// Deliberately set no credential headers of any kind.
	rec := httptest.NewRecorder()

	s.Handler().ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Fatalf("unauthenticated write rejected with %d — auth middleware exists but README/docs deny it; update the security posture docs in the same change", rec.Code)
	}
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("unconfigured export: got %d, want 501 (request must reach the handler, proving no auth boundary intercepts it)", rec.Code)
	}
}
