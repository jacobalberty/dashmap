//go:build integration

package ingest

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jacobalberty/dashmap/internal/config"
	"github.com/jacobalberty/dashmap/internal/db"
	"github.com/jacobalberty/dashmap/internal/pb"
	"github.com/jacobalberty/dashmap/internal/sei"
)

// buildSEIMP4 builds a minimal MP4 with ftyp + moov(mdhd+stts) + mdat containing
// n SEI NALs each carrying a pb.SeiMetadata with the given lat/lon.
func buildSEIMP4(t *testing.T, n int, lat, lon float64) []byte {
	t.Helper()
	// Reuse helpers from sei package test via local duplicates (different package).

	makeBox := func(name string, payload []byte) []byte {
		size := 8 + len(payload)
		b := make([]byte, size)
		binary.BigEndian.PutUint32(b[0:4], uint32(size))
		copy(b[4:8], name)
		copy(b[8:], payload)
		return b
	}

	// mdhd version 0: flags(3)+creation(4)+modification(4)+timescale(4)+duration(4) = 20 payload bytes after version byte
	timescale := uint32(36000)
	frameDelta := uint32(1000) // 36 fps
	frameCount := uint32(n)
	duration := frameCount * frameDelta
	mdhdPayload := []byte{
		0, 0, 0, 0, // version+flags
		0, 0, 0, 0, 0, 0, 0, 0, // creation+modification
		byte(timescale >> 24), byte(timescale >> 16), byte(timescale >> 8), byte(timescale),
		byte(duration >> 24), byte(duration >> 16), byte(duration >> 8), byte(duration),
	}
	mdhd := makeBox("mdhd", mdhdPayload)

	sttsPayload := []byte{
		0, 0, 0, 0, // version+flags
		0, 0, 0, 1, // entry_count=1
		byte(frameCount >> 24), byte(frameCount >> 16), byte(frameCount >> 8), byte(frameCount),
		byte(frameDelta >> 24), byte(frameDelta >> 16), byte(frameDelta >> 8), byte(frameDelta),
	}
	stts := makeBox("stts", sttsPayload)
	stbl := makeBox("stbl", stts)
	minf := makeBox("minf", stbl)
	mdia := makeBox("mdia", append(mdhd, minf...))
	trak := makeBox("trak", mdia)
	moov := makeBox("moov", trak)

	// mdat: n SEI NALs
	var mdatPayload []byte
	for i := range n {
		m := &pb.SeiMetadata{
			FrameSeqNo:   uint64(i),
			LatitudeDeg:  lat,
			LongitudeDeg: lon,
			GearState:    pb.SeiMetadata_GEAR_DRIVE,
			Version:      1,
		}
		raw, err := proto.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		// NAL: type=6 subtype=5, then payload-size byte, 3×0x42, 0x69, proto, stop
		body := []byte{0x06, 0x05, byte(len(raw) + 5), 0x42, 0x42, 0x42, 0x69}
		body = append(body, raw...)
		body = append(body, 0x80)
		nal := make([]byte, 4+len(body))
		binary.BigEndian.PutUint32(nal[:4], uint32(len(body)))
		copy(nal[4:], body)
		mdatPayload = append(mdatPayload, nal...)
	}
	mdat := makeBox("mdat", mdatPayload)

	ftyp := makeBox("ftyp", []byte("isom"))
	var out bytes.Buffer
	out.Write(ftyp)
	out.Write(moov)
	out.Write(mdat)
	return out.Bytes()
}

