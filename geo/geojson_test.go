package geo_test

import (
	"context"
	"math"
	"os"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/geo"
	"github.com/davidkohl/goaixm/gml"
)

// makeSurfacePosList builds a minimal SurfaceType from a space-separated
// "lat lon lat lon ..." string.
func makeSurfacePosList(posList string) *aixm.Surface {
	return &aixm.Surface{
		Patches: &gml.SurfacePatches{
			PolygonPatch: []gml.PolygonPatch{
				{
					Exterior: &gml.AbstractRingProperty{
						LinearRing: &gml.LinearRing{
							PosList: &gml.DirectPositionList{Value: posList},
						},
					},
				},
			},
		},
	}
}

// TestAirspaceToGeoJSONBasic verifies that a simple airspace with a LinearRing
// produces a valid GeoJSON Polygon.
func TestAirspaceToGeoJSONBasic(t *testing.T) {
	a := &aixm.Airspace{}
	seq := uint32(1)
	zero := uint32(0)
	ts := aixm.AirspaceTimeSlice{
		TimeSliceCommon: aixm.TimeSliceCommon{Interpretation: "BASELINE", SequenceNumber: &seq, CorrectionNumber: &zero},
	}
	// Build a simple square geometry: lat lon pairs.
	surf := makeSurfacePosList("51.0 -1.0 51.0 0.0 52.0 0.0 52.0 -1.0 51.0 -1.0")
	ts.GeometryComponent = []aixm.AirspaceGeometryComponent{
		{

			TheAirspaceVolume: &aixm.AirspaceVolume{
				HorizontalProjection: &aixm.SurfaceProperty{Surface: surf},
			},
		},
	}
	a.TimeSlice = []aixm.AirspaceTimeSlice{ts}

	fc, err := geo.AirspaceToGeoJSON(a)
	if err != nil {
		t.Fatalf("AirspaceToGeoJSON: %v", err)
	}
	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(fc.Features))
	}
	poly, ok := fc.Features[0].Geometry.(geo.Polygon)
	if !ok {
		t.Fatalf("geometry is %T, want Polygon", fc.Features[0].Geometry)
	}
	if len(poly.Coordinates) != 1 {
		t.Errorf("expected 1 ring (exterior only), got %d", len(poly.Coordinates))
	}
	if len(poly.Coordinates[0]) != 5 {
		t.Errorf("expected 5 points in exterior ring, got %d", len(poly.Coordinates[0]))
	}
	// GeoJSON is [lon, lat]; first point should be lon=-1, lat=51.
	if poly.Coordinates[0][0] != [2]float64{-1.0, 51.0} {
		t.Errorf("first point = %v, want [-1,51]", poly.Coordinates[0][0])
	}
}

// TestAirspaceToGeoJSONWithHole verifies that interior rings (holes) are
// included as additional coordinate rings in the GeoJSON Polygon.
func TestAirspaceToGeoJSONWithHole(t *testing.T) {
	exterior := &gml.AbstractRingProperty{
		LinearRing: &gml.LinearRing{
			PosList: &gml.DirectPositionList{
				Value: "51.0 -1.0 51.0 1.0 53.0 1.0 53.0 -1.0 51.0 -1.0",
			},
		},
	}
	hole := gml.AbstractRingProperty{
		LinearRing: &gml.LinearRing{
			PosList: &gml.DirectPositionList{
				Value: "51.5 -0.5 51.5 0.5 52.5 0.5 52.5 -0.5 51.5 -0.5",
			},
		},
	}
	surf := &aixm.Surface{
		Patches: &gml.SurfacePatches{
			PolygonPatch: []gml.PolygonPatch{
				{Exterior: exterior, Interior: []gml.AbstractRingProperty{hole}},
			},
		},
	}

	a := buildAirspaceWithSurface(surf)
	fc, err := geo.AirspaceToGeoJSON(a)
	if err != nil {
		t.Fatalf("AirspaceToGeoJSON: %v", err)
	}
	poly := fc.Features[0].Geometry.(geo.Polygon)
	if len(poly.Coordinates) != 2 {
		t.Errorf("expected 2 rings (exterior + 1 hole), got %d", len(poly.Coordinates))
	}
}

// TestAirspaceToGeoJSONNoTimeslice verifies an error is returned for features
// with no timeslice.
func TestAirspaceToGeoJSONNoTimeslice(t *testing.T) {
	a := &aixm.Airspace{}
	_, err := geo.AirspaceToGeoJSON(a)
	if err == nil {
		t.Fatal("expected error for airspace with no timeslices")
	}
}

