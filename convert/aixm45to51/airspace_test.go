package aixm45to51_test

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/convert/aixm45to51"
	"github.com/davidkohl/goaixm/geo"
	"github.com/davidkohl/goaixm/xmlnode"
)

func aseKey(typ, id string) string { return "Ase(codeId=" + id + ";codeType=" + typ + ")" }

// airspace returns the time slice of the Airspace converted from an Ase.
func airspace(t *testing.T, res result, typ, id string) *xmlnode.Node {
	t.Helper()
	for _, m := range res.members {
		if m.Kind == "Airspace" && strings.HasPrefix(m.Provenance.SourceKey, aseKey(typ, id)) {
			return ts(t, m)
		}
	}
	t.Fatalf("no Airspace for %s", aseKey(typ, id))
	return nil
}

func posList(n *xmlnode.Node) string {
	return n.TextAt("geometryComponent/AirspaceGeometryComponent/theAirspaceVolume/AirspaceVolume/horizontalProjection/Surface/patches/PolygonPatch/exterior/Ring/curveMember/Curve/segments/GeodesicString/posList")
}

func segments(n *xmlnode.Node) []string {
	var out []string
	for _, c := range n.FindAll("geometryComponent/AirspaceGeometryComponent/theAirspaceVolume/AirspaceVolume/horizontalProjection/Surface/patches/PolygonPatch/exterior/Ring/curveMember") {
		if c.Href() != "" {
			out = append(out, "href")
			continue
		}
		for _, s := range c.FindAll("Curve/segments/*") {
			out = append(out, s.Local())
		}
	}
	return out
}

func issueKeys(rep *convert.Report, code string) map[string]int {
	out := map[string]int{}
	for _, i := range issues(rep, code) {
		out[i.Key] += i.Count
	}
	return out
}

