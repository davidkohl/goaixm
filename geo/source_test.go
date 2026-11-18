package geo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/geo"
	"github.com/davidkohl/goaixm/xmlnode"
)

func donlonRaw(t *testing.T) *goaixm.Message {
	t.Helper()
	msg, err := goaixm.ParseFile(context.Background(), donlonPath, goaixm.Options{KeepRaw: true})
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// sourceTokens returns every coordinate value of g in traversal order.
func sourceTokens(g *geo.SourceGeometry) []string {
	var out []string
	pos := func(p *geo.SourcePos) {
		if p != nil {
			for _, c := range p.C {
				out = append(out, c...)
			}
		}
	}
	var shape func(s *geo.SourceShape)
	shape = func(s *geo.SourceShape) {
		if s == nil {
			return
		}
		pos(s.Pos)
		for _, seg := range s.Segments {
			pos(seg.Pos)
			shape(seg.Centre)
		}
		for _, p := range s.Patches {
			rings := p.Interior
			if p.Exterior != nil {
				rings = append([]geo.SourceRing{*p.Exterior}, rings...)
			}
			for _, r := range rings {
				pos(r.Pos)
				for _, m := range r.Members {
					shape(m.Curve)
				}
			}
		}
	}
	for _, p := range g.Geoms {
		shape(p.Geom)
	}
	for _, c := range g.Components {
		if c.Volume != nil {
			shape(c.Volume.HorizontalProjection)
			shape(c.Volume.Centreline)
		}
	}
	return out
}

// xmlTokens returns every gml:pos / gml:posList value under the time
// slice, as an independent oracle.
func xmlTokens(ts *xmlnode.Node) []string {
	var out []string
	ts.Walk(func(n *xmlnode.Node) bool {
		if n.Name.Space == xmlnode.NSGML && (n.Local() == "pos" || n.Local() == "posList") {
			out = append(out, strings.Fields(n.Text)...)
		}
		return true
	})
	return out
}

// TestSourceOfDONLON: every time slice of every DONLON feature gives a
// source geometry that keeps every coordinate as its decimal text (the
// multiset of values equals the file's gml:pos/posList values), round-trips
// through JSON and encodes canonically.
//
// Verifies: AQ-TRF-011
func TestSourceOfDONLON(t *testing.T) {
	msg := donlonRaw(t)
	var feats, withGeom, slicesN int
	for _, mem := range msg.Members() {
		root, err := mem.Node()
		if err != nil {
			t.Fatal(err)
		}
		feats++
		for i, w := range root.ChildrenNamed("timeSlice") {
			if len(w.Children) == 0 {
				continue
			}
			slicesN++
			src, err := geo.SourceOf(root, i)
			if err != nil {
				t.Fatalf("%s %d: %v", mem.Kind, i, err)
			}
			got, want := sourceTokens(src), xmlTokens(w.Children[0])
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("%s slice %d: %d coordinate values kept, file has %d", mem.Kind, i, len(got), len(want))
			}
			if len(want) > 0 {
				withGeom++
			}
			checkRoundTrip(t, src)
		}
	}
	if withGeom < 80 {
		t.Errorf("only %d of %d slices (%d features) with geometry", withGeom, slicesN, feats)
	}
	t.Logf("%d features, %d slices, %d with geometry", feats, slicesN, withGeom)
}

// checkRoundTrip: decode(encode(src)) has the same canonical bytes and
// hash, also after the keys are reordered (as PostgreSQL jsonb does).
func checkRoundTrip(t *testing.T, src *geo.SourceGeometry) {
	t.Helper()
	canon, err := src.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	h, _ := src.Hash()
	var back geo.SourceGeometry
	if err := json.Unmarshal(canon, &back); err != nil {
		t.Fatal(err)
	}
	c2, _ := back.Canonical()
	if string(c2) != string(canon) {
		t.Fatalf("%s: round trip changes the canonical encoding", src.Kind)
	}
	if h2, _ := back.Hash(); h2 != h {
		t.Fatalf("%s: round trip changes the hash", src.Kind)
	}
	// Keys in another order and with whitespace: same canonical bytes.
	var generic any
	if err := json.Unmarshal(canon, &generic); err != nil {
		t.Fatal(err)
	}
	pretty, _ := json.MarshalIndent(reverseKeys(generic), "", "  ")
	c3, err := geo.CanonicalJSON(pretty)
	if err != nil {
		t.Fatal(err)
	}
	if string(c3) != string(canon) {
		t.Fatalf("%s: reordered keys give another canonical encoding", src.Kind)
	}
}

