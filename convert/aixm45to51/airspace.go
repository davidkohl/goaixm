package aixm45to51

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/davidkohl/goaixm/aixm45"
	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/xmlnode"
)

// Airspaces: Ase (properties) with its Abd (boundary) or Adg (derivation)
// becomes one Airspace; Gbr becomes a GeoBorder.  Pass 1 indexes Abd, Adg
// and Gbr compactly (geometry.go); between the passes resolveAirspaces
// pairs them with their airspaces and plans every ring; pass 2 writes each
// Airspace at its Ase and accounts for Abd and Adg leaves at their own
// position from the planned statuses.

// aseOcc is one occurrence of an Ase in document order.
type aseOcc struct {
	key, mid string
	index    int
}

// airspaceIndex is the pass-1 state of airspaces and borders.
type airspaceIndex struct {
	aseOcc      map[string][]aseOcc // by base key
	boundaries  []*boundary
	boundaryAt  map[int]*boundary
	derivations []*derivation
	derivAt     map[int]*derivation
	borders     map[string]*border // by base key (first occurrence)
	borderAt    map[int]*border
	abdOf       map[string][]*boundary   // by numbered Ase key
	adgOf       map[string][]*derivation // by numbered Ase key
	rhumbTol    float64
	borderTol   float64
}

func (x *airspaceIndex) init(cfg Config) {
	x.aseOcc = map[string][]aseOcc{}
	x.boundaryAt, x.derivAt = map[int]*boundary{}, map[int]*derivation{}
	x.borders, x.borderAt = map[string]*border{}, map[int]*border{}
	x.abdOf, x.adgOf = map[string][]*boundary{}, map[string][]*derivation{}
	x.rhumbTol, x.borderTol = cfg.RhumbTolerance, cfg.BorderTolerance
	if x.rhumbTol <= 0 {
		x.rhumbTol = DefaultRhumbTolerance
	}
	if x.borderTol <= 0 {
		x.borderTol = DefaultBorderTolerance
	}
}

// index1 indexes one element in pass 1.
func (r *run) index1(e aixm45.Element, kind, base, key string) {
	n := e.Node
	switch kind {
	case "Ase":
		mid, _ := n.Child("AseUid").Attr("mid")
		r.aseOcc[base] = append(r.aseOcc[base], aseOcc{key, mid, e.Index})
	case "Abd":
		v := aixm45.NewAbd(n)
		b := &boundary{index: e.Index, ref: refOfAse(v.Ase), mid: v.Uid.MID}
		for _, a := range v.Vertex {
			b.vx = append(b.vx, indexVertex(a))
		}
		if c := v.Circle; c != nil {
			cv := vtx{typ: "CIR"}
			cv.p, cv.ok = parsePos(c.Centre)
			if is(c.Centre.Datum) {
				cv.datum = c.Centre.Datum.Text
			}
			if m, t, u, ok := parseRadius(c.Radius, c.RadiusUom); ok {
				cv.radius, cv.radiusText, cv.radiusU, cv.arcOK = m, t, u, true
			}
			b.circle, b.hasCircle = &cv, true
		}
		b.remarks = boundaryRemarks(v)
		r.boundaries = append(r.boundaries, b)
		r.boundaryAt[e.Index] = b
	case "Adg":
		v := aixm45.NewAdg(n)
		d := &derivation{index: e.Index, ref: refOfAse(v.Ase), mid: v.Uid.MID}
		if v.SameExtent.OK() {
			a := refOfAse(v.SameExtent)
			d.same = &a
		}
		if v.Base.OK() {
			a := refOfAse(v.Base)
			d.base = &a
		}
		for _, p := range v.Parts {
			d.parts = append(d.parts, derivPart{op: p.Opr.Text, ase: refOfAse(p.Ase)})
		}
		r.derivations = append(r.derivations, d)
		r.derivAt[e.Index] = d
	case "Gbr":
		bd := r.indexBorder(aixm45.NewGbr(n), e.Index)
		r.borderAt[e.Index] = bd
		if key == base {
			r.borders[base] = bd
		}
	}
}

