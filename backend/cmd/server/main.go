package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/abdulzizi/gosphera/pkg/aggregator"
	"github.com/abdulzizi/gosphera/pkg/spatial"
	"github.com/abdulzizi/gosphera/pkg/telemetry"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"
)

func main() {
	spatialClient := spatial.NewClient()
	firmsCache := aggregator.NewCache()
	firmsWorker := aggregator.NewFIRMSWorker(firmsCache)
	pipeline := telemetry.NewPipeline(4)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("[server] starting FIRMS worker…")
	firmsWorker.Start(ctx)

	log.Println("[server] starting telemetry pipeline…")
	pipeline.Start(ctx)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
	})

	r.Get("/api/spatial", handleSpatialQuery(spatialClient))
	r.Get("/api/situational", handleSituational(firmsCache))
	r.Post("/api/telemetry", handleTelemetryIngest(pipeline))

	handler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	}).Handler(r)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Println("[server] listening on :8080")
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

// handleSituational handles GET /api/situational
func handleSituational(cache *aggregator.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, cache.GetAll())
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
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	writeJSON(w, status, map[string]string{"error": msg})
}