// TestAirspaceToGeoJSONDonlon runs AirspaceToGeoJSON against all DONLON
// airspaces that have inline geometry and verifies no error is returned.
func TestAirspaceToGeoJSONDonlon(t *testing.T) {
	f, err := os.Open("../testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	msg, err := goaixm.Parse(context.Background(), f, goaixm.Options{})
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	// 17 DONLON airspaces convert from the feature alone.  Two borrow a
	// boundary from a GeoBorder and one (EAMM) is assembled from contributor
	// airspaces; both are held elsewhere in the message, which only
	// AirspaceToGeoJSONWithRefs can resolve.  Exact counts keep a regression
	// from hiding.
	const wantAlone, wantWithRefs = 17, 20

	alone, withRefs := 0, 0
	for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
		if _, err := geo.AirspaceToGeoJSON(a); err == nil {
			alone++
		}
		if _, err := geo.AirspaceToGeoJSONWithRefs(a, msg); err == nil {
			withRefs++
		}
	}
	if alone != wantAlone {
		t.Errorf("DONLON airspaces converting without a message = %d, want %d", alone, wantAlone)
	}
	if withRefs != wantWithRefs {
		t.Errorf("DONLON airspaces converting with the message = %d, want %d", withRefs, wantWithRefs)
	}
}

// TestDiscretizeArc verifies that an arc over three points returns the expected
// number of interpolated coordinates and that the endpoints match the inputs.
func TestDiscretizeArc(t *testing.T) {
	// Three points on the unit circle at 0°, 90°, 180° (in lon/lat order).
	// Circumcenter is (0,0), radius 1.
	surf := &aixm.Surface{
		Patches: &gml.SurfacePatches{
			PolygonPatch: []gml.PolygonPatch{
				{
					Exterior: &gml.AbstractRingProperty{
						Ring: &gml.Ring{
							CurveMember: []gml.CurveMember{
								{
									Curve: &gml.Curve{
										Segments: &gml.CurveSegments{
											Arc: []gml.Arc{
												{
													// GML pos = lat lon
													Pos: []gml.DirectPosition{
														{Value: "0.0 1.0"},  // lat=0, lon=1 → point (1,0)
														{Value: "1.0 0.0"},  // lat=1, lon=0 → point (0,1)
														{Value: "0.0 -1.0"}, // lat=0, lon=-1 → point (-1,0)
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	a := buildAirspaceWithSurface(surf)
	fc, err := geo.AirspaceToGeoJSON(a)
	if err != nil {
		t.Fatalf("AirspaceToGeoJSON with arc: %v", err)
	}
	poly := fc.Features[0].Geometry.(geo.Polygon)
	coords := poly.Coordinates[0]

	// A 180-degree sweep uses half of DefaultArcPoints, and the ring is then
	// closed back on its first position as RFC 7946 requires.
	wantArc := geo.DefaultArcPoints/2 + 1
	if len(coords) != wantArc+1 {
		t.Errorf("arc: expected %d points (%d arc + closing), got %d",
			wantArc+1, wantArc, len(coords))
	}

	// The ring must be closed.
	if coords[0] != coords[len(coords)-1] {
		t.Errorf("arc ring not closed: first %v, last %v", coords[0], coords[len(coords)-1])
	}

	// The two arc endpoints are (1,0) and (-1,0); orientation decides which
	// comes first, so accept either order.
	first, last := coords[0], coords[len(coords)-2]
	okForward := approxEqual(first[0], 1.0, 1e-6) && approxEqual(last[0], -1.0, 1e-6)
	okReverse := approxEqual(first[0], -1.0, 1e-6) && approxEqual(last[0], 1.0, 1e-6)
	if !okForward && !okReverse {
		t.Errorf("arc endpoints = %v .. %v, want (1,0) and (-1,0) in some order", first, last)
	}
	if !approxEqual(first[1], 0.0, 1e-6) || !approxEqual(last[1], 0.0, 1e-6) {
		t.Errorf("arc endpoints not on the equator: %v .. %v", first, last)
	}
}

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// buildAirspaceWithSurface is a test helper.
func buildAirspaceWithSurface(surf *aixm.Surface) *aixm.Airspace {
	a := &aixm.Airspace{}
	seq := uint32(1)
	zero := uint32(0)
	ts := aixm.AirspaceTimeSlice{
		TimeSliceCommon: aixm.TimeSliceCommon{Interpretation: "BASELINE", SequenceNumber: &seq, CorrectionNumber: &zero},
	}
	ts.GeometryComponent = []aixm.AirspaceGeometryComponent{
		{

			TheAirspaceVolume: &aixm.AirspaceVolume{
				HorizontalProjection: &aixm.SurfaceProperty{Surface: surf},
			},
		},
	}
	a.TimeSlice = []aixm.AirspaceTimeSlice{ts}
	return a
}
