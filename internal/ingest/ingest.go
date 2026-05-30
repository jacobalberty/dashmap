package ingest

import (
	"context"
	"log/slog"
	"time"
)

// Run ticks on interval, logging scan events. Real ingest logic is implemented in Story 03.
func Run(ctx context.Context, videoRoot string, interval time.Duration) {
	slog.Info("ingest worker started", "video_root", videoRoot, "interval", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("ingest worker stopped")
			return
		case t := <-ticker.C:
			slog.Info("scan tick", "at", t.Format(time.RFC3339))
		}
	}
}
