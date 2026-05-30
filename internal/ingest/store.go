package ingest

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// segmentState holds the DB-persisted state used for idempotency decisions.
type segmentState struct {
	id                   int64
	sizBytes             *int64
	fileMtime            *time.Time
	seiRateHz            *float32
	seiExtractorVersion  *int32
	hasSEI               bool
}

// upsertCar upserts a car row by key and returns its id.
func upsertCar(ctx context.Context, pool *pgxpool.Pool, key, displayName, hwGen string) (int64, error) {
	var id int64
	err := pool.QueryRow(ctx, `
		INSERT INTO cars (key, display_name, hw_generation)
		VALUES ($1, NULLIF($2,''), NULLIF($3,''))
		ON CONFLICT (key) DO UPDATE
		  SET display_name  = EXCLUDED.display_name,
		      hw_generation = EXCLUDED.hw_generation
		RETURNING id`,
		key, displayName, hwGen,
	).Scan(&id)
	return id, err
}

// updateCarHW updates only the hw_generation when it has changed.
func updateCarHW(ctx context.Context, pool *pgxpool.Pool, carID int64, hwGen string) error {
	_, err := pool.Exec(ctx, `
		UPDATE cars SET hw_generation = $2
		WHERE id = $1 AND (hw_generation IS DISTINCT FROM $2)`,
		carID, hwGen,
	)
	return err
}

// upsertEvent upserts an event row and returns its id.
func upsertEvent(ctx context.Context, tx pgx.Tx, carID int64, folderRelPath string, ev *parsedEvent, folderTS time.Time, thumbRelPath *string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO events (car_id, event_ts, folder_ts, kind, city, est_lat, est_lon, reason, camera, folder_rel_path, thumb_rel_path, raw)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7, NULLIF($8,''), NULLIF($9,''), $10, $11, $12::jsonb)
		ON CONFLICT (folder_rel_path) DO UPDATE
		  SET event_ts       = EXCLUDED.event_ts,
		      city           = EXCLUDED.city,
		      est_lat        = EXCLUDED.est_lat,
		      est_lon        = EXCLUDED.est_lon,
		      reason         = EXCLUDED.reason,
		      camera         = EXCLUDED.camera,
		      thumb_rel_path = EXCLUDED.thumb_rel_path,
		      raw            = EXCLUDED.raw
		RETURNING id`,
		carID,
		ev.EventTS,
		folderTS,
		string(KindSaved), // kind overridden by caller when inserting sentry
		ev.City,
		ev.EstLat,
		ev.EstLon,
		ev.Reason,
		ev.Camera,
		folderRelPath,
		thumbRelPath,
		string(ev.Raw),
	).Scan(&id)
	return id, err
}

// upsertEventWithKind is like upsertEvent but accepts an explicit kind.
func upsertEventWithKind(ctx context.Context, tx pgx.Tx, carID int64, kind ClipKind, folderRelPath string, ev *parsedEvent, folderTS time.Time, thumbRelPath *string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO events (car_id, event_ts, folder_ts, kind, city, est_lat, est_lon, reason, camera, folder_rel_path, thumb_rel_path, raw)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7, NULLIF($8,''), NULLIF($9,''), $10, $11, $12::jsonb)
		ON CONFLICT (folder_rel_path) DO UPDATE
		  SET event_ts       = EXCLUDED.event_ts,
		      city           = EXCLUDED.city,
		      est_lat        = EXCLUDED.est_lat,
		      est_lon        = EXCLUDED.est_lon,
		      reason         = EXCLUDED.reason,
		      camera         = EXCLUDED.camera,
		      thumb_rel_path = EXCLUDED.thumb_rel_path,
		      raw            = EXCLUDED.raw
		RETURNING id`,
		carID,
		ev.EventTS,
		folderTS,
		string(kind),
		ev.City,
		ev.EstLat,
		ev.EstLon,
		ev.Reason,
		ev.Camera,
		folderRelPath,
		thumbRelPath,
		string(ev.Raw),
	).Scan(&id)
	return id, err
}

