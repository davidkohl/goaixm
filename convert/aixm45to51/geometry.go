package aixm45to51

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/davidkohl/goaixm/aixm45"
	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/geo"
	"github.com/davidkohl/goaixm/xmlnode"
)

// BorderMode says how a boundary that follows a border (Avx codeType FNT)
// is written.
type BorderMode int

const (
	// Inline writes the border stretch as positions of the airspace's own
	// ring: every Airspace is self-contained, which streaming consumers
	// need.
	Inline BorderMode = iota
	// Reference writes the stretch as a gml:curveMember referencing the
	// GeoBorder (xlink:href), which consumers resolve and cut.  Stretches
	// of a closed border are still written inline (the cut of a referenced
	// border cannot run through its closing position).
	Reference
)

// Defaults of the geometry tolerances (Config).
const (
	DefaultRhumbTolerance  = 1.0   // metres
	DefaultBorderTolerance = 100.0 // metres
)

// pos is a position: [lon, lat] as the decoder reads the written text, and
// the text ("lat lon", decimal degrees).
type pos struct {
	ll   [2]float64
	text string
}

// posOf renders a parsed 4.5 coordinate pair exactly to the source
// precision.
func posOf(lat, long aixm45.Coord) pos {
	t := lat.Decimal(lat.Decimals()) + " " + long.Decimal(long.Decimals())
	return posText(t)
}

// posText parses a "lat lon" text into a pos.
func posText(t string) pos {
	f := strings.Fields(t)
	la, _ := strconv.ParseFloat(f[0], 64)
	lo, _ := strconv.ParseFloat(f[1], 64)
	return pos{ll: [2]float64{lo, la}, text: t}
}

// computed renders a computed position ([lon, lat]) with 9 decimals (0.1
// mm), and reads it back so that every later use sees the written value.
func computed(ll [2]float64) pos {
	return posText(strconv.FormatFloat(ll[1], 'f', 9, 64) + " " + strconv.FormatFloat(ll[0], 'f', 9, 64))
}

// parsePos parses a position view; ok is false when it is unusable.
func parsePos(p aixm45.Position) (pos, bool) {
	if !is(p.Lat) || !is(p.Long) {
		return pos{}, false
	}
	lat, err1 := aixm45.ParseLat(p.Lat.Text)
	long, err2 := aixm45.ParseLong(p.Long.Text)
	if err1 != nil || err2 != nil {
		return pos{}, false
	}
	return posOf(lat, long), true
}

// vtx is one indexed boundary or border vertex.
type vtx struct {
	typ   string
	p     pos
	ok    bool   // position usable
	datum string // codeDatum text, "" when absent
	// arc: centre, radius in metres and as written (value, 5.1.1 uom).
	centre              pos
	arcOK               bool
	radius              float64
	radiusText, radiusU string
	gbr                 string // GbrUid key
}

// parseRadius checks a radius value and unit (uom_dist); metres.
func parseRadius(v, u aixm45.Value) (m float64, text, uom string, ok bool) {
	if !is(v) || !is(u) || !ntDistance.valid(v.Text) {
		return 0, "", "", false
	}
	uom, ok = list("uom_dist").Codes[u.Text]
	if !ok {
		return 0, "", "", false
	}
	f, err := strconv.ParseFloat(v.Text, 64)
	if err != nil || f <= 0 {
		return 0, "", "", false
	}
	m, err = geo.LengthToMetres(f, uom)
	return m, v.Text, uom, err == nil
}

func indexVertex(a aixm45.Avx) vtx {
	v := vtx{typ: a.CodeType.Text}
	v.p, v.ok = parsePos(a.Position)
	if is(a.Position.Datum) {
		v.datum = a.Position.Datum.Text
	}
	if c, ok := parsePos(aixm45.Position{Lat: a.ArcLat, Long: a.ArcLong}); ok {
		if m, t, u, ok := parseRadius(a.Radius, a.RadiusUom); ok {
			v.centre, v.radius, v.radiusText, v.radiusU, v.arcOK = c, m, t, u, true
		}
	}
	if a.Gbr.OK() {
		v.gbr = a.Gbr.Key
	}
	return v
}

