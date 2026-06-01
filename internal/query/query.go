// Package query is the read-side data layer for the HTTP API (Story 04). It is
// the counterpart to internal/ingest's write-side store: typed methods returning
// DTOs, raw pgx, no ORM.
package query

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jacobalberty/dashmap/internal/ingest"
)

// Store runs read queries against the pool.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// cameraRankCase is a SQL "CASE camera WHEN … END" expression built from the
// canonical camera order so the DB can sort cameras/segments consistently with
// ingest. The camera literals are fixed, safe identifiers (no user input).
var cameraRankCase = buildCameraRankCase()

func buildCameraRankCase() string {
	var b strings.Builder
	b.WriteString("CASE camera")
	for i, c := range ingest.CameraPriority {
		fmt.Fprintf(&b, " WHEN '%s' THEN %d", c, i)
	}
	fmt.Fprintf(&b, " ELSE %d END", len(ingest.CameraPriority))
	return b.String()
}

func mediaURL(segmentID int64) string    { return fmt.Sprintf("/api/media/%d", segmentID) }
func downloadURL(segmentID int64) string { return fmt.Sprintf("/api/download/%d", segmentID) }
func clipZipURL(clipID int64) string     { return fmt.Sprintf("/api/download/clip/%d", clipID) }
func thumbURL(eventID int64) string      { return fmt.Sprintf("/api/thumb/%d", eventID) }

// ---- DTOs --------------------------------------------------------------------

type Car struct {
	ID           int64   `json:"id"`
	Key          string  `json:"key"`
	DisplayName  *string `json:"display_name"`
	HWGeneration *string `json:"hw_generation"`
}

type CalendarDay struct {
	Date   string `json:"date"` // YYYY-MM-DD (UTC)
	Total  int64  `json:"total"`
	Recent int64  `json:"recent"`
	Saved  int64  `json:"saved"`
	Sentry int64  `json:"sentry"`
}

type ClipListItem struct {
	ID        int64     `json:"id"`
	CarID     int64     `json:"car_id"`
	ClipTS    time.Time `json:"clip_ts"`
	Kind      string    `json:"kind"`
	Lat       *float64  `json:"lat"`
	Lon       *float64  `json:"lon"`
	HasSEI    bool      `json:"has_sei"`
	DurationS *float32  `json:"duration_s"`
	Reason    *string   `json:"reason"`
	City      *string   `json:"city"`
	ThumbURL  *string   `json:"thumb_url"`
	EventID   *int64    `json:"event_id"` // groups clips of one Saved/Sentry event
	Cameras   []string  `json:"cameras"`
}

type ClipMarker struct {
	ID     int64     `json:"id"`
	ClipTS time.Time `json:"clip_ts"`
	Lat    float64   `json:"lat"`
	Lon    float64   `json:"lon"`
	Kind   string    `json:"kind"`
	HasSEI bool      `json:"has_sei"`
}

type TrackPoint struct {
	TS     time.Time `json:"ts"`
	Lat    float64   `json:"lat"`
	Lon    float64   `json:"lon"`
	ClipID int64     `json:"clip_id"`
}

type ClipSegment struct {
	SegmentID   int64  `json:"segment_id"`
	Camera      string `json:"camera"`
	MediaURL    string `json:"media_url"`
	DownloadURL string `json:"download_url"`
	HasSEI      bool   `json:"has_sei"`
	SizeBytes   *int64 `json:"size_bytes"`
}

