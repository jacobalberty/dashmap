package api

import (
	"net/http"
	"os"
)

func (s *Server) handleBasemap(w http.ResponseWriter, r *http.Request) {
	path := s.deps.Cfg.BasemapPMTiles
	if path == "" {
		writeErr(w, http.StatusNotFound, codeNotFound, "basemap not configured")
		return
	}
	f, err := os.Open(path) //nolint:gosec // G304: path comes from operator config (BASEMAP_PMTILES)
	if err != nil {
		writeErr(w, http.StatusNotFound, codeNotFound, "basemap not found")
		return
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, "basemap unavailable")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, "basemap.pmtiles", fi.ModTime(), f)
}