// boundary is an indexed Abd.
type boundary struct {
	index     int
	ref       aseRef // AbdUid/AseUid
	mid       string // own AbdUid mid
	vx        []vtx
	circle    *vtx // Abd/Circle: centre in p, radius as for arcs
	hasCircle bool
	// Set when the airspaces are resolved (after pass 1).
	remarks []string
	ase     string // the airspace's numbered key, "" when not in the document
	used    bool   // merged into its airspace
	plan    *ring
}

// aseRef is a reference to an airspace: natural key and mid, resolved to
// the numbered key of one Ase occurrence.
type aseRef struct {
	base, mid string
}

func refOfAse(r aixm45.Ref) aseRef { return aseRef{base: r.Key, mid: r.MID} }

// border is an indexed Gbr: its polyline as written in the GeoBorder
// (source vertices and densified rhumb-line positions) and the source
// vertex statuses.
type border struct {
	index  int
	coords [][2]float64
	text   map[[2]float64]string
	pts    []pos
	stat   []vstat // per Gbv
	closed bool
}

// derivation is an indexed Adg.
type derivation struct {
	index int
	ref   aseRef
	mid   string
	same  *aseRef
	base  *aseRef
	parts []derivPart
	ase   string
	used  bool
}

type derivPart struct {
	op  string // codeOpr, "" when missing
	ase aseRef
}

// vstat is the outcome of one vertex (Avx or Gbv): per group of leaves.
type vstat struct {
	qual    string // path qualifier, "[codeType=…]"
	typRule string
	typ     convert.Outcome
	typNote string
	pos     convert.Outcome
	posNote string
	datum   convert.Outcome
	arc     convert.Outcome // geoLatArc, geoLongArc, valRadiusArc, uomRadiusArc
	arcNote string
	gbr     convert.Outcome // GbrUid/txtName
	used    bool            // the vertex was resolved (status valid)
}

// ---- rings ----

type segKind int

const (
	segGeodesic segKind = iota
	segArc
	segCircle
	segBorder
)

// seg is one piece of a ring as written.
type seg struct {
	kind        segKind
	pos         []string // segGeodesic: "lat lon" texts
	centre      string
	radius, uom string
	start, end  float64 // segArc: bearings, signed sweep (goaixm/geo convention)
	border      string  // segBorder: GbrUid key
}

// ring is the planned horizontal projection of an airspace.
type ring struct {
	segs []seg
	// degenerate, when set, says why the boundary does not enclose an area;
	// points then holds the source positions (for the note).
	degenerate string
	points     []string
	status     []vstat // per Avx
	circle     vstat   // Abd/Circle
	datum      string  // non-WGS-84 datum found, for the WARNING note
	remarks    []string
}

// geodesic collects the positions of the current gml:GeodesicString.
type geodesic struct {
	r   *ring
	cur []string
}

func (g *geodesic) add(p string) {
	if n := len(g.cur); n > 0 && g.cur[n-1] == p {
		return
	}
	g.cur = append(g.cur, p)
}

func (g *geodesic) flush() {
	if len(g.cur) >= 2 {
		g.r.segs = append(g.r.segs, seg{kind: segGeodesic, pos: g.cur})
	}
	g.cur = nil
}

// angle renders a bearing in degrees with 6 decimals (below 1 mm along an
// arc of 100 km radius).
func angle(a float64) string { return strconv.FormatFloat(a, 'f', 6, 64) }

