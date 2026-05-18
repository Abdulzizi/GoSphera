# GoSphera

High-performance Geospatial Intelligence & Data Correlation platform.

```
┌──────────────────────────────────────────────────────────────┐
│  Browser (Vue 3 + Leaflet)    port 5173 (dev) / 80 (docker) │
│  ┌──────────┐ ┌──────────────────────────┐ ┌─────────────┐  │
│  │LeftPanel │ │  MainGlobe (Leaflet/OSM) │ │ RightPanel  │  │
│  │ Tag Qry  │ │  Interactive map + GeoJSON│ │ FIRMS / SSE │  │
│  └────┬─────┘ └────────────┬─────────────┘ └──────┬──────┘  │
└───────┼────────────────────┼────────────────────────┼────────┘
        │ GET /api/spatial   │                        │ EventSource
        │                    │            GET /api/events (SSE)
        ▼                    ▼                        ▼
┌──────────────────────────────────────────────────────────────┐
│  Go Backend (chi)                             port 8080      │
│  ┌──────────────┐  ┌────────────────┐  ┌──────────────────┐ │
│  │ pkg/spatial  │  │ pkg/telemetry  │  │  pkg/aggregator  │ │
│  │ Overpass QL  │  │ Pipeline(4w)   │  │  FIRMS Worker    │ │
│  │ OSM→GeoJSON  │  │ AnomalyDetect  │  │  CSV Cache       │ │
│  └──────────────┘  └───────┬────────┘  └──────────────────┘ │
│                            │ EventBroker (SSE fan-out)       │
└────────────────────────────┴────────────────────────────────-┘
                    External APIs
          Overpass API  ·  NASA FIRMS CSV feed
```

## Prerequisites

- Go 1.24+
- Node.js 20+
- Docker + Docker Compose (for containerised deployment)

## Quick Start — Local Dev

```bash
# Terminal 1: backend
make dev-backend

# Terminal 2: frontend
make dev-frontend
# → open http://localhost:5173
```

## Quick Start — Docker

```bash
# Copy and edit env if needed
cp backend/.env.example backend/.env

make docker-up
# → open http://localhost
```

## Available Make Targets

| Target | Description |
|--------|-------------|
| `make dev-backend` | Run Go server with live reload |
| `make dev-frontend` | Run Vite dev server |
| `make build-backend` | Compile `backend/bin/server` |
| `make build-frontend` | Vite production build → `frontend/dist/` |
| `make test` | Run all Go tests |
| `make lint` | `go vet` + `vue-tsc` |
| `make docker-up` | Build and start both containers |
| `make docker-down` | Stop and remove containers |

## API Reference

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Server health + SSE client count |
| `GET` | `/api/spatial?tags=k=v&bbox=s,w,n,e` | Overpass QL → GeoJSON FeatureCollection |
| `GET` | `/api/situational` | Cached NASA FIRMS fire records (JSON) |
| `POST` | `/api/telemetry` | Ingest a TelemetryEvent (body: JSON) |
| `GET` | `/api/events` | **Server-Sent Events** stream of processed events |

### Spatial query example

```bash
curl "http://localhost:8080/api/spatial?tags=amenity=cafe&bbox=25.7,-80.5,26.0,-80.0"
```

### Telemetry ingest + SSE stream

```bash
# Subscribe
curl -N http://localhost:8080/api/events

# Push event (in another terminal) — appears in the SSE stream after anomaly detection
curl -X POST http://localhost:8080/api/telemetry \
  -H "Content-Type: application/json" \
  -d '{"id":"sensor-1","lat":25.76,"lon":-80.19,"properties":{}}'
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `FIRMS_URL` | NASA FIRMS global 7-day CSV | Active fire feed URL |
| `FIRMS_INTERVAL_MINUTES` | `5` | Cache refresh interval (minutes) |

## Architecture Notes

- **Telemetry pipeline** — 4 concurrent goroutines read from a buffered ingest channel, run anomaly detection (Δt threshold 30 s), then push processed events to an `EventBroker`.
- **EventBroker** — fan-out broadcaster; each SSE client gets its own buffered channel (64 slots). Slow clients are dropped per-event, never block the pipeline.
- **FIRMS cache** — `sync.RWMutex`-protected slice; refreshed every 5 minutes by a background worker.
- **Leaflet map** — wrapped in `shallowRef` to prevent Vue deep-proxying the map instance. Tile layer rendered with a CSS dark-mode filter.
