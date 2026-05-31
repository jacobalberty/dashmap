package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeTempPMTiles(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "basemap.pmtiles")
	if err := os.WriteFile(p, []byte("PMTiles\x03fake-data-0123456789abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestHandleBasemapUnset(t *testing.T) {
	srv := testServer(t, &stubQuerier{}, nil)
	rec := do(srv, http.MethodGet, "/tiles/basemap.pmtiles")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleBasemapMissing(t *testing.T) {
	srv := testServer(t, &stubQuerier{}, nil)
	srv.deps.Cfg.BasemapPMTiles = "/nonexistent/basemap.pmtiles"
	rec := do(srv, http.MethodGet, "/tiles/basemap.pmtiles")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandleBasemapRange(t *testing.T) {
	path := writeTempPMTiles(t)
	srv := testServer(t, &stubQuerier{}, nil)
	srv.deps.Cfg.BasemapPMTiles = path

	req := httptest.NewRequest(http.MethodGet, "/tiles/basemap.pmtiles", nil)
	req.Header.Set("Range", "bytes=0-6")
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", rec.Code)
	}
	if body := rec.Body.String(); body != "PMTiles" {
		t.Errorf("body = %q, want PMTiles", body)
	}
}

func TestHandleBasemapHead(t *testing.T) {
	path := writeTempPMTiles(t)
	srv := testServer(t, &stubQuerier{}, nil)
	srv.deps.Cfg.BasemapPMTiles = path

	rec := do(srv, http.MethodHead, "/tiles/basemap.pmtiles")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleCalendarInvalidTZ(t *testing.T) {
	rec := do(testServer(t, &stubQuerier{carID: 1}, nil), http.MethodGet, "/api/calendar?tz=Not/AReal_Zone")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	e := decodeEnvelope(t, rec)
	if e.Error == nil || e.Error.Code != codeBadRequest {
		t.Fatalf("error = %+v, want bad_request", e.Error)
	}
}

func TestHandleCalendarValidTZ(t *testing.T) {
	rec := do(testServer(t, &stubQuerier{carID: 1}, nil), http.MethodGet, "/api/calendar?tz=America/Los_Angeles")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
