package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jacobalberty/dashmap/internal/config"
	"github.com/jacobalberty/dashmap/internal/media"
	"github.com/jacobalberty/dashmap/internal/query"
)

// stubQuerier is a controllable Querier for handler tests.
type stubQuerier struct {
	carID     int64
	carErr    error
	cars      []query.Car
	clips     []query.ClipListItem
	clipsMore bool
	markers   []query.ClipMarker
	track     []query.TrackPoint
	detail    *query.ClipDetail
	detailErr error
	segRel    string
	segErr    error
	thumbRel  string
	thumbErr  error
	zip       *query.ClipZipInfo
	zipErr    error
	status    *query.Status
}

func (s *stubQuerier) CarIDByKey(context.Context, string) (int64, error) { return s.carID, s.carErr }
func (s *stubQuerier) ListCars(context.Context) ([]query.Car, error)     { return s.cars, nil }
func (s *stubQuerier) Calendar(context.Context, int64, time.Time, time.Time, string) ([]query.CalendarDay, error) {
	return []query.CalendarDay{}, nil
}
func (s *stubQuerier) ListClips(context.Context, int64, time.Time, time.Time, []string, int, int) ([]query.ClipListItem, bool, error) {
	return s.clips, s.clipsMore, nil
}
func (s *stubQuerier) ClipMarkers(context.Context, int64, time.Time, time.Time, int) ([]query.ClipMarker, bool, error) {
	return s.markers, false, nil
}
func (s *stubQuerier) Track(context.Context, int64, time.Time, time.Time) ([]query.TrackPoint, error) {
	return s.track, nil
}
func (s *stubQuerier) ClipDetail(context.Context, int64) (*query.ClipDetail, error) {
	return s.detail, s.detailErr
}
func (s *stubQuerier) SegmentRelPath(context.Context, int64) (string, error) {
	return s.segRel, s.segErr
}
func (s *stubQuerier) ClipZip(context.Context, int64) (*query.ClipZipInfo, error) {
	return s.zip, s.zipErr
}
func (s *stubQuerier) EventThumbRelPath(context.Context, int64) (string, error) {
	return s.thumbRel, s.thumbErr
}
func (s *stubQuerier) Status(context.Context, int64, string, time.Duration) (*query.Status, error) {
	return s.status, nil
}

func testServer(t *testing.T, q Querier, res *media.Resolver) *Server {
	t.Helper()
	return NewServer(":0", Deps{
		Querier:  q,
		Resolver: res,
		Pinger:   &stubPinger{},
		Cfg: &config.Config{
			CarKey:            "default",
			ScanInterval:      5 * time.Minute,
			ZipCompression:    "store",
			TrackGap:          2 * time.Minute,
			TrackSimplifyM:    5,
			TrackMaxPoints:    10000,
			ClipMarkersMax:    10000,
			ClipsDefaultLimit: 500,
			ClipsMaxLimit:     2000,
		},
	})
}

func do(srv *Server, method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)
	return rec
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) envelope {
	t.Helper()
	var e envelope
	if err := json.NewDecoder(rec.Body).Decode(&e); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	return e
}

func TestHandleCars(t *testing.T) {
	q := &stubQuerier{cars: []query.Car{{ID: 1, Key: "default"}}}
	rec := do(testServer(t, q, nil), http.MethodGet, "/api/cars")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	e := decodeEnvelope(t, rec)
	if e.Error != nil {
		t.Fatalf("error = %+v, want nil", e.Error)
	}
}

func TestHandleClipsMissingRange(t *testing.T) {
	rec := do(testServer(t, &stubQuerier{}, nil), http.MethodGet, "/api/clips")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	e := decodeEnvelope(t, rec)
	if e.Error == nil || e.Error.Code != codeBadRequest {
		t.Fatalf("error = %+v, want bad_request", e.Error)
	}
}

func TestHandleClipsPaginationMeta(t *testing.T) {
	q := &stubQuerier{clips: []query.ClipListItem{{ID: 1}}, clipsMore: true}
	rec := do(testServer(t, q, nil), http.MethodGet, "/api/clips?from=2026-05-01&to=2026-05-31")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	e := decodeEnvelope(t, rec)
	meta, ok := e.Meta.(map[string]any)
	if !ok {
		t.Fatalf("meta type = %T, want object", e.Meta)
	}
	if meta["has_more"] != true {
		t.Errorf("has_more = %v, want true", meta["has_more"])
	}
}

