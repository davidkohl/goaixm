package aixm45to51

import (
	"fmt"
	"strings"

	"github.com/davidkohl/goaixm/aixm45"
	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/version"
	"github.com/davidkohl/goaixm/xmlnode"
)

type leafID struct {
	n    *xmlnode.Node
	attr string
}

type mark struct {
	o          convert.Outcome
	rule, note string
}

// elem is one source element being converted.  Every value the converter
// uses is marked with a rule and an outcome; finish records one outcome per
// leaf, the fallback rule for leaves nobody marked.
type elem struct {
	r        *run
	kind     string
	n        *xmlnode.Node
	index    int
	ks       *convert.KindStats
	marks    map[leafID]mark
	fallback string
	// base and key are the natural key without and with the #n suffix;
	// uuid is the identity derived from key.
	base, key, uuid string
	// datumNote is the WARNING note text written for a non-WGS-84 datum.
	datumNote string
	// qual qualifies the report paths below an element, e.g.
	// "[codeType=RHL]" for an Avx.
	qual map[*xmlnode.Node]string
}

// set marks a leaf; the first mark wins.
func (e *elem) set(n *xmlnode.Node, attr, ruleID string, o convert.Outcome, note string) {
	id := leafID{n, attr}
	if _, done := e.marks[id]; done {
		return
	}
	e.marks[id] = mark{o, ruleID, note}
}

// is reports whether v is a usable leaf: present and without child
// elements.
func is(v aixm45.Value) bool { return v.Node != nil && len(v.Node.Children) == 0 }

// mark marks a value's text with the rule's nominal outcome.
func (e *elem) mark(v aixm45.Value, ruleID string) {
	if is(v) {
		e.set(v.Node, "", ruleID, rule(ruleID).Outcome, "")
	}
}

// markAs marks a value's text with outcome o.
func (e *elem) markAs(v aixm45.Value, ruleID string, o convert.Outcome, note string) {
	if is(v) {
		e.set(v.Node, "", ruleID, o, note)
	}
}

// markTree marks every leaf below n: mid attributes with REF-MID, the rest
// with ruleID and outcome o.
func (e *elem) markTree(n *xmlnode.Node, ruleID string, o convert.Outcome) {
	if n == nil {
		return
	}
	convert.WalkLeaves(n, func(l convert.Leaf) {
		if l.Attr == "mid" {
			e.set(l.Node, l.Attr, "REF-MID", convert.Unmapped, "")
			return
		}
		e.set(l.Node, l.Attr, ruleID, o, "")
	})
}

// finish records the outcome of every leaf.
func (e *elem) finish() {
	e.ks.Leaves += convert.CountLeaves(e.n)
	sk := e.key
	if sk == "" {
		sk = fmt.Sprintf("[%d]", e.index)
	}
	qualified(e.n, e.qual, func(l convert.Leaf) {
		m, ok := e.marks[leafID{l.Node, l.Attr}]
		if !ok {
			m = mark{rule(e.fallback).Outcome, e.fallback, ""}
		}
		ru := rule(m.rule)
		var s *convert.Sample
		if m.o != convert.Mapped && m.o != convert.Derived {
			s = &convert.Sample{Key: sk, Value: l.Value, Note: m.note}
		}
		e.ks.Record(l.Path, m.o, ru.ID, ru.Target, s)
	})
}

func (e *elem) issue(code string, sev convert.Severity, key, msg string) {
	e.r.rep.AddIssue(convert.Issue{Code: code, Severity: sev, Kind: e.kind, Key: key, Message: msg})
}

// str writes a text property of type tt.  A value that is not valid in tt
// is kept as a note instead (GEN-VALUE, lossy).
func (e *elem) str(p *props, prop string, v aixm45.Value, ruleID string, tt textType) bool {
	if !is(v) {
		return false
	}
	if v.Text == "" {
		e.markAs(v, ruleID, convert.Invalid, "empty")
		return false
	}
	if tt.valid(v.Text) {
		p.add(prop, el(prop, v.Text))
		e.mark(v, ruleID)
		return true
	}
	p.note(prop, "REMARK", v.Text)
	e.markAs(v, ruleID, convert.Lossy, "not a valid "+tt.name+": kept as a note (GEN-VALUE)")
	e.issue("value_invalid", convert.Warning, e.key, fmt.Sprintf("%s %q is not a valid %s: kept as a note", prop, v.Text, tt.name))
	return false
}

