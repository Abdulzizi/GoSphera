package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/abdulzizi/gosphera/pkg/aggregator"
	"github.com/abdulzizi/gosphera/pkg/aircraft"
	"github.com/abdulzizi/gosphera/pkg/camera"
	"github.com/abdulzizi/gosphera/pkg/maritime"
	"github.com/abdulzizi/gosphera/pkg/spatial"
	"github.com/abdulzizi/gosphera/pkg/telemetry"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	// Load .env if present (silently ignored when not found)
	if err := godotenv.Load(); err == nil {
		log.Println("[server] loaded .env file")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	spatialClient := spatial.NewClient()
	firmsCache := aggregator.NewCache()
	firmsWorker := aggregator.NewFIRMSWorker(firmsCache)
	aircraftCache := aircraft.NewCache()
	aircraftWorker := aircraft.NewWorker(aircraftCache)
	maritimeCache := maritime.NewCache()
	maritimeWorker := maritime.NewWorker(maritimeCache)
	cameraCache := camera.NewCache()
	pipeline := telemetry.NewPipeline(4)
	broker := telemetry.NewEventBroker()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("[server] starting FIRMS worker…")
	firmsWorker.Start(ctx)

	log.Println("[server] starting OpenSky aircraft worker…")
	aircraftWorker.Start(ctx)

	log.Println("[server] starting AISStream maritime worker…")
	maritimeWorker.Start(ctx)

	log.Println("[server] fetching camera list (TfL + Jakarta)…")
	go camera.FetchAndPopulate(cameraCache)

	log.Println("[server] starting telemetry pipeline…")
	pipeline.Start(ctx)

	// Drain the processed channel and fan-out to all SSE clients.
	go func() {
		for event := range pipeline.GetProcessedChannel() {
			broker.Broadcast(event)
		}
	}()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","timestamp":"%s","sse_clients":%d}`,
			time.Now().Format(time.RFC3339), broker.ClientCount())
	})

	r.Get("/api/spatial", handleSpatialQuery(spatialClient))
	r.Get("/api/situational", handleSituational(firmsCache))
	r.Get("/api/aircraft", handleAircraft(aircraftCache))
	r.Get("/api/ships", handleShips(maritimeCache))
	r.Get("/api/cameras", handleCameras(cameraCache))
	r.Get("/api/camera-proxy/{id}", handleCameraProxy(cameraCache))
	r.Post("/api/telemetry", handleTelemetryIngest(pipeline))
	r.Get("/api/events", handleSSE(broker))

	handler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	}).Handler(r)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // SSE connections are long-lived; disable write timeout
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("[server] listening on :%s\n", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[server] fatal: %v", err)
		}
	}()

	<-quit
	log.Println("[server] shutdown signal received")

	pipeline.Stop()
	cancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Printf("[server] shutdown error: %v", err)
	}
	log.Println("[server] stopped")
}

// handleSSE streams processed TelemetryEvents to a browser EventSource.
func handleSSE(broker *telemetry.EventBroker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ch := broker.Subscribe()
		defer broker.Unsubscribe(ch)

		for {
			select {
			case event, ok := <-ch:
				if !ok {
					return
				}
				data, err := json.Marshal(event)
				if err != nil {
					continue
				}
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()

			case <-r.Context().Done():
				return
			}
		}
	}
}

// handleSpatialQuery handles GET /api/spatial?tags=k=v,k2=v2&bbox=minLat,minLon,maxLat,maxLon
func handleSpatialQuery(client *spatial.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tagsParam := r.URL.Query().Get("tags")
		bboxParam := r.URL.Query().Get("bbox")

		if tagsParam == "" || bboxParam == "" {
			jsonError(w, "tags and bbox query parameters are required", http.StatusBadRequest)
			return
		}

		var tags []spatial.Tag
		for _, pair := range strings.Split(tagsParam, ",") {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 && parts[0] != "" {
				tags = append(tags, spatial.Tag{Key: parts[0], Value: parts[1]})
			}
		}

		bboxParts := strings.Split(bboxParam, ",")
		if len(bboxParts) != 4 {
			jsonError(w, "bbox must be minLat,minLon,maxLat,maxLon", http.StatusBadRequest)
			return
		}
		bbox, err := parseBBox(bboxParts)
		if err != nil {
			jsonError(w, "invalid bbox values", http.StatusBadRequest)
			return
		}

		fc, err := client.Query(tags, bbox)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, fc)
	}
}

// bboxFilter holds parsed viewport bounds from query parameters.
type bboxFilter struct {
	minLat, minLon, maxLat, maxLon float64
	active                         bool
}

