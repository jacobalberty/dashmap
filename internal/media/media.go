// Package media serves dashcam video segments and thumbnails via HTTP range
// requests, confined to a single video root (Story 04).
package media

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
)

// immutableCache is sent for media, downloads and thumbnails: a given
// segment_id/event_id maps to a write-once dashcam file whose bytes never change.
const immutableCache = "private, max-age=31536000, immutable"

// Resolver opens files by their root-relative path while preventing path
// traversal outside the configured video root. It is backed by an *os.Root,
// which confines all access (rejecting "../" and root-escaping symlinks). This is
// path-confinement, not a hardened security boundary against an adversarial
// filesystem — acceptable here because the tree is trusted and read-only.
type Resolver struct {
	root *os.Root
}

// NewResolver opens videoRoot as an os.Root. It fails fast if the directory does
// not exist or cannot be opened, so the serve command can refuse to start.
func NewResolver(videoRoot string) (*Resolver, error) {
	r, err := os.OpenRoot(videoRoot)
	if err != nil {
		return nil, err
	}
	return &Resolver{root: r}, nil
}

// Close releases the underlying directory handle.
func (r *Resolver) Close() error { return r.root.Close() }

// Open opens a regular file by its slash-separated path relative to the video
// root. Any path that escapes the root (or is missing) returns an error.
func (r *Resolver) Open(rel string) (*os.File, fs.FileInfo, error) {
	f, err := r.root.Open(rel)
	if err != nil {
		return nil, nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	if fi.IsDir() {
		_ = f.Close()
		return nil, nil, fmt.Errorf("media: %q is a directory", rel)
	}
	return f, fi, nil
}

// Serve streams the file at rel with HTTP range support (206). contentType is set
// explicitly because the distroless runtime image has no system mime database.
// When attachmentName is non-empty a download Content-Disposition is added.
// Any resolution error is returned before the response is written, so the caller
// can map it to a 404.
func (r *Resolver) Serve(w http.ResponseWriter, req *http.Request, rel, contentType, attachmentName string) error {
	f, fi, err := r.Open(rel)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", immutableCache)
	if attachmentName != "" {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", attachmentName))
	}
	// name="" so ServeContent does not override our explicit Content-Type.
	http.ServeContent(w, req, "", fi.ModTime(), f)
	return nil
}

// ZipEntry is one file to include in a clip zip: its root-relative source path
// and the name it should have inside the archive.
type ZipEntry struct {
	RelPath string
	Name    string
}

// ZipClip streams a zip archive of entries to w using the given method
// (zip.Store or zip.Deflate). All source files are opened first, so a missing
// file is reported before any bytes are written. The returned started flag is
// true once response headers have been written: if err != nil && !started the
// caller can still send an error status; otherwise the error can only be logged.
func (r *Resolver) ZipClip(w http.ResponseWriter, archiveName string, entries []ZipEntry, method uint16) (started bool, err error) {
	type openFile struct {
		f    *os.File
		fi   fs.FileInfo
		name string
	}
	files := make([]openFile, 0, len(entries))
	defer func() {
		for _, of := range files {
			_ = of.f.Close()
		}
	}()
	for _, e := range entries {
		f, fi, openErr := r.Open(e.RelPath)
		if openErr != nil {
			return false, openErr
		}
		files = append(files, openFile{f: f, fi: fi, name: e.Name})
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Cache-Control", immutableCache)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", archiveName))
	started = true

	zw := zip.NewWriter(w)
	defer func() { _ = zw.Close() }()
	for _, of := range files {
		hdr := &zip.FileHeader{Name: of.name, Method: method, Modified: of.fi.ModTime()}
		fw, createErr := zw.CreateHeader(hdr)
		if createErr != nil {
			return started, createErr
		}
		if _, copyErr := io.Copy(fw, of.f); copyErr != nil {
			return started, copyErr
		}
	}
	return started, nil
}