// reverseKeys turns objects into ordered key lists in reverse order (so
// that the encoding's key order differs from the canonical one).
type revObject []struct {
	k string
	v any
}

func (o revObject) MarshalJSON() ([]byte, error) {
	var b strings.Builder
	b.WriteString("{")
	for i, kv := range o {
		if i > 0 {
			b.WriteString(",")
		}
		k, _ := json.Marshal(kv.k)
		v, err := json.Marshal(kv.v)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteString(":")
		b.Write(v)
	}
	b.WriteString("}")
	return []byte(b.String()), nil
}

func reverseKeys(v any) any {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		slices.Reverse(keys)
		var o revObject
		for _, k := range keys {
			o = append(o, struct {
				k string
				v any
			}{k, reverseKeys(x[k])})
		}
		return o
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = reverseKeys(x[i])
		}
		return out
	}
	return v
}

// TestSourceArcByCenterPoint: EAR1's arc keeps the decimal text of its
// centre, radius and bearings (a float64 would give 25 for "25.0" and 497
// for "497.0"), its unit and numArc, in document order between the two
// geodesic strings.
//
// Verifies: AQ-TRF-011
func TestSourceArcByCenterPoint(t *testing.T) {
	msg := donlonRaw(t)
	src := sourceByDesignator(t, msg, "EAR1")
	segs := src.Components[0].Volume.HorizontalProjection.Patches[0].Exterior.Members[0].Curve.Segments
	var kinds []string
	for _, s := range segs {
		kinds = append(kinds, s.T)
	}
	if !slices.Contains(kinds, "ArcByCenterPoint") {
		t.Fatalf("segments %v", kinds)
	}
	i := slices.Index(kinds, "ArcByCenterPoint")
	if i == 0 || i == len(kinds)-1 || kinds[i-1] != "GeodesicString" || kinds[i+1] != "GeodesicString" {
		t.Errorf("segment order %v", kinds)
	}
	a := segs[i]
	if a.NumArc != "1" || !a.Pos.List || !slices.Equal(a.Pos.C[0], []string{"55.2333333333333", "-36.166666666666664"}) {
		t.Errorf("centre %+v numArc %q", a.Pos, a.NumArc)
	}
	if *a.Radius != (geo.SourceMeasure{V: "25.0", Uom: "NM"}) || *a.StartAngle != (geo.SourceMeasure{V: "270.0", Uom: "deg"}) ||
		*a.EndAngle != (geo.SourceMeasure{V: "497.0", Uom: "deg"}) {
		t.Errorf("radius %+v start %+v end %+v", a.Radius, a.StartAngle, a.EndAngle)
	}
}

func sourceByDesignator(t *testing.T, msg *goaixm.Message, designator string) *geo.SourceGeometry {
	t.Helper()
	for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
		if a.TimeSlice[0].Designator == nil || a.TimeSlice[0].Designator.String() != designator {
			continue
		}
		mem, _ := msg.Member(a)
		root, err := mem.Node()
		if err != nil {
			t.Fatal(err)
		}
		src, err := geo.SourceOf(root, 0)
		if err != nil {
			t.Fatal(err)
		}
		return src
	}
	t.Fatalf("no airspace %s", designator)
	return nil
}

