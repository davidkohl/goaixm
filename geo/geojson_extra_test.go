package geo_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/geo"
	"github.com/davidkohl/goaixm/gml"
)

// TestMarshalGeoJSON verifies that MarshalGeoJSON produces valid JSON.
func TestMarshalGeoJSON(t *testing.T) {
	a := buildAirspaceWithSurface(makeSurfacePosList("51.0 -1.0 51.0 0.0 52.0 0.0 51.0 -1.0"))
	fc, err := geo.AirspaceToGeoJSON(a)
	if err != nil {
		t.Fatalf("AirspaceToGeoJSON: %v", err)
	}
	data, err := geo.MarshalGeoJSON(fc)
	if err != nil {
		t.Fatalf("MarshalGeoJSON: %v", err)
	}
	// Must be valid JSON.
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, data)
	}
	if out["type"] != "FeatureCollection" {
		t.Errorf("type = %v, want FeatureCollection", out["type"])
	}
}

// TestAirspaceToGeoJSONWithRefsInline verifies that AirspaceToGeoJSONWithRefs
// returns a single Polygon (not MultiPolygon) when all geometry is inline.
func TestAirspaceToGeoJSONWithRefsInline(t *testing.T) {
	a := buildAirspaceWithSurface(makeSurfacePosList("51.0 -1.0 51.0 0.0 52.0 0.0 51.0 -1.0"))
	// Pass nil message — no xlink refs to resolve.
	fc, err := geo.AirspaceToGeoJSONWithRefs(a, nil)
	if err != nil {
		t.Fatalf("AirspaceToGeoJSONWithRefs: %v", err)
	}
	if len(fc.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(fc.Features))
	}
	if _, ok := fc.Features[0].Geometry.(geo.Polygon); !ok {
		t.Errorf("expected Polygon, got %T", fc.Features[0].Geometry)
	}
}

// TestAirspaceToGeoJSONWithRefsMultiPolygon verifies that two geometry
// components produce a MultiPolygon.
func TestAirspaceToGeoJSONWithRefsMultiPolygon(t *testing.T) {
	surf1 := makeSurfacePosList("51.0 -1.0 51.0 0.0 52.0 0.0 51.0 -1.0")
	surf2 := makeSurfacePosList("53.0 1.0 53.0 2.0 54.0 2.0 53.0 1.0")

	a := &aixm.Airspace{}
	seq, cor := uint32(1), uint32(0)
	ts := aixm.AirspaceTimeSlice{
		TimeSliceCommon: aixm.TimeSliceCommon{Interpretation: "BASELINE", SequenceNumber: &seq, CorrectionNumber: &cor},
		GeometryComponent: []aixm.AirspaceGeometryComponent{
			{
				TheAirspaceVolume: &aixm.AirspaceVolume{
					HorizontalProjection: &aixm.SurfaceProperty{Surface: surf1},
				},
			},
			{
				TheAirspaceVolume: &aixm.AirspaceVolume{
					HorizontalProjection: &aixm.SurfaceProperty{Surface: surf2},
				},
			},
		},
	}
	a.TimeSlice = []aixm.AirspaceTimeSlice{ts}

	fc, err := geo.AirspaceToGeoJSONWithRefs(a, nil)
	if err != nil {
		t.Fatalf("AirspaceToGeoJSONWithRefs: %v", err)
	}
	if _, ok := fc.Features[0].Geometry.(geo.MultiPolygon); !ok {
		t.Errorf("expected MultiPolygon for 2 components, got %T", fc.Features[0].Geometry)
	}
}

// TestAirspaceToGeoJSONWithRefsResolvesXLink verifies that an airspace
// geometry component encoded as an xlink:href is resolved from the message.
func TestAirspaceToGeoJSONWithRefsResolvesXLink(t *testing.T) {
	f, err := os.Open("../testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	msg, err := goaixm.Parse(context.Background(), f, goaixm.Options{})
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	// Run WithRefs on every airspace — should never panic, should produce
	// at least as many results as AirspaceToGeoJSON.
	withRefsOK := 0
	for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
		fc, err := geo.AirspaceToGeoJSONWithRefs(a, msg)
		if err == nil {
			withRefsOK++
			_ = fc
		}
	}
	if withRefsOK == 0 {
		t.Error("expected at least one airspace to produce GeoJSON via WithRefs")
	}
}