// arcAngles returns the start bearing and the end bearing of an arc about
// c from p to q in goaixm's convention (geo: the end is the start plus the
// signed sweep, positive clockwise): clockwise end = start + sweep,
// counter-clockwise end = start − sweep, sweep in (0, 360).  ok is false
// for a zero sweep (p and q on the same bearing) or coincident points.
func arcAngles(c, p, q [2]float64, clockwise bool) (start, end float64, ok bool) {
	s, ok1 := geo.GeodesicAzimuth(c[1], c[0], p[1], p[0])
	e, ok2 := geo.GeodesicAzimuth(c[1], c[0], q[1], q[0])
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	// Round as written, so the sweep is computed from the written angles.
	s, _ = strconv.ParseFloat(angle(s), 64)
	e, _ = strconv.ParseFloat(angle(e), 64)
	if clockwise {
		sw := math.Mod(e-s+360, 360)
		return s, s + sw, sw > 0
	}
	sw := math.Mod(s-e+360, 360)
	return s, s - sw, sw > 0
}

// snapReport is the smallest arc-end snap (metres) reported as an issue.
const snapReport = 0.01

// arcTolerance is the radius check: max(20 m, 0.5 % of the radius).
func arcTolerance(r float64) float64 { return math.Max(20, 0.005*r) }

// geomIssue records an issue of an airspace's geometry.
type geomIssue func(code string, sev convert.Severity, msg string)

