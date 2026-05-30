package ingest

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jacobalberty/dashmap/internal/pb"
)

// ---------------------------------------------------------------------------
// parseSegment
// ---------------------------------------------------------------------------

func TestParseSegment_Valid(t *testing.T) {
	cases := []struct {
		name   string
		wantTs string
		wantCam string
	}{
		{"2026-05-29_09-49-34-front.mp4", "2026-05-29T09:49:34Z", "front"},
		{"2025-01-03_13-08-16-back.mp4", "2025-01-03T13:08:16Z", "back"},
		{"2026-01-01_00-00-00-left_repeater.mp4", "2026-01-01T00:00:00Z", "left_repeater"},
		{"2026-01-01_00-00-00-right_repeater.mp4", "2026-01-01T00:00:00Z", "right_repeater"},
		{"2026-01-01_00-00-00-left_pillar.mp4", "2026-01-01T00:00:00Z", "left_pillar"},
		{"2026-01-01_00-00-00-right_pillar.mp4", "2026-01-01T00:00:00Z", "right_pillar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts, cam, ok := parseSegment(tc.name)
			if !ok {
				t.Fatalf("parseSegment(%q): ok=false", tc.name)
			}
			if cam != tc.wantCam {
				t.Errorf("camera: got %q, want %q", cam, tc.wantCam)
			}
			if ts.UTC().Format(time.RFC3339) != tc.wantTs {
				t.Errorf("ts: got %q, want %q", ts.UTC().Format(time.RFC3339), tc.wantTs)
			}
			if ts.Location() != time.UTC {
				t.Errorf("ts not UTC: %v", ts.Location())
			}
		})
	}
}

func TestParseSegment_Rejected(t *testing.T) {
	rejects := []string{
		"event.mp4",
		"thumb.png",
		"event.json",
		"2026-05-29_09-49-34-unknown_cam.mp4",
		"2026-05-29_09-49-34.mp4",
		"notanmp4.txt",
		"",
	}
	for _, name := range rejects {
		_, _, ok := parseSegment(name)
		if ok {
			t.Errorf("parseSegment(%q) should return ok=false", name)
		}
	}
}

// ---------------------------------------------------------------------------
// groupClips
// ---------------------------------------------------------------------------

func TestGroupClips(t *testing.T) {
	ts1 := time.Date(2026, 5, 29, 9, 49, 34, 0, time.UTC)
	ts2 := time.Date(2026, 5, 29, 9, 50, 34, 0, time.UTC)

	segs := []segment{
		{ts: ts1, camera: "front"},
		{ts: ts1, camera: "back"},
		{ts: ts2, camera: "front"},
		{ts: ts1, camera: "left_repeater"},
	}
	clips := groupClips(segs)
	if len(clips) != 2 {
		t.Fatalf("expected 2 clips, got %d", len(clips))
	}
	// Sorted by ascending ts.
	if !clips[0].ts.Equal(ts1) {
		t.Errorf("clip[0].ts = %v, want %v", clips[0].ts, ts1)
	}
	if len(clips[0].segments) != 3 {
		t.Errorf("clip[0] segments: got %d, want 3", len(clips[0].segments))
	}
	if !clips[1].ts.Equal(ts2) {
		t.Errorf("clip[1].ts = %v, want %v", clips[1].ts, ts2)
	}
}

// ---------------------------------------------------------------------------
// hwGeneration
// ---------------------------------------------------------------------------

