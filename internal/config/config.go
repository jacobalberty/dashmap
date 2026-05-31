package config

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr          string
	DatabaseURL       string
	VideoRoot         string
	DefaultFPS        float64
	ScanInterval      time.Duration
	LogLevel          string
	LogFormat         string
	AutoMigrate       bool
	CarKey            string
	CarDisplayName    string
	SEISampleRate     float64
	IngestConcurrency int

	// API (Story 04)
	ZipCompression    string        // "store" | "deflate"
	TrackGap          time.Duration // gap that starts a new track run
	TrackSimplifyM    float64       // Douglas–Peucker tolerance, metres
	TrackMaxPoints    int           // hard cap on /api/track points
	ClipMarkersMax    int           // hard cap on /api/clip-markers rows
	ClipsDefaultLimit int           // default /api/clips page size
	ClipsMaxLimit     int           // max /api/clips page size
}

func Load() (*Config, error) {
	c := &Config{
		HTTPAddr:       getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		VideoRoot:      os.Getenv("VIDEO_ROOT"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		LogFormat:      getEnv("LOG_FORMAT", "json"),
		AutoMigrate:    getEnvBool("AUTO_MIGRATE", true),
		CarKey:         getEnv("CAR_KEY", "default"),
		CarDisplayName: os.Getenv("CAR_DISPLAY_NAME"),
		ZipCompression: getEnv("ZIP_COMPRESSION", "store"),
	}

	fps, err := strconv.ParseFloat(getEnv("DEFAULT_FPS", "36"), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid DEFAULT_FPS: %w", err)
	}
	c.DefaultFPS = fps

	interval, err := time.ParseDuration(getEnv("SCAN_INTERVAL", "5m"))
	if err != nil {
		return nil, fmt.Errorf("invalid SCAN_INTERVAL: %w", err)
	}
	c.ScanInterval = interval

	seiRate, err := strconv.ParseFloat(getEnv("SEI_SAMPLE_RATE", "1.0"), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid SEI_SAMPLE_RATE: %w", err)
	}
	c.SEISampleRate = seiRate

	concurrency, err := strconv.Atoi(getEnv("INGEST_CONCURRENCY", strconv.Itoa(runtime.NumCPU())))
	if err != nil || concurrency < 1 {
		return nil, fmt.Errorf("invalid INGEST_CONCURRENCY: %w", err)
	}
	c.IngestConcurrency = concurrency

	if c.ZipCompression != "store" && c.ZipCompression != "deflate" {
		return nil, fmt.Errorf("invalid ZIP_COMPRESSION %q: want store or deflate", c.ZipCompression)
	}

	trackGap, err := time.ParseDuration(getEnv("TRACK_GAP", "2m"))
	if err != nil {
		return nil, fmt.Errorf("invalid TRACK_GAP: %w", err)
	}
	c.TrackGap = trackGap

	simplify, err := strconv.ParseFloat(getEnv("TRACK_SIMPLIFY_M", "5"), 64)
	if err != nil || simplify < 0 {
		return nil, fmt.Errorf("invalid TRACK_SIMPLIFY_M: %w", err)
	}
	c.TrackSimplifyM = simplify

	c.TrackMaxPoints, err = getEnvPositiveInt("TRACK_MAX_POINTS", 10000)
	if err != nil {
		return nil, err
	}
	c.ClipMarkersMax, err = getEnvPositiveInt("CLIP_MARKERS_MAX", 10000)
	if err != nil {
		return nil, err
	}
	c.ClipsDefaultLimit, err = getEnvPositiveInt("CLIPS_DEFAULT_LIMIT", 500)
	if err != nil {
		return nil, err
	}
	c.ClipsMaxLimit, err = getEnvPositiveInt("CLIPS_MAX_LIMIT", 2000)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func getEnvPositiveInt(key string, def int) (int, error) {
	n, err := strconv.Atoi(getEnv(key, strconv.Itoa(def)))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid %s: want a positive integer", key)
	}
	return n, nil
}
