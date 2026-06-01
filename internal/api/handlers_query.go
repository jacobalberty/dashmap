package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jacobalberty/dashmap/internal/query"
)

// Querier is the read-side data interface the handlers depend on. *query.Store
// implements it; tests use a stub.
type Querier interface {
	CarIDByKey(ctx context.Context, key string) (int64, error)
	ListCars(ctx context.Context) ([]query.Car, error)
	Calendar(ctx context.Context, carID int64, from, to time.Time, tz string) ([]query.CalendarDay, error)
	ListClips(ctx context.Context, carID int64, from, to time.Time, kinds []string, limit, offset int) ([]query.ClipListItem, bool, error)
	ClipMarkers(ctx context.Context, carID int64, from, to time.Time, max int) ([]query.ClipMarker, bool, error)
	Track(ctx context.Context, carID int64, from, to time.Time) ([]query.TrackPoint, error)
	ClipDetail(ctx context.Context, clipID int64) (*query.ClipDetail, error)
	ClipTrack(ctx context.Context, clipID int64) ([]query.ClipTrackPoint, error)
	SegmentRelPath(ctx context.Context, segmentID int64) (string, error)
	ClipZip(ctx context.Context, clipID int64) (*query.ClipZipInfo, error)
	EventThumbRelPath(ctx context.Context, eventID int64) (string, error)
	Status(ctx context.Context, carID int64, key string, liveWindow time.Duration) (*query.Status, error)
}

var validKinds = map[string]bool{"recent": true, "saved": true, "sentry": true}

// resolveCar resolves the ?car= key (defaulting to the configured car) to a
// car id. On failure it writes the response and returns ok=false.
func (s *Server) resolveCar(w http.ResponseWriter, r *http.Request) (carID int64, key string, ok bool) {
	key = r.URL.Query().Get("car")
	if key == "" {
		key = s.deps.Cfg.CarKey
	}
	id, err := s.deps.Querier.CarIDByKey(r.Context(), key)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, codeCarNotFound, "unknown car "+strconv.Quote(key))
		} else {
			writeErr(w, http.StatusInternalServerError, codeInternal, "database error")
		}
		return 0, key, false
	}
	return id, key, true
}

func (s *Server) handleCars(w http.ResponseWriter, r *http.Request) {
	cars, err := s.deps.Querier.ListCars(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	writeJSON(w, http.StatusOK, cars, nil)
}

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	carID, _, ok := s.resolveCar(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	var from, to time.Time
	if q.Get("from") == "" && q.Get("to") == "" {
		from, to = currentMonthUTC(time.Now())
	} else {
		var err error
		from, to, err = parseRange(r)
		if err != nil {
			writeErr(w, http.StatusBadRequest, codeBadRequest, err.Error())
			return
		}
	}
	tz := q.Get("tz")
	if tz == "" {
		tz = "UTC"
	} else if _, err := time.LoadLocation(tz); err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, "invalid tz: "+tz)
		return
	}
	days, err := s.deps.Querier.Calendar(r.Context(), carID, from, to, tz)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	writeJSON(w, http.StatusOK, days, nil)
}

func (s *Server) handleClips(w http.ResponseWriter, r *http.Request) {
	carID, _, ok := s.resolveCar(w, r)
	if !ok {
		return
	}
	from, to, err := parseRange(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	q := r.URL.Query()

	var kinds []string
	if k := q.Get("kind"); k != "" {
		for _, part := range strings.Split(k, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if !validKinds[part] {
				writeErr(w, http.StatusBadRequest, codeBadRequest, "invalid kind "+strconv.Quote(part))
				return
			}
			kinds = append(kinds, part)
		}
	}

	limit := s.deps.Cfg.ClipsDefaultLimit
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeErr(w, http.StatusBadRequest, codeBadRequest, "invalid limit")
			return
		}
		limit = n
	}
	if limit > s.deps.Cfg.ClipsMaxLimit {
		limit = s.deps.Cfg.ClipsMaxLimit
	}
	offset := 0
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeErr(w, http.StatusBadRequest, codeBadRequest, "invalid offset")
			return
		}
		offset = n
	}

	clips, hasMore, err := s.deps.Querier.ListClips(r.Context(), carID, from, to, kinds, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	meta := map[string]any{"limit": limit, "offset": offset, "has_more": hasMore}
	writeJSON(w, http.StatusOK, clips, meta)
}

func (s *Server) handleClipMarkers(w http.ResponseWriter, r *http.Request) {
	carID, _, ok := s.resolveCar(w, r)
	if !ok {
		return
	}
	from, to, err := parseRange(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	markers, capped, err := s.deps.Querier.ClipMarkers(r.Context(), carID, from, to, s.deps.Cfg.ClipMarkersMax)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	writeJSON(w, http.StatusOK, markers, map[string]any{"capped": capped})
}

func (s *Server) handleTrack(w http.ResponseWriter, r *http.Request) {
	carID, _, ok := s.resolveCar(w, r)
	if !ok {
		return
	}
	from, to, err := parseRange(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	pts, err := s.deps.Querier.Track(r.Context(), carID, from, to)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	runs, returned := buildTrackRuns(pts, s.deps.Cfg.TrackGap, s.deps.Cfg.TrackSimplifyM, s.deps.Cfg.TrackMaxPoints)
	meta := map[string]any{
		"simplified": returned < len(pts),
		"runs":       len(runs),
		"returned":   returned,
		"total":      len(pts),
	}
	writeJSON(w, http.StatusOK, runs, meta)
}

func (s *Server) handleClipDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	detail, err := s.deps.Querier.ClipDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeErr(w, http.StatusNotFound, codeNotFound, "clip not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	writeJSON(w, http.StatusOK, detail, nil)
}

func (s *Server) handleClipTrack(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	pts, err := s.deps.Querier.ClipTrack(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	writeJSON(w, http.StatusOK, pts, nil)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	carID, key, ok := s.resolveCar(w, r)
	if !ok {
		return
	}
	st, err := s.deps.Querier.Status(r.Context(), carID, key, 2*s.deps.Cfg.ScanInterval)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, codeInternal, "database error")
		return
	}
	writeJSON(w, http.StatusOK, st, nil)
}
