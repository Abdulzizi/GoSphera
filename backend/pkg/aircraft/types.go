package aircraft

// AircraftState holds the real-time position and telemetry of a single aircraft
// as reported by the OpenSky Network state vector API.
type AircraftState struct {
	ICAO24      string  `json:"icao24"`
	Callsign    string  `json:"callsign"`
	Country     string  `json:"country"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
	Altitude    float64 `json:"altitude"`    // metres (barometric)
	Velocity    float64 `json:"velocity"`    // m/s
	Heading     float64 `json:"heading"`     // degrees clockwise from north
	OnGround    bool    `json:"on_ground"`
	LastContact int64   `json:"last_contact"` // Unix timestamp
}
