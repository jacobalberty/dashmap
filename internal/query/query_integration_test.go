//go:build integration

package query_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jacobalberty/dashmap/internal/db"
	"github.com/jacobalberty/dashmap/internal/query"
)

func TestQueryReadLayer(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is required for integration tests")
	}
	ctx := context.Background()

	if err := db.Migrate(ctx, dsn); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer pool.Close()

	const carKey = "query-it-test"
	var carID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO cars (key, display_name, hw_generation) VALUES ($1,'IT Car','hw3') RETURNING id`,
		carKey).Scan(&carID); err != nil {
		t.Fatalf("insert car: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM clips WHERE car_id=$1`, carID)
		_, _ = pool.Exec(ctx, `DELETE FROM events WHERE car_id=$1`, carID)
		_, _ = pool.Exec(ctx, `DELETE FROM ingest_state WHERE car_id=$1`, carID)
		_, _ = pool.Exec(ctx, `DELETE FROM cars WHERE id=$1`, carID)
	})

	day := time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC)

	// Event (saved) with a thumbnail.
	var eventID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO events (car_id, event_ts, folder_ts, kind, reason, city, camera, folder_rel_path, thumb_rel_path, raw)
		VALUES ($1,$2,$2,'saved','object detected','SF','front','SavedClips/2026-05-30_12-00-00',
		        'SavedClips/2026-05-30_12-00-00/thumb.png','{}'::jsonb) RETURNING id`,
		carID, day.Add(12*time.Hour)).Scan(&eventID); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	// Saved clip (with GPS) + recent clip (no GPS).
	var clipSaved, clipRecent int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO clips (car_id, event_id, kind, clip_ts, lat, lon, has_sei, duration_s)
		VALUES ($1,$2,'saved',$3,37.77,-122.41,true,60) RETURNING id`,
		carID, eventID, day.Add(12*time.Hour)).Scan(&clipSaved); err != nil {
		t.Fatalf("insert saved clip: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO clips (car_id, kind, clip_ts, has_sei)
		VALUES ($1,'recent',$2,false) RETURNING id`,
		carID, day.Add(13*time.Hour)).Scan(&clipRecent); err != nil {
		t.Fatalf("insert recent clip: %v", err)
	}

	// Segments for the saved clip, inserted out of canonical order.
	var frontSeg int64
	for _, cam := range []string{"left_repeater", "back", "front"} {
		var id int64
		rel := "SavedClips/2026-05-30_12-00-00/2026-05-30_12-00-00-" + cam + ".mp4"
		if err := pool.QueryRow(ctx, `
			INSERT INTO segments (clip_id, camera, rel_path, size_bytes, has_sei)
			VALUES ($1,$2,$3,1000,true) RETURNING id`,
			clipSaved, cam, rel).Scan(&id); err != nil {
			t.Fatalf("insert segment %s: %v", cam, err)
		}
		if cam == "front" {
			frontSeg = id
		}
	}

	// Track points for the saved clip.
	for i := 0; i < 5; i++ {
		if _, err := pool.Exec(ctx, `
			INSERT INTO track_points (car_id, clip_id, segment_id, ts, lat, lon)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			carID, clipSaved, frontSeg, day.Add(12*time.Hour+time.Duration(i)*time.Second),
			37.77+float64(i)*0.0001, -122.41); err != nil {
			t.Fatalf("insert track point: %v", err)
		}
	}

	// Scan-state: a scan that just completed.
	if _, err := pool.Exec(ctx, `
		INSERT INTO ingest_state (car_id, last_scan_started_at, last_scan_completed_at, clips_processed, track_points_written)
		VALUES ($1, now(), now(), 2, 5)`, carID); err != nil {
		t.Fatalf("insert ingest_state: %v", err)
	}

	s := query.New(pool)
	from := day
	to := day.AddDate(0, 0, 1)

	// CarIDByKey
	if got, err := s.CarIDByKey(ctx, carKey); err != nil || got != carID {
		t.Fatalf("CarIDByKey = %d,%v want %d", got, err, carID)
	}
	if _, err := s.CarIDByKey(ctx, "does-not-exist"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("CarIDByKey(missing) err = %v, want ErrNoRows", err)
	}

	// Calendar
	days, err := s.Calendar(ctx, carID, from, to)
	if err != nil || len(days) != 1 {
		t.Fatalf("Calendar = %+v,%v want 1 day", days, err)
	}
	if days[0].Date != "2026-05-30" || days[0].Total != 2 || days[0].Saved != 1 || days[0].Recent != 1 {
		t.Errorf("Calendar day = %+v", days[0])
	}

	// ListClips: newest first, cameras ordered, thumb only on saved.
	clips, hasMore, err := s.ListClips(ctx, carID, from, to, nil, 10, 0)
	if err != nil || len(clips) != 2 {
		t.Fatalf("ListClips = %d,%v want 2", len(clips), err)
	}
	if hasMore {
		t.Error("hasMore = true, want false")
	}
	if clips[0].ID != clipRecent {
		t.Error("first clip is not the newest (recent)")
	}
	if clips[0].ThumbURL != nil {
		t.Error("recent clip thumb_url should be nil")
	}
	var saved *query.ClipListItem
	for i := range clips {
		if clips[i].ID == clipSaved {
			saved = &clips[i]
		}
	}
	if saved == nil {
		t.Fatal("saved clip missing from list")
	}
	if saved.ThumbURL == nil || *saved.ThumbURL == "" {
		t.Error("saved clip thumb_url should be set")
	}
	wantCams := []string{"front", "back", "left_repeater"}
	if len(saved.Cameras) != 3 || saved.Cameras[0] != wantCams[0] || saved.Cameras[1] != wantCams[1] || saved.Cameras[2] != wantCams[2] {
		t.Errorf("cameras = %v, want %v", saved.Cameras, wantCams)
	}

	// Kind filter + pagination.
	recentOnly, _, err := s.ListClips(ctx, carID, from, to, []string{"recent"}, 10, 0)
	if err != nil || len(recentOnly) != 1 || recentOnly[0].ID != clipRecent {
		t.Fatalf("ListClips(recent) = %+v,%v", recentOnly, err)
	}
	page, more, err := s.ListClips(ctx, carID, from, to, nil, 1, 0)
	if err != nil || len(page) != 1 || !more {
		t.Fatalf("ListClips(limit=1) = %d,%v,%v want 1,true", len(page), more, err)
	}

	// Markers: only the GPS clip.
	markers, capped, err := s.ClipMarkers(ctx, carID, from, to, 100)
	if err != nil || capped || len(markers) != 1 || markers[0].ID != clipSaved {
		t.Fatalf("ClipMarkers = %+v capped=%v err=%v", markers, capped, err)
	}

	// Track.
	pts, err := s.Track(ctx, carID, from, to)
	if err != nil || len(pts) != 5 {
		t.Fatalf("Track = %d,%v want 5", len(pts), err)
	}

	// Clip detail.
	detail, err := s.ClipDetail(ctx, clipSaved)
	if err != nil {
		t.Fatalf("ClipDetail: %v", err)
	}
	if detail.Event == nil || detail.ThumbURL == nil {
		t.Error("detail event/thumb missing")
	}
	if len(detail.Segments) != 3 || detail.Segments[0].Camera != "front" {
		t.Errorf("detail segments = %+v", detail.Segments)
	}
	if detail.Segments[0].MediaURL == "" || detail.DownloadURL == "" {
		t.Error("detail urls missing")
	}
	if _, err := s.ClipDetail(ctx, 999999999); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("ClipDetail(missing) err = %v, want ErrNoRows", err)
	}

	// Segment path + zip.
	rel, err := s.SegmentRelPath(ctx, frontSeg)
	if err != nil || rel == "" {
		t.Fatalf("SegmentRelPath = %q,%v", rel, err)
	}
	zipInfo, err := s.ClipZip(ctx, clipSaved)
	if err != nil || len(zipInfo.RelPaths) != 3 {
		t.Fatalf("ClipZip = %+v,%v", zipInfo, err)
	}

	// Thumb.
	if _, err := s.EventThumbRelPath(ctx, eventID); err != nil {
		t.Errorf("EventThumbRelPath: %v", err)
	}

	// Status.
	st, err := s.Status(ctx, carID, carKey, 10*time.Minute)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.Counts.Clips != 2 || st.Counts.Events != 1 || st.Counts.Segments != 3 || st.Counts.TrackPoints != 5 {
		t.Errorf("status counts = %+v", st.Counts)
	}
	if !st.Car.Live || st.Car.LastScanCompletedAt == nil {
		t.Errorf("status car not live: %+v", st.Car)
	}
	if st.Car.LastScan.Processed != 2 || st.Car.LastScan.TrackPoints != 5 {
		t.Errorf("status last_scan = %+v", st.Car.LastScan)
	}
}