func TestHandleClipsInvalidKind(t *testing.T) {
	rec := do(testServer(t, &stubQuerier{}, nil), http.MethodGet, "/api/clips?from=2026-05-01&to=2026-05-31&kind=bogus")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleUnknownCar(t *testing.T) {
	q := &stubQuerier{carErr: pgx.ErrNoRows}
	rec := do(testServer(t, q, nil), http.MethodGet, "/api/clips?from=2026-05-01&to=2026-05-31&car=nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	e := decodeEnvelope(t, rec)
	if e.Error == nil || e.Error.Code != codeCarNotFound {
		t.Fatalf("error = %+v, want car_not_found", e.Error)
	}
}

func TestHandleClipDetailNotFound(t *testing.T) {
	q := &stubQuerier{detailErr: pgx.ErrNoRows}
	rec := do(testServer(t, q, nil), http.MethodGet, "/api/clips/5")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleClipDetailBadID(t *testing.T) {
	rec := do(testServer(t, &stubQuerier{}, nil), http.MethodGet, "/api/clips/abc")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleAPINotFound(t *testing.T) {
	rec := do(testServer(t, &stubQuerier{}, nil), http.MethodGet, "/api/nonexistent")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	e := decodeEnvelope(t, rec)
	if e.Error == nil || e.Error.Code != codeNotFound {
		t.Fatalf("error = %+v, want not_found", e.Error)
	}
}

func TestHandleClipsMethodNotAllowed(t *testing.T) {
	rec := do(testServer(t, &stubQuerier{}, nil), http.MethodPost, "/api/clips")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// --- media handlers (real resolver over a temp dir) ---

func tempResolver(t *testing.T) (*media.Resolver, string) {
	t.Helper()
	dir := t.TempDir()
	clipDir := filepath.Join(dir, "SavedClips", "2026-05-30_12-00-00")
	if err := os.MkdirAll(clipDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clipDir, "2026-05-30_12-00-00-front.mp4"), []byte("0123456789abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := media.NewResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Close() })
	return res, "SavedClips/2026-05-30_12-00-00/2026-05-30_12-00-00-front.mp4"
}

func TestHandleMediaRange(t *testing.T) {
	res, rel := tempResolver(t)
	srv := testServer(t, &stubQuerier{segRel: rel}, res)

	req := httptest.NewRequest(http.MethodGet, "/api/media/1", nil)
	req.Header.Set("Range", "bytes=0-3")
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "video/mp4" {
		t.Errorf("content-type = %q, want video/mp4", ct)
	}
	if body := rec.Body.String(); body != "0123" {
		t.Errorf("body = %q, want 0123", body)
	}
}

func TestHandleMediaTraversalRejected(t *testing.T) {
	res, _ := tempResolver(t)
	srv := testServer(t, &stubQuerier{segRel: "../escape.mp4"}, res)
	rec := do(srv, http.MethodGet, "/api/media/1")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleMediaMissingSegment(t *testing.T) {
	res, _ := tempResolver(t)
	srv := testServer(t, &stubQuerier{segErr: pgx.ErrNoRows}, res)
	rec := do(srv, http.MethodGet, "/api/media/1")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleDownloadSegment(t *testing.T) {
	res, rel := tempResolver(t)
	srv := testServer(t, &stubQuerier{segRel: rel}, res)
	rec := do(srv, http.MethodGet, "/api/download/1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	cd := rec.Header().Get("Content-Disposition")
	if cd != `attachment; filename="2026-05-30_12-00-00-front.mp4"` {
		t.Errorf("content-disposition = %q", cd)
	}
}

func TestHandleDownloadClipZip(t *testing.T) {
	res, rel := tempResolver(t)
	q := &stubQuerier{zip: &query.ClipZipInfo{
		ClipTS:   time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
		RelPaths: []string{rel},
	}}
	srv := testServer(t, q, res)
	rec := do(srv, http.MethodGet, "/api/download/clip/1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("content-type = %q, want application/zip", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="2026-05-30_12-00-00.zip"` {
		t.Errorf("content-disposition = %q", cd)
	}
}

func TestHandleThumbMissing(t *testing.T) {
	res, _ := tempResolver(t)
	srv := testServer(t, &stubQuerier{thumbErr: pgx.ErrNoRows}, res)
	rec := do(srv, http.MethodGet, "/api/thumb/1")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
