-- +goose Up
CREATE TABLE cars (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key           TEXT NOT NULL UNIQUE,           -- stable path/config-derived key
    display_name  TEXT,
    hw_generation TEXT,                           -- 'hw3'|'hw4'|'ai5'|'unknown' (open)
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE events (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    car_id          BIGINT NOT NULL REFERENCES cars(id),
    event_ts        TIMESTAMPTZ NOT NULL,         -- WHAT TRIGGERED THE SAVE (sentry trip/horn/crash/manual); map+timeline markers key off this; as-UTC
    folder_ts       TIMESTAMPTZ NOT NULL,         -- the video/clip-group time (folder name / event.json "store"); as-UTC
    kind            TEXT NOT NULL CHECK (kind IN ('saved','sentry')),
    city            TEXT,
    est_lat         DOUBLE PRECISION,
    est_lon         DOUBLE PRECISION,
    reason          TEXT,
    camera          TEXT,                          -- triggering camera code (e.g. "6")
    folder_rel_path TEXT NOT NULL UNIQUE,          -- "store"/folder key recoverable from this + raw
    thumb_rel_path  TEXT,
    raw             JSONB NOT NULL,                -- verbatim event.json (markused + future)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX events_car_id_idx ON events (car_id);

CREATE TABLE clips (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    car_id      BIGINT NOT NULL REFERENCES cars(id),
    event_id    BIGINT REFERENCES events(id) ON DELETE SET NULL,
    clip_ts     TIMESTAMPTZ NOT NULL,             -- wall-clock as-UTC
    kind        TEXT NOT NULL CHECK (kind IN ('recent','saved','sentry')),
    lat         DOUBLE PRECISION,                  -- representative point (nullable)
    lon         DOUBLE PRECISION,
    has_sei     BOOLEAN NOT NULL DEFAULT false,    -- clip has a track
    duration_s  REAL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (car_id, kind, event_id, clip_ts)
);
CREATE INDEX clips_clip_ts_idx  ON clips (clip_ts);
CREATE INDEX clips_event_id_idx ON clips (event_id);
-- car_id filtering served by the leading column of the UNIQUE index

CREATE TABLE segments (
    id                    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clip_id               BIGINT NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    camera                TEXT NOT NULL,           -- open TEXT (front/back/repeaters/pillars/...)
    rel_path              TEXT NOT NULL UNIQUE,
    size_bytes            BIGINT,
    file_mtime            TIMESTAMPTZ,
    file_hash             TEXT,                    -- xxh3 hex, nullable
    has_sei               BOOLEAN NOT NULL DEFAULT false,  -- file contains SEI NALs
    sei_rate_hz           REAL,                    -- provenance: downsample rate used
    sei_extractor_version INTEGER,                 -- provenance: parser version
    sei_processed_at      TIMESTAMPTZ,
    meta                  JSONB,                   -- future per-file sidecar (nullable)
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX segments_clip_id_idx ON segments (clip_id);

CREATE TABLE track_points (
    id                         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    car_id                     BIGINT NOT NULL REFERENCES cars(id),       -- denormalized
    clip_id                    BIGINT NOT NULL REFERENCES clips(id) ON DELETE CASCADE,
    segment_id                 BIGINT NOT NULL REFERENCES segments(id) ON DELETE CASCADE,
    ts                         TIMESTAMPTZ NOT NULL,                       -- wall-clock as-UTC
    lat                        DOUBLE PRECISION NOT NULL,
    lon                        DOUBLE PRECISION NOT NULL,
    speed_mps                  REAL,
    heading_deg                DOUBLE PRECISION,
    gear                       SMALLINT,           -- pb.SeiMetadata_Gear code
    autopilot                  SMALLINT,           -- pb.SeiMetadata_AutopilotState code
    blinker_left               BOOLEAN,
    blinker_right              BOOLEAN,
    brake_applied              BOOLEAN,
    accelerator_pedal_position REAL,
    steering_wheel_angle       REAL,
    accel_x                    DOUBLE PRECISION,
    accel_y                    DOUBLE PRECISION,
    accel_z                    DOUBLE PRECISION,
    sei_version                INTEGER             -- proto field is uint32; INTEGER covers realistic schema versions
);
CREATE INDEX track_points_ts_idx         ON track_points (ts);            -- core query (no car)
CREATE INDEX track_points_car_ts_idx     ON track_points (car_id, ts);    -- per-car map query
CREATE INDEX track_points_clip_id_idx    ON track_points (clip_id);
CREATE INDEX track_points_segment_id_idx ON track_points (segment_id);

-- +goose Down
DROP TABLE IF EXISTS track_points;
DROP TABLE IF EXISTS segments;
DROP TABLE IF EXISTS clips;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS cars;