// buildRing plans the ring of a boundary, writing border stretches as
// mode says.  Vertex i and the curve from it to vertex i+1 (the last to
// the first) become segments of one closed ring: consecutive straight
// pieces (GRC, RHL densified, FNT inline) share one gml:GeodesicString.
func (r *run) buildRing(b *boundary, mode BorderMode, issue geomIssue) *ring {
	rg := &ring{status: make([]vstat, len(b.vx)), remarks: b.remarks}
	datum := func(st *vstat, d string) {
		switch d {
		case "":
			st.datum = ""
		case "WGE":
			st.datum = convert.Mapped
		default:
			st.datum = convert.Lossy
			if rg.datum == "" {
				rg.datum = d
			}
		}
	}
	if b.hasCircle {
		st := &rg.circle
		c := b.circle
		st.used, st.typRule = true, "CIR"
		datum(st, c.datum)
		if !c.ok || !c.arcOK {
			st.pos, st.arc = convert.Invalid, convert.Invalid
			if c.ok {
				st.pos = convert.Mapped
			}
			if !c.ok {
				st.posNote = "centre not usable"
			}
			if !c.arcOK {
				st.arcNote = "radius not usable"
			}
			rg.degenerate = "circle without a usable centre or radius"
			issue("geometry_invalid", convert.Error, "Abd/Circle: centre or radius not usable: no horizontal projection")
		} else {
			st.pos, st.arc = convert.Mapped, convert.Mapped
			rg.segs = append(rg.segs, seg{kind: segCircle, centre: c.p.text, radius: c.radiusText, uom: c.radiusU})
		}
		if len(b.vx) > 0 {
			issue("geometry_invalid", convert.Error, "Abd has both a Circle and vertices: the vertices are not used")
		}
		for i := range b.vx {
			rg.status[i] = vstat{used: false}
		}
		r.finishDatum(rg, issue)
		return rg
	}
	// Usable vertices.
	var idx []int
	for i, v := range b.vx {
		st := &rg.status[i]
		st.used = true
		st.qual = "[codeType=" + v.typ + "]"
		datum(st, v.datum)
		if !v.ok {
			st.pos, st.posNote = convert.Invalid, "position not usable: vertex dropped"
			st.typ, st.typRule, st.typNote = convert.Invalid, "AVX-POS", "vertex dropped"
			st.arc, st.gbr = convert.Invalid, convert.Invalid
			issue("coordinate_invalid", convert.Error, fmt.Sprintf("boundary vertex %d: position not usable: vertex dropped", i+1))
			continue
		}
		st.pos = convert.Mapped
		idx = append(idx, i)
	}
	// Arc ends: a vertex next to an arc whose computed end (as a consumer
	// computes it from centre, radius and angle: same bearing, the nominal
	// radius) lies within the radius tolerance of the arc check is written
	// as that computed end, so the ring is continuous and does not cross
	// itself where the vertex lies a little off the circle; vertices
	// farther away are kept (arc_radius_mismatch; the consumer bridges the
	// gap).
	type arcPlan struct {
		start, end float64
		ok         bool
	}
	plans := make([]arcPlan, len(idx))
	eff := make([]pos, len(idx))
	snapped := make([]float64, len(idx))
	for k, i := range idx {
		eff[k] = b.vx[i].p
		snapped[k] = -1
	}
	at := func(v vtx, angle float64) pos {
		la, lo := geo.DestinationPoint(v.centre.ll[1], v.centre.ll[0], angle, v.radius)
		return computed([2]float64{lo, la})
	}
	snap := func(k int, p pos, tol float64) {
		if d := geo.GeodesicDistance(p.ll[1], p.ll[0], eff[k].ll[1], eff[k].ll[0]); d <= tol && snapped[k] < 0 {
			eff[k], snapped[k] = p, d
		}
	}
	for k, i := range idx {
		v, w := b.vx[i], b.vx[idx[(k+1)%len(idx)]]
		if (v.typ == "CWA" || v.typ == "CCA") && v.arcOK {
			start, end, ok := arcAngles(v.centre.ll, v.p.ll, w.p.ll, v.typ == "CWA")
			plans[k] = arcPlan{start, end, ok}
			if ok {
				snap(k, at(v, start), arcTolerance(v.radius))
			}
		}
	}
	for k, i := range idx {
		if plans[k].ok {
			snap((k+1)%len(idx), at(b.vx[i], plans[k].end), arcTolerance(b.vx[i].radius))
		}
	}
	nSnap, maxSnap := 0, 0.0
	for k, i := range idx {
		if snapped[k] > 0 {
			rg.status[i].pos, rg.status[i].posNote = convert.Lossy, fmt.Sprintf("replaced by the computed arc end %.3f m away", snapped[k])
			nSnap++
			maxSnap = math.Max(maxSnap, snapped[k])
		}
	}
	if maxSnap >= snapReport {
		// Reported, not silent: the arc's radius and the source vertices
		// disagree by up to maxSnap (within the arc tolerance).  Below 1 cm
		// it is rounding (the report still has each as lossy).
		issue("arc_end_snapped", convert.Info, fmt.Sprintf("%d boundary vertices next to arcs replaced by the computed arc ends (up to %.2f m away): radius and vertices disagree within the arc tolerance", nSnap, maxSnap))
	}
	g := &geodesic{r: rg}
	arcs := 0
	for k, i := range idx {
		v := b.vx[i]
		w := b.vx[idx[(k+1)%len(idx)]]
		v.p, w.p = eff[k], eff[(k+1)%len(idx)]
		src, srcW := b.vx[i].p, w
		srcW.p = b.vx[idx[(k+1)%len(idx)]].p
		st := &rg.status[i]
		st.arc, st.gbr = convert.Unmapped, convert.Unmapped // overwritten where used
		typ := v.typ
		switch typ {
		case "GRC", "RHL", "CWA", "CCA", "FNT":
		default:
			st.typ, st.typRule, st.typNote = convert.Invalid, "AVX-GRC", "unknown codeType: taken as a great circle"
			issue("code_unknown", convert.Error, fmt.Sprintf("boundary vertex %d: codeType %q unknown: taken as a great circle (GRC)", i+1, typ))
			typ = "GRC"
		}
		switch typ {
		case "GRC":
			if st.typRule == "" {
				st.typ, st.typRule = convert.Mapped, "AVX-GRC"
			}
			g.add(v.p.text)
		case "RHL":
			g.add(v.p.text)
			pts := geo.RhumbLine(v.p.ll, w.p.ll, r.rhumbTol)
			for _, p := range pts {
				g.add(computed(p).text)
			}
			st.typ, st.typRule, st.typNote = convert.Lossy, "AVX-RHL", fmt.Sprintf("rhumb line as a geodesic string, %d positions added", len(pts))
		case "CWA", "CCA":
			st.typRule = "AVX-ARC"
			if !v.arcOK {
				st.typ, st.typNote = convert.Invalid, "arc centre or radius not usable: taken as a great circle"
				st.arc = convert.Invalid
				issue("arc_invalid", convert.Error, fmt.Sprintf("boundary vertex %d: arc centre or radius not usable: great circle used", i+1))
				g.add(v.p.text)
				break
			}
			st.arc = convert.Mapped
			start, end, ok := plans[k].start, plans[k].end, plans[k].ok
			if !ok {
				st.typ, st.typNote = convert.Lossy, "zero-length arc omitted"
				issue("arc_degenerate", convert.Warning, fmt.Sprintf("boundary vertex %d: arc of zero length (its end is its start): omitted", i+1))
				g.add(v.p.text)
				break
			}
			st.typ = convert.Derived
			tol := arcTolerance(v.radius)
			dp := geo.GeodesicDistance(v.centre.ll[1], v.centre.ll[0], src.ll[1], src.ll[0])
			dq := geo.GeodesicDistance(v.centre.ll[1], v.centre.ll[0], srcW.p.ll[1], srcW.p.ll[0])
			if d := math.Max(math.Abs(dp-v.radius), math.Abs(dq-v.radius)); d > tol {
				issue("arc_radius_mismatch", convert.Warning, fmt.Sprintf("boundary vertex %d: arc ends %.0f m and %.0f m from the centre, radius %.0f m (tolerance %.0f m); radius kept", i+1, dp, dq, v.radius, tol))
			}
			g.add(v.p.text)
			g.flush()
			rg.segs = append(rg.segs, seg{kind: segArc, centre: v.centre.text, radius: v.radiusText, uom: v.radiusU, start: start, end: end})
			arcs++
		case "FNT":
			st.typRule = "AVX-FNT"
			bd := r.borders[v.gbr]
			if bd == nil || len(bd.coords) < 2 {
				st.typ, st.typNote, st.gbr = convert.Invalid, "border not in the document: great circle used", convert.Invalid
				what := "no GbrUid"
				if v.gbr != "" {
					what = "border " + v.gbr + " not in the document"
				}
				issue("border_missing", convert.Error, fmt.Sprintf("boundary vertex %d: %s: great circle used", i+1, what))
				g.add(v.p.text)
				break
			}
			st.typ, st.gbr = convert.Derived, convert.Derived
			s, err := geo.BorderStretch(bd.coords, v.p.ll, w.p.ll, geo.StretchOptions{Wrap: true})
			if err != nil { // not reached: the border has two positions
				g.add(v.p.text)
				break
			}
			if s.FromOffset > r.borderTol {
				issue("border_far", convert.Warning, fmt.Sprintf("boundary vertex %d: %.0f m from border %s (tolerance %.0f m)", i+1, s.FromOffset, v.gbr, r.borderTol))
			}
			if s.ToOffset > r.borderTol {
				issue("border_far", convert.Warning, fmt.Sprintf("boundary vertex %d: end of the border stretch %.0f m from border %s (tolerance %.0f m)", i+1, s.ToOffset, v.gbr, r.borderTol))
			}
			if s.Closed {
				issue("border_ambiguous", convert.Warning, fmt.Sprintf("boundary vertex %d: border %s is closed: the shorter way round was taken", i+1, v.gbr))
			}
			if mode == Reference && !s.Closed {
				g.add(v.p.text)
				g.flush()
				rg.segs = append(rg.segs, seg{kind: segBorder, border: v.gbr})
				break
			}
			if mode == Reference {
				issue("border_inline", convert.Info, fmt.Sprintf("boundary vertex %d: closed border %s written inline", i+1, v.gbr))
			}
			g.add(v.p.text)
			for k := 1; k+1 < len(s.Coords); k++ {
				g.add(bd.text[s.Coords[k]])
			}
		}
	}
	if len(idx) > 0 {
		g.add(eff[0].text)
	}
	g.flush()
	// Degenerate: fewer than three distinct positions and no arc.
	distinct := map[string]bool{}
	for _, s := range rg.segs {
		for _, p := range s.pos {
			distinct[p] = true
		}
	}
	for _, i := range idx {
		distinct[b.vx[i].p.text] = true
	}
	if arcs == 0 && !hasBorderRef(rg.segs) && len(distinct) < 3 {
		rg.segs = nil
		for _, i := range idx {
			rg.points = append(rg.points, b.vx[i].p.text)
		}
		rg.degenerate = fmt.Sprintf("%d distinct position(s), no area", len(distinct))
		if len(idx) == 0 {
			rg.degenerate = "no usable vertex"
		}
		for _, i := range idx {
			st := &rg.status[i]
			st.pos, st.posNote = convert.Lossy, "boundary encloses no area: position kept in a note"
			if st.typ == convert.Mapped || st.typ == convert.Derived {
				st.typ = convert.Lossy
				st.typNote = "boundary encloses no area"
			}
		}
		issue("geometry_degenerate", convert.Warning, "boundary encloses no area ("+rg.degenerate+"): no horizontal projection, positions kept in a note")
	}
	r.finishDatum(rg, issue)
	return rg
}

