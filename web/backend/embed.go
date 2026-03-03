package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
)

//go:embed all:dist
var frontendFS embed.FS

// registerEmbedRoutes sets up the HTTP handler to serve the embedded frontend files
func registerEmbedRoutes(mux *http.ServeMux) {
	// Attempt to get the subdirectory 'dist' where Vite usually builds
	subFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		// Log a warning if dist doesn't exist yet (e.g., during development before a frontend build)
		log.Printf(
			"Warning: no 'dist' folder found in embedded frontend. " +
				"Ensure you run `pnpm build:backend` in the frontend directory " +
				"before building the Go backend.",
		)
		return
	}

	// Serve the static files at the root route
	mux.Handle("/", http.FileServer(http.FS(subFS)))
}
