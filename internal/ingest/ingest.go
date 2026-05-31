package ingest

import (
	"context"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jacobalberty/dashmap/internal/config"
	"github.com/jacobalberty/dashmap/internal/pb"
	"github.com/jacobalberty/dashmap/internal/sei"
)

// Run upserts the car, performs an initial full scan, and then rescans on every
// cfg.ScanInterval tick until ctx is cancelled.
func Run(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool) {
	slog.Info("ingest worker started",
		"video_root", cfg.VideoRoot,
		"interval", cfg.ScanInterval,
		"concurrency", cfg.IngestConcurrency,
	)

	// Upsert car; hw_generation updated after each scan when camera set is known.
	carID, err := upsertCar(ctx, pool, cfg.CarKey, cfg.CarDisplayName, "")
	if err != nil {
		slog.Error("ingest: upsert car failed", "err", err)
		return
	}

	scan := func() {
		slog.Info("ingest: scan started")
		if err := markScanStarted(ctx, pool, carID); err != nil {
			slog.Warn("ingest: mark scan started failed", "err", err)
		}
		n, err := runScan(ctx, cfg, pool, carID)
		if err != nil {
			slog.Error("ingest: scan error", "err", err)
			return
		}
		if err := markScanCompleted(ctx, pool, carID, n); err != nil {
			slog.Warn("ingest: mark scan completed failed", "err", err)
		}
		slog.Info("ingest: scan complete",
			"clips_processed", n.processed,
			"clips_skipped", n.skipped,
			"clips_errored", n.errored,
			"track_points_written", n.trackPoints,
		)
	}

	scan() // initial

	ticker := time.NewTicker(cfg.ScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("ingest worker stopped")
			return
		case t := <-ticker.C:
			slog.Info("ingest: rescan tick", "at", t.Format(time.RFC3339))
			scan()
		}
	}
}

type scanCounts struct {
	processed   int64
	skipped     int64
	errored     int64
	trackPoints int64
}

func runScan(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, carID int64) (scanCounts, error) {
	root := detectRoot(cfg.VideoRoot)
	events, recentAbs, err := walkFolders(root, cfg.VideoRoot)
	if err != nil {
		return scanCounts{}, err
	}

	var counts scanCounts
	allCameras := map[string]bool{}
	var camMu sync.Mutex

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.IngestConcurrency)

	// Process event folders (SavedClips + SentryClips)
	for _, ef := range events {
		ef := ef
		g.Go(func() error {
			tp, cams, proc, err2 := processEventFolder(gctx, cfg, pool, carID, ef)
			if err2 != nil {
				slog.Warn("ingest: clip error", "folder", ef.relPath, "err", err2)
				atomic.AddInt64(&counts.errored, 1)
				return nil // log-and-continue
			}
			atomic.AddInt64(&counts.trackPoints, int64(tp))
			if proc {
				atomic.AddInt64(&counts.processed, 1)
			} else {
				atomic.AddInt64(&counts.skipped, 1)
			}
			camMu.Lock()
			for c := range cams {
				allCameras[c] = true
			}
			camMu.Unlock()
			return nil
		})
	}

	// Process RecentClips if present
	if recentAbs != "" {
		segs, cams, err2 := readSegments(recentAbs, cfg.VideoRoot)
		if err2 != nil {
			slog.Warn("ingest: RecentClips read error", "err", err2)
		} else {
			for c := range cams {
				allCameras[c] = true
			}
			for _, cl := range groupClips(segs) {
				cl := cl
				g.Go(func() error {
					tp, proc, err3 := processClip(gctx, cfg, pool, carID, KindRecent, nil, cl)
					if err3 != nil {
						slog.Warn("ingest: recent clip error", "clip_ts", cl.ts, "err", err3)
						atomic.AddInt64(&counts.errored, 1)
						return nil
					}
					atomic.AddInt64(&counts.trackPoints, int64(tp))
					if proc {
						atomic.AddInt64(&counts.processed, 1)
					} else {
						atomic.AddInt64(&counts.skipped, 1)
					}
					return nil
				})
			}
		}
	}

	if err := g.Wait(); err != nil {
		return counts, err
	}

	// Update hw_generation on the car row if the camera set is now known.
	if len(allCameras) > 0 {
		if err := updateCarHW(ctx, pool, carID, hwGeneration(allCameras)); err != nil {
			slog.Warn("ingest: updateCarHW error", "err", err)
		}
	}

	return counts, nil
}