func TestHwGeneration(t *testing.T) {
	cases := []struct {
		cams map[string]bool
		want string
	}{
		{map[string]bool{"front": true, "back": true, "left_repeater": true, "right_repeater": true}, "hw3"},
		{map[string]bool{"front": true, "left_pillar": true}, "hw4"},
		{map[string]bool{}, "unknown"},
	}
	for _, tc := range cases {
		got := hwGeneration(tc.cams)
		if got != tc.want {
			t.Errorf("hwGeneration(%v) = %q, want %q", tc.cams, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// parseEventJSON
// ---------------------------------------------------------------------------

func TestParseEventJSON_Full(t *testing.T) {
	data := []byte(`{
		"timestamp":"2026-05-29T09:50:37",
		"city":"Baton Rouge",
		"street":"Essen Ln",
		"est_lat":"30.4141",
		"est_lon":"-91.0989",
		"reason":"user_interaction_honk",
		"camera":"0"
	}`)
	ev, err := parseEventJSON(data)
	if err != nil {
		t.Fatalf("parseEventJSON: %v", err)
	}
	want := time.Date(2026, 5, 29, 9, 50, 37, 0, time.UTC)
	if !ev.EventTS.Equal(want) {
		t.Errorf("EventTS: got %v, want %v", ev.EventTS, want)
	}
	if ev.City != "Baton Rouge" {
		t.Errorf("City: got %q", ev.City)
	}
	if ev.EstLat == nil || math.Abs(*ev.EstLat-30.4141) > 1e-4 {
		t.Errorf("EstLat: got %v", ev.EstLat)
	}
	if ev.EstLon == nil || math.Abs(*ev.EstLon-(-91.0989)) > 1e-4 {
		t.Errorf("EstLon: got %v", ev.EstLon)
	}
	if ev.Reason != "user_interaction_honk" {
		t.Errorf("Reason: got %q", ev.Reason)
	}
	if string(ev.Raw) == "" {
		t.Error("Raw should be non-empty")
	}
}

func TestParseEventJSON_MissingLatLon(t *testing.T) {
	data := []byte(`{"timestamp":"2025-01-03T13:09:09","city":"Test","reason":"foo","camera":"0"}`)
	ev, err := parseEventJSON(data)
	if err != nil {
		t.Fatalf("parseEventJSON: %v", err)
	}
	if ev.EstLat != nil {
		t.Errorf("EstLat should be nil for missing field, got %v", *ev.EstLat)
	}
	if ev.EstLon != nil {
		t.Errorf("EstLon should be nil for missing field, got %v", *ev.EstLon)
	}
}

func TestParseEventJSON_UnknownField(t *testing.T) {
	// Extra fields like "street" should not cause an error.
	data := []byte(`{"timestamp":"2026-01-01T00:00:00","street":"Main St","est_lat":"1.0","est_lon":"2.0","reason":"x","camera":"0","city":""}`)
	_, err := parseEventJSON(data)
	if err != nil {
		t.Fatalf("parseEventJSON with extra field: %v", err)
	}
}

// ---------------------------------------------------------------------------
// detectRoot
// ---------------------------------------------------------------------------

func TestDetectRoot_DirectSubdirs(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "SavedClips"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := detectRoot(root)
	if got != root {
		t.Errorf("detectRoot = %q, want %q", got, root)
	}
}

func TestDetectRoot_TeslaCamParent(t *testing.T) {
	root := t.TempDir()
	teslaCam := filepath.Join(root, "TeslaCam")
	if err := os.MkdirAll(filepath.Join(teslaCam, "RecentClips"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := detectRoot(root)
	if got != teslaCam {
		t.Errorf("detectRoot = %q, want %q", got, teslaCam)
	}
}

func TestDetectRoot_NoSubdirs(t *testing.T) {
	root := t.TempDir()
	got := detectRoot(root)
	if got != root {
		t.Errorf("detectRoot = %q, want %q (fallback)", got, root)
	}
}

// ---------------------------------------------------------------------------
// downsample
// ---------------------------------------------------------------------------

func makeSamples(seqStart uint64, count int, lat, lon float64) []*pb.SeiMetadata {
	samples := make([]*pb.SeiMetadata, count)
	for i := range count {
		samples[i] = &pb.SeiMetadata{
			FrameSeqNo:   seqStart + uint64(i),
			LatitudeDeg:  lat,
			LongitudeDeg: lon,
			GearState:    pb.SeiMetadata_GEAR_DRIVE,
		}
	}
	return samples
}

func TestDownsample_1Hz_36fps(t *testing.T) {
	// 2160 samples at 36 fps = 60 seconds → expect ~60 rows at 1 Hz.
	clipTS := time.Date(2026, 5, 29, 9, 49, 34, 0, time.UTC)
	samples := makeSamples(0, 2160, 30.41, -91.09)
	rows := downsample(samples, clipTS, 36, 1.0)

	if len(rows) < 58 || len(rows) > 62 {
		t.Errorf("expected ~60 rows at 1Hz/36fps, got %d", len(rows))
	}
	// First row should be at (or very close to) clipTS.
	delta := rows[0].ts.Sub(clipTS).Abs()
	if delta > 100*time.Millisecond {
		t.Errorf("first row ts delta too large: %v", delta)
	}
	// Rows should be ascending.
	for i := 1; i < len(rows); i++ {
		if !rows[i].ts.After(rows[i-1].ts) {
			t.Errorf("rows not ascending at index %d", i)
		}
	}
}

func TestDownsample_SkipsZeroGPS(t *testing.T) {
	// First 36 samples have no GPS fix; next 36 have a fix.
	clipTS := time.Date(2026, 5, 29, 9, 49, 34, 0, time.UTC)
	var samples []*pb.SeiMetadata
	for range 36 {
		samples = append(samples, &pb.SeiMetadata{FrameSeqNo: uint64(len(samples)), LatitudeDeg: 0, LongitudeDeg: 0})
	}
	for range 36 {
		samples = append(samples, &pb.SeiMetadata{FrameSeqNo: uint64(len(samples)), LatitudeDeg: 30.41, LongitudeDeg: -91.09})
	}
	rows := downsample(samples, clipTS, 36, 1.0)
	// Second second bucket: should have exactly 1 row (the first fixed sample in it).
	// First bucket has no fix so is dropped entirely.
	if len(rows) != 1 {
		t.Errorf("expected 1 row (second bucket only), got %d", len(rows))
	}
}

func TestDownsample_GlobalMonotonicSeq(t *testing.T) {
	// frame_seq_no starts at 93654, not 0 — baseline subtraction must be applied.
	clipTS := time.Date(2026, 5, 29, 9, 49, 34, 0, time.UTC)
	samples := makeSamples(93654, 72, 30.41, -91.09) // 2 seconds at 36fps
	rows := downsample(samples, clipTS, 36, 1.0)
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
	// First row should be at clipTS + 0s.
	if rows[0].ts.Sub(clipTS) > time.Millisecond {
		t.Errorf("first row offset too large: %v", rows[0].ts.Sub(clipTS))
	}
}

func TestDownsample_Empty(t *testing.T) {
	clipTS := time.Now()
	rows := downsample(nil, clipTS, 36, 1.0)
	if len(rows) != 0 {
		t.Errorf("expected 0 rows for nil input, got %d", len(rows))
	}
}

func TestDownsample_FieldMapping(t *testing.T) {
	clipTS := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m := &pb.SeiMetadata{
		FrameSeqNo:              0,
		LatitudeDeg:             30.41,
		LongitudeDeg:            -91.09,
		VehicleSpeedMps:         11.16,
		HeadingDeg:              21.5,
		GearState:               pb.SeiMetadata_GEAR_DRIVE,
		AutopilotState:          pb.SeiMetadata_TACC,
		BlinkerOnLeft:           true,
		BlinkerOnRight:          false,
		BrakeApplied:            false,
		AcceleratorPedalPosition: 0.3,
		SteeringWheelAngle:      5.1,
		LinearAccelerationMps2X: 0.1,
		LinearAccelerationMps2Y: 0.2,
		LinearAccelerationMps2Z: 9.8,
		Version:                 1,
	}
	rows := downsample([]*pb.SeiMetadata{m}, clipTS, 36, 1.0)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.gear != int16(pb.SeiMetadata_GEAR_DRIVE.Number()) {
		t.Errorf("gear: got %d", r.gear)
	}
	if r.autopilot != int16(pb.SeiMetadata_TACC.Number()) {
		t.Errorf("autopilot: got %d", r.autopilot)
	}
	if !r.blinkerLeft {
		t.Error("blinkerLeft should be true")
	}
	if r.seiVersion != 1 {
		t.Errorf("seiVersion: got %d", r.seiVersion)
	}
}