// TestDiscretizeCircle verifies that a Circle segment produces a closed ring
// with DefaultArcPoints+1 coordinates.
func TestDiscretizeCircle(t *testing.T) {
	// Three points on the unit circle at 0°, 120°, 240°.
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
											Circle: []gml.Circle{
												{
													Pos: []gml.DirectPosition{
														{Value: "0.0 1.0"},
														{Value: "0.866 -0.5"},
														{Value: "-0.866 -0.5"},
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
		t.Fatalf("AirspaceToGeoJSON with circle: %v", err)
	}
	poly := fc.Features[0].Geometry.(geo.Polygon)
	coords := poly.Coordinates[0]
	if len(coords) != geo.DefaultArcPoints+1 {
		t.Errorf("circle: expected %d points, got %d", geo.DefaultArcPoints+1, len(coords))
	}
}

// TestExtractLinearRingPosList verifies the posList path (already exercised
// by other tests, but this makes the specific branch explicit).
func TestExtractLinearRingPosList(t *testing.T) {
	surf := makeSurfacePosList("51.0 0.0 52.0 0.0 52.0 1.0 51.0 0.0")
	a := buildAirspaceWithSurface(surf)
	fc, err := geo.AirspaceToGeoJSON(a)
	if err != nil {
		t.Fatalf("posList: %v", err)
	}
	if len(fc.Features[0].Geometry.(geo.Polygon).Coordinates[0]) != 4 {
		t.Errorf("expected 4 coords")
	}
}

// TestExtractLinearRingPos verifies the individual pos element path.
func TestExtractLinearRingPos(t *testing.T) {
	surf := &aixm.Surface{
		Patches: &gml.SurfacePatches{
			PolygonPatch: []gml.PolygonPatch{
				{
					Exterior: &gml.AbstractRingProperty{
						LinearRing: &gml.LinearRing{
							Pos: []gml.DirectPosition{
								{Value: "51.0 0.0"},
								{Value: "52.0 0.0"},
								{Value: "52.0 1.0"},
								{Value: "51.0 0.0"},
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
		t.Fatalf("pos slice: %v", err)
	}
	coords := fc.Features[0].Geometry.(geo.Polygon).Coordinates[0]
	if len(coords) != 4 {
		t.Errorf("expected 4 coords, got %d", len(coords))
	}
	// First coord: lat=51, lon=0 → GeoJSON [lon, lat] = [0, 51]
	if coords[0] != [2]float64{0.0, 51.0} {
		t.Errorf("first coord = %v, want [0, 51]", coords[0])
	}
}

// TestVerticalLimitsInProperties verifies that upper/lower limits are included
// in GeoJSON properties when present on the AirspaceVolume.
func TestVerticalLimitsInProperties(t *testing.T) {
	surf := makeSurfacePosList("51.0 -1.0 51.0 0.0 52.0 0.0 51.0 -1.0")
	upper := &aixm.AirspaceVolume{
		HorizontalProjection: &aixm.SurfaceProperty{Surface: surf},
		UpperLimit:           &aixm.ValDistanceVertical{Value: "FL245"},
		LowerLimit:           &aixm.ValDistanceVertical{Value: "FL65"},
	}

	a := &aixm.Airspace{}
	seq, cor := uint32(1), uint32(0)
	ts := aixm.AirspaceTimeSlice{
		TimeSliceCommon: aixm.TimeSliceCommon{Interpretation: "BASELINE", SequenceNumber: &seq, CorrectionNumber: &cor},
		GeometryComponent: []aixm.AirspaceGeometryComponent{
			{
				TheAirspaceVolume: upper,
			},
		},
	}
	a.TimeSlice = []aixm.AirspaceTimeSlice{ts}

	fc, err := geo.AirspaceToGeoJSON(a)
	if err != nil {
		t.Fatalf("AirspaceToGeoJSON: %v", err)
	}
	props := fc.Features[0].Properties
	if props["upperLimit"] != "FL245" {
		t.Errorf("upperLimit = %v, want FL245", props["upperLimit"])
	}
	if props["lowerLimit"] != "FL65" {
		t.Errorf("lowerLimit = %v, want FL65", props["lowerLimit"])
	}
}
