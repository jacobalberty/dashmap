package api

import (
	"math"
	"time"

	"github.com/jacobalberty/dashmap/internal/query"
)

const metersPerDegLat = 111320.0

// buildTrackRuns splits ordered points into runs (breaking when the gap between
// consecutive points exceeds gap), simplifies each run with Douglas–Peucker at
// toleranceM, and applies a global uniform stride if the total still exceeds
// maxPoints. It returns the runs and the total number of points returned.
func buildTrackRuns(points []query.TrackPoint, gap time.Duration, toleranceM float64, maxPoints int) ([][]query.TrackPoint, int) {
	runs := splitRuns(points, gap)
	for i, run := range runs {
		runs[i] = douglasPeucker(run, toleranceM)
	}
	total := countPoints(runs)
	if maxPoints > 0 && total > maxPoints {
		runs = strideRuns(runs, total, maxPoints)
		total = countPoints(runs)
	}
	return runs, total
}

func splitRuns(points []query.TrackPoint, gap time.Duration) [][]query.TrackPoint {
	runs := [][]query.TrackPoint{}
	if len(points) == 0 {
		return runs
	}
	cur := []query.TrackPoint{points[0]}
	for i := 1; i < len(points); i++ {
		if points[i].TS.Sub(points[i-1].TS) > gap {
			runs = append(runs, cur)
			cur = []query.TrackPoint{points[i]}
			continue
		}
		cur = append(cur, points[i])
	}
	runs = append(runs, cur)
	return runs
}

func countPoints(runs [][]query.TrackPoint) int {
	n := 0
	for _, r := range runs {
		n += len(r)
	}
	return n
}

// douglasPeucker simplifies a polyline keeping points whose perpendicular
// distance (in metres) from the reduced line exceeds toleranceM. Endpoints are
// always kept.
func douglasPeucker(pts []query.TrackPoint, toleranceM float64) []query.TrackPoint {
	if len(pts) < 3 || toleranceM <= 0 {
		return pts
	}
	first, last := 0, len(pts)-1
	maxD, idx := 0.0, 0
	for i := first + 1; i < last; i++ {
		if d := perpDistM(pts[i], pts[first], pts[last]); d > maxD {
			maxD, idx = d, i
		}
	}
	if maxD > toleranceM {
		left := douglasPeucker(pts[:idx+1], toleranceM)
		right := douglasPeucker(pts[idx:], toleranceM)
		return append(left[:len(left)-1], right...)
	}
	return []query.TrackPoint{pts[first], pts[last]}
}

// perpDistM is the perpendicular distance from p to the line a→b in metres,
// using an equirectangular projection (longitude scaled by cos(latitude)).
func perpDistM(p, a, b query.TrackPoint) float64 {
	mPerLon := metersPerDegLat * math.Cos(a.Lat*math.Pi/180)
	ax, ay := a.Lon*mPerLon, a.Lat*metersPerDegLat
	bx, by := b.Lon*mPerLon, b.Lat*metersPerDegLat
	px, py := p.Lon*mPerLon, p.Lat*metersPerDegLat
	dx, dy := bx-ax, by-ay
	d := math.Hypot(dx, dy)
	if d == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	return math.Abs(dx*(ay-py)-dy*(ax-px)) / d
}

// strideRuns thins points by a uniform stride applied across the global point
// index so the overall total is guaranteed to fit within maxPoints. Run grouping
// is preserved (kept points stay in their run, in order); runs that lose all
// their points are dropped. This is a coarse safety net for pathologically wide
// ranges — the Douglas–Peucker pass handles ordinary simplification.
func strideRuns(runs [][]query.TrackPoint, total, maxPoints int) [][]query.TrackPoint {
	stride := (total + maxPoints - 1) / maxPoints // ceil
	if stride < 2 {
		return runs
	}
	out := make([][]query.TrackPoint, 0, len(runs))
	idx := 0
	for _, run := range runs {
		var kept []query.TrackPoint
		for _, p := range run {
			if idx%stride == 0 {
				kept = append(kept, p)
			}
			idx++
		}
		if len(kept) > 0 {
			out = append(out, kept)
		}
	}
	return out
}
