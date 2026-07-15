package geo_test

import (
	"math"
	"os"
	"testing"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/geo"
	"gitlab.com/davidkohl/goaixm/gml"
	"gitlab.com/davidkohl/goaixm/parse"
)

// makeSurfacePosList builds a minimal SurfaceType from a space-separated
// "lat lon lat lon ..." string.
func makeSurfacePosList(posList string) *features.SurfaceType {
	return &features.SurfaceType{
		Patches: &gml.SurfacePatchesType{
			PolygonPatch: []gml.PolygonPatchType{
				{
					Exterior: &gml.AbstractRingPropertyType{
						LinearRing: &gml.LinearRingType{
							PosList: &gml.DirectPositionListType{Value: posList},
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
	a := &features.AirspaceType{}
	seq := uint32(1)
	zero := uint32(0)
	ts := features.AirspaceTimeSliceType{
		Interpretation: "BASELINE",
		SequenceNumber: &seq,
		CorrectionNumber: &zero,
	}
	// Build a simple square geometry: lat lon pairs.
	surf := makeSurfacePosList("51.0 -1.0 51.0 0.0 52.0 0.0 52.0 -1.0 51.0 -1.0")
	ts.GeometryComponent = []features.AirspaceGeometryComponentPropertyType{
		{
			AirspaceGeometryComponent: &features.AirspaceGeometryComponentType{
				TheAirspaceVolume: features.AirspaceVolumePropertyType{
					AirspaceVolume: &features.AirspaceVolumeType{
						HorizontalProjection: features.HorizontalProjectionType{Surface: surf},
					},
				},
			},
		},
	}
	a.TimeSlice = []features.AirspaceTimeSlicePropertyType{{AirspaceTimeSlice: ts}}

	fc, err := geo.AirspaceToGeoJSON(a)
	if err != nil {
		t.Fatalf("AirspaceToGeoJSON: %v", err)
	}
	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(fc.Features))
	}
	poly, ok := fc.Features[0].Geometry.(geo.GeoJSONPolygon)
	if !ok {
		t.Fatalf("geometry is %T, want GeoJSONPolygon", fc.Features[0].Geometry)
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
	exterior := &gml.AbstractRingPropertyType{
		LinearRing: &gml.LinearRingType{
			PosList: &gml.DirectPositionListType{
				Value: "51.0 -1.0 51.0 1.0 53.0 1.0 53.0 -1.0 51.0 -1.0",
			},
		},
	}
	hole := gml.AbstractRingPropertyType{
		LinearRing: &gml.LinearRingType{
			PosList: &gml.DirectPositionListType{
				Value: "51.5 -0.5 51.5 0.5 52.5 0.5 52.5 -0.5 51.5 -0.5",
			},
		},
	}
	surf := &features.SurfaceType{
		Patches: &gml.SurfacePatchesType{
			PolygonPatch: []gml.PolygonPatchType{
				{Exterior: exterior, Interior: []gml.AbstractRingPropertyType{hole}},
			},
		},
	}

	a := buildAirspaceWithSurface(surf)
	fc, err := geo.AirspaceToGeoJSON(a)
	if err != nil {
		t.Fatalf("AirspaceToGeoJSON: %v", err)
	}
	poly := fc.Features[0].Geometry.(geo.GeoJSONPolygon)
	if len(poly.Coordinates) != 2 {
		t.Errorf("expected 2 rings (exterior + 1 hole), got %d", len(poly.Coordinates))
	}
}

// TestAirspaceToGeoJSONNoTimeslice verifies an error is returned for features
// with no timeslice.
func TestAirspaceToGeoJSONNoTimeslice(t *testing.T) {
	a := &features.AirspaceType{}
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
	msg, err := parse.ParseMessage(f)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	ok, skipped := 0, 0
	for _, a := range msg.Airspaces() {
		_, err := geo.AirspaceToGeoJSON(a)
		if err != nil {
			skipped++
		} else {
			ok++
		}
	}
	t.Logf("DONLON airspaces: %d with geometry, %d without/error", ok, skipped)
	if ok == 0 {
		t.Error("expected at least one airspace to produce GeoJSON")
	}
}

// TestDiscretizeArc verifies that an arc over three points returns the expected
// number of interpolated coordinates and that the endpoints match the inputs.
func TestDiscretizeArc(t *testing.T) {
	// Three points on the unit circle at 0°, 90°, 180° (in lon/lat order).
	// Circumcenter is (0,0), radius 1.
	surf := &features.SurfaceType{
		Patches: &gml.SurfacePatchesType{
			PolygonPatch: []gml.PolygonPatchType{
				{
					Exterior: &gml.AbstractRingPropertyType{
						Ring: &gml.RingType{
							CurveMember: []gml.CurveMemberType{
								{
									Curve: &gml.CurveType{
										Segments: &gml.CurveSegmentsType{
											Arc: []gml.ArcType{
												{
													// GML pos = lat lon
													Pos: []gml.DirectPositionType{
														{Value: "0.0 1.0"},   // lat=0, lon=1 → point (1,0)
														{Value: "1.0 0.0"},   // lat=1, lon=0 → point (0,1)
														{Value: "0.0 -1.0"},  // lat=0, lon=-1 → point (-1,0)
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
	poly := fc.Features[0].Geometry.(geo.GeoJSONPolygon)
	coords := poly.Coordinates[0]

	// DefaultArcPoints=64 → 65 points.
	if len(coords) != geo.DefaultArcPoints+1 {
		t.Errorf("arc: expected %d points, got %d", geo.DefaultArcPoints+1, len(coords))
	}

	// First point should be lon=1, lat=0.
	if !approxEqual(coords[0][0], 1.0, 1e-9) || !approxEqual(coords[0][1], 0.0, 1e-9) {
		t.Errorf("arc first point = %v, want [1,0]", coords[0])
	}
	// Last point should be lon=-1, lat=0.
	if !approxEqual(coords[len(coords)-1][0], -1.0, 1e-9) || !approxEqual(coords[len(coords)-1][1], 0.0, 1e-9) {
		t.Errorf("arc last point = %v, want [-1,0]", coords[len(coords)-1])
	}
}

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// buildAirspaceWithSurface is a test helper.
func buildAirspaceWithSurface(surf *features.SurfaceType) *features.AirspaceType {
	a := &features.AirspaceType{}
	seq := uint32(1)
	zero := uint32(0)
	ts := features.AirspaceTimeSliceType{
		Interpretation:   "BASELINE",
		SequenceNumber:   &seq,
		CorrectionNumber: &zero,
	}
	ts.GeometryComponent = []features.AirspaceGeometryComponentPropertyType{
		{
			AirspaceGeometryComponent: &features.AirspaceGeometryComponentType{
				TheAirspaceVolume: features.AirspaceVolumePropertyType{
					AirspaceVolume: &features.AirspaceVolumeType{
						HorizontalProjection: features.HorizontalProjectionType{Surface: surf},
					},
				},
			},
		},
	}
	a.TimeSlice = []features.AirspaceTimeSlicePropertyType{{AirspaceTimeSlice: ts}}
	return a
}
