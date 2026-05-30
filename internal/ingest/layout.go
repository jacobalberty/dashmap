package ingest

import (
	"os"
	"path/filepath"
)

// ClipKind mirrors the DB check constraint values.
type ClipKind string

const (
	KindRecent ClipKind = "recent"
	KindSaved  ClipKind = "saved"
	KindSentry ClipKind = "sentry"
)

// eventFolder represents one SavedClips or SentryClips event subfolder.
type eventFolder struct {
	kind    ClipKind
	absPath string
	relPath string // relative to videoRoot
}

// detectRoot returns whichever of videoRoot or videoRoot/TeslaCam contains at
// least one of RecentClips, SavedClips, or SentryClips.  Returns videoRoot
// itself when neither check succeeds (walk will just find no subdirs).
func detectRoot(videoRoot string) string {
	candidates := []string{videoRoot, filepath.Join(videoRoot, "TeslaCam")}
	targets := []string{"RecentClips", "SavedClips", "SentryClips"}
	for _, c := range candidates {
		for _, t := range targets {
			if fi, err := os.Stat(filepath.Join(c, t)); err == nil && fi.IsDir() {
				return c
			}
		}
	}
	return videoRoot
}

// walkFolders yields all event folders and the RecentClips directory (if
// present) under root.  Missing subdirectories are silently skipped.
func walkFolders(root, videoRoot string) ([]eventFolder, string, error) {
	var events []eventFolder
	var recentAbs string

	recent := filepath.Join(root, "RecentClips")
	if fi, err := os.Stat(recent); err == nil && fi.IsDir() {
		recentAbs = recent
	}

	for _, pair := range []struct {
		dir  string
		kind ClipKind
	}{
		{filepath.Join(root, "SavedClips"), KindSaved},
		{filepath.Join(root, "SentryClips"), KindSentry},
	} {
		entries, err := os.ReadDir(pair.dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		for _, e := range entries {
			abs := filepath.Join(pair.dir, e.Name())
			// Use os.Stat (follows symlinks) so that symlinked dirs are included.
			fi, err := os.Stat(abs)
			if err != nil || !fi.IsDir() {
				continue
			}
			rel, err := filepath.Rel(videoRoot, abs)
			if err != nil {
				rel = abs
			}
			events = append(events, eventFolder{
				kind:    pair.kind,
				absPath: abs,
				relPath: filepath.ToSlash(rel),
			})
		}
	}
	return events, recentAbs, nil
}
