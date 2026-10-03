//go:build embed_ui

package ui

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
)

//go:embed dist/*
var distFS embed.FS

// Handler serves the embedded statically compiled React SPA.
// This file is only compiled when the 'embed_ui' build tag IS provided.
func Handler() http.Handler {
	log.Println("UI: Running in PROD mode, serving embedded static files")

	subFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		log.Fatalf("failed to create sub filesystem for UI: %v", err)
	}

	fileServer := http.FileServer(http.FS(subFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if the requested file exists
		f, err := subFS.Open(strings.TrimPrefix(path, "/"))
		if os.IsNotExist(err) {
			// SPA fallback: route doesn't match a static asset, serve index.html
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		} else if err == nil {
			f.Close()
		}

		fileServer.ServeHTTP(w, r)
	})
}
