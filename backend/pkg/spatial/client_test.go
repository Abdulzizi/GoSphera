package spatial

import (
	"strings"
	"testing"
)

func TestCompileOverpassQL_NoTags(t *testing.T) {
	c := NewClient()
	bbox := BoundingBox{MinLat: 25.7, MinLon: -80.5, MaxLat: 26.0, MaxLon: -80.0}
	q := c.compileOverpassQL(nil, bbox)

	if !strings.Contains(q, "[out:json]") {
		t.Error("expected [out:json]")
	}
	if !strings.Contains(q, "[bbox:25.700000,-80.500000,26.000000,-80.000000]") {
		t.Errorf("unexpected bbox in query: %s", q)
	}
	if !strings.Contains(q, "out geom;") {
		t.Error("expected out geom;")
	}
}

func TestCompileOverpassQL_WithTags(t *testing.T) {
	c := NewClient()
	tags := []Tag{{Key: "amenity", Value: "cafe"}}
	bbox := BoundingBox{MinLat: 25.7, MinLon: -80.5, MaxLat: 26.0, MaxLon: -80.0}
	q := c.compileOverpassQL(tags, bbox)

	if !strings.Contains(q, `["amenity"="cafe"]`) {
		t.Errorf("expected tag filter in query: %s", q)
	}
	// All three element types must carry the filter
	for _, elemType := range []string{"node", "way", "relation"} {
		if !strings.Contains(q, elemType+`["amenity"="cafe"]`) {
			t.Errorf("expected %s with tag filter in query: %s", elemType, q)
		}
	}
}

func TestCompileOverpassQL_MultipleTags(t *testing.T) {
	c := NewClient()
	tags := []Tag{
		{Key: "amenity", Value: "cafe"},
		{Key: "outdoor_seating", Value: "yes"},
	}
	bbox := BoundingBox{}
	q := c.compileOverpassQL(tags, bbox)

	if !strings.Contains(q, `["amenity"="cafe"]`) {
		t.Error("missing first tag")
	}
	if !strings.Contains(q, `["outdoor_seating"="yes"]`) {
		t.Error("missing second tag")
	}
}
