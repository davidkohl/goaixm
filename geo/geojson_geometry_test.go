package geo_test

import (
	"encoding/xml"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gitlab.com/davidkohl/goaixm"
	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/geo"
	"gitlab.com/davidkohl/goaixm/gml"
	"gitlab.com/davidkohl/goaixm/types"
)

const donlonPath = "../testdata/donlon/EA_AIP_DS_FULL_20170701.xml"

// ringsOf returns every linear ring in a feature collection.
func ringsOf(fc *geo.GeoJSONFeatureCollection) [][][2]float64 {
	var out [][][2]float64
	for _, f := range fc.Features {
		switch g := f.Geometry.(type) {
		case geo.GeoJSONPolygon:
			out = append(out, g.Coordinates...)
		case geo.GeoJSONMultiPolygon:
			for _, poly := range g.Coordinates {
				out = append(out, poly...)
			}
		}
	}
	return out
}

// checkRing asserts the RFC 7946 requirements for a linear ring.
func checkRing(t *testing.T, label string, ring [][2]float64, exterior bool) {
	t.Helper()
	if len(ring) < 4 {
		t.Errorf("%s: ring has %d positions, need at least 4", label, len(ring))
		return
	}
	if ring[0] != ring[len(ring)-1] {
		t.Errorf("%s: ring is not closed (%v .. %v)", label, ring[0], ring[len(ring)-1])
	}
	var area float64
	for i := 0; i < len(ring)-1; i++ {
		area += ring[i][0]*ring[i+1][1] - ring[i+1][0]*ring[i][1]
	}
	if exterior && area < 0 {
		t.Errorf("%s: exterior ring is clockwise, want counter-clockwise", label)
	}
}

// TestAirspaceToGeoJSONDonlonAll requires that every airspace in the reference
// dataset converts, and that every ring it produces is valid GeoJSON.
func TestAirspaceToGeoJSONDonlonAll(t *testing.T) {
	msg, err := goaixm.ParseFile(donlonPath)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	airspaces := msg.Airspaces()
	if len(airspaces) == 0 {
		t.Fatal("no airspaces in the reference dataset")
	}

	for _, a := range airspaces {
		name := a.GetIdentifier()
		if len(a.TimeSlice) > 0 {
			if d := a.TimeSlice[0].AirspaceTimeSlice.Designator; d != nil {
				name = d.Value
			}
		}

		fc, err := geo.AirspaceToGeoJSONWithRefs(a, msg)
		if err != nil {
			t.Errorf("airspace %s: %v", name, err)
			continue
		}
		for i, ring := range ringsOf(fc) {
			checkRing(t, name, ring, i == 0)
		}
	}
}

// TestDonlonAuxiliaryDatasets converts the temporality and digital NOTAM
// samples, which also carry by-centre geometry.
func TestDonlonAuxiliaryDatasets(t *testing.T) {
	var paths []string
	for _, dir := range []string{"../testdata/donlon/temporality", "../testdata/donlon/digitalNOTAM"} {
		matches, err := filepath.Glob(filepath.Join(dir, "*.xml"))
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, matches...)
	}
	if len(paths) == 0 {
		t.Skip("auxiliary DONLON datasets not present")
	}

	for _, path := range paths {
		msg, err := goaixm.ParseFile(path)
		if err != nil {
			// A few of the digital NOTAM samples are not well-formed XML
			// (an unescaped "--" inside a comment, a mismatched tag).  That
			// is a property of the samples, not of geometry conversion.
			t.Logf("%s: skipped, will not parse: %v", filepath.Base(path), err)
			continue
		}
		for _, a := range msg.Airspaces() {
			fc, err := geo.AirspaceToGeoJSONWithRefs(a, msg)
			if err != nil {
				// Not every sample carries geometry; only malformed output matters.
				continue
			}
			for i, ring := range ringsOf(fc) {
				checkRing(t, filepath.Base(path)+" "+a.GetIdentifier(), ring, i == 0)
			}
		}
	}
}