// note writes a free-text value as an annotation note.
func (e *elem) note(p *props, propertyName, purpose string, v aixm45.Value, ruleID string) {
	if !is(v) {
		return
	}
	if v.Text == "" {
		e.markAs(v, ruleID, convert.Invalid, "empty")
		return
	}
	text := v.Text
	o := rule(ruleID).Outcome
	if !ttNote.valid(text) {
		text = string([]rune(text)[:ttNote.max])
		o = convert.Lossy
	}
	p.note(propertyName, purpose, text)
	e.markAs(v, ruleID, o, "")
}

// code writes a coded property through a code list.  A code not in the
// list becomes OTHER with the source value in a note (GEN-CODE, lossy) when
// the 5.1.1 type accepts OTHER, and is invalid otherwise.
func (e *elem) code(p *props, prop string, v aixm45.Value, ruleID, listName string) (string, bool) {
	if !is(v) {
		return "", false
	}
	l := list(listName)
	if to, ok := l.Codes[v.Text]; ok {
		p.add(prop, el(prop, to))
		e.mark(v, ruleID)
		return to, true
	}
	if l.Other {
		p.add(prop, el(prop, "OTHER"))
		if v.Text != "" {
			p.note(prop, "REMARK", v.Text)
		}
		e.markAs(v, ruleID, convert.Lossy, "not in "+listName+": OTHER (GEN-CODE)")
		e.issue("code_unknown", convert.Warning, "", fmt.Sprintf("%s: %q not in list %s: OTHER", prop, v.Text, listName))
		return "OTHER", true
	}
	e.markAs(v, ruleID, convert.Invalid, "not in "+listName)
	e.issue("code_unknown", convert.Error, "", fmt.Sprintf("%s: %q not in list %s: dropped", prop, v.Text, listName))
	return "", false
}

// num writes a numeric property without a unit.
func (e *elem) num(p *props, prop string, v aixm45.Value, ruleID string, nt numType) bool {
	if !is(v) {
		return false
	}
	if !nt.valid(v.Text) {
		e.markAs(v, ruleID, convert.Invalid, "not a valid "+nt.name)
		e.issue("value_invalid", convert.Error, e.key, fmt.Sprintf("%s %q is not a valid %s: dropped", prop, v.Text, nt.name))
		return false
	}
	p.add(prop, el(prop, v.Text))
	e.mark(v, ruleID)
	return true
}

// measure writes a value with its unit (uom attribute).
func (e *elem) measure(p *props, prop string, v, uom aixm45.Value, ruleID string, nt numType, uomList string) bool {
	if !is(v) {
		return false
	}
	bad := func(why string) bool {
		e.markAs(v, ruleID, convert.Invalid, why)
		e.issue("value_invalid", convert.Error, e.key, fmt.Sprintf("%s %q %s: dropped", prop, v.Text, why))
		return false
	}
	if !nt.valid(v.Text) {
		return bad("is not a valid " + nt.name)
	}
	if !is(uom) || uom.Text == "" {
		return bad("has no unit")
	}
	u, ok := list(uomList).Codes[uom.Text]
	if !ok {
		e.markAs(uom, ruleID, convert.Invalid, "not in "+uomList)
		return bad(fmt.Sprintf("has unit %q, not in %s", uom.Text, uomList))
	}
	p.add(prop, el(prop, v.Text, "uom", u))
	e.mark(v, ruleID)
	e.mark(uom, ruleID)
	return true
}