// parseBboxFilter reads optional ?minLat=&minLon=&maxLat=&maxLon= query params.
// Returns an inactive filter if any param is missing or unparseable.
func parseBboxFilter(r *http.Request) bboxFilter {
	q := r.URL.Query()
	sMinLat, sMinLon := q.Get("minLat"), q.Get("minLon")
	sMaxLat, sMaxLon := q.Get("maxLat"), q.Get("maxLon")
	if sMinLat == "" || sMinLon == "" || sMaxLat == "" || sMaxLon == "" {
		return bboxFilter{}
	}
	minLat, e1 := strconv.ParseFloat(sMinLat, 64)
	minLon, e2 := strconv.ParseFloat(sMinLon, 64)
	maxLat, e3 := strconv.ParseFloat(sMaxLat, 64)
	maxLon, e4 := strconv.ParseFloat(sMaxLon, 64)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		return bboxFilter{}
	}
	return bboxFilter{minLat, minLon, maxLat, maxLon, true}
}

// handleSituational handles GET /api/situational with optional bbox filtering.
// With bbox: returns all records within the viewport.
// Without bbox: caps at 5000 records to protect mobile clients on initial load.
// Always sets X-Total-Count to the full cached record count.
func handleSituational(cache *aggregator.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		records := cache.GetAll()
		w.Header().Set("X-Total-Count", strconv.Itoa(len(records)))

		bbox := parseBboxFilter(r)
		if bbox.active {
			filtered := records[:0:0]
			for _, rec := range records {
				if rec.Lat >= bbox.minLat && rec.Lat <= bbox.maxLat &&
					rec.Lon >= bbox.minLon && rec.Lon <= bbox.maxLon {
					filtered = append(filtered, rec)
				}
			}
			records = filtered
		} else {
			// No viewport supplied: protect mobile from 203k records on first load
			const defaultCap = 5000
			if len(records) > defaultCap {
				records = records[:defaultCap]
			}
		}

		writeJSON(w, http.StatusOK, records)
	}
}

// handleAircraft handles GET /api/aircraft with optional bbox filtering.
// With bbox: returns all aircraft within the viewport (no additional limit).
// Without bbox: applies default limit of 1500, max 5000.
// Always sets X-Total-Count to the full cached state count.
func handleAircraft(cache *aircraft.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		states := cache.GetAll()
		w.Header().Set("X-Total-Count", strconv.Itoa(len(states)))

		bbox := parseBboxFilter(r)
		if bbox.active {
			filtered := states[:0:0]
			for _, s := range states {
				if s.Lat >= bbox.minLat && s.Lat <= bbox.maxLat &&
					s.Lon >= bbox.minLon && s.Lon <= bbox.maxLon {
					filtered = append(filtered, s)
				}
			}
			states = filtered
		} else {
			limit := 1500
			if s := r.URL.Query().Get("limit"); s != "" {
				if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 5000 {
					limit = n
				}
			}
			if limit < len(states) {
				states = states[:limit]
			}
		}

		writeJSON(w, http.StatusOK, states)
	}
}

// handleShips handles GET /api/ships — returns all cached AIS ship states.
func handleShips(cache *maritime.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		states := cache.GetAll()
		w.Header().Set("X-Total-Count", strconv.Itoa(len(states)))
		writeJSON(w, http.StatusOK, states)
	}
}

// handleTelemetryIngest handles POST /api/telemetry
func handleTelemetryIngest(p *telemetry.Pipeline) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var event telemetry.TelemetryEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			jsonError(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now()
		}
		if event.Properties == nil {
			event.Properties = make(map[string]interface{})
		}

		p.Ingest(event)
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted", "id": event.ID})
	}
}

// handleCameras handles GET /api/cameras with optional bbox filtering.
func handleCameras(cache *camera.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cams := cache.GetAll()
		bbox := parseBboxFilter(r)
		if bbox.active {
			filtered := cams[:0:0]
			for _, c := range cams {
				if c.Lat >= bbox.minLat && c.Lat <= bbox.maxLat &&
					c.Lon >= bbox.minLon && c.Lon <= bbox.maxLon {
					filtered = append(filtered, c)
				}
			}
			cams = filtered
		}
		writeJSON(w, http.StatusOK, cams)
	}
}

// handleCameraProxy proxies a camera's StreamURL to the browser, solving CORS for
// MJPEG sources that don't set Access-Control-Allow-Origin headers.
// Only proxies cameras whose StreamURL is non-empty.
func handleCameraProxy(cache *camera.Cache) http.HandlerFunc {
	proxyClient := &http.Client{Timeout: 30 * time.Second}
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		cam := cache.FindByID(id)
		if cam == nil || cam.StreamURL == "" {
			http.Error(w, "camera not found or no stream URL", http.StatusNotFound)
			return
		}
		resp, err := proxyClient.Get(cam.StreamURL)
		if err != nil {
			http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		ct := resp.Header.Get("Content-Type")
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "no-cache, no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body) //nolint:errcheck
	}
}

// helpers

func parseBBox(parts []string) (spatial.BoundingBox, error) {
	vals := make([]float64, 4)
	for i, s := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return spatial.BoundingBox{}, err
		}
		vals[i] = v
	}
	return spatial.BoundingBox{
		MinLat: vals[0], MinLon: vals[1],
		MaxLat: vals[2], MaxLon: vals[3],
	}, nil
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	writeJSON(w, status, map[string]string{"error": msg})
}
