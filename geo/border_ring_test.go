package geo_test

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/geo"
)

// Synthetic borders around a DONLON-style "country" with corners
// A (52N 032W), B (52N 030W), C (53N 030W) and D (53N 032W).  Each border
// has 21 slightly wiggling positions, 0.1° apart.
//
//	south  A→B   stored in ring direction
//	east   C→B   stored against the ring direction
//	north  C→D   stored in ring direction
func borderSouth() [][2]float64 { // lon, lat
	var out [][2]float64
	for i := 0; i <= 20; i++ {
		out = append(out, [2]float64{-32 + 0.1*float64(i), 52 + wiggle(i)})
	}
	return out
}

func borderEast() [][2]float64 {
	var out [][2]float64
	for i := 0; i <= 20; i++ {
		out = append(out, [2]float64{-30 + wiggle(i), 53 - 0.05*float64(i)})
	}
	return out
}

func borderNorth() [][2]float64 {
	var out [][2]float64
	for i := 0; i <= 20; i++ {
		out = append(out, [2]float64{-30 - 0.1*float64(i), 53 + wiggle(i)})
	}
	return out
}

// wiggle is a small offset that vanishes at both ends of a border.
func wiggle(i int) float64 {
	if i == 0 || i == 20 {
		return 0
	}
	return 0.01 * math.Sin(float64(i))
}

func posList(cs [][2]float64) string {
	var b strings.Builder
	for _, c := range cs {
		fmt.Fprintf(&b, "%.12f %.12f ", c[1], c[0])
	}
	return strings.TrimSpace(b.String())
}

func geoBorderXML(uuid, name string, cs [][2]float64) string {
	return fmt.Sprintf(`<message:hasMember><aixm:GeoBorder gml:id="uuid.%[1]s">
  <gml:identifier codeSpace="urn:uuid:">%[1]s</gml:identifier>
  <aixm:timeSlice><aixm:GeoBorderTimeSlice gml:id="ts.%[1]s">
    <aixm:interpretation>BASELINE</aixm:interpretation>
    <aixm:name>%[2]s</aixm:name><aixm:type>STATE</aixm:type>
    <aixm:border><aixm:Curve gml:id="crv.%[1]s"><gml:segments>
      <gml:GeodesicString><gml:posList>%[3]s</gml:posList></gml:GeodesicString>
    </gml:segments></aixm:Curve></aixm:border>
  </aixm:GeoBorderTimeSlice></aixm:timeSlice>
</aixm:GeoBorder></message:hasMember>`, uuid, name, posList(cs))
}

// ringMember is either an inline curve or a reference to a border.
type ringMember struct {
	href   string
	coords [][2]float64
}

func airspaceXML(uuid, designator string, members []ringMember) string {
	var ring strings.Builder
	for i, m := range members {
		if m.href != "" {
			fmt.Fprintf(&ring, `<gml:curveMember xlink:href="urn:uuid:%s"/>`, m.href)
			continue
		}
		fmt.Fprintf(&ring, `<gml:curveMember><gml:Curve gml:id="c.%s.%d"><gml:segments>
<gml:GeodesicString><gml:posList>%s</gml:posList></gml:GeodesicString></gml:segments></gml:Curve></gml:curveMember>`,
			uuid, i, posList(m.coords))
	}
	return fmt.Sprintf(`<message:hasMember><aixm:Airspace gml:id="uuid.%[1]s">
  <gml:identifier codeSpace="urn:uuid:">%[1]s</gml:identifier>
  <aixm:timeSlice><aixm:AirspaceTimeSlice gml:id="ts.%[1]s">
    <aixm:interpretation>BASELINE</aixm:interpretation>
    <aixm:type>FIR</aixm:type><aixm:designator>%[2]s</aixm:designator>
    <aixm:geometryComponent><aixm:AirspaceGeometryComponent gml:id="gc.%[1]s">
      <aixm:theAirspaceVolume><aixm:AirspaceVolume gml:id="vol.%[1]s">
        <aixm:horizontalProjection><aixm:Surface gml:id="srf.%[1]s"><gml:patches><gml:PolygonPatch>
          <gml:exterior><gml:Ring>%[3]s</gml:Ring></gml:exterior>
        </gml:PolygonPatch></gml:patches></aixm:Surface></aixm:horizontalProjection>
      </aixm:AirspaceVolume></aixm:theAirspaceVolume>
    </aixm:AirspaceGeometryComponent></aixm:geometryComponent>
  </aixm:AirspaceTimeSlice></aixm:timeSlice>
</aixm:Airspace></message:hasMember>`, uuid, designator, ring.String())
}

const (
	uuidSouth = "00000000-0000-0000-0000-00000000b001"
	uuidEast  = "00000000-0000-0000-0000-00000000b002"
	uuidNorth = "00000000-0000-0000-0000-00000000b003"
)

