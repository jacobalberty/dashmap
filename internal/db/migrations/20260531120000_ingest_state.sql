-- +goose Up
CREATE TABLE ingest_state (
    car_id                 BIGINT PRIMARY KEY REFERENCES cars(id) ON DELETE CASCADE,
    last_scan_started_at   TIMESTAMPTZ,         -- set when a scan begins
    last_scan_completed_at TIMESTAMPTZ,         -- set when a scan finishes; drives liveness
    clips_processed        BIGINT NOT NULL DEFAULT 0,
    clips_skipped          BIGINT NOT NULL DEFAULT 0,
    clips_errored          BIGINT NOT NULL DEFAULT 0,
    track_points_written   BIGINT NOT NULL DEFAULT 0,
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS ingest_state;
