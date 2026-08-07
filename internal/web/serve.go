// Package web serves the embedded SPA shell at the application root.
//
// Routes registered when Handler() is composed into a mux:
//
//	GET  /              -> index.html (the SPA shell)
//	GET  /css/*         -> static CSS (from the embedded FS)
//	GET  /js/*          -> static JS  (from the embedded FS)
//	ANY  /*             -> SPA history fallback (any non-API path
//	                       that doesn't match a static asset returns
//	                       index.html so the client router can take over)
//
// The /api/v1/* and /openapi.json and /health paths are owned by
// the api package and are not intercepted here.
//
// All assets come from the embedded web.FS — there is no runtime
// disk access. The binary is self-contained.
package web

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	offbyweb "github.com/totalwindupflightsystems/off-by-one/web"
)

// Handler returns an http.Handler that serves the SPA shell and
// static assets. Compose into a parent mux with the API routes;
// this handler will defer to the parent's not-found behaviour for
// any path it does not recognise.
//
// The SPA fallback is implemented by checking whether the request
// path resolves to a file in the embedded FS; if it does, serve it
// with the correct Content-Type; otherwise, if the path looks like
// an SPA client route (no extension, no /api/ prefix), return
// index.html with status 200. Real 404s — paths with extensions
// that don't exist — return a plain 404.
func Handler() http.Handler {
	subFS := offbyweb.FS()
	fileServer := http.FileServer(http.FS(subFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only handle GET/HEAD for the shell and assets.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		upath := r.URL.Path
		if upath == "" || upath == "/" {
			serveIndex(w)
			return
		}

		// Strip leading slash, look the file up in the embedded FS.
		// http.FS uses slash-separated paths rooted at the sub-FS.
		clean := strings.TrimPrefix(upath, "/")
		if clean == "" {
			serveIndex(w)
			return
		}

		// Probe whether the asset exists. We do this manually rather
		// than delegating to fileServer so we can distinguish "asset
		// exists" (serve with content-type) from "asset missing, but
		// path is a client route" (serve index.html as fallback)
		// from "asset missing AND path looks like a real file"
		// (return 404).
		if _, err := fs.Stat(subFS, clean); err == nil {
			setContentType(w, clean)
			// No heuristic caching for assets: the SPA shell is served with
			// no-cache too, so a deployed binary's new JS/CSS always reaches
			// browsers instead of stale cache copies.
			w.Header().Set("Cache-Control", "no-cache")
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: a path without an extension is treated as a
		// client route. The client-side router will take over.
		if !hasExt(clean) && !strings.HasPrefix(clean, "api/") {
			serveIndex(w)
			return
		}

		http.NotFound(w, r)
	})
}

// serveIndex writes the embedded index.html with a far-future cache
// header. The shell itself is small and versioned; the per-asset
// URLs use content-hashing once we add a build step.
func serveIndex(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(offbyweb.IndexHTML())
}

// setContentType picks a Content-Type from the file extension. We
// keep this small and explicit; if a future build emits assets with
// exotic extensions (e.g. .woff2) the type can be added here.
func setContentType(w http.ResponseWriter, name string) {
	ext := path.Ext(name)
	if ext == "" {
		return
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
}

func hasExt(p string) bool {
	return strings.Contains(path.Base(p), ".")
}
