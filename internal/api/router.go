package api

import (
	"encoding/json"
	"net/http"

	"github.com/jacobalberty/dashmap/internal/web"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", s.handleLivez)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	mux.HandleFunc("GET /api/cars", s.handleCars)
	mux.HandleFunc("GET /api/calendar", s.handleCalendar)
	mux.HandleFunc("GET /api/clips", s.handleClips)
	mux.HandleFunc("GET /api/clips/{id}", s.handleClipDetail)
	mux.HandleFunc("GET /api/clip-markers", s.handleClipMarkers)
	mux.HandleFunc("GET /api/track", s.handleTrack)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/media/{segment_id}", s.handleMedia)
	mux.HandleFunc("GET /api/download/{segment_id}", s.handleDownloadSegment)
	mux.HandleFunc("GET /api/download/clip/{id}", s.handleDownloadClip)
	mux.HandleFunc("GET /api/thumb/{event_id}", s.handleThumb)
	// JSON 404 for any unmatched /api/* path (so it never falls through to the SPA).
	mux.HandleFunc("GET /api/", s.handleAPINotFound)

	// Basemap: exact path registered unconditionally so it 404s cleanly (not index.html).
	mux.HandleFunc("GET /tiles/basemap.pmtiles", s.handleBasemap)
	mux.HandleFunc("HEAD /tiles/basemap.pmtiles", s.handleBasemap)

	if spaHandler := web.Handler(); spaHandler != nil {
		mux.Handle("/", spaHandler)
	}

	return mux
}

func (s *Server) handleLivez(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Pinger.Ping(r.Context()); err != nil {
		http.Error(w, `{"status":"unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleAPINotFound(w http.ResponseWriter, _ *http.Request) {
	writeErr(w, http.StatusNotFound, codeNotFound, "not found")
}