type ClipTrackPoint struct {
	TS                       time.Time `json:"ts"`
	Lat                      float64   `json:"lat"`
	Lon                      float64   `json:"lon"`
	SpeedMPS                 *float32  `json:"speed_mps"`
	HeadingDeg               *float64  `json:"heading_deg"`
	Gear                     *int16    `json:"gear"`
	Autopilot                *int16    `json:"autopilot"`
	BlinkerLeft              *bool     `json:"blinker_left"`
	BlinkerRight             *bool     `json:"blinker_right"`
	BrakeApplied             *bool     `json:"brake_applied"`
	AcceleratorPedalPosition *float32  `json:"accelerator_pedal_position"`
	SteeringWheelAngle       *float32  `json:"steering_wheel_angle"`
	AccelX                   *float64  `json:"accel_x"`
	AccelY                   *float64  `json:"accel_y"`
	AccelZ                   *float64  `json:"accel_z"`
	SeiVersion               *int32    `json:"sei_version"`
}

type ClipEvent struct {
	EventTS time.Time `json:"event_ts"`
	Reason  *string   `json:"reason"`
	City    *string   `json:"city"`
	Camera  *string   `json:"camera"`
}

type ClipDetail struct {
	ID          int64         `json:"id"`
	CarID       int64         `json:"car_id"`
	ClipTS      time.Time     `json:"clip_ts"`
	Kind        string        `json:"kind"`
	Lat         *float64      `json:"lat"`
	Lon         *float64      `json:"lon"`
	HasSEI      bool          `json:"has_sei"`
	DurationS   *float32      `json:"duration_s"`
	ThumbURL    *string       `json:"thumb_url"`
	DownloadURL string        `json:"download_url"` // zip of all cameras
	Event       *ClipEvent    `json:"event"`
	Segments    []ClipSegment `json:"segments"`
	PrevClipID  *int64        `json:"prev_clip_id"`
	NextClipID  *int64        `json:"next_clip_id"`
}

type StatusCounts struct {
	Cars        int64 `json:"cars"`
	Events      int64 `json:"events"`
	Clips       int64 `json:"clips"`
	Segments    int64 `json:"segments"`
	TrackPoints int64 `json:"track_points"`
}

type ScanCounts struct {
	Processed   int64 `json:"processed"`
	Skipped     int64 `json:"skipped"`
	Errored     int64 `json:"errored"`
	TrackPoints int64 `json:"track_points"`
}

type CarStatus struct {
	CarID               int64      `json:"car_id"`
	Key                 string     `json:"key"`
	LastScanStartedAt   *time.Time `json:"last_scan_started_at"`
	LastScanCompletedAt *time.Time `json:"last_scan_completed_at"`
	Live                bool       `json:"live"`
	LastScan            ScanCounts `json:"last_scan"`
}

type Status struct {
	DBReachable bool         `json:"db_reachable"`
	Counts      StatusCounts `json:"counts"`
	Car         CarStatus    `json:"car"`
}

// ClipZipInfo is the data needed to build a clip download zip.
type ClipZipInfo struct {
	ClipTS   time.Time
	RelPaths []string
}

// ---- Queries -----------------------------------------------------------------

