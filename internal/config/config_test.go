package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Clear relevant env vars so defaults apply.
	for _, k := range []string{
		"HTTP_ADDR", "DATABASE_URL", "VIDEO_ROOT", "DEFAULT_FPS", "SCAN_INTERVAL", "LOG_LEVEL", "LOG_FORMAT", "AUTO_MIGRATE",
		"ZIP_COMPRESSION", "TRACK_GAP", "TRACK_SIMPLIFY_M", "TRACK_MAX_POINTS", "CLIP_MARKERS_MAX", "CLIPS_DEFAULT_LIMIT", "CLIPS_MAX_LIMIT",
	} {
		_ = os.Unsetenv(k)
	}

	c, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if c.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", c.HTTPAddr)
	}
	if c.DefaultFPS != 36 {
		t.Errorf("DefaultFPS = %v, want 36", c.DefaultFPS)
	}
	if c.ScanInterval != 5*time.Minute {
		t.Errorf("ScanInterval = %v, want 5m", c.ScanInterval)
	}
	if c.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", c.LogLevel)
	}
	if c.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want json", c.LogFormat)
	}
	if !c.AutoMigrate {
		t.Error("AutoMigrate = false, want true")
	}
	if c.ZipCompression != "store" {
		t.Errorf("ZipCompression = %q, want store", c.ZipCompression)
	}
	if c.TrackGap != 2*time.Minute {
		t.Errorf("TrackGap = %v, want 2m", c.TrackGap)
	}
	if c.TrackSimplifyM != 5 {
		t.Errorf("TrackSimplifyM = %v, want 5", c.TrackSimplifyM)
	}
	if c.TrackMaxPoints != 10000 {
		t.Errorf("TrackMaxPoints = %d, want 10000", c.TrackMaxPoints)
	}
	if c.ClipMarkersMax != 10000 {
		t.Errorf("ClipMarkersMax = %d, want 10000", c.ClipMarkersMax)
	}
	if c.ClipsDefaultLimit != 500 {
		t.Errorf("ClipsDefaultLimit = %d, want 500", c.ClipsDefaultLimit)
	}
	if c.ClipsMaxLimit != 2000 {
		t.Errorf("ClipsMaxLimit = %d, want 2000", c.ClipsMaxLimit)
	}
}

func TestLoadAPIEnvOverride(t *testing.T) {
	t.Setenv("ZIP_COMPRESSION", "deflate")
	t.Setenv("TRACK_GAP", "90s")
	t.Setenv("TRACK_SIMPLIFY_M", "10")
	t.Setenv("TRACK_MAX_POINTS", "5000")
	t.Setenv("CLIPS_DEFAULT_LIMIT", "100")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if c.ZipCompression != "deflate" {
		t.Errorf("ZipCompression = %q, want deflate", c.ZipCompression)
	}
	if c.TrackGap != 90*time.Second {
		t.Errorf("TrackGap = %v, want 90s", c.TrackGap)
	}
	if c.TrackSimplifyM != 10 {
		t.Errorf("TrackSimplifyM = %v, want 10", c.TrackSimplifyM)
	}
	if c.TrackMaxPoints != 5000 {
		t.Errorf("TrackMaxPoints = %d, want 5000", c.TrackMaxPoints)
	}
	if c.ClipsDefaultLimit != 100 {
		t.Errorf("ClipsDefaultLimit = %d, want 100", c.ClipsDefaultLimit)
	}
}

func TestLoadInvalidZipCompression(t *testing.T) {
	t.Setenv("ZIP_COMPRESSION", "gzip")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error for invalid ZIP_COMPRESSION")
	}
}

func TestLoadInvalidTrackMaxPoints(t *testing.T) {
	t.Setenv("TRACK_MAX_POINTS", "0")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error for non-positive TRACK_MAX_POINTS")
	}
}

func TestLoadEnvOverride(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("DEFAULT_FPS", "30")
	t.Setenv("SCAN_INTERVAL", "10m")
	t.Setenv("AUTO_MIGRATE", "false")
	t.Setenv("LOG_FORMAT", "text")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if c.HTTPAddr != ":9090" {
		t.Errorf("HTTPAddr = %q, want :9090", c.HTTPAddr)
	}
	if c.DefaultFPS != 30 {
		t.Errorf("DefaultFPS = %v, want 30", c.DefaultFPS)
	}
	if c.ScanInterval != 10*time.Minute {
		t.Errorf("ScanInterval = %v, want 10m", c.ScanInterval)
	}
	if c.AutoMigrate {
		t.Error("AutoMigrate = true, want false")
	}
	if c.LogFormat != "text" {
		t.Errorf("LogFormat = %q, want text", c.LogFormat)
	}
}
