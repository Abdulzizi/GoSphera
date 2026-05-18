package spatial

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const overpassAPIURL = "https://overpass-api.de/api/interpreter"

// Client sends queries to the Overpass API and returns GeoJSON.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a Client with a 30-second timeout.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Query compiles an Overpass QL query from tags and bbox, executes it,
// and returns the result as a GeoJSON FeatureCollection.
func (c *Client) Query(tags []Tag, bbox BoundingBox) (FeatureCollection, error) {
	query := c.compileOverpassQL(tags, bbox)

	req, err := http.NewRequest(http.MethodPost, overpassAPIURL, strings.NewReader(query))
	if err != nil {
		return FeatureCollection{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return FeatureCollection{}, fmt.Errorf("overpass request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return FeatureCollection{}, fmt.Errorf("overpass status %d: %s", resp.StatusCode, body)
	}

	return c.parseOSMResponse(resp.Body)
}

// compileOverpassQL builds an Overpass QL string that ANDs all tags together
// within the given bounding box.
func (c *Client) compileOverpassQL(tags []Tag, bbox BoundingBox) string {
	// bbox format Overpass expects: south,west,north,east
	bboxPart := fmt.Sprintf("[bbox:%.6f,%.6f,%.6f,%.6f]",
		bbox.MinLat, bbox.MinLon, bbox.MaxLat, bbox.MaxLon)

	var tagFilter strings.Builder
	for _, t := range tags {
		fmt.Fprintf(&tagFilter, `["%s"="%s"]`, t.Key, t.Value)
	}
	tf := tagFilter.String()

	return fmt.Sprintf(
		`[out:json]%s;(node%s;way%s;relation%s;);out geom;`,
		bboxPart, tf, tf, tf,
	)
}

// osmResponse mirrors the JSON structure returned by the Overpass API.
type osmResponse struct {
	Elements []struct {
		Type     string  `json:"type"`
		ID       int64   `json:"id"`
		Lat      float64 `json:"lat"`
		Lon      float64 `json:"lon"`
		Geometry []struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"geometry"`
		Tags map[string]string `json:"tags"`
	} `json:"elements"`
}

// parseOSMResponse converts an Overpass API JSON body into a FeatureCollection.
func (c *Client) parseOSMResponse(body io.Reader) (FeatureCollection, error) {
	var raw osmResponse
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return FeatureCollection{}, fmt.Errorf("decode overpass JSON: %w", err)
	}

	fc := NewFeatureCollection()

	for _, el := range raw.Elements {
		props := make(map[string]interface{}, len(el.Tags)+1)
		props["osm_id"] = el.ID
		props["osm_type"] = el.Type
		for k, v := range el.Tags {
			props[k] = v
		}

		var geom Geometry

		switch el.Type {
		case "node":
			geom = Geometry{
				Type:        "Point",
				Coordinates: [2]float64{el.Lon, el.Lat},
			}

		case "way":
			if len(el.Geometry) == 0 {
				continue
			}
			coords := make([][2]float64, len(el.Geometry))
			for i, g := range el.Geometry {
				coords[i] = [2]float64{g.Lon, g.Lat}
			}
			// Closed ring → Polygon, open → LineString
			if len(coords) >= 4 && coords[0] == coords[len(coords)-1] {
				geom = Geometry{Type: "Polygon", Coordinates: [][][2]float64{coords}}
			} else {
				geom = Geometry{Type: "LineString", Coordinates: coords}
			}

		case "relation":
			if len(el.Geometry) == 0 {
				continue
			}
			geom = Geometry{
				Type:        "Point",
				Coordinates: [2]float64{el.Geometry[0].Lon, el.Geometry[0].Lat},
			}

		default:
			continue
		}

		fc.Features = append(fc.Features, Feature{
			Type:       "Feature",
			Geometry:   geom,
			Properties: props,
		})
	}

	return fc, nil
}