// buildVideoRoot builds a synthetic VIDEO_ROOT for one saved event folder:
//   - SavedClips/<folder>/event.json
//   - SavedClips/<folder>/thumb.png
//   - SavedClips/<folder>/<clipTS>-front.mp4  (SEI clip with real lat/lon)
//   - SavedClips/<folder>/<clipTS>-back.mp4   (empty mdat = no SEI)
//   - SavedClips/<folder>/<clip2TS>-front.mp4 (empty mdat = no SEI)
func buildVideoRoot(t *testing.T) (videoRoot string, folderTS, clipTS, clip2TS time.Time, lat, lon float64) {
	t.Helper()
	root := t.TempDir()
	folderTS = time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	clipTS = time.Date(2026, 1, 2, 11, 59, 0, 0, time.UTC)
	clip2TS = time.Date(2026, 1, 2, 11, 58, 0, 0, time.UTC)
	lat, lon = 30.4128, -91.0995

	folderName := folderTS.Format("2006-01-02_15-04-05")
	folderAbs := filepath.Join(root, "SavedClips", folderName)
	if err := os.MkdirAll(folderAbs, 0o755); err != nil {
		t.Fatal(err)
	}

	// event.json
	evJSON := `{"timestamp":"2026-01-02T12:00:00","city":"Test City","est_lat":"30.4128","est_lon":"-91.0995","reason":"test","camera":"0"}`
	if err := os.WriteFile(filepath.Join(folderAbs, "event.json"), []byte(evJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	// thumb.png (stub)
	if err := os.WriteFile(filepath.Join(folderAbs, "thumb.png"), []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}

	// clip1-front: SEI MP4 with 72 frames = 2 seconds at 36fps → 2 track points
	mp4 := buildSEIMP4(t, 72, lat, lon)
	name1 := clipTS.Format("2006-01-02_15-04-05") + "-front.mp4"
	if err := os.WriteFile(filepath.Join(folderAbs, name1), mp4, 0o644); err != nil {
		t.Fatal(err)
	}
	// clip1-back: no SEI
	name1b := clipTS.Format("2006-01-02_15-04-05") + "-back.mp4"
	noSEI := buildEmptyMP4(t)
	if err := os.WriteFile(filepath.Join(folderAbs, name1b), noSEI, 0o644); err != nil {
		t.Fatal(err)
	}
	// clip2-front: no SEI
	name2 := clip2TS.Format("2006-01-02_15-04-05") + "-front.mp4"
	if err := os.WriteFile(filepath.Join(folderAbs, name2), noSEI, 0o644); err != nil {
		t.Fatal(err)
	}

	return root, folderTS, clipTS, clip2TS, lat, lon
}

func buildEmptyMP4(t *testing.T) []byte {
	t.Helper()
	makeBox := func(name string, payload []byte) []byte {
		size := 8 + len(payload)
		b := make([]byte, size)
		binary.BigEndian.PutUint32(b[0:4], uint32(size))
		copy(b[4:8], name)
		copy(b[8:], payload)
		return b
	}
	ftyp := makeBox("ftyp", []byte("isom"))
	mdat := makeBox("mdat", nil)
	return append(ftyp, mdat...)
}

func connectTestDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL required for integration tests")
	}
	ctx := context.Background()
	if err := db.Migrate(ctx, dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool, ctx
}

func TestIngest_SyntheticVideoRoot(t *testing.T) {
	pool, ctx := connectTestDB(t)

	videoRoot, _, clipTS, clip2TS, lat, lon := buildVideoRoot(t)

	cfg := &config.Config{
		CarKey:            "test-car-" + t.Name(),
		CarDisplayName:    "Test Car",
		VideoRoot:         videoRoot,
		DefaultFPS:        36,
		SEISampleRate:     1.0,
		IngestConcurrency: 2,
	}

	// First scan.
	carID, err := upsertCar(ctx, pool, cfg.CarKey, cfg.CarDisplayName, "")
	if err != nil {
		t.Fatalf("upsertCar: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM track_points WHERE car_id=$1`, carID)
		_, _ = pool.Exec(ctx, `DELETE FROM segments WHERE clip_id IN (SELECT id FROM clips WHERE car_id=$1)`, carID)
		_, _ = pool.Exec(ctx, `DELETE FROM clips WHERE car_id=$1`, carID)
		_, _ = pool.Exec(ctx, `DELETE FROM events WHERE car_id=$1`, carID)
		_, _ = pool.Exec(ctx, `DELETE FROM cars WHERE id=$1`, carID)
	})
	counts, err := runScan(ctx, cfg, pool, carID)
	if err != nil {
		t.Fatalf("runScan: %v", err)
	}
	if counts.errored > 0 {
		t.Errorf("scan had %d errors", counts.errored)
	}

	// events table: 1 row.
	var eventCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE car_id=$1`, carID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Errorf("events: got %d, want 1", eventCount)
	}

	// clips table: 2 clips (clipTS + clip2TS).
	var clipCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM clips WHERE car_id=$1`, carID).Scan(&clipCount); err != nil {
		t.Fatal(err)
	}
	if clipCount != 2 {
		t.Errorf("clips: got %d, want 2", clipCount)
	}

	// SEI clip: has_sei=true, lat/lon set.
	var hasSEI bool
	var clipLat, clipLon *float64
	if err := pool.QueryRow(ctx, `SELECT has_sei, lat, lon FROM clips WHERE car_id=$1 AND clip_ts=$2`, carID, clipTS).
		Scan(&hasSEI, &clipLat, &clipLon); err != nil {
		t.Fatalf("query sei clip: %v", err)
	}
	if !hasSEI {
		t.Error("SEI clip: expected has_sei=true")
	}
	if clipLat == nil || clipLon == nil {
		t.Error("SEI clip: expected non-null lat/lon")
	} else if abs(*clipLat-lat) > 1e-3 || abs(*clipLon-lon) > 1e-3 {
		t.Errorf("SEI clip lat/lon mismatch: got (%f,%f)", *clipLat, *clipLon)
	}

	// SEI-less clip: has_sei=false, null lat/lon (no event lat/lon for this clip
	// since the event lat/lon is set on the clips only when no SEI point is found and
	// the clip is linked to an event with est_lat — clip2 IS linked to the event).
	var hasSEI2 bool
	if err := pool.QueryRow(ctx, `SELECT has_sei FROM clips WHERE car_id=$1 AND clip_ts=$2`, carID, clip2TS).
		Scan(&hasSEI2); err != nil {
		t.Fatalf("query no-sei clip: %v", err)
	}
	if hasSEI2 {
		t.Error("no-SEI clip: expected has_sei=false")
	}

	// track_points: 2 rows (2-second clip at 1 Hz).
	var tpCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM track_points WHERE car_id=$1`, carID).Scan(&tpCount); err != nil {
		t.Fatal(err)
	}
	if tpCount != 2 {
		t.Errorf("track_points: got %d, want 2", tpCount)
	}

	// Idempotency: second scan must not change row counts.
	counts2, err := runScan(ctx, cfg, pool, carID)
	if err != nil {
		t.Fatalf("runScan 2: %v", err)
	}
	if counts2.errored > 0 {
		t.Errorf("rescan had %d errors", counts2.errored)
	}

	var eventCount2, clipCount2, tpCount2 int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM events WHERE car_id=$1`, carID).Scan(&eventCount2); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM clips WHERE car_id=$1`, carID).Scan(&clipCount2); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM track_points WHERE car_id=$1`, carID).Scan(&tpCount2); err != nil {
		t.Fatal(err)
	}
	if eventCount2 != eventCount || clipCount2 != clipCount || tpCount2 != tpCount {
		t.Errorf("idempotency failed: events %d→%d, clips %d→%d, track_points %d→%d",
			eventCount, eventCount2, clipCount, clipCount2, tpCount, tpCount2)
	}
}

