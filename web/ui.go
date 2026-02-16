package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Content holds the static web assets.
//
//go:embed all:dist
var Content embed.FS

// NewUIHandler returns an http.Handler that serves the web UI.
// It handles SPA routing by serving index.html for unknown paths.
func NewUIHandler() http.Handler {
	// Root of the embedded FS is 'dist'
	distFS, err := fs.Sub(Content, "dist")
	if err != nil {
		// This might happen if 'dist' folder is missing during build
		// In production, we expect it to be there.
		panic(err)
	}

	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path
		upath := r.URL.Path
		if !strings.HasPrefix(upath, "/") {
			upath = "/" + upath
		}
		upath = path.Clean(upath)

		// Check if the file exists in the embedded FS
		f, err := distFS.Open(strings.TrimPrefix(upath, "/"))
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// If it's a request for a file (has extension) and not found, return 404
		if path.Ext(upath) != "" {
			http.NotFound(w, r)
			return
		}

		// Otherwise, serve index.html for SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
