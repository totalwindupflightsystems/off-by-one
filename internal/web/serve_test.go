package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandler_RootServesIndex asserts the SPA shell is served at
// GET /. The response must include the brand title and the
// essential script tag.
func TestHandler_RootServesIndex(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type: got %q, want text/html…", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	for _, needle := range []string{"Off-by-One", "/js/app.js", "/css/style.css"} {
		if !strings.Contains(string(body), needle) {
			t.Errorf("index body missing %q", needle)
		}
	}
}

// TestHandler_CSSAsset asserts /css/style.css is served with the
// correct Content-Type and is non-empty.
func TestHandler_CSSAsset(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/css/style.css")
	if err != nil {
		t.Fatalf("GET /css/style.css: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/css") {
		t.Errorf("content-type: got %q, want text/css…", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Errorf("css body is empty")
	}
}

// TestHandler_JSAsset asserts /js/app.js is served with a JS
// Content-Type. The file should at least contain the IIFE wrapper
// used in the shell.
func TestHandler_JSAsset(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/js/app.js")
	if err != nil {
		t.Fatalf("GET /js/app.js: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/javascript") && !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("content-type: got %q, want javascript…", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Off-by-One") {
		t.Errorf("app.js body does not look like the shell bootstrap")
	}
}

// TestHandler_SearchJSAsset asserts /js/search.js (WI-009) is
// served with a JS Content-Type and contains the module entry point.
func TestHandler_SearchJSAsset(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/js/search.js")
	if err != nil {
		t.Fatalf("GET /js/search.js: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/javascript") && !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("content-type: got %q, want javascript…", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "initView_search") {
		t.Errorf("search.js body missing initView_search registration")
	}
}

// TestHandler_SPAFallback asserts a path with no extension and no
// /api/ prefix is treated as a SPA client route and returns
// index.html. This is what allows the client-side router to handle
// arbitrary URLs.
func TestHandler_SPAFallback(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	for _, p := range []string{"/search", "/submit/foo", "/explore/class-1"} {
		t.Run(strings.TrimPrefix(p, "/"), func(t *testing.T) {
			resp, err := http.Get(srv.URL + p)
			if err != nil {
				t.Fatalf("GET %s: %v", p, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status: got %d, want 200", resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), "<!DOCTYPE html>") {
				t.Errorf("spa fallback did not return index.html")
			}
		})
	}
}

// TestHandler_APIPathNotHandled asserts /api/* paths are NOT served
// by the web handler — those belong to the API mux. The web handler
// returns 404 (since there's no /api/ asset in the embedded FS).
func TestHandler_APIPathNotHandled(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/stats")
	if err != nil {
		t.Fatalf("GET /api/v1/stats: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

// TestHandler_UnknownAssetReturns404 asserts a path that LOOKS like
// a static asset (has an extension) but doesn't exist in the embed
// returns 404, not the SPA shell.
func TestHandler_UnknownAssetReturns404(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/js/nonexistent.js")
	if err != nil {
		t.Fatalf("GET /js/nonexistent.js: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}

// TestHandler_HeadOnRoot asserts HEAD requests are accepted (the
// shell will return 200 with the headers and an empty body —
// http.ServeContent semantics).
func TestHandler_HeadOnRoot(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Head(srv.URL + "/")
	if err != nil {
		t.Fatalf("HEAD /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		t.Errorf("HEAD body should be empty, got %d bytes", len(body))
	}
}

// TestHandler_PostReturns404 asserts non-GET/HEAD methods fall
// through to 404 (the shell is read-only; the API has its own
// router for POST/PUT/DELETE).
func TestHandler_PostReturns404(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", resp.StatusCode)
	}
}