// TestSourceCanonicalStable pins the canonical encoding and hash of a
// small feature: a change of the representation or the encoding must
// show here (and bump SourceVersion).
//
// Verifies: AQ-TRF-011
func TestSourceCanonicalStable(t *testing.T) {
	const doc = `<aixm:DesignatedPoint xmlns:aixm="http://www.aixm.aero/schema/5.1.1" xmlns:gml="http://www.opengis.net/gml/3.2" gml:id="dp1">
  <gml:identifier codeSpace="urn:uuid:">00000000-0000-0000-0000-0000000d0001</gml:identifier>
  <aixm:timeSlice><aixm:DesignatedPointTimeSlice gml:id="ts1">
    <aixm:interpretation>BASELINE</aixm:interpretation>
    <aixm:location><aixm:Point gml:id="p1" srsName="urn:ogc:def:crs:EPSG::4326"><gml:pos>52.370000 &lt;9.10</gml:pos></aixm:Point></aixm:location>
  </aixm:DesignatedPointTimeSlice></aixm:timeSlice>
</aixm:DesignatedPoint>`
	root, err := xmlnode.Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	src, err := geo.SourceOf(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	canon, _ := src.Canonical()
	const want = `{"geoms":[{"geom":{"id":"p1","pos":{"c":[["52.370000","<9.10"]]},"srs":"urn:ogc:def:crs:EPSG::4326","t":"Point"},"path":"location"}],"kind":"DesignatedPoint","slice":0,"v":1}`
	if string(canon) != want {
		t.Errorf("canonical\n got %s\nwant %s", canon, want)
	}
	h, _ := src.Hash()
	const wantHash = "ecd81ddf790a35a3560ef83f62d2631a5628e84ade4317f17c7435be7d0ac8ec"
	if h != wantHash {
		t.Errorf("hash %s, want %s", h, wantHash)
	}
}

const (
	uuidB1 = "00000000-0000-0000-0000-0000000e0001"
	uuidA1 = "00000000-0000-0000-0000-0000000e0010"
	uuidA2 = "00000000-0000-0000-0000-0000000e0020"
)

// twoSliceBorderXML is a GeoBorder with two BASELINE slices: the first
// valid from 2020, the second (another curve) from 2021.
func twoSliceBorderXML() string {
	slice := func(n int, from string, cs [][2]float64) string {
		return fmt.Sprintf(`<aixm:timeSlice><aixm:GeoBorderTimeSlice gml:id="ts.b.%d">
    <gml:validTime><gml:TimePeriod gml:id="vt.b.%[1]d"><gml:beginPosition>%[2]s</gml:beginPosition><gml:endPosition indeterminatePosition="unknown"/></gml:TimePeriod></gml:validTime>
    <aixm:interpretation>BASELINE</aixm:interpretation><aixm:sequenceNumber>%[1]d</aixm:sequenceNumber>
    <aixm:border><aixm:Curve gml:id="crv.b.%[1]d"><gml:segments>
      <gml:GeodesicString><gml:posList>%[3]s</gml:posList></gml:GeodesicString>
    </gml:segments></aixm:Curve></aixm:border>
  </aixm:GeoBorderTimeSlice></aixm:timeSlice>`, n, from, posList(cs))
	}
	return fmt.Sprintf(`<message:hasMember><aixm:GeoBorder gml:id="uuid.%[1]s">
  <gml:identifier codeSpace="urn:uuid:">%[1]s</gml:identifier>%s%s
</aixm:GeoBorder></message:hasMember>`, uuidB1,
		slice(1, "2020-01-01T00:00:00Z", [][2]float64{{0, 50}, {2, 50}}),
		slice(2, "2021-01-01T00:00:00Z", [][2]float64{{0, 50}, {1, 49.9}, {2, 50}}))
}

func depMessage(t *testing.T) *goaixm.Message {
	t.Helper()
	doc := `<?xml version="1.0" encoding="UTF-8"?>
<message:AIXMBasicMessage xmlns:message="http://www.aixm.aero/schema/5.1.1/message"
  xmlns:aixm="http://www.aixm.aero/schema/5.1.1" xmlns:gml="http://www.opengis.net/gml/3.2"
  xmlns:xlink="http://www.w3.org/1999/xlink" gml:id="M-deps">` + twoSliceBorderXML() +
		airspaceXML(uuidA1, "A1", []ringMember{
			{coords: [][2]float64{{2, 50}, {2, 51}, {0, 51}, {0, 50}}},
			{href: uuidB1},
		}) +
		aggregateXML(uuidA2, "A2", [][2]string{{"BASE", uuidA1}, {"UNION", "00000000-0000-0000-0000-0000000e0099"}}) +
		`</message:AIXMBasicMessage>`
	msg, err := goaixm.Parse(context.Background(), strings.NewReader(doc), goaixm.Options{KeepRaw: true})
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// TestSourceDeps: the GeoBorder of a ring and the contributor airspaces
// are listed with the slice the conversion uses and the hash of their
// source geometry; a contributor's own border transitively (Via); an
// unresolvable contributor with slice -1.
//
// Verifies: AQ-TRF-011
func TestSourceDeps(t *testing.T) {
	msg := depMessage(t)
	a2 := sourceByDesignator(t, msg, "A2")
	deps, err := geo.Options{}.SourceDeps(a2, msg)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 3 {
		t.Fatalf("deps %+v", deps)
	}
	if d := deps[0]; d.Role != "contributor" || d.ID != uuidA1 || d.Kind != "Airspace" || d.Slice != 0 || d.Via != "" {
		t.Errorf("first %+v", d)
	} else if a1 := sourceByDesignator(t, msg, "A1"); d.SrcHash != mustHash(t, a1) {
		t.Errorf("contributor hash %s", d.SrcHash)
	}
	if d := deps[1]; d.Role != "border" || d.ID != uuidB1 || d.Kind != "GeoBorder" || d.Slice != 0 || d.Via != "urn:uuid:"+uuidA1 || d.SrcHash == "" {
		t.Errorf("second %+v", d)
	}
	if d := deps[2]; d.Role != "contributor" || d.Slice != -1 || d.Kind != "" || d.SrcHash != "" {
		t.Errorf("third %+v", d)
	}
	canon, err := geo.CanonicalDeps(deps)
	if err != nil || !strings.HasPrefix(string(canon), `[{"href":"urn:uuid:`) {
		t.Errorf("canonical deps %s %v", canon, err)
	}
	// Without the message (a streamed file): listed, unresolved.
	if deps, err := (geo.Options{}).SourceDeps(a2, nil); err != nil || len(deps) != 2 || deps[0].Slice != -1 || deps[1].Kind != "" {
		t.Errorf("without message: %+v %v", deps, err)
	}
	if c, _ := geo.CanonicalDeps(nil); string(c) != "[]" {
		t.Errorf("empty deps %s", c)
	}
}

func mustHash(t *testing.T, g *geo.SourceGeometry) string {
	t.Helper()
	h, err := g.Hash()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// TestReferenceSliceAt: by default a referenced GeoBorder is read from
// its first slice (as before); with Options.At from the slice valid then,
// both in the conversion and in SourceDeps.
func TestReferenceSliceAt(t *testing.T) {
	msg := depMessage(t)
	var a1 *aixm.Airspace
	for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
		if a.TimeSlice[0].Designator.String() == "A1" {
			a1 = a
		}
	}
	ring := func(o geo.Options) [][2]float64 {
		comps, err := o.AirspaceComponents(a1, msg)
		if err != nil {
			t.Fatal(err)
		}
		return comps[0].Geometry.(geo.Polygon).Coordinates[0]
	}
	hasDip := func(r [][2]float64) bool { return slices.Contains(r, [2]float64{1, 49.9}) }
	if r := ring(geo.Options{}); hasDip(r) {
		t.Errorf("default: border slice 2 used: %v", r)
	}
	at2021 := time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC)
	if r := ring(geo.Options{At: at2021}); !hasDip(r) {
		t.Errorf("At 2021: border slice 1 used: %v", r)
	}
	if r := ring(geo.Options{At: time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC)}); hasDip(r) {
		t.Errorf("At 2020: border slice 2 used: %v", r)
	}
	src := sourceByDesignator(t, msg, "A1")
	for _, c := range []struct {
		o     geo.Options
		slice int
	}{{geo.Options{}, 0}, {geo.Options{At: at2021}, 1}, {geo.Options{At: time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)}, 0}} {
		deps, err := c.o.SourceDeps(src, msg)
		if err != nil {
			t.Fatal(err)
		}
		if len(deps) != 1 || deps[0].Slice != c.slice {
			t.Errorf("At %v: deps %+v, want slice %d", c.o.At, deps, c.slice)
		}
	}
}
