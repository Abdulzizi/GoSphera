package camera

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Camera describes a public traffic/surveillance camera.
//
// StreamType controls how the frontend renders the live feed:
//   - "snapshot"  — frontend polls SnapshotURL every 5 s with a cache-busting timestamp.
//   - "mjpeg"     — frontend loads StreamURL in a plain <img>; the browser decodes the
//                   multipart JPEG stream natively (no library required).
//   - "hls"       — frontend loads /api/stream/{ID}/playlist.m3u8 (the Go HLS relay proxy)
//                   via hls.js; the proxy fetches StreamURL, rewrites segment refs, and
//                   serves the modified playlist so CORS is never an issue.
//
// To add a new HLS camera: set StreamType="hls" and StreamURL to the source .m3u8 URL.
// The frontend automatically uses /api/stream/{ID}/playlist.m3u8 for all HLS cameras.
type Camera struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	City        string  `json:"city"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	SnapshotURL string  `json:"snapshot_url"`
	StreamURL   string  `json:"stream_url,omitempty"` // source HLS .m3u8 or MJPEG endpoint
	StreamType  string  `json:"stream_type"`           // "snapshot" | "mjpeg" | "hls"
}

// ── Live HLS cameras (verified public CDN streams) ─────────────────────────────
//
// These cameras serve actual HLS video via public DOT / agency CDNs.
// All route through /api/stream/{id}/playlist.m3u8 (the Go relay proxy),
// so CORS is handled server-side — no browser restriction ever applies.
var hlsCameras = []Camera{
	// Iowa DOT — live 24/7 highway intersection feeds on SkyVDN/AWS CloudFront.
	{
		ID: "us-ia-dubuque", Name: "US-20 @ JFK Rd", City: "Dubuque, Iowa",
		Lat: 42.492, Lon: -90.714,
		SnapshotURL: "", // HLS only — no static thumbnail available
		StreamURL:   "https://iowadotsfs1.us-east-1.skyvdn.com/rtplive/dqtv17lb/playlist.m3u8",
		StreamType:  "hls",
	},
	{
		ID: "us-ia-desmoines", Name: "I-235 Expressway", City: "Des Moines, Iowa",
		Lat: 41.596, Lon: -93.599,
		SnapshotURL: "",
		StreamURL:   "https://iowadotsfs2.us-east-1.skyvdn.com/rtplive/dmtv05lb/playlist.m3u8",
		StreamType:  "hls",
	},
	// NASA Television — continuous public broadcast (Akamai CDN).
	// Useful for verifying the proxy handles high-bitrate continuous streams.
	{
		ID: "nasa-tv-ops", Name: "NASA TV Live", City: "Cape Canaveral",
		Lat: 28.572, Lon: -80.648,
		SnapshotURL: "",
		StreamURL:   "https://nasatv-lh.akamaihd.net/i/NASA_101@319270/index_1000_av-p.m3u8",
		StreamType:  "hls",
	},
	// Colorado DOT (COtrip) — live 24/7 highway stream on Streamlock CDN.
	{
		ID: "us-co-sfork", Name: "CO Hwy – S Fork W", City: "South Fork, Colorado",
		Lat: 37.672, Lon: -106.636,
		SnapshotURL: "",
		StreamURL:   "https://5b106cf3460bf.streamlock.net:443/cotrip/160_S_Fork_W.stream/playlist.m3u8",
		StreamType:  "hls",
	},
	// Florida DOT District 5 ATMS — live intersection CCTV.
	{
		ID: "us-fl-c81109", Name: "FDOT D5 – C81109", City: "Orlando, Florida",
		Lat: 28.538, Lon: -81.379,
		SnapshotURL: "",
		StreamURL:   "https://atms.dot.state.fl.us/D5/CCTV/C81109/C81109.m3u8",
		StreamType:  "hls",
	},
}

// ── Camera registry ────────────────────────────────────────────────────────────
//
// Jakarta (Dinas Perhubungan DKI Jakarta)
// The official Dishub CCTV portal serves JPEG snapshots via Jakarta Smart City.
// URLs follow the pattern used by cctv.jakartasmartcity.com.
// Replace SnapshotURL with real working endpoints when they change.
var jakartaCameras = []Camera{
	{
		ID: "jkt-001", Name: "Bundaran HI", City: "Jakarta",
		Lat: -6.1950, Lon: 106.8229,
		SnapshotURL: "https://cctv.jakartasmartcity.com/snapshot/bundaran-hi.jpg",
		StreamType:  "snapshot",
	},
	{
		ID: "jkt-002", Name: "Semanggi Interchange", City: "Jakarta",
		Lat: -6.2088, Lon: 106.8197,
		SnapshotURL: "https://cctv.jakartasmartcity.com/snapshot/semanggi.jpg",
		StreamType:  "snapshot",
	},
	{
		ID: "jkt-003", Name: "Thamrin – Sarinah", City: "Jakarta",
		Lat: -6.1866, Lon: 106.8231,
		SnapshotURL: "https://cctv.jakartasmartcity.com/snapshot/sarinah.jpg",
		StreamType:  "snapshot",
	},
	{
		ID: "jkt-004", Name: "Blok M", City: "Jakarta",
		Lat: -6.2441, Lon: 106.7993,
		SnapshotURL: "https://cctv.jakartasmartcity.com/snapshot/blok-m.jpg",
		StreamType:  "snapshot",
	},
	{
		ID: "jkt-005", Name: "Sudirman – Polda Metro", City: "Jakarta",
		Lat: -6.2174, Lon: 106.8225,
		SnapshotURL: "https://cctv.jakartasmartcity.com/snapshot/sudirman-polda.jpg",
		StreamType:  "snapshot",
	},
}

// ── Cache ──────────────────────────────────────────────────────────────────────

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

// FindByID returns a copy of the camera with the given ID, or nil if not found.
func (c *Cache) FindByID(id string) *Camera {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := range c.data {
		if c.data[i].ID == id {
			cp := c.data[i]
			return &cp
		}
	}
	return nil
}

// ── TfL JamCam fetcher ─────────────────────────────────────────────────────────
//
// TfL JamCams provide JPEG snapshots only (updated ~every 60–90 s by Amazon S3).
// There is no official HLS feed from TfL. All TfL cameras are stream_type "snapshot".

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

// FetchAndPopulate fetches up to 60 TfL JamCam cameras (London) and appends the
// hardcoded Jakarta cameras. Falls back to Jakarta-only on any fetch/parse error.
func FetchAndPopulate(cache *Cache) {
	client := &http.Client{Timeout: 15 * time.Second}

	req, _ := http.NewRequest("GET", tflJamCamURL, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "GoSphera/1.0")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[cameras] TfL fetch failed: %v — using Jakarta + HLS only\n", err)
		cache.Set(append(jakartaCameras, hlsCameras...))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[cameras] TfL returned HTTP %d — using Jakarta + HLS only\n", resp.StatusCode)
		cache.Set(append(jakartaCameras, hlsCameras...))
		return
	}

	var places []tflPlace
	if err := json.NewDecoder(resp.Body).Decode(&places); err != nil {
		fmt.Printf("[cameras] TfL parse failed: %v — using Jakarta + HLS only\n", err)
		cache.Set(append(jakartaCameras, hlsCameras...))
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
			// TfL offers JPEG snapshots only — no HLS endpoint exists.
			// To upgrade to HLS when available: set StreamURL to the .m3u8 and StreamType to "hls".
			StreamType: "snapshot",
		})
	}

	cams = append(cams, jakartaCameras...)
	cams = append(cams, hlsCameras...)
	cache.Set(cams)
	tflCount := len(cams) - len(jakartaCameras) - len(hlsCameras)
	fmt.Printf("[cameras] loaded %d TfL + %d Jakarta + %d HLS cameras\n", tflCount, len(jakartaCameras), len(hlsCameras))
}
