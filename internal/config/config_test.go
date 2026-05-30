package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Clear relevant env vars so defaults apply.
	for _, k := range []string{"HTTP_ADDR", "DATABASE_URL", "VIDEO_ROOT", "DEFAULT_FPS", "SCAN_INTERVAL", "LOG_LEVEL", "LOG_FORMAT", "AUTO_MIGRATE"} {
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