// upsertClip upserts a clip row and returns its id.
func upsertClip(ctx context.Context, tx pgx.Tx, carID int64, eventID *int64, kind ClipKind, clipTS time.Time) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO clips (car_id, event_id, kind, clip_ts)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (car_id, kind, event_id, clip_ts) DO UPDATE
		  SET car_id = EXCLUDED.car_id
		RETURNING id`,
		carID, eventID, string(kind), clipTS,
	).Scan(&id)
	return id, err
}

// upsertSegment upserts a segment row and returns its current state (for
// idempotency checks).
func upsertSegment(ctx context.Context, tx pgx.Tx, clipID int64, camera, relPath string, size int64, mtime time.Time) (segmentState, error) {
	var st segmentState
	err := tx.QueryRow(ctx, `
		INSERT INTO segments (clip_id, camera, rel_path, size_bytes, file_mtime)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (rel_path) DO UPDATE
		  SET size_bytes = EXCLUDED.size_bytes,
		      file_mtime = EXCLUDED.file_mtime
		RETURNING id, size_bytes, file_mtime, sei_rate_hz, sei_extractor_version, has_sei`,
		clipID, camera, relPath, size, mtime,
	).Scan(&st.id, &st.sizBytes, &st.fileMtime, &st.seiRateHz, &st.seiExtractorVersion, &st.hasSEI)
	return st, err
}

// updateSegmentSEI records SEI extraction provenance and has_sei on a segment.
func updateSegmentSEI(ctx context.Context, tx pgx.Tx, segID int64, hasSEI bool, rateHz float32, version int32) error {
	_, err := tx.Exec(ctx, `
		UPDATE segments
		SET has_sei               = $2,
		    sei_rate_hz           = $3,
		    sei_extractor_version = $4,
		    sei_processed_at      = now()
		WHERE id = $1`,
		segID, hasSEI, rateHz, version,
	)
	return err
}

// updateSegmentHasSEI records only the has_sei flag (for non-representative cameras).
func updateSegmentHasSEI(ctx context.Context, tx pgx.Tx, segID int64, hasSEI bool) error {
	_, err := tx.Exec(ctx, `
		UPDATE segments SET has_sei = $2 WHERE id = $1`,
		segID, hasSEI,
	)
	return err
}

// updateClipDerived sets the derived lat/lon/has_sei/duration_s on a clip.
func updateClipDerived(ctx context.Context, tx pgx.Tx, clipID int64, lat, lon *float64, hasSEI bool, durationS *float32) error {
	_, err := tx.Exec(ctx, `
		UPDATE clips
		SET lat        = $2,
		    lon        = $3,
		    has_sei    = $4,
		    duration_s = $5
		WHERE id = $1`,
		clipID, lat, lon, hasSEI, durationS,
	)
	return err
}

// replaceTrackPoints deletes all track_points for a segment and batch-inserts
// the new ones using pgx CopyFrom.
func replaceTrackPoints(ctx context.Context, tx pgx.Tx, carID, clipID, segID int64, rows []trackPointRow) error {
	if _, err := tx.Exec(ctx, `DELETE FROM track_points WHERE segment_id = $1`, segID); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	cols := []string{
		"car_id", "clip_id", "segment_id", "ts",
		"lat", "lon", "speed_mps", "heading_deg",
		"gear", "autopilot",
		"blinker_left", "blinker_right", "brake_applied",
		"accelerator_pedal_position", "steering_wheel_angle",
		"accel_x", "accel_y", "accel_z", "sei_version",
	}
	source := pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
		r := rows[i]
		return []any{
			carID, clipID, segID, r.ts,
			r.lat, r.lon, r.speedMPS, r.headingDeg,
			r.gear, r.autopilot,
			r.blinkerLeft, r.blinkerRight, r.brakeApplied,
			r.acceleratorPedalPos, r.steeringWheelAngle,
			r.accelX, r.accelY, r.accelZ, r.seiVersion,
		}, nil
	})
	_, err := tx.CopyFrom(ctx, pgx.Identifier{"track_points"}, cols, source)
	return err
}

// trackPointRow holds one downsampled SEI sample ready for DB insertion.
type trackPointRow struct {
	ts                  time.Time
	lat                 float64
	lon                 float64
	speedMPS            float32
	headingDeg          float64
	gear                int16
	autopilot           int16
	blinkerLeft         bool
	blinkerRight        bool
	brakeApplied        bool
	acceleratorPedalPos float32
	steeringWheelAngle  float32
	accelX              float64
	accelY              float64
	accelZ              float64
	seiVersion          int32
}