func borderMessage(t *testing.T, airspaces ...string) *goaixm.Message {
	t.Helper()
	doc := `<?xml version="1.0" encoding="UTF-8"?>
<message:AIXMBasicMessage xmlns:message="http://www.aixm.aero/schema/5.1.1/message"
  xmlns:aixm="http://www.aixm.aero/schema/5.1.1" xmlns:gml="http://www.opengis.net/gml/3.2"
  xmlns:xlink="http://www.w3.org/1999/xlink" gml:id="M-borders">` +
		geoBorderXML(uuidSouth, "SOUTH", borderSouth()) +
		geoBorderXML(uuidEast, "EAST", borderEast()) +
		geoBorderXML(uuidNorth, "NORTH", borderNorth()) +
		strings.Join(airspaces, "") + `</message:AIXMBasicMessage>`
	msg, err := goaixm.Parse(context.Background(), strings.NewReader(doc), goaixm.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// exteriorOf converts the named airspace and returns its exterior ring and
// any warnings.
func exteriorOf(t *testing.T, msg *goaixm.Message, designator string) ([][2]float64, []string) {
	t.Helper()
	for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
		if ts := a.TimeSlice; len(ts) == 0 || ts[0].Designator == nil || ts[0].Designator.String() != designator {
			continue
		}
		comps, err := geo.AirspaceComponents(a, msg)
		if err != nil {
			t.Fatal(err)
		}
		if len(comps) != 1 {
			t.Fatalf("%d components", len(comps))
		}
		p, ok := comps[0].Geometry.(geo.Polygon)
		if !ok {
			t.Fatalf("geometry %T", comps[0].Geometry)
		}
		return p.Coordinates[0], comps[0].Warnings
	}
	t.Fatalf("no airspace %s", designator)
	return nil, nil
}

// maxStep returns the longest edge of a ring, in degrees.
func maxStep(ring [][2]float64) float64 {
	m := 0.0
	for i := 1; i < len(ring); i++ {
		m = math.Max(m, math.Hypot(ring[i][0]-ring[i-1][0], ring[i][1]-ring[i-1][1]))
	}
	return m
}

// TestRingOfWholeBorders: a ring made of several whole referenced borders,
// one of them stored against the ring's direction, plus an inline curve.
// Each border is used whole and joined end to end; none is trimmed against a
// distant member or cut short by a chord.
func TestRingOfWholeBorders(t *testing.T) {
	msg := borderMessage(t, airspaceXML("00000000-0000-0000-0000-0000000a0001", "WHOLE", []ringMember{
		{href: uuidSouth},
		{href: uuidEast},
		{href: uuidNorth},
		{coords: [][2]float64{{-32, 53}, {-32.5, 52.5}, {-32, 52}}}, // D, a sea point, A
	}))
	ring, warns := exteriorOf(t, msg, "WHOLE")
	if len(warns) > 0 {
		t.Errorf("warnings: %v", warns)
	}
	// 3 × 21 border positions, less the 2 shared corners, plus the sea point
	// and the closing position.
	if want := 3*21 - 2 + 1 + 1; len(ring) != want {
		t.Errorf("ring has %d positions, want %d", len(ring), want)
	}
	if s := maxStep(ring); s > 0.71 {
		t.Errorf("longest edge %.3f°: a border was cut short", s)
	}
	for _, b := range [][][2]float64{borderSouth(), borderEast(), borderNorth()} {
		for _, p := range b {
			if !contains(ring, p) {
				t.Fatalf("border position %v missing from the ring", p)
			}
		}
	}
}

// TestRingUsesPartOfBorder: an inline curve leaves the border and returns to
// it between border positions; the border is referenced twice in a row (as
// two sections) and travelled against its direction.  Only the stretch
// between the two attachment points is used.
func TestRingUsesPartOfBorder(t *testing.T) {
	s := borderSouth()
	mid := func(a, b [2]float64) [2]float64 { return [2]float64{(a[0] + b[0]) / 2, (a[1] + b[1]) / 2} }
	west, east := mid(s[4], s[5]), mid(s[14], s[15])
	msg := borderMessage(t, airspaceXML("00000000-0000-0000-0000-0000000a0002", "PART", []ringMember{
		{coords: [][2]float64{west, {-31, 51.5}, east}},
		{href: uuidSouth},
		{href: uuidSouth},
	}))
	ring, warns := exteriorOf(t, msg, "PART")
	if len(warns) > 0 {
		t.Errorf("warnings: %v", warns)
	}
	// 3 inline positions, border positions 14..5, closing position.
	if want := 3 + 10 + 1; len(ring) != want {
		t.Errorf("ring has %d positions, want %d: %v", len(ring), want, ring)
	}
	for i, p := range s {
		if in := contains(ring, p); in != (i >= 5 && i <= 14) {
			t.Errorf("border position %d %v: in ring = %v", i, p, in)
		}
	}
	if s := maxStep(ring); s > 1.2 {
		t.Errorf("longest edge %.3f°", s)
	}
}

// TestRingGapIsReported: members that do not meet are joined by a straight
// line, and the gap is reported rather than passed over silently.
func TestRingGapIsReported(t *testing.T) {
	msg := borderMessage(t, airspaceXML("00000000-0000-0000-0000-0000000a0003", "GAP", []ringMember{
		{href: uuidSouth},
		{href: uuidNorth}, // the east border is missing
		{coords: [][2]float64{{-32, 53}, {-32, 52}}},
	}))
	_, warns := exteriorOf(t, msg, "GAP")
	if len(warns) == 0 || !strings.Contains(strings.Join(warns, " "), "does not connect") {
		t.Errorf("warnings %v, want a gap", warns)
	}
}

func contains(ring [][2]float64, p [2]float64) bool {
	for _, c := range ring {
		if math.Abs(c[0]-p[0]) < 1e-9 && math.Abs(c[1]-p[1]) < 1e-9 {
			return true
		}
	}
	return false
}
