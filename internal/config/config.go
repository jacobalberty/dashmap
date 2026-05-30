package config

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr           string
	DatabaseURL        string
	VideoRoot          string
	DefaultFPS         float64
	ScanInterval       time.Duration
	LogLevel           string
	LogFormat          string
	AutoMigrate        bool
	CarKey             string
	CarDisplayName     string
	SEISampleRate      float64
	IngestConcurrency  int
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
