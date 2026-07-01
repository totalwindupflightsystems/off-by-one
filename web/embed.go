// Package web provides the embedded web UI assets (HTML, CSS, JS).
//
// The actual HTTP handler lives in internal/web/serve.go. This file
// exists solely to declare the go:embed directives — Go's embed
// package can only reference files at-or-below the directory of the
// embedding source file, so the embed must live in a package
// co-located with the assets.
//
// Exposed symbols are read-only — the embed FS is not meant to be
// mutated at runtime.
package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html
var indexHTML []byte

//go:embed css js
var assetsFS embed.FS

// IndexHTML returns the raw bytes of the SPA index.html. Served at
// the application root ("/") and also at any non-API path that
// doesn't match a static asset (SPA history fallback).
func IndexHTML() []byte { return indexHTML }

// FS returns a sub-FS rooted at the static assets directory (css/, js/).
// Used by the serve handler to serve /css/style.css and /js/app.js.
//
// The root of the returned FS contains only "css" and "js" directories
// — index.html is served separately because it's also the SPA
// fallback document.
func FS() fs.FS {
	sub, err := fs.Sub(assetsFS, ".")
	if err != nil {
		// fs.Sub on a valid embed.FS only fails on programmer error.
		// Returning nil here would panic later; we surface the
		// problem at startup via the constructor in internal/web.
		panic("web: fs.Sub failed: " + err.Error())
	}
	return sub
}