// resolveAse returns the numbered key of the Ase a reference designates:
// the only occurrence of its natural key, else the one whose own mid equals
// the reference's mid, else the first ("" when the key is not in the
// document).  ambiguous reports the last case.
func (r *run) resolveAse(a aseRef) (key string, ambiguous bool) {
	occ := r.aseOcc[a.base]
	switch {
	case len(occ) == 0:
		return "", false
	case len(occ) == 1:
		return occ[0].key, false
	}
	for _, o := range occ {
		if a.mid != "" && o.mid == a.mid {
			return o.key, false
		}
	}
	return occ[0].key, true
}

// resolveAirspaces pairs boundaries and derivations with their airspaces
// and plans the rings (after pass 1).
func (r *run) resolveAirspaces() {
	pair := func(kind string, a aseRef) string {
		key, amb := r.resolveAse(a)
		if amb {
			r.rep.AddIssue(convert.Issue{Code: "ref_ambiguous", Severity: convert.Warning, Kind: kind, Key: a.base,
				Message: "airspace key occurs more than once and no mid matches: paired with the first occurrence"})
		}
		if key == "" {
			r.rep.AddIssue(convert.Issue{Code: "dangling_ref", Severity: convert.Warning, Kind: kind, Key: a.base,
				Message: "boundary or derivation of an airspace that is not in the document: dropped"})
		}
		return key
	}
	for _, b := range r.boundaries {
		if b.ase = pair("Abd", b.ref); b.ase != "" {
			r.abdOf[b.ase] = append(r.abdOf[b.ase], b)
		}
	}
	for _, d := range r.derivations {
		if d.ase = pair("Adg", d.ref); d.ase != "" {
			r.adgOf[d.ase] = append(r.adgOf[d.ase], d)
		}
	}
	// Document order of the airspaces, for deterministic issues.
	var keys []aseOcc
	for _, occ := range r.aseOcc {
		keys = append(keys, occ...)
	}
	sortOcc(keys)
	for _, o := range keys {
		bs, ds := r.abdOf[o.key], r.adgOf[o.key]
		issue := func(code string, sev convert.Severity, msg string) {
			r.rep.AddIssue(convert.Issue{Code: code, Severity: sev, Kind: "Ase", Key: o.key, Message: msg})
		}
		switch {
		case len(bs) > 0:
			bs[0].used = true
			bs[0].plan = r.buildRing(bs[0], issue)
			if len(bs) > 1 {
				issue("boundary_duplicate", convert.Error, fmt.Sprintf("%d boundaries (Abd) for one airspace: the first is used", len(bs)))
			}
			if len(ds) > 0 {
				issue("geometry_both", convert.Info, "airspace has a boundary (Abd) and a derivation (Adg): the boundary is used")
			}
		case len(ds) > 0:
			ds[0].used = true
			if len(ds) > 1 {
				issue("derivation_duplicate", convert.Error, fmt.Sprintf("%d derivations (Adg) for one airspace: the first is used", len(ds)))
			}
		}
	}
}

func sortOcc(o []aseOcc) {
	for i := 1; i < len(o); i++ {
		for j := i; j > 0 && o[j].index < o[j-1].index; j-- {
			o[j], o[j-1] = o[j-1], o[j]
		}
	}
}

// ---- Ase ----

var (
	ttAseDesig = textType{"CodeAirspaceDesignatorType", 1, 10, reChar3}
	ttTime     = textType{"TimeType", 5, 5, regexp.MustCompile(`^((([0-1][0-9]|2[0-3]):[0-5][0-9])|(24:00))$`)}
	ttDate     = textType{"DateMonthDayType", 5, 5, regexp.MustCompile(`^((((0[1-9])|(1[0-9])|(2[0-9]))-((0[1-9])|10|11|12))|(30-(01|03|04|05|06|07|08|09|10|11|12))|(31-(01|03|05|07|08|10|12))|SDLST|EDLST)$`)}
)

