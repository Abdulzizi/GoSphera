package maritime

// ShipState holds the last known position and metadata for an AIS-tracked vessel.
type ShipState struct {
	MMSI     int     `json:"mmsi"`
	Name     string  `json:"name"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Speed    float64 `json:"speed"`    // knots
	Heading  float64 `json:"heading"`  // degrees true
	ShipType int     `json:"ship_type"`
	LastSeen int64   `json:"last_seen"` // unix seconds
}
