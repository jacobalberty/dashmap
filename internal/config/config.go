package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	VideoRoot      string
	DefaultFPS     float64
	ScanInterval   time.Duration
	LogLevel       string
	LogFormat      string
	AutoMigrate    bool
}

func Load() (*Config, error) {
	c := &Config{
		HTTPAddr:     getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		VideoRoot:    os.Getenv("VIDEO_ROOT"),
		LogLevel:     getEnv("LOG_LEVEL", "info"),
		LogFormat:    getEnv("LOG_FORMAT", "json"),
		AutoMigrate:  getEnvBool("AUTO_MIGRATE", true),
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