func (e *elem) ase() error {
	v := aixm45.NewAse(e.n)
	f := newFeature("Airspace", e.uuid)
	p := f.props
	e.code(p, "type", v.CodeType, "ASE-TYPE", "airspace_type")
	e.str(p, "designator", v.CodeID, "ASE-DESIG", ttAseDesig)
	e.str(p, "localType", v.LocalType, "ASE-LOCALTYPE", ttName)
	e.str(p, "name", v.Name, "ASE-NAME", ttName)
	if is(v.CodeClass) {
		cp := f.newProps()
		if _, ok := e.code(cp, "classification", v.CodeClass, "ASE-CLASS", "airspace_class"); ok {
			p.add("class", wrap("class", cp.object("AirspaceLayerClass", "alc", orderLayerClass)))
		}
	}
	e.mark(v.CodeLocInd, "ASE-LOCIND")
	limits := f.newProps()
	e.limits(limits, v)
	prov := e.provenance()
	bs, ds := e.r.abdOf[e.key], e.r.adgOf[e.key]
	switch {
	case len(bs) > 0:
		e.boundaryComponent(f, limits, bs[0], &prov)
	case len(ds) > 0:
		e.derivedComponents(f, limits, ds[0], &prov)
	default:
		e.issue("geometry_missing", convert.Warning, e.key, "airspace without a boundary (Abd) or derivation (Adg): no horizontal projection")
		if len(limits.list) > 0 {
			vol := f.newProps()
			vol.list = append(vol.list, limits.list...)
			p.add("geometryComponent", wrap("geometryComponent", component(f, "BASE", 1, vol)))
		}
	}
	e.activations(f, v)
	e.common(p, v.Common)
	e.ks.Out++
	e.ks.Default(e.kind+"/(featureLifetime)", "TS-LIFE", rule("TS-LIFE").Target)
	return e.r.put(f, orderAirspace, prov)
}

func component(f *feature, op string, seq int, vol *props) string {
	c := f.newProps()
	c.add("operation", el("operation", op))
	c.add("operationSequence", el("operationSequence", strconv.Itoa(seq)))
	c.add("theAirspaceVolume", wrap("theAirspaceVolume", vol.object("AirspaceVolume", "vol", orderVolume)))
	return c.object("AirspaceGeometryComponent", "agc", orderGeomComp)
}

func addSource(prov *convert.Provenance, kind string, index int, mid string) {
	prov.SourceKind += "+" + kind
	prov.SourceIndex = append(prov.SourceIndex, index)
	if mid != "" {
		prov.SourceIDs = append(prov.SourceIDs, mid)
	}
}

// boundaryComponent writes the airspace's one BASE volume with the
// horizontal projection of its boundary.
func (e *elem) boundaryComponent(f *feature, limits *props, b *boundary, prov *convert.Provenance) {
	addSource(prov, "Abd", b.index, b.mid)
	rg := b.plan
	vol := f.newProps()
	vol.list = append(vol.list, limits.list...)
	if rg.degenerate == "" {
		vol.add("horizontalProjection", wrap("horizontalProjection", e.r.surface(f, rg)))
	} else {
		text := "Source boundary encloses no area (" + rg.degenerate + ")"
		if len(rg.points) > 0 {
			text += "; positions (lat long, decimal degrees): " + strings.Join(rg.points, "; ")
		}
		vol.note("horizontalProjection", "REMARK", text)
	}
	for _, rm := range rg.remarks {
		vol.note("horizontalProjection", "REMARK", rm)
	}
	if rg.datum != "" {
		f.props.note("", "WARNING", fmt.Sprintf("Horizontal datum of the source boundary: %s (AIXM 4.5 codeDatum); coordinates not transformed to WGS-84.", rg.datum))
	}
	f.props.add("geometryComponent", wrap("geometryComponent", component(f, "BASE", 1, vol)))
}

// aggregation maps codeOpr to CodeAirspaceAggregationType.
func aggregation(op string) (string, bool) {
	to, ok := list("aggregation").Codes[op]
	return to, ok
}