func hasBorderRef(segs []seg) bool {
	for _, s := range segs {
		if s.kind == segBorder {
			return true
		}
	}
	return false
}

func (r *run) finishDatum(rg *ring, issue geomIssue) {
	missing := false
	for _, st := range append([]vstat{rg.circle}, rg.status...) {
		if st.used && st.datum == "" {
			missing = true
		}
	}
	if missing {
		issue("datum_missing", convert.Warning, "boundary vertex without codeDatum: coordinates taken as WGS-84")
	}
	switch rg.datum {
	case "":
	case "U":
		issue("datum_unknown", convert.Error, "boundary coordinates on unknown datum (U), carried untransformed")
	default:
		issue("datum_not_wgs84", convert.Error, "boundary coordinates on datum "+rg.datum+", carried untransformed")
	}
}

// surface renders the horizontal projection of a ring as an aixm:Surface.
func (r *run) surface(f *feature, rg *ring) string {
	var b strings.Builder
	b.WriteString(`<aixm:Surface gml:id="` + f.id("sfc") + `" srsName="` + srs4326 + `"><gml:patches><gml:PolygonPatch><gml:exterior><gml:Ring>`)
	open := false
	member := func() {
		if !open {
			b.WriteString(`<gml:curveMember><gml:Curve gml:id="` + f.id("crv") + `"><gml:segments>`)
			open = true
		}
	}
	closeMember := func() {
		if open {
			b.WriteString(`</gml:segments></gml:Curve></gml:curveMember>`)
			open = false
		}
	}
	for _, s := range rg.segs {
		switch s.kind {
		case segGeodesic:
			member()
			b.WriteString(`<gml:GeodesicString><gml:posList>` + strings.Join(s.pos, " ") + `</gml:posList></gml:GeodesicString>`)
		case segArc:
			member()
			b.WriteString(`<gml:ArcByCenterPoint numArc="1"><gml:pos>` + s.centre + `</gml:pos><gml:radius uom="` + s.uom + `">` + s.radius +
				`</gml:radius><gml:startAngle uom="deg">` + angle(s.start) + `</gml:startAngle><gml:endAngle uom="deg">` + angle(s.end) + `</gml:endAngle></gml:ArcByCenterPoint>`)
		case segCircle:
			member()
			b.WriteString(`<gml:CircleByCenterPoint numArc="1"><gml:pos>` + s.centre + `</gml:pos><gml:radius uom="` + s.uom + `">` + s.radius + `</gml:radius></gml:CircleByCenterPoint>`)
		case segBorder:
			closeMember()
			b.WriteString(`<gml:curveMember xlink:href="urn:uuid:` + r.uuid(s.border) + `"/>`)
		}
	}
	closeMember()
	b.WriteString(`</gml:Ring></gml:exterior></gml:PolygonPatch></gml:patches></aixm:Surface>`)
	return b.String()
}

