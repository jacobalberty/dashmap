package media

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func newTestResolver(t *testing.T) (*Resolver, string) {
	t.Helper()
	dir := t.TempDir()
	sub := filepath.Join(dir, "SavedClips", "clip")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "front.mp4"), []byte("hello-front-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := NewResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Close() })
	return res, "SavedClips/clip/front.mp4"
}

func TestResolverOpenAndTraversal(t *testing.T) {
	res, rel := newTestResolver(t)

	f, fi, err := res.Open(rel)
	if err != nil {
		t.Fatalf("Open(%q) error: %v", rel, err)
	}
	_ = f.Close()
	if fi.Size() != int64(len("hello-front-bytes")) {
		t.Errorf("size = %d", fi.Size())
	}

	for _, bad := range []string{"../escape.mp4", "SavedClips/../../etc/passwd"} {
		if _, _, err := res.Open(bad); err == nil {
			t.Errorf("Open(%q) succeeded, want traversal error", bad)
		}
	}
}

func TestResolverServeFull(t *testing.T) {
	res, rel := newTestResolver(t)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	if err := res.Serve(rec, req, rel, "video/mp4", ""); err != nil {
		t.Fatalf("Serve error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != immutableCache {
		t.Errorf("cache-control = %q", cc)
	}
	if rec.Body.String() != "hello-front-bytes" {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestResolverZipStore(t *testing.T) {
	res, rel := newTestResolver(t)
	rec := httptest.NewRecorder()
	started, err := res.ZipClip(rec, "clip.zip", []ZipEntry{{RelPath: rel, Name: "front.mp4"}}, zip.Store)
	if err != nil {
		t.Fatalf("ZipClip error: %v (started=%v)", err, started)
	}

	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("zip read: %v", err)
	}
	if len(zr.File) != 1 {
		t.Fatalf("zip has %d files, want 1", len(zr.File))
	}
	zf := zr.File[0]
	if zf.Name != "front.mp4" {
		t.Errorf("entry name = %q, want front.mp4", zf.Name)
	}
	if zf.Method != zip.Store {
		t.Errorf("method = %d, want Store(0)", zf.Method)
	}
	rc, err := zf.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()
	got, _ := io.ReadAll(rc)
	if string(got) != "hello-front-bytes" {
		t.Errorf("entry body = %q", string(got))
	}
}

func TestResolverZipMissingFile(t *testing.T) {
	res, _ := newTestResolver(t)
	rec := httptest.NewRecorder()
	started, err := res.ZipClip(rec, "clip.zip", []ZipEntry{{RelPath: "SavedClips/clip/missing.mp4", Name: "missing.mp4"}}, zip.Store)
	if err == nil {
		t.Fatal("ZipClip error = nil, want missing-file error")
	}
	if started {
		t.Error("started = true, want false (failed before writing)")
	}
}
