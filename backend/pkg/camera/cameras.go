package camera

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Camera describes a public traffic/surveillance camera with a live snapshot URL.
type Camera struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	City        string  `json:"city"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	SnapshotURL string  `json:"snapshot_url"`
}

// ── Jakarta hardcoded cameras (Dinas Perhubungan DKI open CCTV) ────────────────
// These stream via the official Jakarta CCTV portal.
var jakartaCameras = []Camera{
	{ID: "jkt-001", Name: "Bundaran HI", City: "Jakarta", Lat: -6.1950, Lon: 106.8229,
		SnapshotURL: "https://cctv.jakarta.go.id/assets/cam/bundaran_hi.jpg"},
	{ID: "jkt-002", Name: "Semanggi Interchange", City: "Jakarta", Lat: -6.2088, Lon: 106.8197,
		SnapshotURL: "https://cctv.jakarta.go.id/assets/cam/semanggi.jpg"},
	{ID: "jkt-003", Name: "Thamrin – Sarinah", City: "Jakarta", Lat: -6.1866, Lon: 106.8231,
		SnapshotURL: "https://cctv.jakarta.go.id/assets/cam/sarinah.jpg"},
	{ID: "jkt-004", Name: "Blok M", City: "Jakarta", Lat: -6.2441, Lon: 106.7993,
		SnapshotURL: "https://cctv.jakarta.go.id/assets/cam/blokm.jpg"},
	{ID: "jkt-005", Name: "Sudirman – Polda", City: "Jakarta", Lat: -6.2174, Lon: 106.8225,
		SnapshotURL: "https://cctv.jakarta.go.id/assets/cam/sudirman_polda.jpg"},
}

// ── Cache (holds cameras fetched from TfL + Jakarta hardcoded) ────────────────

type Cache struct {
	mu   sync.RWMutex
	data []Camera
}

func NewCache() *Cache { return &Cache{} }

func (c *Cache) Set(cams []Camera) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = cams
}

func (c *Cache) GetAll() []Camera {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Camera, len(c.data))
	copy(out, c.data)
	return out
}

// ── TfL JamCam fetcher ────────────────────────────────────────────────────────

const tflJamCamURL = "https://api.tfl.gov.uk/Place/Type/JamCam"

type tflPlace struct {
	CommonName           string  `json:"commonName"`
	Lat                  float64 `json:"lat"`
	Lon                  float64 `json:"lon"`
	AdditionalProperties []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"additionalProperties"`
}

// FetchAndPopulate fetches TfL JamCam data into the cache, then appends
// the hardcoded Jakarta cameras. Falls back to Jakarta-only on error.
func FetchAndPopulate(cache *Cache) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(tflJamCamURL)
	if err != nil {
		fmt.Printf("[cameras] TfL fetch failed: %v — using Jakarta only\n", err)
		cache.Set(jakartaCameras)
		return
	}
	defer resp.Body.Close()

	var places []tflPlace
	if err := json.NewDecoder(resp.Body).Decode(&places); err != nil {
		fmt.Printf("[cameras] TfL parse failed: %v — using Jakarta only\n", err)
		cache.Set(jakartaCameras)
		return
	}

	const maxTfL = 60
	var cams []Camera
	for i, p := range places {
		if i >= maxTfL {
			break
		}
		var imgURL string
		for _, prop := range p.AdditionalProperties {
			if prop.Key == "imageUrl" {
				imgURL = prop.Value
				break
			}
		}
		if imgURL == "" {
			continue
		}
		cams = append(cams, Camera{
			ID:          fmt.Sprintf("tfl-%d", i+1),
			Name:        p.CommonName,
			City:        "London",
			Lat:         p.Lat,
			Lon:         p.Lon,
			SnapshotURL: imgURL,
		})
	}

	// Append Jakarta cameras
	cams = append(cams, jakartaCameras...)
	cache.Set(cams)
	fmt.Printf("[cameras] loaded %d TfL + %d Jakarta cameras\n", len(cams)-len(jakartaCameras), len(jakartaCameras))
}
