package api

import (
	"testing"
	"time"

	"github.com/jacobalberty/dashmap/internal/query"
)

func pt(sec int, lat, lon float64) query.TrackPoint {
	return query.TrackPoint{
		TS:  time.Date(2026, 5, 30, 12, 0, sec, 0, time.UTC),
		Lat: lat, Lon: lon, ClipID: 1,
	}
}

func TestSplitRuns(t *testing.T) {
	// Two points close in time, then a 10-minute gap, then two more.
	pts := []query.TrackPoint{
		pt(0, 1, 1), pt(1, 1, 1),
		{TS: time.Date(2026, 5, 30, 12, 10, 0, 0, time.UTC), Lat: 2, Lon: 2, ClipID: 2},
		{TS: time.Date(2026, 5, 30, 12, 10, 1, 0, time.UTC), Lat: 2, Lon: 2, ClipID: 2},
	}
	runs := splitRuns(pts, 2*time.Minute)
	if len(runs) != 2 {
		t.Fatalf("got %d runs, want 2", len(runs))
	}
	if len(runs[0]) != 2 || len(runs[1]) != 2 {
		t.Fatalf("run sizes = %d,%d, want 2,2", len(runs[0]), len(runs[1]))
	}
}

func TestDouglasPeuckerCollinear(t *testing.T) {
	// Three collinear points along a meridian → middle dropped.
	pts := []query.TrackPoint{pt(0, 0, 0), pt(1, 0.001, 0), pt(2, 0.002, 0)}
	out := douglasPeucker(pts, 5)
	if len(out) != 2 {
		t.Fatalf("collinear simplified to %d points, want 2", len(out))
	}
}

func TestDouglasPeuckerKeepsCorner(t *testing.T) {
	// Middle point is well off the line a→b (~100m east) → kept.
	pts := []query.TrackPoint{pt(0, 0, 0), pt(1, 0.001, 0.001), pt(2, 0.002, 0)}
	out := douglasPeucker(pts, 5)
	if len(out) != 3 {
		t.Fatalf("cornered simplified to %d points, want 3", len(out))
	}
}

func TestBuildTrackRunsStrideCap(t *testing.T) {
	// A zig-zag run that DP can't simplify below the cap, forcing a stride.
	var pts []query.TrackPoint
	for i := range 100 {
		lon := 0.0
		if i%2 == 1 {
			lon = 0.001
		}
		pts = append(pts, pt(i, float64(i)*0.001, lon))
	}
	runs, returned := buildTrackRuns(pts, time.Minute, 1, 20)
	if returned > 20 {
		t.Fatalf("returned %d points, want <= 20", returned)
	}
	if len(runs) != 1 {
		t.Fatalf("got %d runs, want 1", len(runs))
	}
}

func TestBuildTrackRunsEmpty(t *testing.T) {
	runs, returned := buildTrackRuns(nil, time.Minute, 5, 100)
	if runs == nil {
		t.Fatal("runs is nil, want non-nil empty slice")
	}
	if len(runs) != 0 || returned != 0 {
		t.Fatalf("got %d runs / %d returned, want 0/0", len(runs), returned)
	}
}