// CarIDByKey returns the id of the car with the given key, or pgx.ErrNoRows.
func (s *Store) CarIDByKey(ctx context.Context, key string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT id FROM cars WHERE key = $1`, key).Scan(&id)
	return id, err
}

// ListCars returns all cars ordered for a selector.
func (s *Store) ListCars(ctx context.Context) ([]Car, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, key, display_name, hw_generation FROM cars ORDER BY display_name NULLS LAST, key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cars := []Car{}
	for rows.Next() {
		var c Car
		if err := rows.Scan(&c.ID, &c.Key, &c.DisplayName, &c.HWGeneration); err != nil {
			return nil, err
		}
		cars = append(cars, c)
	}
	return cars, rows.Err()
}

// Calendar returns per-day clip counts bucketed by tz (e.g. "America/Los_Angeles") in [from, to).
// tz defaults to "UTC" when empty.
func (s *Store) Calendar(ctx context.Context, carID int64, from, to time.Time, tz string) ([]CalendarDay, error) {
	if tz == "" {
		tz = "UTC"
	}
	rows, err := s.pool.Query(ctx, `
		SELECT (clip_ts AT TIME ZONE $4)::date AS d,
		       count(*)                              AS total,
		       count(*) FILTER (WHERE kind='recent') AS recent,
		       count(*) FILTER (WHERE kind='saved')  AS saved,
		       count(*) FILTER (WHERE kind='sentry') AS sentry
		FROM clips
		WHERE car_id = $1 AND clip_ts >= $2 AND clip_ts < $3
		GROUP BY d
		ORDER BY d`,
		carID, from, to, tz)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	days := []CalendarDay{}
	for rows.Next() {
		var d time.Time
		var day CalendarDay
		if err := rows.Scan(&d, &day.Total, &day.Recent, &day.Saved, &day.Sentry); err != nil {
			return nil, err
		}
		day.Date = d.Format("2006-01-02")
		days = append(days, day)
	}
	return days, rows.Err()
}

// ListClips returns a page of clips (newest first) in [from, to), optionally
// filtered by kinds. It fetches limit+1 rows to report hasMore.
func (s *Store) ListClips(ctx context.Context, carID int64, from, to time.Time, kinds []string, limit, offset int) (items []ClipListItem, hasMore bool, err error) {
	args := []any{carID, from, to}
	q := fmt.Sprintf(`
		SELECT c.id, c.car_id, c.clip_ts, c.kind, c.lat, c.lon, c.has_sei, c.duration_s,
		       e.reason, e.city, c.event_id, e.thumb_rel_path,
		       COALESCE((SELECT array_agg(camera ORDER BY %s, camera)
		                 FROM segments WHERE clip_id = c.id), '{}') AS cameras
		FROM clips c
		LEFT JOIN events e ON e.id = c.event_id
		WHERE c.car_id = $1 AND c.clip_ts >= $2 AND c.clip_ts < $3`, cameraRankCase)
	if len(kinds) > 0 {
		args = append(args, kinds)
		q += fmt.Sprintf(" AND c.kind = ANY($%d)", len(args))
	}
	args = append(args, limit+1)
	q += fmt.Sprintf(" ORDER BY c.clip_ts DESC, c.id DESC LIMIT $%d", len(args))
	args = append(args, offset)
	q += fmt.Sprintf(" OFFSET $%d", len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	items = []ClipListItem{}
	for rows.Next() {
		var it ClipListItem
		var eventID *int64
		var thumbRel *string
		if err := rows.Scan(&it.ID, &it.CarID, &it.ClipTS, &it.Kind, &it.Lat, &it.Lon,
			&it.HasSEI, &it.DurationS, &it.Reason, &it.City, &eventID, &thumbRel, &it.Cameras); err != nil {
			return nil, false, err
		}
		it.EventID = eventID
		if eventID != nil && thumbRel != nil {
			u := thumbURL(*eventID)
			it.ThumbURL = &u
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(items) > limit {
		hasMore = true
		items = items[:limit]
	}
	return items, hasMore, nil
}

// ClipMarkers returns clips with GPS in [from, to), capped at max+? (fetches
// max+1 to report capped). Only clips with non-null lat/lon are returned.
func (s *Store) ClipMarkers(ctx context.Context, carID int64, from, to time.Time, max int) (markers []ClipMarker, capped bool, err error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, clip_ts, lat, lon, kind, has_sei
		FROM clips
		WHERE car_id = $1 AND clip_ts >= $2 AND clip_ts < $3
		  AND lat IS NOT NULL AND lon IS NOT NULL
		ORDER BY clip_ts ASC
		LIMIT $4`,
		carID, from, to, max+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	markers = []ClipMarker{}
	for rows.Next() {
		var m ClipMarker
		if err := rows.Scan(&m.ID, &m.ClipTS, &m.Lat, &m.Lon, &m.Kind, &m.HasSEI); err != nil {
			return nil, false, err
		}
		markers = append(markers, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(markers) > max {
		capped = true
		markers = markers[:max]
	}
	return markers, capped, nil
}

// Track returns all GPS points in [from, to) ordered by time. track_points only
// exist for driving clips with SEI, so the volume is modest; run-splitting and
// simplification happen in the API layer.
func (s *Store) Track(ctx context.Context, carID int64, from, to time.Time) ([]TrackPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ts, lat, lon, clip_id
		FROM track_points
		WHERE car_id = $1 AND ts >= $2 AND ts < $3
		ORDER BY ts ASC`,
		carID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pts := []TrackPoint{}
	for rows.Next() {
		var p TrackPoint
		if err := rows.Scan(&p.TS, &p.Lat, &p.Lon, &p.ClipID); err != nil {
			return nil, err
		}
		pts = append(pts, p)
	}
	return pts, rows.Err()
}

// ClipDetail returns a clip with its event meta and ordered segments. Returns
// pgx.ErrNoRows if the clip does not exist.
func (s *Store) ClipDetail(ctx context.Context, clipID int64) (*ClipDetail, error) {
	var d ClipDetail
	var eventID *int64
	var thumbRel *string
	var ev ClipEvent
	var evTS *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.car_id, c.clip_ts, c.kind, c.lat, c.lon, c.has_sei, c.duration_s,
		       c.event_id, e.event_ts, e.reason, e.city, e.camera, e.thumb_rel_path,
		       (SELECT id FROM clips WHERE car_id = c.car_id
		        AND (clip_ts, id) < (c.clip_ts, c.id)
		        ORDER BY clip_ts DESC, id DESC LIMIT 1),
		       (SELECT id FROM clips WHERE car_id = c.car_id
		        AND (clip_ts, id) > (c.clip_ts, c.id)
		        ORDER BY clip_ts ASC, id ASC LIMIT 1)
		FROM clips c
		LEFT JOIN events e ON e.id = c.event_id
		WHERE c.id = $1`,
		clipID,
	).Scan(&d.ID, &d.CarID, &d.ClipTS, &d.Kind, &d.Lat, &d.Lon, &d.HasSEI, &d.DurationS,
		&eventID, &evTS, &ev.Reason, &ev.City, &ev.Camera, &thumbRel,
		&d.PrevClipID, &d.NextClipID)
	if err != nil {
		return nil, err
	}
	d.DownloadURL = clipZipURL(d.ID)
	if eventID != nil {
		if evTS != nil {
			ev.EventTS = *evTS
		}
		d.Event = &ev
		if thumbRel != nil {
			u := thumbURL(*eventID)
			d.ThumbURL = &u
		}
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, camera, has_sei, size_bytes
		FROM segments WHERE clip_id = $1
		ORDER BY %s, camera`, cameraRankCase), clipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	d.Segments = []ClipSegment{}
	for rows.Next() {
		var seg ClipSegment
		if err := rows.Scan(&seg.SegmentID, &seg.Camera, &seg.HasSEI, &seg.SizeBytes); err != nil {
			return nil, err
		}
		seg.MediaURL = mediaURL(seg.SegmentID)
		seg.DownloadURL = downloadURL(seg.SegmentID)
		d.Segments = append(d.Segments, seg)
	}
	return &d, rows.Err()
}

// SegmentRelPath returns the root-relative file path for a segment, or
// pgx.ErrNoRows.
func (s *Store) SegmentRelPath(ctx context.Context, segmentID int64) (string, error) {
	var rel string
	err := s.pool.QueryRow(ctx, `SELECT rel_path FROM segments WHERE id = $1`, segmentID).Scan(&rel)
	return rel, err
}

// ClipZip returns the clip timestamp and the ordered segment paths for a zip
// download. Returns pgx.ErrNoRows if the clip has no segments (or doesn't exist).
func (s *Store) ClipZip(ctx context.Context, clipID int64) (*ClipZipInfo, error) {
	var info ClipZipInfo
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT c.clip_ts, s.rel_path
		FROM clips c
		JOIN segments s ON s.clip_id = c.id
		WHERE c.id = $1
		ORDER BY %s, s.camera`, strings.ReplaceAll(cameraRankCase, "camera", "s.camera")), clipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rel string
		if err := rows.Scan(&info.ClipTS, &rel); err != nil {
			return nil, err
		}
		info.RelPaths = append(info.RelPaths, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(info.RelPaths) == 0 {
		return nil, pgx.ErrNoRows
	}
	return &info, nil
}

// ClipTrack returns all telemetry points for a clip in ascending time order.
// Returns an empty slice (not an error) when the clip has no SEI data.
func (s *Store) ClipTrack(ctx context.Context, clipID int64) ([]ClipTrackPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ts, lat, lon, speed_mps, heading_deg, gear, autopilot,
		       blinker_left, blinker_right, brake_applied,
		       accelerator_pedal_position, steering_wheel_angle,
		       accel_x, accel_y, accel_z, sei_version
		FROM track_points WHERE clip_id = $1 ORDER BY ts ASC`,
		clipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pts := []ClipTrackPoint{}
	for rows.Next() {
		var p ClipTrackPoint
		if err := rows.Scan(&p.TS, &p.Lat, &p.Lon, &p.SpeedMPS, &p.HeadingDeg,
			&p.Gear, &p.Autopilot, &p.BlinkerLeft, &p.BlinkerRight, &p.BrakeApplied,
			&p.AcceleratorPedalPosition, &p.SteeringWheelAngle,
			&p.AccelX, &p.AccelY, &p.AccelZ, &p.SeiVersion); err != nil {
			return nil, err
		}
		pts = append(pts, p)
	}
	return pts, rows.Err()
}

// EventThumbRelPath returns the thumb path for an event, or pgx.ErrNoRows when
// the event does not exist or has no thumbnail.
func (s *Store) EventThumbRelPath(ctx context.Context, eventID int64) (string, error) {
	var rel *string
	err := s.pool.QueryRow(ctx, `SELECT thumb_rel_path FROM events WHERE id = $1`, eventID).Scan(&rel)
	if err != nil {
		return "", err
	}
	if rel == nil {
		return "", pgx.ErrNoRows
	}
	return *rel, nil
}

// Status returns per-car counts and the worker scan-state. liveWindow is the
// staleness threshold for the live flag (typically 2× ScanInterval).
func (s *Store) Status(ctx context.Context, carID int64, key string, liveWindow time.Duration) (*Status, error) {
	st := &Status{DBReachable: true, Car: CarStatus{CarID: carID, Key: key}}
	err := s.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM cars),
		       (SELECT count(*) FROM events WHERE car_id = $1),
		       (SELECT count(*) FROM clips  WHERE car_id = $1),
		       (SELECT count(*) FROM segments s JOIN clips c ON c.id = s.clip_id WHERE c.car_id = $1),
		       (SELECT count(*) FROM track_points WHERE car_id = $1)`,
		carID,
	).Scan(&st.Counts.Cars, &st.Counts.Events, &st.Counts.Clips, &st.Counts.Segments, &st.Counts.TrackPoints)
	if err != nil {
		return nil, err
	}

	err = s.pool.QueryRow(ctx, `
		SELECT last_scan_started_at, last_scan_completed_at,
		       clips_processed, clips_skipped, clips_errored, track_points_written
		FROM ingest_state WHERE car_id = $1`,
		carID,
	).Scan(&st.Car.LastScanStartedAt, &st.Car.LastScanCompletedAt,
		&st.Car.LastScan.Processed, &st.Car.LastScan.Skipped, &st.Car.LastScan.Errored, &st.Car.LastScan.TrackPoints)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	if st.Car.LastScanCompletedAt != nil {
		st.Car.Live = time.Since(*st.Car.LastScanCompletedAt) < liveWindow
	}
	return st, nil
}
