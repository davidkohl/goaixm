package geo_test

import (
	"testing"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/geo"
	"gitlab.com/davidkohl/goaixm/gml"
)

func TestFromPoint(t *testing.T) {
	p := &features.ElevatedPointType{
		Pos: &gml.DirectPositionType{Value: "51.4775 -0.4614"},
	}
	c, err := geo.FromPoint(p)
	if err != nil {
		t.Fatalf("FromPoint: %v", err)
	}
	if c.Lat != 51.4775 || c.Lon != -0.4614 {
		t.Errorf("got lat=%g lon=%g, want 51.4775 -0.4614", c.Lat, c.Lon)
	}
}

func TestFromPointNil(t *testing.T) {
	_, err := geo.FromPoint(nil)
	if err == nil {
		t.Error("expected error for nil ElevatedPoint")
	}
}

func TestWKTPoint(t *testing.T) {
	c := geo.Coordinate{Lon: -0.4614, Lat: 51.4775}
	got := geo.WKTPoint(c)
	want := "POINT(-0.4614 51.4775)"
	if got != want {
		t.Errorf("WKTPoint = %q, want %q", got, want)
	}
}

func TestWKTPolygon(t *testing.T) {
	coords := []geo.Coordinate{
		{Lon: 0, Lat: 0},
		{Lon: 1, Lat: 0},
		{Lon: 1, Lat: 1},
		{Lon: 0, Lat: 0},
	}
	got := geo.WKTPolygon(coords)
	want := "POLYGON((0 0,1 0,1 1,0 0))"
	if got != want {
		t.Errorf("WKTPolygon = %q, want %q", got, want)
	}
}

func TestWKTPolygonEmpty(t *testing.T) {
	got := geo.WKTPolygon(nil)
	if got != "POLYGON EMPTY" {
		t.Errorf("expected POLYGON EMPTY, got %q", got)
	}
}

func TestBoundingBox(t *testing.T) {
	coords := []geo.Coordinate{
		{Lon: 1, Lat: 2},
		{Lon: -1, Lat: 5},
		{Lon: 3, Lat: 0},
	}
	minLon, minLat, maxLon, maxLat := geo.BoundingBox(coords)
	if minLon != -1 || minLat != 0 || maxLon != 3 || maxLat != 5 {
		t.Errorf("BoundingBox = (%g,%g,%g,%g), want (-1,0,3,5)", minLon, minLat, maxLon, maxLat)
	}
}

func TestFromSurface(t *testing.T) {
	s := &features.ElevatedSurfaceType{
		Patches: &gml.SurfacePatchesType{
			PolygonPatch: []gml.PolygonPatchType{
				{
					Exterior: &gml.AbstractRingPropertyType{
						LinearRing: &gml.LinearRingType{
							PosList: &gml.DirectPositionListType{
								Value: "0 0 1 0 1 1 0 0",
							},
						},
					},
				},
			},
		},
	}
	coords, err := geo.FromSurface(s)
	if err != nil {
		t.Fatalf("FromSurface: %v", err)
	}
	if len(coords) != 4 {
		t.Errorf("expected 4 coords, got %d", len(coords))
	}
}