// processEventFolder handles one SavedClips/SentryClips folder.
// It parses event.json (if present), upserts the event row, then processes each clip.
// processed is true if at least one clip ran SEI extraction (as opposed to being skipped).
func processEventFolder(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, carID int64, ef eventFolder) (trackPoints int, cameras map[string]bool, processed bool, _ error) {
	segs, cams, err := readSegments(ef.absPath, cfg.VideoRoot)
	if err != nil {
		return 0, nil, false, err
	}

	// Parse folder timestamp from the folder name.
	folderName := filepath.Base(ef.absPath)
	folderTS, err := time.Parse("2006-01-02_15-04-05", folderName)
	if err != nil {
		return 0, nil, false, err
	}
	folderTS = time.Date(folderTS.Year(), folderTS.Month(), folderTS.Day(),
		folderTS.Hour(), folderTS.Minute(), folderTS.Second(), 0, time.UTC)

	// Read event.json if present.
	var ev *parsedEvent
	evPath := filepath.Join(ef.absPath, "event.json")
	if data, err2 := os.ReadFile(evPath); err2 == nil {
		ev, err = parseEventJSON(data)
		if err != nil {
			slog.Warn("ingest: bad event.json", "path", evPath, "err", err)
			ev = nil
		}
	}
	if ev == nil {
		// Synthesize a minimal event so we always have a row.
		ev = &parsedEvent{EventTS: folderTS, Raw: []byte("{}")}
	}

	// thumb.png
	var thumbRelPath *string
	thumbAbs := filepath.Join(ef.absPath, "thumb.png")
	if _, err2 := os.Stat(thumbAbs); err2 == nil {
		rel, _ := filepath.Rel(cfg.VideoRoot, thumbAbs)
		s := filepath.ToSlash(rel)
		thumbRelPath = &s
	}

	// We need a transaction just for the event upsert; clips get their own txs.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, nil, false, err
	}
	eventID, err := upsertEventWithKind(ctx, tx, carID, ef.kind, ef.relPath, ev, folderTS, thumbRelPath)
	if err != nil {
		_ = tx.Rollback(ctx)
		return 0, nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, false, err
	}

	clips := groupClips(segs)
	totalTP := 0
	proc := false
	for _, cl := range clips {
		tp, p, err2 := processClip(ctx, cfg, pool, carID, ef.kind, &eventID, cl)
		if err2 != nil {
			slog.Warn("ingest: clip error", "folder", ef.relPath, "clip_ts", cl.ts, "err", err2)
			continue
		}
		totalTP += tp
		if p {
			proc = true
		}
	}
	return totalTP, cams, proc, nil
}

// processClip processes one clip (all its camera segments) in a single transaction.
// Returns (track points written, was-processed, error).
// "processed" means SEI extraction ran; false means the clip was skipped (idempotent).
func processClip(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, carID int64, kind ClipKind, eventID *int64, cl clip) (int, bool, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	clipID, err := upsertClip(ctx, tx, carID, eventID, kind, cl.ts)
	if err != nil {
		return 0, false, err
	}

	// Upsert all segments and gather their persisted states.
	type segWithState struct {
		seg segment
		st  segmentState
	}
	var segStates []segWithState
	for _, s := range cl.segments {
		st, err2 := upsertSegment(ctx, tx, clipID, s.camera, s.relPath, s.size, s.mtime)
		if err2 != nil {
			return 0, false, err2
		}
		segStates = append(segStates, segWithState{seg: s, st: st})
	}

	// Determine the representative segment.
	repCamera := preferredCamera(cl.segments)
	var repSS *segWithState
	for i := range segStates {
		if segStates[i].seg.camera == repCamera {
			repSS = &segStates[i]
			break
		}
	}
	if repSS == nil && len(segStates) > 0 {
		repSS = &segStates[0]
	}

	// Idempotency check: skip extraction if nothing changed and version is current.
	if repSS != nil && canSkip(repSS.st, cfg.SEISampleRate) {
		_ = tx.Commit(ctx)
		return 0, false, nil
	}

	// Extract SEI from the representative camera file.
	var rows []trackPointRow
	var clipLat, clipLon *float64
	var clipDur *float32
	clipHasSEI := false

	if repSS != nil {
		f, err2 := os.Open(repSS.seg.absPath)
		if err2 != nil {
			return 0, false, err2
		}
		result, err2 := sei.Extract(f, cfg.DefaultFPS)
		_ = f.Close()
		if err2 != nil {
			slog.Warn("ingest: SEI extract error", "path", repSS.seg.relPath, "err", err2)
		} else {
			if result.DurationS > 0 {
				d := float32(result.DurationS)
				clipDur = &d
			}
			if result.HasSEI {
				rows = downsample(result.Samples, cl.ts, result.FPS, cfg.SEISampleRate)
				clipHasSEI = len(rows) > 0
				if clipHasSEI {
					clipLat = &rows[0].lat
					clipLon = &rows[0].lon
				}
			}
			if err3 := updateSegmentSEI(ctx, tx, repSS.st.id, result.HasSEI,
				float32(cfg.SEISampleRate), sei.ExtractorVersion); err3 != nil {
				return 0, false, err3
			}
		}
	}

	// Probe has_sei for non-representative cameras.
	for i := range segStates {
		ss := &segStates[i]
		if repSS != nil && ss.st.id == repSS.st.id {
			continue
		}
		f, err2 := os.Open(ss.seg.absPath)
		if err2 != nil {
			slog.Warn("ingest: open for probe failed", "path", ss.seg.relPath, "err", err2)
			continue
		}
		hasSEI, _ := sei.Probe(f)
		_ = f.Close()
		if err3 := updateSegmentHasSEI(ctx, tx, ss.st.id, hasSEI); err3 != nil {
			return 0, false, err3
		}
	}

	// Write track points.
	if repSS != nil {
		if err2 := replaceTrackPoints(ctx, tx, carID, clipID, repSS.st.id, rows); err2 != nil {
			return 0, false, err2
		}
	}

	// Update clip derived fields.
	if err2 := updateClipDerived(ctx, tx, clipID, clipLat, clipLon, clipHasSEI, clipDur); err2 != nil {
		return 0, false, err2
	}

	if err2 := tx.Commit(ctx); err2 != nil {
		return 0, false, err2
	}
	return len(rows), true, nil
}

