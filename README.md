# dashmap

Browse locally-stored Tesla dashcam footage on a **map + calendar**: pick a date range,
see where the car was, click a point, play back the multi-camera clip with optional Tesla
metadata overlay, and download clips.

This project is built across the planning prompts in [`stories/`](stories/). Start with
[`stories/README.md`](stories/README.md), then work `01` → `07` in order.

Stack: Go backend (single **distroless** binary embedding the Vue SPA) · Vue 3 + Vite +
MapLibre GL · PostgreSQL · no-transcode raw streaming. Built with **buildx**, shipped via
**GitHub Actions**.

## Development

**Prerequisites:** Go 1.27+, Node 24+, Docker, [buf](https://buf.build/docs/installation)

```sh
# Copy and fill in .env
cp .env.example .env

# Start Postgres
docker compose up postgres -d

# Run API server (no embedded SPA — serves API only)
make run-api

# Run Vue dev server with hot reload (proxies /api /livez /readyz → localhost:8080)
make web-dev

# Run background worker
make run-worker

# Apply migrations manually
make migrate

# Regenerate protobuf Go code
make proto-gen
```

**Health endpoints:** `GET /livez` (liveness) · `GET /readyz` (DB readiness)

**Full stack via Docker Compose:**
```sh
docker compose up
# app → http://localhost:8080
```

## Basemap Setup (Story 05+)

The map uses self-hosted Protomaps **PMTiles** for privacy (no external tile servers). It's
optional — without it, the map still shows the GPS track and clip markers on a blank dark
background.

To enable it, extract a regional cutout of the Protomaps planet basemap and point the server
at the resulting file.

1. **Install the `pmtiles` CLI** — download a binary from
   [go-pmtiles releases](https://github.com/protomaps/go-pmtiles/releases), or use the
   `protomaps/go-pmtiles` Docker image.

2. **Find your bounding box** for the area your footage covers (e.g. via
   [bboxfinder.com](http://bboxfinder.com/)) as `minLon,minLat,maxLon,maxLat`.

3. **Extract a regional cutout directly from a daily build** — `pmtiles extract` reads the
   remote file over HTTP range requests, so you only download your region (not the ~120 GB
   planet). Pick a recent build date from
   [maps.protomaps.com/builds](https://maps.protomaps.com/builds/):
   ```sh
   pmtiles extract https://build.protomaps.com/20260518.pmtiles my-region.pmtiles \
     --bbox=-122.5,37.7,-122.3,37.8 \
     --maxzoom=14
   ```
   Each extra zoom level roughly doubles the file size; `--maxzoom=14` is plenty for
   street-level detail. Use a **v4** basemap build (its tile schema matches the
   `@protomaps/basemaps` v5 style used by the frontend).

4. **Run with the basemap:**
   ```sh
   export BASEMAP_PMTILES=/path/to/my-region.pmtiles
   make run-api
   ```
   The server serves it at `GET /tiles/basemap.pmtiles` with HTTP range support.

5. **Docker Compose** — mount the file and set the env var:
   ```yaml
   services:
     app:
       environment:
         BASEMAP_PMTILES: /basemap/my-region.pmtiles
       volumes:
         - /path/to/my-region.pmtiles:/basemap/my-region.pmtiles:ro
   ```

**Run tests:**
```sh
make test                # unit tests (no DB required)
make test-integration    # requires DATABASE_URL set
```

## Releases & security

- **Cut a release:** push a `vX.Y.Z` tag, or run the *Release* workflow manually (leave the
  version empty to compute the next semver from conventional commits). It creates the GitHub
  release with a changelog, then builds and pushes a multi-arch (`amd64`/`arm64`) image to
  `ghcr.io/jacobalberty/dashmap` (`X.Y.Z`, `X.Y`, `X`, `latest`), Trivy-scans it and signs it
  with cosign.
- **Gates before any image is published:** `go vet`, race + integration tests, `go fix`,
  golangci-lint (incl. gosec), govulncheck, web typecheck/lint/test/build, `npm audit`,
  buf lint/diff, and a full-history gitleaks secret scan. CodeQL and Dependabot run separately.
- **Health:** the distroless image has no shell, so the container `HEALTHCHECK` runs
  `/dashmap healthcheck` (probes `/livez`). `/dashmap version` prints the build version.