// ---- borders ----

// indexBorder builds the polyline of a Gbr as its GeoBorder writes it.
func (r *run) indexBorder(g aixm45.Gbr, index int) *border {
	bd := &border{index: index, text: map[[2]float64]string{}, stat: make([]vstat, len(g.Vertex))}
	var ok []int
	for i, a := range g.Vertex {
		st := &bd.stat[i]
		st.used = true
		st.qual = "[codeType=" + a.CodeType.Text + "]"
		if _, good := parsePos(a.Position); !good {
			st.pos, st.typ, st.typRule, st.posNote = convert.Invalid, convert.Invalid, "GBV-POS", "position not usable: vertex dropped"
			continue
		}
		st.pos = convert.Mapped
		ok = append(ok, i)
	}
	add := func(p pos) {
		if n := len(bd.pts); n > 0 && bd.pts[n-1].text == p.text {
			return
		}
		bd.pts = append(bd.pts, p)
	}
	for k, i := range ok {
		a := g.Vertex[i]
		p, _ := parsePos(a.Position)
		st := &bd.stat[i]
		add(p)
		last := k == len(ok)-1
		switch a.CodeType.Text {
		case "GRC":
			st.typ, st.typRule = convert.Mapped, "GBV-TYPE"
		case "END":
			st.typ, st.typRule = convert.Mapped, "GBV-TYPE"
			if !last {
				st.typNote = "END before the last vertex: taken as a great circle"
				st.typ = convert.Lossy
			}
		case "RHL":
			st.typ, st.typRule = convert.Lossy, "GBV-RHL"
			if !last {
				q, _ := parsePos(g.Vertex[ok[k+1]].Position)
				pts := geo.RhumbLine(p.ll, q.ll, r.rhumbTol)
				for _, c := range pts {
					add(computed(c))
				}
				st.typNote = fmt.Sprintf("rhumb line as a geodesic string, %d positions added", len(pts))
			}
		default:
			st.typ, st.typRule, st.typNote = convert.Invalid, "GBV-TYPE", "unknown codeType: taken as a great circle"
		}
	}
	for _, p := range bd.pts {
		bd.coords = append(bd.coords, p.ll)
		bd.text[p.ll] = p.text
	}
	n := len(bd.coords)
	bd.closed = n >= 4 && bd.coords[0] == bd.coords[n-1]
	return bd
}