// TestCircleByCenterPointGeodesicRadius checks that every vertex of a
// by-centre circle lies at the declared radius, measured with the Vincenty
// inverse solution — an independent code path from the direct solution used to
// generate the vertices.
func TestCircleByCenterPointGeodesicRadius(t *testing.T) {
	const (
		centreLat = 52.36666666666667
		centreLon = -22.1
	)
	tests := []struct {
		name       string
		radius     float64
		uom        string
		wantMetres float64
	}{
		{"15 NM", 15, "NM", 15 * 1852},
		{"10 [nmi_i]", 10, "[nmi_i]", 10 * 1852}, // UCUM spelling
		{"20 km", 20, "km", 20000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ring := circleRing(t, centreLat, centreLon, tc.radius, tc.uom)

			if len(ring) != geo.DefaultArcPoints+1 {
				t.Errorf("got %d positions, want %d", len(ring), geo.DefaultArcPoints+1)
			}
			if ring[0] != ring[len(ring)-1] {
				t.Errorf("ring is not closed: %v .. %v", ring[0], ring[len(ring)-1])
			}
			for i, c := range ring {
				d := geo.GeodesicDistance(centreLat, centreLon, c[1], c[0])
				if math.Abs(d-tc.wantMetres) > 0.5 {
					t.Errorf("vertex %d is %.3f m from the centre, want %.3f m",
						i, d, tc.wantMetres)
					break
				}
			}
		})
	}
}

// circleRing builds an airspace around a single CircleByCenterPoint and
// returns its exterior ring.
func circleRing(t *testing.T, lat, lon, radius float64, uom string) [][2]float64 {
	t.Helper()
	surf := &features.SurfaceType{
		Patches: &gml.SurfacePatchesType{
			PolygonPatch: []gml.PolygonPatchType{{
				Exterior: &gml.AbstractRingPropertyType{
					Ring: &gml.RingType{
						CurveMember: []gml.CurveMemberType{{
							Curve: &gml.CurveType{
								Segments: &gml.CurveSegmentsType{
									CircleByCenterPoint: []gml.CircleByCenterPointType{{
										PosList: &gml.DirectPositionListType{
											Value: formatLatLon(lat, lon),
										},
										Radius: gml.MeasureType{Value: radius, Uom: uom},
									}},
								},
							},
						}},
					},
				},
			}},
		},
	}

	fc, err := geo.AirspaceToGeoJSON(buildAirspaceWithSurface(surf))
	if err != nil {
		t.Fatalf("AirspaceToGeoJSON: %v", err)
	}
	poly, ok := fc.Features[0].Geometry.(geo.GeoJSONPolygon)
	if !ok {
		t.Fatalf("geometry is %T, want a Polygon", fc.Features[0].Geometry)
	}
	return poly.Coordinates[0]
}

func formatLatLon(lat, lon float64) string {
	return strconv.FormatFloat(lat, 'g', -1, 64) + " " +
		strconv.FormatFloat(lon, 'g', -1, 64)
}

// TestUnknownRadiusUnitIsRejected pins the decision to fail on an unrecognised
// unit rather than assume one: silently treating nautical miles as metres
// would shrink a circle by a factor of 1852.
func TestUnknownRadiusUnitIsRejected(t *testing.T) {
	surf := &features.SurfaceType{
		Patches: &gml.SurfacePatchesType{
			PolygonPatch: []gml.PolygonPatchType{{
				Exterior: &gml.AbstractRingPropertyType{
					Ring: &gml.RingType{
						CurveMember: []gml.CurveMemberType{{
							Curve: &gml.CurveType{
								Segments: &gml.CurveSegmentsType{
									CircleByCenterPoint: []gml.CircleByCenterPointType{{
										PosList: &gml.DirectPositionListType{Value: "52.0 -22.0"},
										Radius:  gml.MeasureType{Value: 15, Uom: "FURLONGS"},
									}},
								},
							},
						}},
					},
				},
			}},
		},
	}
	if _, err := geo.AirspaceToGeoJSON(buildAirspaceWithSurface(surf)); err == nil {
		t.Error("expected an error for an unrecognised radius unit, got nil")
	}
}