// ref resolves a reference …Uid: its key leaves are derived into the UUID
// of the referenced feature, mid attributes unmapped.  A key not in the
// document gives a dangling_ref warning; the reference is still written.
func (e *elem) ref(r aixm45.Ref, ruleID string) (string, bool) {
	if !r.OK() {
		return "", false
	}
	e.markTree(r.Node, ruleID, convert.Derived)
	if e.r.keys[r.Key] == 0 {
		e.issue("dangling_ref", convert.Warning, r.Key, "reference to a "+r.Kind+" that is not in the document")
	}
	return e.r.uuid(r.Key), true
}

// common handles the own Uid's mid, txtRmk and valCrc.
func (e *elem) common(p *props, c aixm45.Common) {
	if c.Uid.OK() {
		if _, ok := c.Uid.Node.Attr("mid"); ok {
			e.set(c.Uid.Node, "mid", "ID-MID", convert.Mapped, "")
		}
	}
	e.note(p, "", "REMARK", c.Remark, "GEN-RMK")
	e.mark(c.Crc, "GEN-CRC")
}

// timesheets marks the timesheet children (phase 2) as unmapped.
func (e *elem) timesheets(names ...string) {
	for _, name := range names {
		for _, c := range e.n.ChildrenNamed(name) {
			e.markTree(c, "GEN-TT", convert.Unmapped)
		}
	}
}

// position parses a horizontal position into the properties of a point
// (gml:pos only; the caller adds accuracy and elevation and renders it).
// It returns nil when there is no usable position.
func (e *elem) position(f *feature, ps aixm45.Position, ruleID string) *props {
	if !is(ps.Lat) && !is(ps.Long) {
		return nil
	}
	lat, err1 := aixm45.ParseLat(ps.Lat.Text)
	long, err2 := aixm45.ParseLong(ps.Long.Text)
	if !is(ps.Lat) || !is(ps.Long) || err1 != nil || err2 != nil {
		e.markAs(ps.Lat, ruleID, convert.Invalid, "position not usable")
		e.markAs(ps.Long, ruleID, convert.Invalid, "position not usable")
		e.issue("coordinate_invalid", convert.Error, e.key, fmt.Sprintf("position %q %q not usable: no point written", ps.Lat.Text, ps.Long.Text))
		return nil
	}
	e.mark(ps.Lat, ruleID)
	e.mark(ps.Long, ruleID)
	switch {
	case !is(ps.Datum):
		e.issue("datum_missing", convert.Warning, e.key, "no codeDatum: coordinates taken as WGS-84")
	case ps.Datum.Text == "WGE":
		e.mark(ps.Datum, "GEO-DATUM")
	default:
		code, what := "datum_not_wgs84", "datum "+ps.Datum.Text
		if ps.Datum.Text == "U" {
			code, what = "datum_unknown", "unknown datum (U)"
		}
		e.markAs(ps.Datum, "GEO-DATUM", convert.Lossy, "coordinates not transformed")
		e.issue(code, convert.Error, e.key, "coordinates on "+what+", carried untransformed")
		if e.datumNote == "" {
			e.datumNote = fmt.Sprintf("Horizontal datum of the source coordinates: %s (AIXM 4.5 codeDatum); coordinates not transformed to WGS-84.", ps.Datum.Text)
			f.props.note("", "WARNING", e.datumNote)
		}
	}
	pp := f.newProps()
	pp.add("pos", "<gml:pos>"+lat.Decimal(lat.Decimals())+" "+long.Decimal(long.Decimals())+"</gml:pos>")
	return pp
}

// elevation adds valElev, valGeoidUndulation and valElevAccuracy to a
// point.
func (e *elem) elevation(pp *props, ev aixm45.Elevation) {
	e.measure(pp, "elevation", ev.Elev, ev.Uom, "GEO-ELEV", ntVertical, "uom_distver")
	e.measure(pp, "geoidUndulation", ev.GeoidUndulation, ev.Uom, "GEO-GEOID", ntSigned, "uom_distver")
	e.measure(pp, "verticalAccuracy", ev.Accuracy, ev.Uom, "GEO-ELEVACC", ntDistance, "uom_dist")
}

func point(pp *props, elevated bool) string {
	tag := "Point"
	if elevated {
		tag = "ElevatedPoint"
	}
	return pp.object(tag, "pt", orderPoint, "srsName", srs4326)
}