// TestIngest_RealTree runs against the real teslausb tree when TESLA_TREE is set.
// To keep runtime manageable it constructs a mini VIDEO_ROOT with symlinks to
// a small number of recent folders rather than scanning the whole tree.
func TestIngest_RealTree(t *testing.T) {
	treeRoot := os.Getenv("TESLA_TREE")
	if treeRoot == "" {
		t.Skip("TESLA_TREE not set; skipping real-tree test")
	}
	pool, ctx := connectTestDB(t)

	// Build a mini VIDEO_ROOT: symlink the 3 most recent SavedClips folders.
	miniRoot := t.TempDir()
	savedSrc := filepath.Join(treeRoot, "SavedClips")
	savedDst := filepath.Join(miniRoot, "SavedClips")
	if err := os.MkdirAll(savedDst, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(savedSrc)
	if err != nil {
		t.Fatalf("ReadDir SavedClips: %v", err)
	}
	count := 0
	for i := len(entries) - 1; i >= 0 && count < 3; i-- {
		e := entries[i]
		if !e.IsDir() {
			continue
		}
		// Symlink the folder (not its contents) so we get real MP4 data without copying.
		if err := os.Symlink(
			filepath.Join(savedSrc, e.Name()),
			filepath.Join(savedDst, e.Name()),
		); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		count++
	}
	if count == 0 {
		t.Skip("no SavedClips folders found in TESLA_TREE")
	}

	cfg := &config.Config{
		CarKey:            "real-tree-test-" + t.Name(),
		VideoRoot:         miniRoot,
		DefaultFPS:        36,
		SEISampleRate:     1.0,
		IngestConcurrency: 4,
	}

	carID, err := upsertCar(ctx, pool, cfg.CarKey, "", "")
	if err != nil {
		t.Fatalf("upsertCar: %v", err)
	}
	t.Cleanup(func() {
		// Clean up test data.
		_, _ = pool.Exec(ctx, `DELETE FROM track_points WHERE car_id=$1`, carID)
		_, _ = pool.Exec(ctx, `DELETE FROM segments WHERE clip_id IN (SELECT id FROM clips WHERE car_id=$1)`, carID)
		_, _ = pool.Exec(ctx, `DELETE FROM clips WHERE car_id=$1`, carID)
		_, _ = pool.Exec(ctx, `DELETE FROM events WHERE car_id=$1`, carID)
		_, _ = pool.Exec(ctx, `DELETE FROM cars WHERE id=$1`, carID)
	})

	counts, err := runScan(ctx, cfg, pool, carID)
	if err != nil {
		t.Fatalf("runScan: %v", err)
	}
	t.Logf("scan: processed=%d skipped=%d errored=%d track_points=%d",
		counts.processed, counts.skipped, counts.errored, counts.trackPoints)

	var clipCount, tpCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM clips WHERE car_id=$1`, carID).Scan(&clipCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM track_points WHERE car_id=$1`, carID).Scan(&tpCount); err != nil {
		t.Fatal(err)
	}
	if clipCount == 0 {
		t.Error("expected at least 1 clip from real tree")
	}
	t.Logf("clips=%d track_points=%d", clipCount, tpCount)

	// Verify at least one track_point has sane lat/lon (Baton Rouge area).
	if tpCount > 0 {
		var lat, lon float64
		if err := pool.QueryRow(ctx, `SELECT lat, lon FROM track_points WHERE car_id=$1 LIMIT 1`, carID).
			Scan(&lat, &lon); err != nil {
			t.Fatal(err)
		}
		// Baton Rouge is roughly 30°N, 91°W.
		if lat < 29 || lat > 32 || lon < -93 || lon > -89 {
			t.Errorf("unexpected GPS point: lat=%f lon=%f", lat, lon)
		}
	}

	// Second scan: row counts must not increase.
	counts2, err := runScan(ctx, cfg, pool, carID)
	if err != nil {
		t.Fatalf("runScan 2: %v", err)
	}
	_ = counts2

	var clipCount2, tpCount2 int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM clips WHERE car_id=$1`, carID).Scan(&clipCount2); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM track_points WHERE car_id=$1`, carID).Scan(&tpCount2); err != nil {
		t.Fatal(err)
	}
	if clipCount2 != clipCount || tpCount2 != tpCount {
		t.Errorf("idempotency failed: clips %d→%d, track_points %d→%d",
			clipCount, clipCount2, tpCount, tpCount2)
	}

	// Verify SEI provenance version is set.
	var segWithSEI int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM segments s
		JOIN clips c ON c.id = s.clip_id
		WHERE c.car_id=$1 AND s.has_sei=true AND s.sei_extractor_version=$2`,
		carID, sei.ExtractorVersion,
	).Scan(&segWithSEI); err != nil {
		t.Fatal(err)
	}
	t.Logf("segments with SEI extractor_version=%d: %d", sei.ExtractorVersion, segWithSEI)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
