package main

import (
	"io/fs"
	"log/slog"
	"net/http"

	autopsyui "github.com/d9705996/autopsy/ui"
)

// registerSPA mounts the embedded React SPA.
// All non-API, non-metrics GET requests are served from ui/dist.
// Unknown routes fall back to index.html to support client-side routing.
func registerSPA(mux *http.ServeMux, log *slog.Logger) {
	sub, err := fs.Sub(autopsyui.FS, "dist")
	if err != nil {
		log.Error("embed ui/dist: sub failed", "err", err)
		return
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.Handle("GET /", spaHandler{fs: fileServer})
}

// spaHandler serves static files and falls back to index.html for unknown paths
// (enabling client-side routing in the SPA).
type spaHandler struct {
	fs http.Handler
}

func (s spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.fs.ServeHTTP(w, r)
}
