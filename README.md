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

**Prerequisites:** Go 1.26+, Node 24+, Docker, [buf](https://buf.build/docs/installation)

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

**Run tests:**
```sh
make test                # unit tests (no DB required)
make test-integration    # requires DATABASE_URL set
```