func (e *elem) provenance() convert.Provenance {
	p := convert.Provenance{SourceVersion: version.AIXM45, SourceKind: e.kind, SourceKey: e.key, SourceIndex: []int{e.index}}
	if uid := e.n.Child(e.kind + "Uid"); uid != nil {
		if mid, ok := uid.Attr("mid"); ok && mid != "" {
			p.SourceIDs = []string{mid}
		}
	}
	return p
}

// emit writes the element's primary record.
func (e *elem) emit(f *feature, order []string) error {
	e.ks.Out++
	e.ks.Default(e.kind+"/(featureLifetime)", "TS-LIFE", rule("TS-LIFE").Target)
	return e.r.put(f, order, e.provenance())
}

// equipInfo is what a Navaid takes from its primary equipment.
type equipInfo struct {
	designator, name string
	location         *props
}

// equipment writes the NavaidEquipment properties shared by VOR, DME, NDB
// and TACAN.
func (e *elem) equipment(f *feature, eq aixm45.Equipment) equipInfo {
	p := f.props
	var info equipInfo
	if e.str(p, "designator", eq.CodeID, "NAV-DESIG", ttNavDesig) {
		info.designator = eq.CodeID.Text
	}
	if e.str(p, "name", eq.Name, "NAV-NAME", ttName) {
		info.name = eq.Name.Text
	}
	e.num(p, "magneticVariation", eq.MagVar, "NAV-MAGVAR", ntMagVar)
	e.num(p, "dateMagneticVariation", eq.MagVarDate, "NAV-MAGVARDATE", ntYear)
	if pp := e.position(f, eq.Position, "NAV-POS"); pp != nil {
		e.elevation(pp, eq.Elevation)
		info.location = pp
		p.add("location", wrap("location", point(pp, true)))
	}
	if u, ok := e.ref(eq.Org, "NAV-ORG"); ok {
		ap := f.newProps()
		ap.add("theOrganisationAuthority", href("theOrganisationAuthority", u))
		p.add("authority", wrap("authority", ap.object("AuthorityForNavaidEquipment", "auth", orderAuthority)))
	}
	e.common(p, eq.Common)
	return info
}

// navaid emits the Navaid derived from a primary equipment (this element)
// and the equipment collocated with it (rule NAV-NAVAID).
func (e *elem) navaid(typ string, info equipInfo, others []member) error {
	key := "Navaid(" + e.key + ")"
	f := newFeature("Navaid", e.r.uuid(key))
	p := f.props
	p.add("type", el("type", typ))
	if info.designator != "" {
		p.add("designator", el("designator", info.designator))
	}
	if info.name != "" {
		p.add("name", el("name", info.name))
	}
	comps := []string{e.uuid}
	prov := e.provenance()
	kinds, keys := []string{e.kind}, []string{e.key}
	for _, m := range others {
		comps = append(comps, e.r.uuid(m.key))
		if !contains(kinds, m.kind) {
			kinds = append(kinds, m.kind)
		}
		keys = append(keys, m.key)
		prov.SourceIndex = append(prov.SourceIndex, m.index)
		if m.mid != "" {
			prov.SourceIDs = append(prov.SourceIDs, m.mid)
		}
	}
	for _, u := range comps {
		cp := f.newProps()
		cp.add("theNavaidEquipment", href("theNavaidEquipment", u))
		p.add("navaidEquipment", wrap("navaidEquipment", cp.object("NavaidComponent", "nc", orderComponent)))
	}
	if info.location != nil {
		lp := f.newProps()
		lp.list = append(lp.list, info.location.list...)
		p.add("location", wrap("location", point(lp, true)))
	}
	if e.datumNote != "" {
		p.note("", "WARNING", e.datumNote)
	}
	prov.SourceKind, prov.SourceKey = strings.Join(kinds, "+"), strings.Join(keys, "+")
	e.ks.Default(e.kind+"/(Navaid/featureLifetime)", "TS-LIFE", "Navaid/featureLifetime")
	return e.r.put(f, orderNavaid, prov)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