// TestSegmentOrderPreserved verifies that a curve mixing straight and curved
// segments is assembled in document order.  Re-ordering the segments by kind
// would produce a self-intersecting boundary.
func TestSegmentOrderPreserved(t *testing.T) {
	const doc = `<segments xmlns="http://www.opengis.net/gml/3.2">
	  <GeodesicString><posList>55.233333 -36.166666 55.231163 -36.894373</posList></GeodesicString>
	  <ArcByCenterPoint numArc="1">
	    <posList>55.2333333333333 -36.166666666666664</posList>
	    <radius uom="NM">25.0</radius>
	    <startAngle uom="deg">270.0</startAngle>
	    <endAngle uom="deg">497.0</endAngle>
	  </ArcByCenterPoint>
	  <GeodesicString><posList>54.928163 -35.674116 55.233333 -36.166666</posList></GeodesicString>
	</segments>`

	var segs gml.CurveSegmentsType
	if err := xml.Unmarshal([]byte(doc), &segs); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	ordered := segs.Ordered()
	if len(ordered) != 3 {
		t.Fatalf("got %d segments, want 3", len(ordered))
	}
	if ordered[0].GeodesicString == nil {
		t.Errorf("segment 0 is not a GeodesicString")
	}
	if ordered[1].ArcByCenterPoint == nil {
		t.Errorf("segment 1 is not an ArcByCenterPoint")
	}
	if ordered[2].GeodesicString == nil {
		t.Errorf("segment 2 is not a GeodesicString")
	}

	// The 270->497 sweep is +227 degrees, not the -133 that reducing 497
	// modulo 360 would give.
	arc := ordered[1].ArcByCenterPoint
	if arc.StartAngle.Value != 270 || arc.EndAngle.Value != 497 {
		t.Errorf("angles = %v..%v, want 270..497", arc.StartAngle.Value, arc.EndAngle.Value)
	}
}

// TestOrderedFallsBackForConstructedValues verifies that a CurveSegmentsType
// built in Go, which never passed through the decoder and so recorded no
// document order, still reports its segments.
func TestOrderedFallsBackForConstructedValues(t *testing.T) {
	segs := gml.CurveSegmentsType{
		Arc:    []gml.ArcType{{}},
		Circle: []gml.CircleType{{}},
	}
	if got := len(segs.Ordered()); got != 2 {
		t.Errorf("Ordered() returned %d segments, want 2", got)
	}
}

// TestCurveMemberAcceptsAIXMCurve verifies that a ring whose members are
// aixm:Curve rather than gml:Curve still contributes its coordinates, and that
// a member holding only a reference is recognised as such.
func TestCurveMemberAcceptsAIXMCurve(t *testing.T) {
	const doc = `<Ring xmlns="http://www.opengis.net/gml/3.2"
	  xmlns:gml="http://www.opengis.net/gml/3.2"
	  xmlns:aixm="http://www.aixm.aero/schema/5.1.1"
	  xmlns:xlink="http://www.w3.org/1999/xlink">
	  <gml:curveMember><gml:Curve gml:id="A"/></gml:curveMember>
	  <gml:curveMember><aixm:Curve gml:id="B"/></gml:curveMember>
	  <gml:curveMember xlink:href="urn:uuid:some-border"/>
	</Ring>`

	var ring gml.RingType
	if err := xml.Unmarshal([]byte(doc), &ring); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(ring.CurveMember) != 3 {
		t.Fatalf("got %d curve members, want 3", len(ring.CurveMember))
	}
	if c := ring.CurveMember[0].TheCurve(); c == nil || c.ID != "A" {
		t.Errorf("gml:Curve member was not decoded")
	}
	if c := ring.CurveMember[1].TheCurve(); c == nil || c.ID != "B" {
		t.Errorf("aixm:Curve member was not decoded")
	}
	if ring.CurveMember[2].TheCurve() != nil {
		t.Errorf("reference-only member should hold no inline curve")
	}
	if got := ring.CurveMember[2].HrefAttr; got != "urn:uuid:some-border" {
		t.Errorf("reference-only member href = %q", got)
	}
}

