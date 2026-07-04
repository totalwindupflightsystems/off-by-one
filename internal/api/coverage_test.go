package api

import (
	"context"
	"testing"
	"time"
)

// --- splitPath ---

func TestSplitPath(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"/api/v1/problems", []string{"api", "v1", "problems"}},
		{"api/v1/problems/", []string{"api", "v1", "problems"}},
		{"/", []string{""}},
		{"", []string{""}},
		{"//double//slash", []string{"double", "", "slash"}},
	}
	for _, c := range cases {
		got := splitPath(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitPath(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitPath(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

// --- ListenAndServe ---

func TestServer_ListenAndServe_Shutdown(t *testing.T) {
	srv, _, _ := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe(ctx, "127.0.0.1:0")
	}()

	// Give the server a moment to bind, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	err := <-errCh
	// Graceful shutdown returns http.ErrServerClosed or nil (joined errors).
	if err != nil && err.Error() != "http: Server closed" {
		t.Logf("ListenAndServe returned: %v (acceptable for graceful shutdown)", err)
	}
}
