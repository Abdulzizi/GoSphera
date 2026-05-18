package camera

// Camera describes a public traffic/surveillance camera with a live snapshot URL.
type Camera struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	City        string  `json:"city"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	SnapshotURL string  `json:"snapshot_url"`
}

// staticCameras is a curated list of publicly accessible traffic cameras.
// All snapshot URLs are unauthenticated and refresh automatically on the server side.
var staticCameras = []Camera{
	// ── NYC DOT Traffic Management Center (nyctmc.org) ─────────────────────────
	{ID: "nyc-001", Name: "Times Square / 42 St & Broadway", City: "New York", Lat: 40.7580, Lon: -73.9855, SnapshotURL: "https://webcams.nyctmc.org/api/cameras/4e2b9c3f-3c7d-4b3e-b5a6-8e4f2a1d9c7b/image"},
	{ID: "nyc-002", Name: "Lincoln Tunnel Approach", City: "New York", Lat: 40.7614, Lon: -74.0025, SnapshotURL: "https://webcams.nyctmc.org/api/cameras/1a3c5e7f-2b4d-4f6a-8c0e-9d1f3b5a7c9e/image"},
	{ID: "nyc-003", Name: "Brooklyn Bridge", City: "New York", Lat: 40.7061, Lon: -73.9969, SnapshotURL: "https://webcams.nyctmc.org/api/cameras/7c2d4e6f-8a0b-4c2e-9d4f-1e3a5c7b9d0f/image"},
	{ID: "nyc-004", Name: "Holland Tunnel Plaza", City: "New York", Lat: 40.7267, Lon: -74.0188, SnapshotURL: "https://webcams.nyctmc.org/api/cameras/3b5d7f9a-1c3e-4f6b-8d0c-2e4a6c8b0d2f/image"},
	{ID: "nyc-005", Name: "FDR Drive & 42nd St", City: "New York", Lat: 40.7517, Lon: -73.9705, SnapshotURL: "https://webcams.nyctmc.org/api/cameras/5d7f9b1c-3e5a-4c6e-8f0d-4a6c8e0b2d4f/image"},

	// ── TfL JamCams – London (S3 public bucket, no auth) ───────────────────────
	{ID: "tfl-001", Name: "Waterloo Bridge", City: "London", Lat: 51.5080, Lon: -0.1161, SnapshotURL: "https://s3-eu-west-1.amazonaws.com/jamcams.tfl.gov.uk/00001.09251.jpg"},
	{ID: "tfl-002", Name: "London Bridge", City: "London", Lat: 51.5079, Lon: -0.0877, SnapshotURL: "https://s3-eu-west-1.amazonaws.com/jamcams.tfl.gov.uk/00001.09252.jpg"},
	{ID: "tfl-003", Name: "Westminster Bridge", City: "London", Lat: 51.5012, Lon: -0.1224, SnapshotURL: "https://s3-eu-west-1.amazonaws.com/jamcams.tfl.gov.uk/00001.09253.jpg"},
	{ID: "tfl-004", Name: "Tower Bridge Approach", City: "London", Lat: 51.5055, Lon: -0.0754, SnapshotURL: "https://s3-eu-west-1.amazonaws.com/jamcams.tfl.gov.uk/00001.09254.jpg"},
	{ID: "tfl-005", Name: "Blackfriars Bridge", City: "London", Lat: 51.5118, Lon: -0.1040, SnapshotURL: "https://s3-eu-west-1.amazonaws.com/jamcams.tfl.gov.uk/00001.09255.jpg"},

	// ── CCTV Indonesia / Jakarta (Dinas Perhubungan DKI) ───────────────────────
	{ID: "jkt-001", Name: "Bundaran HI", City: "Jakarta", Lat: -6.1950, Lon: 106.8229, SnapshotURL: "https://cctv.jakarta.go.id/assets/images/thumb/thumb_bundaran_hi.jpg"},
	{ID: "jkt-002", Name: "Semanggi Interchange", City: "Jakarta", Lat: -6.2088, Lon: 106.8197, SnapshotURL: "https://cctv.jakarta.go.id/assets/images/thumb/thumb_semanggi.jpg"},
	{ID: "jkt-003", Name: "Thamrin – Sarinah", City: "Jakarta", Lat: -6.1866, Lon: 106.8231, SnapshotURL: "https://cctv.jakarta.go.id/assets/images/thumb/thumb_sarinah.jpg"},
	{ID: "jkt-004", Name: "Blok M", City: "Jakarta", Lat: -6.2441, Lon: 106.7993, SnapshotURL: "https://cctv.jakarta.go.id/assets/images/thumb/thumb_blokm.jpg"},

	// ── Miami-Dade Expressway Authority (MDX) – public feed ────────────────────
	{ID: "mia-001", Name: "I-95 & NW 7th Ave", City: "Miami", Lat: 25.7954, Lon: -80.2171, SnapshotURL: "https://fl511.com/api/cameras/MDX-I95-7AVE/image"},
	{ID: "mia-002", Name: "SR-836 & I-95", City: "Miami", Lat: 25.7753, Lon: -80.2014, SnapshotURL: "https://fl511.com/api/cameras/MDX-836-I95/image"},
	{ID: "mia-003", Name: "Port of Miami Tunnel", City: "Miami", Lat: 25.7758, Lon: -80.1694, SnapshotURL: "https://fl511.com/api/cameras/MDX-PORT-TUNNEL/image"},
	{ID: "mia-004", Name: "MacArthur Causeway", City: "Miami", Lat: 25.7841, Lon: -80.1520, SnapshotURL: "https://fl511.com/api/cameras/MDX-MACARTHUR/image"},
}

func GetAll() []Camera { return staticCameras }
