package aggregator

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// FIRMSFeedURL is the NASA FIRMS NOAA-20 VIIRS C2 global 7-day active fire feed.
const FIRMSFeedURL = "https://firms.modaps.eosdis.nasa.gov/data/active_fire/noaa-20-viirs-c2/csv/J1_VIIRS_C2_Global_7d.csv"

// FIRMSWorker fetches and caches NASA FIRMS fire data on a fixed interval.
type FIRMSWorker struct {
	cache      *Cache
	httpClient *http.Client
	interval   time.Duration
}

// NewFIRMSWorker creates a FIRMSWorker that refreshes the cache every 5 minutes.
func NewFIRMSWorker(cache *Cache) *FIRMSWorker {
	return &FIRMSWorker{
		cache:      cache,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		interval:   5 * time.Minute,
	}
}

// Start performs an immediate fetch and then repeats every w.interval until ctx is cancelled.
func (w *FIRMSWorker) Start(ctx context.Context) {
	// First fetch is synchronous so the cache is populated before the server starts serving.
	w.fetchAndCache(ctx)

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.fetchAndCache(ctx)
			}
		}
	}()
}

func (w *FIRMSWorker) fetchAndCache(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, FIRMSFeedURL, nil)
	if err != nil {
		fmt.Printf("[FIRMS] build request error: %v\n", err)
		return
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		fmt.Printf("[FIRMS] fetch error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[FIRMS] unexpected status %d\n", resp.StatusCode)
		return
	}

	records, err := w.ParseCSV(resp.Body)
	if err != nil {
		fmt.Printf("[FIRMS] CSV parse error: %v\n", err)
		return
	}

	w.cache.Set(records)
	fmt.Printf("[FIRMS] cached %d records at %s\n", len(records), time.Now().Format(time.RFC3339))
}

// ParseCSV parses a FIRMS CSV stream and returns []FireRecord.
// Malformed rows are skipped without aborting the parse.
func (w *FIRMSWorker) ParseCSV(body io.Reader) ([]FireRecord, error) {
	r := csv.NewReader(bufio.NewReader(body))

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}

	// Build column-name → index map (trim spaces for robustness).
	colIdx := make(map[string]int, len(header))
	for i, name := range header {
		colIdx[strings.TrimSpace(name)] = i
	}

	required := []string{"latitude", "longitude", "brightness", "acq_date", "confidence"}
	for _, col := range required {
		if _, ok := colIdx[col]; !ok {
			return nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	var records []FireRecord
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Skip malformed rows rather than aborting.
			continue
		}

		lat, err := strconv.ParseFloat(strings.TrimSpace(row[colIdx["latitude"]]), 64)
		if err != nil {
			continue
		}
		lon, err := strconv.ParseFloat(strings.TrimSpace(row[colIdx["longitude"]]), 64)
		if err != nil {
			continue
		}
		brightness, err := strconv.ParseFloat(strings.TrimSpace(row[colIdx["brightness"]]), 64)
		if err != nil {
			continue
		}

		records = append(records, FireRecord{
			Lat:        lat,
			Lon:        lon,
			Brightness: brightness,
			AcqDate:    strings.TrimSpace(row[colIdx["acq_date"]]),
			Confidence: strings.TrimSpace(row[colIdx["confidence"]]),
		})
	}

	return records, nil
}