// parsed parses the XMLWriter form of a result as a native message.
func parsed(t *testing.T, res result) *goaixm.Message {
	t.Helper()
	msg, err := goaixm.Parse(context.Background(), bytes.NewReader(res.xml), goaixm.Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// polygons returns the GeoJSON polygons of an airspace by designator.
func polygons(t *testing.T, msg *goaixm.Message, designator string, opt geo.Options) [][][][2]float64 {
	t.Helper()
	for _, f := range msg.ByKind("Airspace") {
		a := f.(*aixm.Airspace)
		if d := a.TimeSlice[0].Designator; d != nil && d.String() == designator {
			fc, err := opt.AirspaceToGeoJSONWithRefs(a, msg)
			if err != nil {
				t.Fatalf("%s: %v", designator, err)
			}
			var out [][][][2]float64
			for _, ft := range fc.Features {
				switch g := ft.Geometry.(type) {
				case *geo.Polygon:
					out = append(out, g.Coordinates)
				case *geo.MultiPolygon:
					out = append(out, g.Coordinates...)
				case geo.Polygon:
					out = append(out, g.Coordinates)
				case geo.MultiPolygon:
					out = append(out, g.Coordinates...)
				}
			}
			return out
		}
	}
	t.Fatalf("no airspace %s", designator)
	return nil
}

// WGS-84, for the independent expectations below.
const (
	wgsA  = 6378137.0
	wgsF  = 1 / 298.257223563
	wgsE2 = wgsF * (2 - wgsF)
)

var wgsB = wgsA * (1 - wgsF)

// ellipsoidArea returns the area in m² enclosed by a ring of [lon, lat]
// positions on the WGS-84 ellipsoid, the edges taken as straight in lon/lat
// (as GeoJSON draws them): A = |∮ F(φ) dλ| with F the area of the zone
// from the equator to φ per radian of longitude,
//
//	F(φ) = b²/2 · (sin φ / (1 − e² sin² φ) + atanh(e sin φ) / e),
//
// integrated along each edge by Simpson's rule.
func ellipsoidArea(ring [][2]float64) float64 {
	e := math.Sqrt(wgsE2)
	F := func(latDeg float64) float64 {
		s := math.Sin(latDeg * math.Pi / 180)
		return wgsB * wgsB / 2 * (s/(1-wgsE2*s*s) + math.Atanh(e*s)/e)
	}
	sum := 0.0
	for i := 0; i+1 < len(ring); i++ {
		a, b := ring[i], ring[i+1]
		dl := (b[0] - a[0]) * math.Pi / 180
		mid := (a[1] + b[1]) / 2
		sum += dl * (F(a[1]) + 4*F(mid) + F(b[1])) / 6
	}
	return math.Abs(sum)
}

// selfIntersects reports whether two non-adjacent edges of a closed ring
// cross (lon/lat plane).
func selfIntersects(ring [][2]float64) bool {
	n := len(ring) - 1
	cross := func(p, q, r, s [2]float64) bool {
		d := func(a, b, c [2]float64) float64 { return (b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0]) }
		d1, d2, d3, d4 := d(r, s, p), d(r, s, q), d(p, q, r), d(p, q, s)
		return d1*d2 < 0 && d3*d4 < 0
	}
	for i := 0; i < n; i++ {
		for j := i + 2; j < n; j++ {
			if i == 0 && j == n-1 {
				continue
			}
			if cross(ring[i], ring[i+1], ring[j], ring[j+1]) {
				return true
			}
		}
	}
	return false
}

func TestConvertAirspaces(t *testing.T) {
	res := convertBytes(t, fixture(t, "airspace.xml"), aixm45to51.Config{})
	rep := res.rep
	if rep.Targets["Airspace"] != 21 || rep.Targets["GeoBorder"] != 3 || len(rep.Targets) != 2 {
		t.Errorf("targets %v", rep.Targets)
	}
	for kind, want := range map[string][4]int{"Ase": {21, 21, 0, 0}, "Abd": {20, 0, 18, 2}, "Adg": {4, 0, 2, 2}, "Gbr": {3, 3, 0, 0}} {
		k := rep.Kinds[kind]
		if k.In != want[0] || k.Out != want[1] || k.Merged != want[2] || k.Dropped != want[3] {
			t.Errorf("%s: in/out/merged/dropped %d/%d/%d/%d, want %v", kind, k.In, k.Out, k.Merged, k.Dropped, want)
		}
	}

	// Properties, limits, class, working hours, activity, notes.
	r1 := airspace(t, res, "R", "XR1")
	vol := "geometryComponent/AirspaceGeometryComponent/theAirspaceVolume/AirspaceVolume/"
	for path, want := range map[string]string{
		"type": "R", "designator": "XR1", "localType": "EXAMPLE", "name": "EXAMPLE ONE", "class/AirspaceLayerClass/classification": "D",
		"geometryComponent/AirspaceGeometryComponent/operation": "BASE", "geometryComponent/AirspaceGeometryComponent/operationSequence": "1",
		vol + "upperLimit": "065", vol + "upperLimitReference": "STD", vol + "lowerLimit": "0", vol + "lowerLimitReference": "SFC",
		vol + "maximumLimit": "1000", vol + "maximumLimitReference": "SFC", vol + "minimumLimit": "500", vol + "minimumLimitReference": "SFC",
		"activation/AirspaceActivation/timeInterval/Timesheet/day": "ANY", "activation/AirspaceActivation/timeInterval/Timesheet/startTime": "00:00",
		"activation/AirspaceActivation/timeInterval/Timesheet/endTime": "24:00", "activation/AirspaceActivation/timeInterval/Timesheet/timeReference": "UTC",
		"activation/AirspaceActivation/status": "ACTIVE", "activation/AirspaceActivation/activity": "GLIDING",
		"activation/AirspaceActivation/annotation/Note/translatedNote/LinguisticNote/note": "Continuous.",
		vol + "annotation/Note/translatedNote/LinguisticNote/note":                         "Boundary: Boundary remark.",
		"annotation/Note/translatedNote/LinguisticNote/note":                               "Example remark.",
	} {
		if got := r1.TextAt(path); got != want {
			t.Errorf("XR1 %s = %q, want %q", path, got, want)
		}
	}
	if u := r1.Find(vol + "upperLimit"); u.Attrs[0].Value != "FL" {
		t.Errorf("upper limit unit %v", u.Attrs)
	}
	// Positions exact, ring closed on the first vertex (open source ring).
	if got := posList(r1); got != "10.00000000 20.00000000 10.00000000 20.20000000 10.20000000 20.20000000 10.20000000 20.00000000 10.00000000 20.00000000" {
		t.Errorf("XR1 ring %s", got)
	}
	// A closed source ring adds no zero-length piece; its orientation is
	// kept (clockwise).
	if got := posList(airspace(t, res, "R", "XR2")); got != "10.20000000 20.00000000 10.20000000 20.20000000 10.00000000 20.20000000 10.00000000 20.00000000 10.20000000 20.00000000" {
		t.Errorf("XR2 ring %s", got)
	}
	if airspace(t, res, "R", "XR2").TextAt("activation/AirspaceActivation/timeInterval/Timesheet/startEvent") != "SR" {
		t.Error("HJ")
	}
	if e := rep.Kinds["Ase"].Elements["Ase/codeLocInd"]; e == nil || e.Rule != "ASE-LOCIND" || e.Outcomes[convert.Unmapped] != 1 {
		t.Errorf("codeLocInd %+v", e)
	}

	// Timetable rows.
	rhl := airspace(t, res, "D", "XRHL")
	sheets := rhl.FindAll("activation/AirspaceActivation/timeInterval/Timesheet")
	if len(sheets) != 2 {
		t.Fatalf("%d timesheets", len(sheets))
	}
	for path, want := range map[string]string{"timeReference": "UTC", "daylightSavingAdjust": "YES", "day": "WORK_DAY", "dayTil": "FRI", "startDate": "01-01", "endDate": "31-12", "startTime": "07:00", "endTime": "19:00"} {
		if got := sheets[0].TextAt(path); got != want {
			t.Errorf("timesheet 1 %s = %q, want %q", path, got, want)
		}
	}
	for path, want := range map[string]string{"daylightSavingAdjust": "", "startDate": "SDLST", "startEvent": "SR", "startTimeRelativeEvent": "-30", "startEventInterpretation": "EARLIEST", "endEvent": "SS", "endEventInterpretation": "LATEST", "endTime": ""} {
		if got := sheets[1].TextAt(path); got != want {
			t.Errorf("timesheet 2 %s = %q, want %q", path, got, want)
		}
	}
	if rhl.TextAt("activation/AirspaceActivation/status") != "ACTIVE" || len(issues(rep, "value_invalid")) != 1 {
		t.Errorf("TIMSH status / invalid endTime 25:00: %v", issues(rep, "value_invalid"))
	}
	if arc := airspace(t, res, "P", "XARC"); arc.TextAt("activation/AirspaceActivation/annotation/Note/translatedNote/LinguisticNote/note") != "Working hours (AIXM 4.5 codeWorkHr): HX" || arc.Find("activation/AirspaceActivation/status") != nil {
		t.Error("HX kept as a note, no status")
	}

	// Rhumb lines: densified on the parallels, lossy.
	pl := strings.Fields(posList(rhl))
	if len(pl)/2 <= 5 {
		t.Errorf("rhumb lines not densified: %v", pl)
	}
	for i := 0; i < len(pl); i += 2 {
		if pl[i] != "10.500000000" && pl[i] != "10.50000000" && pl[i] != "10.80000000" && pl[i] != "10.800000000" {
			t.Errorf("position off the parallels: %s %s", pl[i], pl[i+1])
		}
	}
	if e := rep.Kinds["Abd"].Elements["Abd/Avx[codeType=RHL]/codeType"]; e == nil || e.Rule != "AVX-RHL" || e.Outcomes[convert.Lossy] != 2 {
		t.Errorf("RHL accounting %+v", e)
	}

	// Arcs: signed sweep, clockwise and counter-clockwise.
	arcPath := "geometryComponent/AirspaceGeometryComponent/theAirspaceVolume/AirspaceVolume/horizontalProjection/Surface/patches/PolygonPatch/exterior/Ring/curveMember/Curve/segments/ArcByCenterPoint/"
	angles := func(n *xmlnode.Node) (float64, float64) {
		var s, e float64
		fmt.Sscan(n.TextAt(arcPath+"startAngle"), &s)
		fmt.Sscan(n.TextAt(arcPath+"endAngle"), &e)
		return s, e
	}
	xarc := airspace(t, res, "P", "XARC")
	if s, e := angles(xarc); math.Abs(s) > 1e-3 || math.Abs(e-90) > 1e-3 || xarc.TextAt(arcPath+"radius") != "5" || xarc.TextAt(arcPath+"pos") != "10.00000000 22.00000000" {
		t.Errorf("XARC clockwise 0 -> 90: %v %v", s, e)
	}
	if got := strings.Join(segments(xarc), ","); got != "ArcByCenterPoint,GeodesicString" {
		t.Errorf("XARC segments %s", got)
	}
	xcca := airspace(t, res, "P", "XCCA")
	if s, e := angles(xcca); math.Abs(s-90) > 1e-3 || math.Abs(e) > 1e-3 || xcca.Find(arcPath + "radius").Attrs[0].Value != "KM" {
		t.Errorf("XCCA counter-clockwise 90 -> 0: %v %v", s, e)
	}
	if e := rep.Kinds["Abd"].Elements["Abd/Avx[codeType=CWA]/VorUidCen/codeId"]; e == nil || e.Rule != "AVX-CENREF" {
		t.Errorf("arc centre navaid %+v", e)
	}
	if e := rep.Kinds["Abd"].Elements["Abd/Avx[codeType=GRC]/DpnUidSpn/codeId"]; e == nil || e.Rule != "AVX-SPN" {
		t.Errorf("spanning point %+v", e)
	}
	if got := strings.Join(segments(airspace(t, res, "CTR", "XCIR")), ","); got != "CircleByCenterPoint" {
		t.Errorf("circle segments %s", got)
	}

	// Borders.
	if got := posList(airspace(t, res, "RAS", "XFNT")); got != "11.05000000 20.25000000 10.95000000 20.50000000 11.05000000 20.75000000 10.60000000 20.75000000 10.60000000 20.25000000 11.05000000 20.25000000" {
		t.Errorf("XFNT ring %s", got)
	}
	// The closed island border is followed the short way round, through
	// its closing corner.
	if got := posList(airspace(t, res, "RAS", "XISL")); !strings.HasPrefix(got, "11.90000000 20.43333333 11.90000000 20.40000000 12.00000000 20.40000000 12.00000000 20.30000000") {
		t.Errorf("XISL ring %s", got)
	}
	types := map[string]string{}
	for _, m := range res.members {
		if m.Kind == "GeoBorder" {
			types[ts(t, m).TextAt("name")] = ts(t, m).TextAt("type")
		}
	}
	if mustJSON(types) != `{"EXAMPLE ISLAND":"COAST","EXAMPLE PARALLEL":"WATER","EXAMPLE RIVER":"STATE"}` {
		t.Errorf("border types %v", types)
	}
	if e := rep.Kinds["Gbr"].Elements["Gbr/Gbv[codeType=RHL]/codeType"]; e == nil || e.Rule != "GBV-RHL" || e.Outcomes[convert.Lossy] != 1 {
		t.Errorf("border RHL %+v", e)
	}

	// Derived airspaces.
	sec := airspace(t, res, "SECTOR-C", "XSEC")
	comps := sec.FindAll("geometryComponent/AirspaceGeometryComponent")
	wantRefs := []string{aseKey("R", "XR1"), aseKey("R", "XR2"), aseKey("D", "XRHL")}
	if len(comps) != 3 {
		t.Fatalf("XSEC: %d components", len(comps))
	}
	for i, c := range comps {
		op := map[bool]string{true: "BASE", false: "UNION"}[i == 0]
		dep := c.Find("theAirspaceVolume/AirspaceVolume/contributorAirspace/AirspaceVolumeDependency")
		if c.TextAt("operation") != op || c.TextAt("operationSequence") != fmt.Sprint(i+1) || dep.TextAt("dependency") != "FULL_GEOMETRY" ||
			hrefOf(dep, "theAirspace") != uuidOf(wantRefs[i]) {
			t.Errorf("XSEC component %d: %v", i, c)
		}
	}
	same := airspace(t, res, "TMA", "XSAME")
	if same.TextAt(vol+"contributorAirspace/AirspaceVolumeDependency/dependency") != "HORZ_PROJECTION" || same.TextAt(vol+"upperLimit") != "115" ||
		hrefOf(same, vol+"contributorAirspace/AirspaceVolumeDependency/theAirspace") != uuidOf(aseKey("R", "XR1")) {
		t.Errorf("XSAME %v", same)
	}

	// Duplicate airspace keys: the boundaries follow the mids.
	var dups []string
	for _, m := range res.members {
		if m.Kind == "Airspace" && strings.HasPrefix(m.Provenance.SourceKey, aseKey("D-OTHER", "XDUP")) {
			dups = append(dups, ts(t, m).TextAt("name")+" "+strings.Fields(posList(ts(t, m)))[0])
		}
	}
	if strings.Join(dups, ",") != "EXAMPLE DUP A 11.00000000,EXAMPLE DUP B 10.00000000" {
		t.Errorf("duplicate airspaces %v", dups)
	}

	// Degenerate and missing geometry.
	pt := airspace(t, res, "D-OTHER", "XPT")
	if pt.Find(vol+"horizontalProjection") != nil || !strings.Contains(pt.TextAt(vol+"annotation/Note/translatedNote/LinguisticNote/note"), "10.16666667 25.00000000") {
		t.Errorf("XPT %v", pt)
	}
	if n := airspace(t, res, "D", "XNONE"); n.TextAt(vol+"upperLimit") != "1500" || n.Find(vol+"horizontalProjection") != nil {
		t.Errorf("XNONE %v", n)
	}
	if got := strings.Join(segments(airspace(t, res, "D", "XZERO")), ","); got != "GeodesicString" {
		t.Errorf("zero-length arc not omitted: %s", got)
	}
	bad := airspace(t, res, "D", "XBAD")
	if strings.Count(posList(bad), " ")+1 != 2*4 {
		t.Errorf("XBAD: bad vertex not dropped: %s", posList(bad))
	}
	if !strings.Contains(bad.TextAt("annotation/Note/translatedNote/LinguisticNote/note"), "datum of the source boundary: U") {
		t.Errorf("XBAD datum note %v", bad)
	}

	// Every problem is an issue, on its airspace.
	for code, want := range map[string]map[string]int{
		"border_far":          {aseKey("RAS", "XFAR"): 1},
		"border_missing":      {aseKey("RAS", "XMISS"): 1},
		"border_ambiguous":    {aseKey("RAS", "XISL"): 1},
		"arc_degenerate":      {aseKey("D", "XZERO"): 1},
		"arc_radius_mismatch": {aseKey("D", "XMISR"): 1},
		"coordinate_invalid":  {aseKey("D", "XBAD"): 1},
		"code_unknown":        {aseKey("D", "XBAD"): 1},
		"datum_unknown":       {aseKey("D", "XBAD"): 1},
		"geometry_degenerate": {aseKey("D-OTHER", "XPT"): 1},
		"geometry_missing":    {aseKey("D", "XNONE"): 1, aseKey("SECTOR-C", "XSEC"): 0},
		"geometry_both":       {aseKey("D", "XBOTH"): 1},
		"geometry_invalid":    {aseKey("D", "XCV"): 1},
		"boundary_duplicate":  {aseKey("R", "XR1"): 1},
		"dangling_ref":        {aseKey("D", "XGONE"): 2},
		"duplicate_key":       {aseKey("D-OTHER", "XDUP"): 1},
	} {
		got := issueKeys(rep, code)
		for k, n := range want {
			if got[k] != n {
				t.Errorf("%s %s: %d, want %d (%v)", code, k, got[k], n, issues(rep, code))
			}
		}
	}
	for path, rule := range map[string]string{
		"Abd[ABD-ORPHAN]/AbdUid/AseUid/codeId": "ABD-ORPHAN", "Abd[ABD-DUP]/AbdUid/AseUid/codeId": "ABD-DUP",
		"Abd/Avx[ABD-UNUSED]/codeType": "ABD-UNUSED", "Adg[ADG-ORPHAN]/AseUidSameExtent/codeId": "ADG-ORPHAN",
		"Adg[ADG-SHADOW]/AseUidSameExtent/codeId": "ADG-SHADOW", "Adg/codeOpr": "ADG-OPR", "Abd/Circle/valRadius": "CIR",
	} {
		if e := rep.Kinds[strings.SplitN(strings.SplitN(path, "/", 2)[0], "[", 2)[0]].Elements[path]; e == nil || e.Rule != rule {
			t.Errorf("%s: %+v", path, e)
		}
	}

	// The written form decodes; every airspace with a projection gives a
	// valid polygon (closed, at least four positions, no self-crossing).
	msg := parsed(t, res)
	for _, f := range msg.ByKind("Airspace") {
		a := f.(*aixm.Airspace)
		d := a.TimeSlice[0].Designator.String()
		fc, err := geo.AirspaceToGeoJSONWithRefs(a, msg)
		if err != nil {
			if d != "XPT" && d != "XNONE" && d != "XCV" {
				t.Errorf("%s: %v", d, err)
			}
			continue
		}
		for _, p := range polygons(t, msg, d, geo.Options{}) {
			ring := p[0]
			if len(ring) < 4 || ring[0] != ring[len(ring)-1] || selfIntersects(ring) {
				t.Errorf("%s: invalid ring %v", d, ring)
			}
		}
		_ = fc
	}
}

// TestRingOrientation: a clockwise and a counter-clockwise source ring of
// the same square give the same polygon.
func TestRingOrientation(t *testing.T) {
	res := convertBytes(t, fixture(t, "airspace.xml"), aixm45to51.Config{})
	msg := parsed(t, res)
	p1, p2 := polygons(t, msg, "XR1", geo.Options{})[0][0], polygons(t, msg, "XR2", geo.Options{})[0][0]
	a1, a2 := ellipsoidArea(p1), ellipsoidArea(p2)
	if math.Abs(a1-a2) > 1e-6*a1 || a1 < 4.8e8 || a1 > 4.9e8 {
		t.Errorf("areas %.0f and %.0f m²", a1, a2)
	}
}

// TestBorderModes: Inline and Reference give the same polygons, vertex by
// vertex within 1e-9°.
func TestBorderModes(t *testing.T) {
	in := convertBytes(t, fixture(t, "airspace.xml"), aixm45to51.Config{})
	ref := convertBytes(t, fixture(t, "airspace.xml"), aixm45to51.Config{BorderMode: aixm45to51.Reference})
	if got := strings.Join(segments(airspace(t, ref, "RAS", "XFNT")), ","); got != "href,GeodesicString" {
		t.Errorf("reference segments %s", got)
	}
	if got := hrefOf(airspace(t, ref, "RAS", "XFNT"), "geometryComponent/AirspaceGeometryComponent/theAirspaceVolume/AirspaceVolume/horizontalProjection/Surface/patches/PolygonPatch/exterior/Ring/curveMember"); got != uuidOf("Gbr(txtName=EXAMPLE RIVER)") {
		t.Errorf("border href %s", got)
	}
	if len(issues(ref.rep, "border_inline")) != 1 {
		t.Errorf("closed border not reported as inline: %v", ref.rep.Issues)
	}
	mi, mr := parsed(t, in), parsed(t, ref)
	// With densification the two agree only where the vertices lie on the
	// border: geo densifies a referenced border before cutting it, so a
	// vertex off the border (XFAR) joins a densified border position.
	for _, d := range []string{"XFNT", "XFAR", "XISL"} {
		opts := []geo.Options{{}, {GeodesicTolerance: 1, ArcTolerance: 1}}
		if d == "XFAR" {
			opts = opts[:1]
		}
		for _, opt := range opts {
			a, b := polygons(t, mi, d, opt), polygons(t, mr, d, opt)
			if len(a) != 1 || len(b) != 1 || len(a[0][0]) != len(b[0][0]) {
				t.Fatalf("%s: %v / %v", d, a, b)
			}
			for i := range a[0][0] {
				p, q := a[0][0][i], b[0][0][i]
				if math.Abs(p[0]-q[0]) > 1e-9 || math.Abs(p[1]-q[1]) > 1e-9 {
					t.Errorf("%s position %d: %v inline, %v reference", d, i, p, q)
				}
			}
		}
	}
}

// meridianArc returns the length in metres of the WGS-84 meridian from the
// equator to lat (degrees), by Simpson's rule over M(φ): independent of
// the Vincenty formulas geo uses.
func meridianArc(lat float64) float64 {
	const n = 2000
	phi := lat * math.Pi / 180
	m := func(p float64) float64 {
		s := math.Sin(p)
		return wgsA * (1 - wgsE2) / math.Pow(1-wgsE2*s*s, 1.5)
	}
	sum := m(0) + m(phi)
	for i := 1; i < n; i++ {
		w := 2.0
		if i%2 == 1 {
			w = 4
		}
		sum += w * m(phi*float64(i)/n)
	}
	return sum * phi / n / 3
}

// latForArc inverts meridianArc by bisection.
func latForArc(d float64) float64 {
	lo, hi := 0.0, 1.0
	for range 100 {
		mid := (lo + hi) / 2
		if meridianArc(mid) < d {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

func snapshot(body string) []byte {
	return []byte(`<AIXM-Snapshot version="4.5" effective="2030-01-24T00:00:00Z">` + body + `</AIXM-Snapshot>`)
}

// dd renders decimal degrees in the 4.5 layout DD.d…H (DDD.d…H for
// longitudes), with 10 decimals.
func dd(v float64, pos, neg string) string {
	h, width := pos, 13
	if pos == "E" {
		width = 14
	}
	if v < 0 {
		h, v = neg, -v
	}
	return fmt.Sprintf("%0*.10f%s", width, v, h)
}

// TestArcPrecision: a clockwise quarter arc of 10 km about a centre on the
// equator.  Its start (bearing 0) and end (bearing 90) are checked against
// closed forms independent of geo's Vincenty solutions: north of the centre
// on the meridian (meridian arc length), east of it on the equator (the
// equator is a geodesic: Δλ = r / a).  Tolerance 1e-9° (0.1 mm); every
// discretised position lies on the circle within 1 mm and runs clockwise.
func TestArcPrecision(t *testing.T) {
	const r = 10000.0
	north := latForArc(r)
	east := r / wgsA * 180 / math.Pi
	doc := snapshot(`<Ase><AseUid><codeType>R</codeType><codeId>XQ</codeId></AseUid><txtName>Q</txtName></Ase>
<Abd><AbdUid><AseUid><codeType>R</codeType><codeId>XQ</codeId></AseUid></AbdUid>
<Avx><codeType>CWA</codeType><geoLat>` + dd(north, "N", "S") + `</geoLat><geoLong>010.0000000000E</geoLong><codeDatum>WGE</codeDatum>
<geoLatArc>00.0000000000N</geoLatArc><geoLongArc>010.0000000000E</geoLongArc><valRadiusArc>10</valRadiusArc><uomRadiusArc>KM</uomRadiusArc></Avx>
<Avx><codeType>GRC</codeType><geoLat>00.0000000000N</geoLat><geoLong>` + dd(10+east, "E", "W") + `</geoLong><codeDatum>WGE</codeDatum></Avx>
<Avx><codeType>GRC</codeType><geoLat>00.0000000000N</geoLat><geoLong>010.0000000000E</geoLong><codeDatum>WGE</codeDatum></Avx></Abd>`)
	res := convertBytes(t, doc, aixm45to51.Config{})
	if len(res.rep.Issues) != 0 {
		t.Errorf("issues %v", res.rep.Issues)
	}
	ring := polygons(t, parsed(t, res), "XQ", geo.Options{ArcTolerance: 0.01})[0][0]
	// The ring as GeoJSON is counter-clockwise: centre, east, …, north,
	// centre.  Find the arc positions (all but the centre).
	var arc [][2]float64
	for _, p := range ring {
		if p != [2]float64{10, 0} {
			arc = append(arc, p)
		}
	}
	// The ends: the easternmost and the northernmost position.
	first, last := arc[0], arc[0]
	for _, p := range arc {
		if p[0] > first[0] {
			first = p
		}
		if p[1] > last[1] {
			last = p
		}
	}
	t.Logf("arc ends off the closed forms: east %.2e°/%.2e°, north %.2e°/%.2e°", first[0]-(10+east), first[1], last[0]-10, last[1]-north)
	if math.Abs(first[0]-(10+east)) > 1e-9 || math.Abs(first[1]) > 1e-9 {
		t.Errorf("east end %v, want (%.10f, 0)", first, 10+east)
	}
	if math.Abs(last[0]-10) > 1e-9 || math.Abs(last[1]-north) > 1e-9 {
		t.Errorf("north end %v, want (10, %.10f)", last, north)
	}
	prev := -1.0
	for i, p := range arc[:len(arc)-1] { // the ring repeats its first position
		d := geo.GeodesicDistance(0, 10, p[1], p[0])
		if math.Abs(d-r) > 1e-3 {
			t.Errorf("position %d %.4f m from the centre", i, d-r)
		}
		br, _ := geo.GeodesicAzimuth(0, 10, p[1], p[0])
		if br > 180 {
			br -= 360
		}
		if i > 0 && math.Abs(br-prev) > 90 || br < -1e-6 || br > 90+1e-6 {
			t.Errorf("position %d at bearing %f", i, br)
		}
		prev = br
	}
}

// TestCircleArea: a circle of 10 km radius about a centre on the equator.
// Its area, discretised with ArcTolerance 1 mm and integrated on the
// ellipsoid (ellipsoidArea), is compared with the area of a geodesic circle
// on a surface of the Gaussian curvature K = 1/(M·N) at the centre,
// 2π/K · (1 − cos(r√K)): the curvature varies by under 1e-7 within the
// circle, the inscribed polygon misses under 2/3 · tol · perimeter.
// Tolerance: relative 1e-6.
func TestCircleArea(t *testing.T) {
	const r = 10000.0
	doc := snapshot(`<Ase><AseUid><codeType>CTR</codeType><codeId>XC</codeId></AseUid><txtName>C</txtName></Ase>
<Abd><AbdUid><AseUid><codeType>CTR</codeType><codeId>XC</codeId></AseUid></AbdUid>
<Circle><geoLatCen>00.0000000000N</geoLatCen><geoLongCen>010.0000000000E</geoLongCen><codeDatum>WGE</codeDatum><valRadius>10</valRadius><uomRadius>KM</uomRadius></Circle></Abd>`)
	res := convertBytes(t, doc, aixm45to51.Config{})
	ring := polygons(t, parsed(t, res), "XC", geo.Options{ArcTolerance: 0.001})[0][0]
	k := 1 / (wgsA * wgsA * (1 - wgsE2)) // M·N at the equator: a(1−e²)·a
	want := 2 * math.Pi / k * (1 - math.Cos(r*math.Sqrt(k)))
	got := ellipsoidArea(ring)
	t.Logf("circle area %.1f m², expected %.1f m², relative %.2e, %d positions", got, want, (got-want)/want, len(ring))
	if rel := (got - want) / want; math.Abs(rel) > 1e-6 || got > want {
		t.Errorf("area %.1f m², want %.1f m² (relative %.2e)", got, want, rel)
	}
	if math.Abs(want-math.Pi*r*r)/want > 1e-5 {
		t.Errorf("expectation %.1f far from πr² %.1f", want, math.Pi*r*r)
	}
}