// ---- accounting of geometry leaves ----

// markVertex marks the leaves of an Avx or Gbv element from its status.
func (e *elem) markVertex(n *xmlnode.Node, st vstat, posRule string) {
	if st.qual != "" {
		e.qual[n] = st.qual
	}
	a := aixm45.NewAvx(n)
	o := func(x convert.Outcome) convert.Outcome {
		if x == "" {
			return convert.Unmapped
		}
		return x
	}
	if st.typRule != "" {
		e.markAs(a.CodeType, st.typRule, o(st.typ), st.typNote)
	}
	e.markAs(a.Position.Lat, posRule, o(st.pos), st.posNote)
	e.markAs(a.Position.Long, posRule, o(st.pos), st.posNote)
	if st.datum != "" {
		note := ""
		if st.datum == convert.Lossy {
			note = "coordinates not transformed"
		}
		e.markAs(a.Position.Datum, "GEO-DATUM", st.datum, note)
	}
	for _, v := range []aixm45.Value{a.ArcLat, a.ArcLong, a.Radius, a.RadiusUom} {
		e.markAs(v, "AVX-ARC", o(st.arc), st.arcNote)
	}
	if a.Gbr.OK() {
		e.markTree(a.Gbr.Node, "AVX-FNT", o(st.gbr))
	}
	e.mark(a.Crc, "GEN-CRC")
	for _, c := range n.Children {
		switch name := c.Local(); {
		case strings.HasSuffix(name, "UidCen"):
			e.markTree(c, "AVX-CENREF", convert.Unmapped)
		case strings.HasSuffix(name, "UidSpn"):
			e.markTree(c, "AVX-SPN", convert.Unmapped)
		}
	}
}