// derivedComponents writes the components of a derived airspace: one per
// operand, each a volume whose contributorAirspace is the operand.  With
// limits of its own the dependency is HORZ_PROJECTION and every volume
// carries the limits; without, FULL_GEOMETRY.
func (e *elem) derivedComponents(f *feature, limits *props, d *derivation, prov *convert.Provenance) {
	addSource(prov, "Adg", d.index, d.mid)
	dep := "FULL_GEOMETRY"
	if len(limits.list) > 0 {
		dep = "HORZ_PROJECTION"
	}
	seq := 0
	add := func(op string, a aseRef) {
		key, _ := e.r.resolveAse(a)
		if key == "" {
			key = a.base
			e.issue("dangling_ref", convert.Warning, a.base, "contributor airspace is not in the document")
		}
		seq++
		vol := f.newProps()
		vol.list = append(vol.list, limits.list...)
		dp := f.newProps()
		dp.add("dependency", el("dependency", dep))
		dp.add("theAirspace", href("theAirspace", e.r.uuid(key)))
		vol.add("contributorAirspace", wrap("contributorAirspace", dp.object("AirspaceVolumeDependency", "avd", orderDependency)))
		f.props.add("geometryComponent", wrap("geometryComponent", component(f, op, seq, vol)))
	}
	switch {
	case d.same != nil:
		add("BASE", *d.same)
		if d.base != nil || len(d.parts) > 0 {
			e.issue("derivation_invalid", convert.Error, e.key, "Adg with AseUidSameExtent and components: the components are not used")
		}
	default:
		parts := d.parts
		if d.base != nil {
			add("BASE", *d.base)
		} else if len(parts) > 0 {
			e.issue("derivation_invalid", convert.Warning, e.key, "Adg without AseUidBase: the first component is the base")
			add("BASE", parts[0].ase)
			parts = parts[1:]
		}
		for _, p := range parts {
			op, ok := aggregation(p.op)
			if !ok {
				e.issue("code_unknown", convert.Error, e.key, fmt.Sprintf("Adg codeOpr %q unknown: component not used", p.op))
				continue
			}
			add(op, p.ase)
		}
	}
}

// limits writes the vertical limits (upper, maximum, lower, minimum).
func (e *elem) limits(p *props, v aixm45.Ase) {
	for _, l := range []struct {
		lim        aixm45.Limit
		name, rule string
	}{{v.Upper, "upperLimit", "ASE-UPPER"}, {v.Max, "maximumLimit", "ASE-MAX"}, {v.Lower, "lowerLimit", "ASE-LOWER"}, {v.Min, "minimumLimit", "ASE-MIN"}} {
		if !l.lim.OK() {
			continue
		}
		e.measure(p, l.name, l.lim.Val, l.lim.Uom, l.rule, ntVertical, "uom_distver")
		ref, ok := e.code(p, l.name+"Reference", l.lim.Code, l.rule+"REF", "vertical_ref")
		if ok && is(l.lim.Uom) && (l.lim.Uom.Text == "FL") != (ref == "STD") {
			e.issue("limit_inconsistent", convert.Warning, e.key, fmt.Sprintf("%s: unit %s with reference %s (flight levels are STD)", l.name, l.lim.Uom.Text, l.lim.Code.Text))
		}
	}
}

