package ingest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// knownCameras is the set of valid Tesla dashcam camera suffixes.
var knownCameras = map[string]bool{
	"front":          true,
	"back":           true,
	"left_repeater":  true,
	"right_repeater": true,
	"left_pillar":    true,
	"right_pillar":   true,
}

// segment represents one camera file parsed from a Tesla segment filename.
type segment struct {
	ts       time.Time
	camera   string
	absPath  string
	relPath  string // relative to videoRoot
	size     int64
	mtime    time.Time
}

// clip is a group of camera segments sharing the same timestamp.
type clip struct {
	ts       time.Time
	segments []segment
}

// parseSegment parses a Tesla dashcam filename of the form
// YYYY-MM-DD_HH-MM-SS-<camera>.mp4 and returns the timestamp (UTC) and camera.
func parseSegment(name string) (ts time.Time, camera string, ok bool) {
	if !strings.HasSuffix(name, ".mp4") {
		return
	}
	base := strings.TrimSuffix(name, ".mp4")
	// Expected: 2026-05-29_09-49-34-front
	// Split at last occurrence of '-' that precedes the camera name.
	for cam := range knownCameras {
		suffix := "-" + cam
		if tsStr, ok2 := strings.CutSuffix(base, suffix); ok2 {
			t, err := time.Parse("2006-01-02_15-04-05", tsStr)
			if err != nil {
				return
			}
			// All Tesla timestamps are naive wall-clock — treat as UTC.
			return time.Date(t.Year(), t.Month(), t.Day(),
				t.Hour(), t.Minute(), t.Second(), 0, time.UTC), cam, true
		}
	}
	return
}

// readSegments reads all segment files from dirAbs and returns them along with
// the set of camera names observed (used for hw_generation detection).
func readSegments(dirAbs, videoRoot string) ([]segment, map[string]bool, error) {
	entries, err := os.ReadDir(dirAbs)
	if err != nil {
		return nil, nil, err
	}
	var segs []segment
	cameras := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ts, camera, ok := parseSegment(e.Name())
		if !ok {
			continue
		}
		abs := filepath.Join(dirAbs, e.Name())
		rel, err := filepath.Rel(videoRoot, abs)
		if err != nil {
			rel = abs
		}
		fi, err := e.Info()
		if err != nil {
			return nil, nil, fmt.Errorf("stat %s: %w", e.Name(), err)
		}
		segs = append(segs, segment{
			ts:      ts,
			camera:  camera,
			absPath: abs,
			relPath: filepath.ToSlash(rel),
			size:    fi.Size(),
			mtime:   fi.ModTime().UTC(),
		})
		cameras[camera] = true
	}
	return segs, cameras, nil
}

// groupClips groups a slice of segments by timestamp into clips, sorted by
// ascending clip timestamp.
func groupClips(segs []segment) []clip {
	byTS := map[time.Time][]segment{}
	for _, s := range segs {
		byTS[s.ts] = append(byTS[s.ts], s)
	}
	clips := make([]clip, 0, len(byTS))
	for ts, ss := range byTS {
		clips = append(clips, clip{ts: ts, segments: ss})
	}
	sort.Slice(clips, func(i, j int) bool {
		return clips[i].ts.Before(clips[j].ts)
	})
	return clips
}

// hwGeneration returns the hw_generation string for a set of observed camera names.
func hwGeneration(cameras map[string]bool) string {
	if cameras["left_pillar"] || cameras["right_pillar"] {
		return "hw4"
	}
	if cameras["front"] || cameras["back"] || cameras["left_repeater"] || cameras["right_repeater"] {
		return "hw3"
	}
	return "unknown"
}

// preferredCamera returns the best camera to use for SEI extraction.
// Prefers "front"; falls back to alphabetical order among present cameras.
func preferredCamera(segs []segment) string {
	for _, s := range segs {
		if s.camera == "front" {
			return "front"
		}
	}
	if len(segs) > 0 {
		return segs[0].camera
	}
	return ""
}
