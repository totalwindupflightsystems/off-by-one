package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// TestHealthReportsVersion pins the /health version contract the SPA
// version chip depends on (REVIEW-OB-021): web/js/app.js reads
// j.version from /health and overwrites the static v0.0.0 fallback in
// web/index.html. Without this field the chip ships a stale version
// forever. handleHealth only touches StartedAt + Version, so a bare
// Server (no store/queue) is enough here.
func TestHealthReportsVersion(t *testing.T) {
	s := &Server{
		Version:   "9.9.9-test",
		StartedAt: time.Now().Add(-90 * time.Second),
	}

	rr := do(t, s, "GET", "/health", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("status field = %v, want ok", body["status"])
	}
	if got, want := body["version"], "9.9.9-test"; got != want {
		t.Errorf("version field = %v, want %q", got, want)
	}
	uptime, ok := body["uptime"].(string)
	if !ok || uptime == "" {
		t.Errorf("uptime field = %v, want a non-empty duration string", body["uptime"])
	}
}