// activations writes the working hours (Att) and the activity.
func (e *elem) activations(f *feature, v aixm45.Ase) {
	var acts []*props
	for _, n := range v.Att {
		a := aixm45.NewAtt(n)
		ap := f.newProps()
		status := ""
		switch a.WorkHr.Text {
		case "H24":
			ap.add("timeInterval", wrap("timeInterval", sheet(f, "00:00", "", "24:00", "")))
			status = "ACTIVE"
		case "HJ":
			ap.add("timeInterval", wrap("timeInterval", sheet(f, "", "SR", "", "SS")))
			status = "ACTIVE"
		case "HN":
			ap.add("timeInterval", wrap("timeInterval", sheet(f, "", "SS", "", "SR")))
			status = "ACTIVE"
		case "TIMSH":
			status = "ACTIVE"
		}
		switch {
		case !is(a.WorkHr):
		case status != "":
			e.mark(a.WorkHr, "ATT-WORKHR")
		default:
			ap.note("", "REMARK", "Working hours (AIXM 4.5 codeWorkHr): "+a.WorkHr.Text)
			e.markAs(a.WorkHr, "ATT-WORKHR", convert.Lossy, "no fixed hours: kept as a note")
		}
		for _, t := range a.Timsh {
			if tp := e.timesheet(f, t); tp != "" {
				ap.add("timeInterval", wrap("timeInterval", tp))
			}
		}
		e.note(ap, "", "REMARK", a.RmkWorkHr, "ATT-RMK")
		if status != "" {
			ap.add("status", el("status", status))
		}
		acts = append(acts, ap)
	}
	if is(v.CodeActivity) {
		if len(acts) == 0 {
			acts = append(acts, f.newProps())
		}
		e.code(acts[0], "activity", v.CodeActivity, "ASE-ACTIVITY", "activity")
	}
	for _, ap := range acts {
		if len(ap.list) > 0 {
			f.props.add("activation", wrap("activation", ap.object("AirspaceActivation", "act", orderActivation)))
		}
	}
}

// sheet renders a daily timesheet (UTC, any day) from start time or event
// to end time or event.
func sheet(f *feature, start, startEvent, end, endEvent string) string {
	p := f.newProps()
	p.add("timeReference", el("timeReference", "UTC"))
	p.add("day", el("day", "ANY"))
	if start != "" {
		p.add("startTime", el("startTime", start))
	}
	if startEvent != "" {
		p.add("startEvent", el("startEvent", startEvent))
	}
	if end != "" {
		p.add("endTime", el("endTime", end))
	}
	if endEvent != "" {
		p.add("endEvent", el("endEvent", endEvent))
	}
	return p.object("Timesheet", "tsh", orderTimesheet)
}

// timesheet renders a Timsh row.
func (e *elem) timesheet(f *feature, t aixm45.Timsh) string {
	p := f.newProps()
	if _, ok := e.code(p, "timeReference", t.TimeRef, "TSH-REF", "time_ref"); ok && t.TimeRef.Text == "UTCW" {
		p.add("daylightSavingAdjust", el("daylightSavingAdjust", "YES"))
	}
	e.str(p, "startDate", t.DateWef, "TSH-DATE", ttDate)
	e.str(p, "endDate", t.DateTil, "TSH-DATE", ttDate)
	e.code(p, "day", t.Day, "TSH-DAY", "day")
	e.code(p, "dayTil", t.DayTil, "TSH-DAY", "day")
	e.str(p, "startTime", t.TimeWef, "TSH-TIME", ttTime)
	e.code(p, "startEvent", t.EventWef, "TSH-EVENT", "time_event")
	e.relative(p, "startTimeRelativeEvent", t.RelEventWef)
	e.code(p, "startEventInterpretation", t.CombWef, "TSH-COMB", "time_comb")
	e.str(p, "endTime", t.TimeTil, "TSH-TIME", ttTime)
	e.code(p, "endEvent", t.EventTil, "TSH-EVENT", "time_event")
	e.relative(p, "endTimeRelativeEvent", t.RelEventTil)
	e.code(p, "endEventInterpretation", t.CombTil, "TSH-COMB", "time_comb")
	if len(p.list) == 0 {
		return ""
	}
	return p.object("Timesheet", "tsh", orderTimesheet)
}

// relative writes a time relative to an event (4.5: minutes).
func (e *elem) relative(p *props, prop string, v aixm45.Value) {
	if !is(v) {
		return
	}
	if !ntSigned.valid(v.Text) {
		e.markAs(v, "TSH-REL", convert.Invalid, "not a number of minutes")
		e.issue("value_invalid", convert.Error, e.key, fmt.Sprintf("%s %q is not a number of minutes: dropped", prop, v.Text))
		return
	}
	p.add(prop, el(prop, v.Text, "uom", "MIN"))
	e.mark(v, "TSH-REL")
}

// ---- Abd, Adg (merged) ----

