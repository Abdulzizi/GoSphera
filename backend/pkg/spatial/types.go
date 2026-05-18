package spatial

// Geometry represents a GeoJSON geometry object (RFC 7946).
type Geometry struct {
	Type        string      `json:"type"`
	Coordinates interface{} `json:"coordinates"`
}

// Feature represents a GeoJSON Feature object.
type Feature struct {
	Type       string                 `json:"type"`
	Geometry   Geometry               `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
}

// FeatureCollection represents a GeoJSON FeatureCollection.
type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

// NewFeatureCollection returns an empty FeatureCollection.
func NewFeatureCollection() FeatureCollection {
	return FeatureCollection{
		Type:     "FeatureCollection",
		Features: []Feature{},
	}
}

// Tag is a key-value pair for Overpass QL filters.
type Tag struct {
	Key   string
	Value string
}

// BoundingBox is a geographic bounding box (south-west to north-east).
type BoundingBox struct {
	MinLat float64
	MinLon float64
	MaxLat float64
	MaxLon float64
}