// canSkip returns true when the segment's persisted state indicates nothing has
// changed and the extractor version is current.
func canSkip(st segmentState, seiSampleRate float64) bool {
	if st.seiExtractorVersion == nil || *st.seiExtractorVersion < sei.ExtractorVersion {
		return false
	}
	if st.seiRateHz == nil || float64(*st.seiRateHz) != seiSampleRate {
		return false
	}
	// size and mtime are always freshly updated by the upsert, so if the ON
	// CONFLICT path ran, the row now reflects the latest values — the presence
	// of a valid sei_extractor_version is the signal that a previous run
	// completed successfully for the current file content.
	return true
}

// downsample converts a slice of pb.SeiMetadata (in file order) into
// track_point rows at ~1/rateHz samples per second.  It uses the first sample
// in each 1/rateHz bucket that has a valid GPS fix (lat≠0 || lon≠0).
// Wall-clock = clipTS + (frame_seq_no − firstSeq) / fps.
func downsample(samples []*pb.SeiMetadata, clipTS time.Time, fps, rateHz float64) []trackPointRow {
	if len(samples) == 0 || fps <= 0 || rateHz <= 0 {
		return nil
	}
	firstSeq := samples[0].GetFrameSeqNo()
	bucketDur := 1.0 / rateHz

	seen := map[int64]bool{}
	var rows []trackPointRow
	for _, m := range samples {
		lat := m.GetLatitudeDeg()
		lon := m.GetLongitudeDeg()
		if lat == 0 && lon == 0 {
			continue
		}
		relSec := float64(m.GetFrameSeqNo()-firstSeq) / fps
		bucket := int64(math.Floor(relSec / bucketDur))
		if seen[bucket] {
			continue
		}
		seen[bucket] = true

		ts := clipTS.Add(time.Duration(relSec * float64(time.Second)))
		rows = append(rows, trackPointRow{
			ts:                  ts,
			lat:                 lat,
			lon:                 lon,
			speedMPS:            m.GetVehicleSpeedMps(),
			headingDeg:          m.GetHeadingDeg(),
			gear:                int16(m.GetGearState().Number()),
			autopilot:           int16(m.GetAutopilotState().Number()),
			blinkerLeft:         m.GetBlinkerOnLeft(),
			blinkerRight:        m.GetBlinkerOnRight(),
			brakeApplied:        m.GetBrakeApplied(),
			acceleratorPedalPos: m.GetAcceleratorPedalPosition(),
			steeringWheelAngle:  m.GetSteeringWheelAngle(),
			accelX:              m.GetLinearAccelerationMps2X(),
			accelY:              m.GetLinearAccelerationMps2Y(),
			accelZ:              m.GetLinearAccelerationMps2Z(),
			seiVersion:          int32(m.GetVersion()),
		})
	}
	return rows
}