func (e *elem) abd() error {
	b := e.r.boundaryAt[e.index]
	if b == nil || !b.used {
		e.ks.Dropped++
		e.fallback = "ABD-ORPHAN"
		if b != nil && b.ase != "" {
			e.fallback = "ABD-DUP"
		}
		e.qual[e.n] = "[" + e.fallback + "]"
		return nil
	}
	e.ks.Merged++
	v := aixm45.NewAbd(e.n)
	e.markTree(v.Ase.Node, "ABD-ASE", convert.Derived)
	if v.Uid.MID != "" {
		e.set(v.Uid.Node, "mid", "ID-MID", convert.Mapped, "")
	}
	rg := b.plan
	for i, a := range v.Vertex {
		st := rg.status[i]
		if !st.used {
			e.qual[a.Node] = "[ABD-UNUSED]"
			e.markTree(a.Node, "ABD-UNUSED", convert.Unmapped)
			continue
		}
		e.markVertex(a.Node, st, "AVX-POS")
		if is(a.Remark) {
			e.markAs(a.Remark, "AVX-RMK", rule("AVX-RMK").Outcome, "")
		}
	}
	if c := v.Circle; c != nil {
		st := rg.circle
		o := func(x convert.Outcome) convert.Outcome {
			if x == "" {
				return convert.Unmapped
			}
			return x
		}
		e.markAs(c.Centre.Lat, "CIR", o(st.pos), st.posNote)
		e.markAs(c.Centre.Long, "CIR", o(st.pos), st.posNote)
		e.markAs(c.Radius, "CIR", o(st.arc), st.arcNote)
		e.markAs(c.RadiusUom, "CIR", o(st.arc), st.arcNote)
		if st.datum != "" {
			e.markAs(c.Centre.Datum, "GEO-DATUM", st.datum, "")
		}
		e.mark(c.Crc, "GEN-CRC")
		e.markAs(c.Remark, "AVX-RMK", rule("AVX-RMK").Outcome, "")
		for _, ch := range c.Node.Children {
			if strings.HasSuffix(ch.Local(), "UidCen") {
				e.markTree(ch, "AVX-CENREF", convert.Unmapped)
			}
		}
	}
	e.markAs(v.Remark, "AVX-RMK", rule("AVX-RMK").Outcome, "")
	e.mark(v.Crc, "GEN-CRC")
	return nil
}

// boundaryRemarks collects the remarks of a boundary (Abd, Avx and Circle
// txtRmk) for the volume's notes; called in pass 1.
func boundaryRemarks(v aixm45.Abd) []string {
	var out []string
	if is(v.Remark) && v.Remark.Text != "" {
		out = append(out, "Boundary: "+v.Remark.Text)
	}
	for i, a := range v.Vertex {
		if is(a.Remark) && a.Remark.Text != "" {
			out = append(out, fmt.Sprintf("Boundary vertex %d: %s", i+1, a.Remark.Text))
		}
	}
	if c := v.Circle; c != nil && is(c.Remark) && c.Remark.Text != "" {
		out = append(out, "Circle: "+c.Remark.Text)
	}
	return out
}

