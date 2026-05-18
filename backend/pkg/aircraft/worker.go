package aircraft

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultOpenSkyURL = "https://opensky-network.org/api/states/all"

func openSkyURL() string {
	if u := os.Getenv("OPENSKY_URL"); u != "" {
		return u
	}
	return defaultOpenSkyURL
}

func openSkyInterval() time.Duration {
	if s := os.Getenv("OPENSKY_INTERVAL_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 10 {
			return time.Duration(n) * time.Second
		}
	}
	return 15 * time.Second
}

// Worker fetches and caches OpenSky Network aircraft state vectors.
type Worker struct {
	cache      *Cache
	httpClient *http.Client
	interval   time.Duration
}

func NewWorker(cache *Cache) *Worker {
	return &Worker{
		cache:      cache,
		httpClient: &http.Client{Timeout: 20 * time.Second},
		interval:   openSkyInterval(),
	}
}

// Start launches a background goroutine that fetches immediately then every interval.
func (w *Worker) Start(ctx context.Context) {
	go func() {
		w.fetchAndCache(ctx)
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

type openSkyResponse struct {
	States [][]interface{} `json:"states"`
}

func (w *Worker) fetchAndCache(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openSkyURL(), nil)
	if err != nil {
		fmt.Printf("[OpenSky] build request error: %v\n", err)
		return
	}

	resp, err := w.httpClient.Do(req)
	if err != nil {
		fmt.Printf("[OpenSky] fetch error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		fmt.Println("[OpenSky] rate limited (429) — backing off")
		return
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[OpenSky] unexpected status %d\n", resp.StatusCode)
		return
	}

	var raw openSkyResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		fmt.Printf("[OpenSky] JSON decode error: %v\n", err)
		return
	}

	states := parseStates(raw.States)
	w.cache.Set(states)
	fmt.Printf("[OpenSky] cached %d aircraft at %s\n", len(states), time.Now().Format(time.RFC3339))
}

// parseStates converts raw OpenSky state vectors (heterogeneous arrays) into typed structs.
// OpenSky state vector field order:
//
//	0  icao24, 1 callsign, 2 origin_country, 3 time_position, 4 last_contact,
//	5  longitude, 6 latitude, 7 baro_altitude, 8 on_ground,
//	9  velocity, 10 true_track, 11 vertical_rate, ...
func parseStates(raw [][]interface{}) []AircraftState {
	out := make([]AircraftState, 0, len(raw))
	for _, s := range raw {
		if len(s) < 11 {
			continue
		}
		lat, latOK := toFloat(s[6])
		lon, lonOK := toFloat(s[5])
		if !latOK || !lonOK {
			continue // no position fix
		}

		state := AircraftState{
			ICAO24:   strings.TrimSpace(toString(s[0])),
			Callsign: strings.TrimSpace(toString(s[1])),
			Country:  toString(s[2]),
			Lat:      lat,
			Lon:      lon,
		}
		if alt, ok := toFloat(s[7]); ok {
			state.Altitude = alt
		}
		if vel, ok := toFloat(s[9]); ok {
			state.Velocity = vel
		}
		if hdg, ok := toFloat(s[10]); ok {
			state.Heading = hdg
		}
		if og, ok := s[8].(bool); ok {
			state.OnGround = og
		}
		if lc, ok := toInt64(s[4]); ok {
			state.LastContact = lc
		}
		out = append(out, state)
	}
	return out
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func toFloat(v interface{}) (float64, bool) {
	if v == nil {
		return 0, false
	}
	if n, ok := v.(float64); ok {
		return n, true
	}
	return 0, false
}

func toInt64(v interface{}) (int64, bool) {
	if v == nil {
		return 0, false
	}
	if n, ok := v.(float64); ok {
		return int64(n), true
	}
	return 0, false
}