// TestPointPropertyAcceptsAIXMPoint verifies that an arc centre given as an
// inline aixm:Point is decoded; DONLON encodes an aerodrome reference point
// this way.
func TestPointPropertyAcceptsAIXMPoint(t *testing.T) {
	const doc = `<pointProperty xmlns="http://www.opengis.net/gml/3.2"
	  xmlns:gml="http://www.opengis.net/gml/3.2"
	  xmlns:aixm="http://www.aixm.aero/schema/5.1.1">
	  <aixm:Point gml:id="p43434">
	    <gml:pos>52.3716666666667 -31.9494444444444</gml:pos>
	  </aixm:Point>
	</pointProperty>`

	var pp gml.PointPropertyType
	if err := xml.Unmarshal([]byte(doc), &pp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if pp.Point == nil {
		t.Fatal("inline aixm:Point was not decoded")
	}
	if pp.Point.ID != "p43434" || pp.Point.Pos == nil {
		t.Errorf("decoded point = %+v", pp.Point)
	}
}

// TestReferencedBorderIsTrimmed verifies that a ring borrowing part of a
// GeoBorder takes only the stretch between its neighbouring segments.
//
// A GeoBorder is a whole national boundary; splicing it in untrimmed inflates
// a control zone to the size of the country.
func TestReferencedBorderIsTrimmed(t *testing.T) {
	msg, err := goaixm.ParseFile(donlonPath)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	// The invariant: an airspace that borrows part of a border must not end up
	// larger than the border it borrowed from.  Comparing against the border's
	// own extent avoids guessing how big any given airspace "should" be, which
	// varies from an aerodrome control zone to a whole flight information
	// region.
	borderSpan := 0.0
	for _, f := range msg.ByType("GeoBorder") {
		b, ok := f.(*features.GeoBorderType)
		if !ok {
			continue
		}
		for _, ts := range b.TimeSlice {
			bts := ts.GeoBorderTimeSlice
			if bts.Border == nil || bts.Border.Curve == nil {
				continue
			}
			for _, seg := range bts.Border.Curve.Segments.Ordered() {
				if seg.GeodesicString == nil || seg.GeodesicString.PosList == nil {
					continue
				}
				lo, hi := math.Inf(1), math.Inf(-1)
				for i, field := range strings.Fields(seg.GeodesicString.PosList.Value) {
					if i%2 == 0 {
						continue // latitude
					}
					v, err := strconv.ParseFloat(field, 64)
					if err != nil {
						continue
					}
					lo, hi = math.Min(lo, v), math.Max(hi, v)
				}
				if hi-lo > borderSpan {
					borderSpan = hi - lo
				}
			}
		}
	}
	if borderSpan == 0 {
		t.Skip("no GeoBorder geometry in the reference dataset")
	}

	for _, a := range msg.Airspaces() {
		if len(a.TimeSlice) == 0 {
			continue
		}
		ts := a.TimeSlice[0].AirspaceTimeSlice

		// Only airspaces that actually reference a border can exhibit the bug.
		if !referencesBorder(ts) {
			continue
		}

		fc, err := geo.AirspaceToGeoJSONWithRefs(a, msg)
		if err != nil {
			continue
		}
		name := a.GetIdentifier()
		if ts.Designator != nil {
			name = ts.Designator.Value
		}

		for _, ring := range ringsOf(fc) {
			minLon, maxLon := math.Inf(1), math.Inf(-1)
			for _, c := range ring {
				minLon, maxLon = math.Min(minLon, c[0]), math.Max(maxLon, c[0])
			}
			if maxLon-minLon >= borderSpan {
				t.Errorf("airspace %s spans %.1f degrees of longitude, as much as the whole border (%.1f); the border was spliced in untrimmed",
					name, maxLon-minLon, borderSpan)
			}
		}
	}
}

// referencesBorder reports whether any ring of the timeslice takes part of its
// boundary from a GeoBorder held elsewhere in the message.
func referencesBorder(ts features.AirspaceTimeSliceType) bool {
	for _, gc := range ts.GeometryComponent {
		agc := gc.AirspaceGeometryComponent
		if agc == nil || agc.TheAirspaceVolume.AirspaceVolume == nil {
			continue
		}
		patches := agc.TheAirspaceVolume.AirspaceVolume.HorizontalProjection.Patches()
		if patches == nil {
			continue
		}
		for _, patch := range patches.PolygonPatch {
			if patch.Exterior == nil || patch.Exterior.Ring == nil {
				continue
			}
			for _, cm := range patch.Exterior.Ring.CurveMember {
				if cm.TheCurve() == nil && cm.HrefAttr != "" {
					return true
				}
			}
		}
	}
	return false
}

// TestReferencedBorderNeedsMessage verifies that a ring depending on a
// referenced border reports that it cannot be resolved, rather than silently
// emitting a ring that is missing a side.
func TestReferencedBorderNeedsMessage(t *testing.T) {
	surf := &features.SurfaceType{
		Patches: &gml.SurfacePatchesType{
			PolygonPatch: []gml.PolygonPatchType{{
				Exterior: &gml.AbstractRingPropertyType{
					Ring: &gml.RingType{
						CurveMember: []gml.CurveMemberType{
							{HrefAttr: "urn:uuid:missing-border"},
						},
					},
				},
			}},
		},
	}
	if _, err := geo.AirspaceToGeoJSON(buildAirspaceWithSurface(surf)); err == nil {
		t.Error("expected an error for an unresolvable border, got nil")
	}
}

// TestElevatedSurfaceProjection verifies that a horizontal projection given as
// an ElevatedSurface is used; only aixm:Surface was recognised before.
func TestElevatedSurfaceProjection(t *testing.T) {
	elevated := &features.ElevatedSurfaceType{
		Patches: &gml.SurfacePatchesType{
			PolygonPatch: []gml.PolygonPatchType{{
				Exterior: &gml.AbstractRingPropertyType{
					LinearRing: &gml.LinearRingType{
						PosList: &gml.DirectPositionListType{
							Value: "51.0 -1.0 51.0 0.0 52.0 0.0 51.0 -1.0",
						},
					},
				},
			}},
		},
	}

	a := &features.AirspaceType{}
	seq, cor := uint32(1), uint32(0)
	a.TimeSlice = []features.AirspaceTimeSlicePropertyType{{
		AirspaceTimeSlice: features.AirspaceTimeSliceType{
			Interpretation:   "BASELINE",
			SequenceNumber:   &seq,
			CorrectionNumber: &cor,
			GeometryComponent: []features.AirspaceGeometryComponentPropertyType{{
				AirspaceGeometryComponent: &features.AirspaceGeometryComponentType{
					TheAirspaceVolume: features.AirspaceVolumePropertyType{
						AirspaceVolume: &features.AirspaceVolumeType{
							HorizontalProjection: features.HorizontalProjectionType{
								ElevatedSurface: elevated,
							},
						},
					},
				},
			}},
		},
	}}

	fc, err := geo.AirspaceToGeoJSON(a)
	if err != nil {
		t.Fatalf("AirspaceToGeoJSON: %v", err)
	}
	rings := ringsOf(fc)
	if len(rings) != 1 {
		t.Fatalf("got %d rings, want 1", len(rings))
	}
	checkRing(t, "elevated surface", rings[0], true)
}

// TestCentrelineCorridor verifies that a corridor airspace — a centreline with
// a width — is emitted as a LineString carrying its width, rather than being
// dropped or buffered into a polygon.
func TestCentrelineCorridor(t *testing.T) {
	msg, err := goaixm.ParseFile(donlonPath)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	var found bool
	for _, a := range msg.Airspaces() {
		if len(a.TimeSlice) == 0 {
			continue
		}
		d := a.TimeSlice[0].AirspaceTimeSlice.Designator
		if d == nil || d.Value != "ACR001" {
			continue
		}
		found = true

		fc, err := geo.AirspaceToGeoJSONWithRefs(a, msg)
		if err != nil {
			t.Fatalf("ACR001: %v", err)
		}
		line, ok := fc.Features[0].Geometry.(geo.GeoJSONLineString)
		if !ok {
			t.Fatalf("ACR001 geometry is %T, want a LineString", fc.Features[0].Geometry)
		}
		if len(line.Coordinates) < 2 {
			t.Errorf("ACR001 centreline has %d positions", len(line.Coordinates))
		}
		if w := fc.Features[0].Properties["width_m"]; w != 30000.0 {
			t.Errorf("ACR001 width_m = %v, want 30000", w)
		}
		if k := fc.Features[0].Properties["geometryKind"]; k != "centreline" {
			t.Errorf("ACR001 geometryKind = %v, want centreline", k)
		}
	}
	if !found {
		t.Skip("ACR001 not present in the reference dataset")
	}
}

// TestContributorAirspaceAggregation verifies that an airspace defining its
// geometry only by reference to contributing airspaces is assembled from them.
func TestContributorAirspaceAggregation(t *testing.T) {
	msg, err := goaixm.ParseFile(donlonPath)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	var found bool
	for _, a := range msg.Airspaces() {
		if len(a.TimeSlice) == 0 {
			continue
		}
		d := a.TimeSlice[0].AirspaceTimeSlice.Designator
		if d == nil || d.Value != "EAMM" {
			continue
		}
		found = true

		// The parser resolves the contributor references while decoding, so
		// the aggregated geometry is reachable from the feature alone.
		if _, err := geo.AirspaceToGeoJSON(a); err != nil {
			t.Errorf("EAMM without a message: %v", err)
		}

		fc, err := geo.AirspaceToGeoJSONWithRefs(a, msg)
		if err != nil {
			t.Fatalf("EAMM with refs: %v", err)
		}
		rings := ringsOf(fc)
		if len(rings) < 2 {
			t.Errorf("EAMM produced %d rings, want at least 2 contributors", len(rings))
		}
		for i, ring := range rings {
			checkRing(t, "EAMM", ring, i == 0)
		}
	}
	if !found {
		t.Skip("EAMM not present in the reference dataset")
	}
}

// TestUnsupportedAggregationRejected verifies that an intersection or
// subtraction is refused rather than silently treated as a union, which would
// produce a wrong shape rather than an approximate one.
func TestUnsupportedAggregationRejected(t *testing.T) {
	surf := makeSurfacePosList("51.0 -1.0 51.0 0.0 52.0 0.0 51.0 -1.0")
	op := types.CodeAirspaceAggregationType{Value: "SUBTR"}

	a := &features.AirspaceType{}
	seq, cor := uint32(1), uint32(0)
	a.TimeSlice = []features.AirspaceTimeSlicePropertyType{{
		AirspaceTimeSlice: features.AirspaceTimeSliceType{
			Interpretation:   "BASELINE",
			SequenceNumber:   &seq,
			CorrectionNumber: &cor,
			GeometryComponent: []features.AirspaceGeometryComponentPropertyType{{
				AirspaceGeometryComponent: &features.AirspaceGeometryComponentType{
					Operation: &op,
					TheAirspaceVolume: features.AirspaceVolumePropertyType{
						AirspaceVolume: &features.AirspaceVolumeType{
							HorizontalProjection: features.HorizontalProjectionType{Surface: surf},
						},
					},
				},
			}},
		},
	}}

	if _, err := geo.AirspaceToGeoJSON(a); err == nil {
		t.Error("expected an error for a SUBTR aggregation, got nil")
	}
}

// TestRingIsClosedAndOriented verifies that an open source ring is closed and
// that an exterior ring is wound counter-clockwise, as RFC 7946 requires.
func TestRingIsClosedAndOriented(t *testing.T) {
	// A clockwise, unclosed triangle.
	surf := makeSurfacePosList("51.0 -1.0 52.0 0.0 51.0 0.0")

	fc, err := geo.AirspaceToGeoJSON(buildAirspaceWithSurface(surf))
	if err != nil {
		t.Fatalf("AirspaceToGeoJSON: %v", err)
	}
	rings := ringsOf(fc)
	if len(rings) != 1 {
		t.Fatalf("got %d rings, want 1", len(rings))
	}
	if got := len(rings[0]); got != 4 {
		t.Errorf("ring has %d positions, want 4 after closing", got)
	}
	checkRing(t, "closed and oriented", rings[0], true)
}

// TestDonlonFixturePresent guards the dataset-backed tests above.
func TestDonlonFixturePresent(t *testing.T) {
	if _, err := os.Stat(donlonPath); err != nil {
		t.Fatalf("reference dataset missing: %v", err)
	}
}