func (e *elem) adg() error {
	d := e.r.derivAt[e.index]
	if d == nil || !d.used {
		e.ks.Dropped++
		e.fallback = "ADG-ORPHAN"
		if d != nil && d.ase != "" {
			e.fallback = "ADG-SHADOW"
		}
		e.qual[e.n] = "[" + e.fallback + "]"
		return nil
	}
	e.ks.Merged++
	v := aixm45.NewAdg(e.n)
	e.markTree(v.Ase.Node, "ADG-ASE", convert.Derived)
	if v.Uid.MID != "" {
		e.set(v.Uid.Node, "mid", "ID-MID", convert.Mapped, "")
	}
	if v.SameExtent.OK() {
		e.markTree(v.SameExtent.Node, "ADG-SAME", convert.Derived)
		if v.Base.OK() {
			e.markTree(v.Base.Node, "ADG-BASE", convert.Unmapped)
		}
		for _, p := range v.Parts {
			e.markTree(p.Ase.Node, "ADG-COMP", convert.Unmapped)
			e.markAs(p.Opr, "ADG-OPR", convert.Unmapped, "not used beside AseUidSameExtent")
		}
	} else {
		if v.Base.OK() {
			e.markTree(v.Base.Node, "ADG-BASE", convert.Derived)
		}
		for i, p := range v.Parts {
			e.markTree(p.Ase.Node, "ADG-COMP", convert.Derived)
			if !is(p.Opr) {
				continue
			}
			if !v.Base.OK() && i == 0 {
				e.markAs(p.Opr, "ADG-OPR", convert.Lossy, "first component taken as the base")
				continue
			}
			if _, ok := aggregation(p.Opr.Text); ok {
				e.mark(p.Opr, "ADG-OPR")
			} else {
				e.markAs(p.Opr, "ADG-OPR", convert.Invalid, "unknown operator: component not used")
			}
		}
	}
	for _, s := range v.Stray {
		e.markAs(s, "ADG-OPR", convert.Invalid, "operator without a component")
	}
	e.mark(v.Crc, "GEN-CRC")
	e.markAs(v.Remark, "GEN-RMK", convert.Unmapped, "remark of a derivation: not carried")
	return nil
}

// ---- Gbr ----

func (e *elem) gbr() error {
	v := aixm45.NewGbr(e.n)
	bd := e.r.borderAt[e.index]
	f := newFeature("GeoBorder", e.uuid)
	p := f.props
	e.str(p, "name", v.Name, "GBR-NAME", ttName)
	e.code(p, "type", v.CodeType, "GBR-TYPE", "border_type")
	datum, missing := "", false
	for i, a := range v.Vertex {
		st := bd.stat[i]
		switch d := a.Position.Datum; {
		case !is(d):
			missing = true
		case d.Text == "WGE":
			st.datum = convert.Mapped
		default:
			st.datum = convert.Lossy
			if datum == "" {
				datum = d.Text
			}
		}
		e.markVertex(a.Node, st, "GBV-POS")
	}
	if missing {
		e.issue("datum_missing", convert.Warning, e.key, "border vertex without codeDatum: coordinates taken as WGS-84")
	}
	if datum != "" {
		code := "datum_not_wgs84"
		if datum == "U" {
			code = "datum_unknown"
		}
		e.issue(code, convert.Error, e.key, "border coordinates on datum "+datum+", carried untransformed")
		p.note("", "WARNING", fmt.Sprintf("Horizontal datum of the source coordinates: %s (AIXM 4.5 codeDatum); coordinates not transformed to WGS-84.", datum))
	}
	if len(bd.pts) >= 2 {
		texts := make([]string, len(bd.pts))
		for i, q := range bd.pts {
			texts[i] = q.text
		}
		p.add("border", wrap("border", `<aixm:Curve gml:id="`+f.id("crv")+`" srsName="`+srs4326+`"><gml:segments><gml:GeodesicString><gml:posList>`+
			strings.Join(texts, " ")+`</gml:posList></gml:GeodesicString></gml:segments></aixm:Curve>`))
	} else {
		e.issue("geometry_invalid", convert.Error, e.key, "border with fewer than two usable vertices: no curve")
	}
	e.common(p, v.Common)
	return e.emit(f, orderGeoBorder)
}

// qualified walks the leaves of n like convert.WalkLeaves, inserting the
// qualifier of an element (e.g. "[codeType=RHL]") after its name in the
// paths below it.
func qualified(n *xmlnode.Node, qual map[*xmlnode.Node]string, fn func(convert.Leaf)) {
	var walk func(n *xmlnode.Node, path string)
	walk = func(n *xmlnode.Node, path string) {
		path += qual[n]
		for _, a := range n.Attrs {
			fn(convert.Leaf{Path: path + "/@" + a.Name.Local, Value: a.Value, Node: n, Attr: a.Name.Local})
		}
		if len(n.Children) == 0 {
			fn(convert.Leaf{Path: path, Value: n.Text, Node: n})
			return
		}
		for _, c := range n.Children {
			walk(c, path+"/"+c.Local())
		}
	}
	walk(n, n.Local())
}
