package api

import (
	"archive/zip"
	"errors"
	"log/slog"
	"net/http"
	"path"

	"github.com/jackc/pgx/v5"

	"github.com/jacobalberty/dashmap/internal/media"
)

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("segment_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	rel, err := s.deps.Querier.SegmentRelPath(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, codeNotFound, "segment not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	if err := s.deps.Resolver.Serve(w, r, rel, "video/mp4", ""); err != nil {
		slog.Warn("api: media serve failed", "rel", rel, "err", err)
		writeErr(w, http.StatusNotFound, codeNotFound, "file not found")
	}
}

func (s *Server) handleDownloadSegment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("segment_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	rel, err := s.deps.Querier.SegmentRelPath(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, codeNotFound, "segment not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	if err := s.deps.Resolver.Serve(w, r, rel, "video/mp4", path.Base(rel)); err != nil {
		slog.Warn("api: download serve failed", "rel", rel, "err", err)
		writeErr(w, http.StatusNotFound, codeNotFound, "file not found")
	}
}

func (s *Server) handleDownloadClip(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	info, err := s.deps.Querier.ClipZip(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, codeNotFound, "clip not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, codeInternal, "database error")
		return
	}

	archive := info.ClipTS.UTC().Format("2006-01-02_15-04-05") + ".zip"
	entries := make([]media.ZipEntry, 0, len(info.RelPaths))
	for _, rel := range info.RelPaths {
		entries = append(entries, media.ZipEntry{RelPath: rel, Name: path.Base(rel)})
	}
	method := uint16(zip.Store)
	if s.deps.Cfg.ZipCompression == "deflate" {
		method = zip.Deflate
	}

	started, err := s.deps.Resolver.ZipClip(w, archive, entries, method)
	if err != nil {
		if !started {
			writeErr(w, http.StatusNotFound, codeNotFound, "file not found")
			return
		}
		slog.Warn("api: clip zip stream failed", "clip_id", id, "err", err)
	}
}

func (s *Server) handleThumb(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("event_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	rel, err := s.deps.Querier.EventThumbRelPath(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, codeNotFound, "thumbnail not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	if err := s.deps.Resolver.Serve(w, r, rel, "image/png", ""); err != nil {
		slog.Warn("api: thumb serve failed", "rel", rel, "err", err)
		writeErr(w, http.StatusNotFound, codeNotFound, "file not found")
	}
}
