package geo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/xmlnode"
)

// Source geometry: the geometry of one time slice of a feature exactly as
// the AIXM source states it, without any processing.
//
// The conversion functions of this package turn curves into coordinates:
// arcs and geodesics become polylines, referenced borders and contributor
// airspaces are resolved and merged, decimal text becomes float64.  A
// SourceGeometry keeps what they consume instead, losslessly: the segment
// types in document order, interpolation attributes, arc centres, radii
// and bearings with their units, every coordinate as its decimal text
// (lat lon order as written, never parsed), srsName and srsDimension,
// gml:ids, the curve members held by reference (GeoBorders), and the
// aggregation of an airspace (operation, sequence, width, contributor
// airspace) as written.  Nothing is resolved: references stay hrefs, and
// SourceDeps lists what they resolve to.
//
// It is built from the lossless element tree (xmlnode, Member.Node), not
// from the decoded structs, which hold radii and angles as float64.
//
//	root, _ := member.Node()
//	src, err := geo.SourceOf(root, 0)
//	canon, _ := src.Canonical() // stable bytes
//	hash := src.Hash()           // sha256 hex of canon
//
// Canonical gives a stable encoding (JSON, object keys sorted, no HTML
// escaping, every value a string except "v" and "slice"), so a stored copy
// — for example in a PostgreSQL jsonb column, which reorders keys — can be
// checked with CanonicalJSON: decoding and encoding it again gives the same
// bytes and the same Hash.

// SourceVersion is the version of the SourceGeometry representation; it
// changes whenever the same source gives a different SourceGeometry.
const SourceVersion = 1

// SourceGeometry is the source geometry of one time slice of a feature.
type SourceGeometry struct {
	// V is SourceVersion.
	V int `json:"v"`
	// Kind is the feature's element name (e.g. "Airspace").
	Kind string `json:"kind"`
	// Slice is the index of the time slice among the feature's
	// aixm:timeSlice elements.
	Slice int `json:"slice"`
	// Geoms are the GML geometries of the slice in document order, except
	// those of an airspace's geometryComponent (Components).
	Geoms []SourceProperty `json:"geoms,omitempty"`
	// Components are an airspace's geometryComponent elements in
	// document order (not sorted by operationSequence).
	Components []SourceComponent `json:"components,omitempty"`
}

// SourceProperty is one geometry in a time slice: the path of local
// element names from the time slice to the geometry's property element
// (e.g. "location" or "part/VerticalStructurePart/horizontalProjection_location")
// and the geometry.
type SourceProperty struct {
	Path string       `json:"path"`
	Geom *SourceShape `json:"geom"`
}

// SourceComponent is one aixm:AirspaceGeometryComponent.
type SourceComponent struct {
	Op     string        `json:"op,omitempty"`  // operation as written
	Seq    string        `json:"seq,omitempty"` // operationSequence as written
	Volume *SourceVolume `json:"volume,omitempty"`
	// VolumeHref: theAirspaceVolume given by reference.
	VolumeHref string `json:"volumeHref,omitempty"`
}

// SourceVolume is the horizontal geometry of an aixm:AirspaceVolume.
type SourceVolume struct {
	HorizontalProjection *SourceShape       `json:"horizontalProjection,omitempty"`
	Centreline           *SourceShape       `json:"centreline,omitempty"`
	Width                *SourceMeasure     `json:"width,omitempty"`
	Contributor          *SourceContributor `json:"contributor,omitempty"`
}

// SourceContributor is an aixm:AirspaceVolumeDependency.
type SourceContributor struct {
	Dependency string `json:"dependency,omitempty"`
	Href       string `json:"href,omitempty"` // theAirspace
}

// SourceShape is one GML geometry object: Point, ElevatedPoint, Curve,
// ElevatedCurve, Surface or ElevatedSurface (T is the element name).
type SourceShape struct {
	T   string `json:"t"`
	ID  string `json:"id,omitempty"`  // gml:id
	Srs string `json:"srs,omitempty"` // srsName
	Dim string `json:"dim,omitempty"` // srsDimension
	// Pos: a point's position.
	Pos *SourcePos `json:"pos,omitempty"`
	// Segments: a curve's segments in document order.
	Segments []SourceSegment `json:"segments,omitempty"`
	// Patches: a surface's polygon patches.
	Patches []SourcePatch `json:"patches,omitempty"`
	// Href: the geometry is given by reference only.
	Href string `json:"href,omitempty"`
}

// SourcePatch is a gml:PolygonPatch.
type SourcePatch struct {
	Exterior *SourceRing  `json:"exterior,omitempty"`
	Interior []SourceRing `json:"interior,omitempty"`
}

// SourceRing is a gml:LinearRing (Pos) or a gml:Ring (Members).
type SourceRing struct {
	T       string         `json:"t"`
	Pos     *SourcePos     `json:"pos,omitempty"`
	Members []SourceMember `json:"members,omitempty"`
}

// SourceMember is a gml:curveMember: an inline curve, or a reference
// (typically to a GeoBorder).
type SourceMember struct {
	Curve *SourceShape `json:"curve,omitempty"`
	Href  string       `json:"href,omitempty"`
}

// SourceSegment is one curve segment.  T is the element name:
// LineStringSegment, GeodesicString, Arc, Circle, ArcByCenterPoint,
// CircleByCenterPoint (others are recorded with their positions).
type SourceSegment struct {
	T string `json:"t"`
	// Interp is the interpolation attribute as written ("" if absent).
	Interp string `json:"interp,omitempty"`
	NumArc string `json:"numArc,omitempty"`
	// Pos: the positions (LineStringSegment, GeodesicString, Arc,
	// Circle), or the centre of ArcByCenterPoint / CircleByCenterPoint
	// given as pos or posList.
	Pos *SourcePos `json:"pos,omitempty"`
	// Centre: a by-centre segment's centre given as pointProperty or
	// pointRep (CentreVia names which): an inline point, or CentreHref.
	CentreVia  string         `json:"centreVia,omitempty"`
	Centre     *SourceShape   `json:"centre,omitempty"`
	CentreHref string         `json:"centreHref,omitempty"`
	Radius     *SourceMeasure `json:"radius,omitempty"`
	StartAngle *SourceMeasure `json:"startAngle,omitempty"`
	EndAngle   *SourceMeasure `json:"endAngle,omitempty"`
}

// SourceMeasure is a value with its unit, both as written.
type SourceMeasure struct {
	V   string `json:"v"`
	Uom string `json:"uom,omitempty"`
	Nil string `json:"nilReason,omitempty"`
}

// SourcePos is a sequence of positions: one gml:posList (List) or gml:pos
// elements.  C holds the coordinate tuples, each value the decimal text
// as written (GML order: latitude, longitude for EPSG:4326); a posList is
// split into tuples of srsDimension values (default 2).
type SourcePos struct {
	List  bool       `json:"list,omitempty"`
	Srs   string     `json:"srs,omitempty"`   // srsName of the posList
	Dim   string     `json:"dim,omitempty"`   // srsDimension of the posList
	Count string     `json:"count,omitempty"` // count of the posList
	C     [][]string `json:"c"`
	// Attrs: srsName / srsDimension of individual gml:pos elements.
	Attrs []SourcePosAttrs `json:"attrs,omitempty"`
}

// SourcePosAttrs are the attributes of the I-th gml:pos element.
type SourcePosAttrs struct {
	I   int    `json:"i"`
	Srs string `json:"srs,omitempty"`
	Dim string `json:"dim,omitempty"`
}

// geometryKinds are the GML geometry elements a time slice property holds.
var geometryKinds = map[string]bool{
	"Point": true, "ElevatedPoint": true, "Curve": true, "ElevatedCurve": true,
	"Surface": true, "ElevatedSurface": true,
}

// timeSlices returns the time slice elements of a feature element (the
// child of each aixm:timeSlice; nil for an empty wrapper), indexed as
// SourceGeometry.Slice.
func timeSlices(feature *xmlnode.Node) []*xmlnode.Node {
	var out []*xmlnode.Node
	for _, w := range feature.ChildrenNamed("timeSlice") {
		if len(w.Children) == 0 {
			out = append(out, nil)
			continue
		}
		out = append(out, w.Children[0])
	}
	return out
}

// SourceOf returns the source geometry of time slice slice (the index
// among the feature's aixm:timeSlice elements) of a feature element.  A
// slice without geometry gives a SourceGeometry with no Geoms and no
// Components.
func SourceOf(feature *xmlnode.Node, slice int) (*SourceGeometry, error) {
	if feature == nil {
		return nil, fmt.Errorf("geo: nil feature")
	}
	tss := timeSlices(feature)
	if slice < 0 || slice >= len(tss) || tss[slice] == nil {
		return nil, fmt.Errorf("geo: %s has no time slice %d", feature.Local(), slice)
	}
	src := &SourceGeometry{V: SourceVersion, Kind: feature.Local(), Slice: slice}
	ts := tss[slice]
	airspace := src.Kind == "Airspace"
	var walk func(n *xmlnode.Node, path []string) error
	walk = func(n *xmlnode.Node, path []string) error {
		for _, c := range n.Children {
			if airspace && len(path) == 0 && c.Local() == "geometryComponent" {
				comp, err := sourceComponent(c)
				if err != nil {
					return err
				}
				src.Components = append(src.Components, comp)
				continue
			}
			if geometryKinds[c.Local()] && len(path) > 0 {
				g, err := sourceShape(c)
				if err != nil {
					return fmt.Errorf("geo: %s: %w", strings.Join(path, "/"), err)
				}
				src.Geoms = append(src.Geoms, SourceProperty{Path: strings.Join(path, "/"), Geom: g})
				continue
			}
			if err := walk(c, append(path[:len(path):len(path)], c.Local())); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(ts, nil); err != nil {
		return nil, err
	}
	return src, nil
}

func sourceComponent(gc *xmlnode.Node) (SourceComponent, error) {
	var comp SourceComponent
	agc := gc.Child("AirspaceGeometryComponent")
	if agc == nil {
		return comp, nil
	}
	comp.Op, comp.Seq = agc.TextAt("operation"), agc.TextAt("operationSequence")
	tv := agc.Child("theAirspaceVolume")
	if tv == nil {
		return comp, nil
	}
	comp.VolumeHref = tv.Href()
	vol := tv.Child("AirspaceVolume")
	if vol == nil {
		return comp, nil
	}
	v := &SourceVolume{}
	var err error
	if p := vol.Child("horizontalProjection"); p != nil {
		if v.HorizontalProjection, err = firstShape(p); err != nil {
			return comp, fmt.Errorf("geo: horizontalProjection: %w", err)
		}
	}
	if p := vol.Child("centreline"); p != nil {
		if v.Centreline, err = firstShape(p); err != nil {
			return comp, fmt.Errorf("geo: centreline: %w", err)
		}
	}
	if w := vol.Child("width"); w != nil {
		v.Width = measure(w)
	}
	if d := vol.Find("contributorAirspace/AirspaceVolumeDependency"); d != nil {
		v.Contributor = &SourceContributor{Dependency: d.TextAt("dependency")}
		if a := d.Child("theAirspace"); a != nil {
			v.Contributor.Href = a.Href()
		}
	}
	comp.Volume = v
	return comp, nil
}

// firstShape returns the geometry inside a property element (nil when it
// holds none; a reference-only property gives a shape with Href).
func firstShape(prop *xmlnode.Node) (*SourceShape, error) {
	for _, c := range prop.Children {
		if geometryKinds[c.Local()] {
			return sourceShape(c)
		}
	}
	if h := prop.Href(); h != "" {
		return &SourceShape{T: "", Href: h}, nil
	}
	return nil, nil
}

func attr(n *xmlnode.Node, local string) string {
	v, _ := n.Attr(local)
	return v
}

func gmlID(n *xmlnode.Node) string {
	v, _ := n.AttrNS(xmlnode.NSGML, "id")
	return v
}

func sourceShape(n *xmlnode.Node) (*SourceShape, error) {
	g := &SourceShape{T: n.Local(), ID: gmlID(n), Srs: attr(n, "srsName"), Dim: attr(n, "srsDimension")}
	var err error
	switch n.Local() {
	case "Point", "ElevatedPoint":
		g.Pos, err = positions(n, g.Dim)
	case "Curve", "ElevatedCurve":
		g.Segments, err = segments(n.Child("segments"), g.Dim)
	case "Surface", "ElevatedSurface":
		g.Patches, err = patches(n.Child("patches"), g.Dim)
	}
	return g, err
}

func segments(segs *xmlnode.Node, dim string) ([]SourceSegment, error) {
	if segs == nil {
		return nil, nil
	}
	var out []SourceSegment
	for _, s := range segs.Children {
		seg := SourceSegment{T: s.Local(), Interp: attr(s, "interpolation"), NumArc: attr(s, "numArc")}
		var err error
		if seg.Pos, err = positions(s, dim); err != nil {
			return nil, fmt.Errorf("%s: %w", s.Local(), err)
		}
		for _, via := range []string{"pointProperty", "pointRep"} {
			p := s.Child(via)
			if p == nil {
				continue
			}
			seg.CentreVia, seg.CentreHref = via, p.Href()
			for _, c := range p.Children {
				if geometryKinds[c.Local()] {
					if seg.Centre, err = sourceShape(c); err != nil {
						return nil, fmt.Errorf("%s centre: %w", s.Local(), err)
					}
					break
				}
			}
			break
		}
		if r := s.Child("radius"); r != nil {
			seg.Radius = measure(r)
		}
		if a := s.Child("startAngle"); a != nil {
			seg.StartAngle = measure(a)
		}
		if a := s.Child("endAngle"); a != nil {
			seg.EndAngle = measure(a)
		}
		out = append(out, seg)
	}
	return out, nil
}

func patches(ps *xmlnode.Node, dim string) ([]SourcePatch, error) {
	if ps == nil {
		return nil, nil
	}
	var out []SourcePatch
	for _, pp := range ps.Children {
		var patch SourcePatch
		for _, r := range pp.Children {
			var ring *xmlnode.Node
			if len(r.Children) > 0 {
				ring = r.Children[0]
			}
			if ring == nil || (r.Local() != "exterior" && r.Local() != "interior") {
				continue
			}
			sr, err := sourceRing(ring, dim)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", r.Local(), err)
			}
			if r.Local() == "exterior" {
				patch.Exterior = &sr
			} else {
				patch.Interior = append(patch.Interior, sr)
			}
		}
		out = append(out, patch)
	}
	return out, nil
}

func sourceRing(ring *xmlnode.Node, dim string) (SourceRing, error) {
	sr := SourceRing{T: ring.Local()}
	if ring.Local() != "Ring" {
		p, err := positions(ring, dim)
		sr.Pos = p
		return sr, err
	}
	for _, cm := range ring.ChildrenNamed("curveMember") {
		m := SourceMember{Href: cm.Href()}
		for _, c := range cm.Children {
			if geometryKinds[c.Local()] || c.Local() == "OrientableCurve" || c.Local() == "CompositeCurve" {
				g, err := sourceShape(c)
				if err != nil {
					return sr, err
				}
				m.Curve = g
				break
			}
		}
		sr.Members = append(sr.Members, m)
	}
	return sr, nil
}

func measure(n *xmlnode.Node) *SourceMeasure {
	m := &SourceMeasure{V: n.Text, Uom: attr(n, "uom")}
	if n.IsNil() {
		m.Nil = attr(n, "nilReason")
		if m.Nil == "" {
			m.Nil = "nil"
		}
	}
	return m
}

// positions collects the gml:posList or gml:pos children of n (nil when
// there are none).  dim is the srsDimension inherited from the geometry.
func positions(n *xmlnode.Node, dim string) (*SourcePos, error) {
	if pl := n.Child("posList"); pl != nil {
		p := &SourcePos{List: true, Srs: attr(pl, "srsName"), Dim: attr(pl, "srsDimension"), Count: attr(pl, "count")}
		d := 2
		for _, s := range []string{p.Dim, dim} {
			if s != "" {
				if _, err := fmt.Sscan(s, &d); err != nil || d < 1 {
					return nil, fmt.Errorf("posList: invalid srsDimension %q", s)
				}
				break
			}
		}
		vals := strings.Fields(pl.Text)
		if len(vals)%d != 0 {
			return nil, fmt.Errorf("posList: %d values are no multiple of the dimension %d", len(vals), d)
		}
		p.C = make([][]string, 0, len(vals)/d)
		for i := 0; i < len(vals); i += d {
			p.C = append(p.C, vals[i:i+d:i+d])
		}
		return p, nil
	}
	ps := n.ChildrenNamed("pos")
	if len(ps) == 0 {
		return nil, nil
	}
	p := &SourcePos{C: make([][]string, 0, len(ps))}
	for i, e := range ps {
		p.C = append(p.C, strings.Fields(e.Text))
		if s, d := attr(e, "srsName"), attr(e, "srsDimension"); s != "" || d != "" {
			p.Attrs = append(p.Attrs, SourcePosAttrs{I: i, Srs: s, Dim: d})
		}
	}
	return p, nil
}

// Canonical returns the canonical encoding of g (see the package
// documentation of SourceGeometry).
func (g *SourceGeometry) Canonical() ([]byte, error) {
	b, err := json.Marshal(g)
	if err != nil {
		return nil, err
	}
	return CanonicalJSON(b)
}

// Hash is the sha256 (hex) of Canonical.
func (g *SourceGeometry) Hash() (string, error) {
	b, err := g.Canonical()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// CanonicalJSON re-encodes a JSON text canonically: object keys sorted
// (bytewise), no insignificant whitespace, no HTML escaping, numbers as
// written.  Applied to any encoding of the same SourceGeometry (or
// SourceDep list) it gives the same bytes.
func CanonicalJSON(b []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("geo: canonical JSON: %w", err)
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}

// SourceDep is a feature the source geometry refers to, as resolved in a
// message: a GeoBorder held by a curveMember, a contributor airspace, or
// the feature a by-centre arc takes its centre from.
type SourceDep struct {
	Role string `json:"role"` // "border", "contributor" or "centre"
	Href string `json:"href"`
	// ID is the identifier the href names (goaixm.HrefID).
	ID string `json:"id,omitempty"`
	// Kind is the resolved feature's element name, "" when the message
	// does not hold it (Slice is then -1).
	Kind string `json:"kind,omitempty"`
	// Slice is the index of the time slice used (Options.At; for a border
	// the one whose curve is used), -1 when unresolved.
	Slice int `json:"slice"`
	// SrcHash is the Hash of the dependency's SourceGeometry at Slice
	// ("" when unresolved or when its element tree is not available).
	SrcHash string `json:"srcHash,omitempty"`
	// Via is the href of the dependency through which this one is used
	// ("" for a direct dependency): a contributor's own borders and
	// contributors are listed too.
	Via string `json:"via,omitempty"`
}

// maxSourceDepDepth bounds the contributor levels SourceDeps follows.
const maxSourceDepDepth = maxContributorDepth

// SourceDeps lists the dependencies of src in msg (nil msg: none); see
// SourceResolver.Deps.  For many features of one message use a
// SourceResolver, which hashes every referenced feature once.
func (o Options) SourceDeps(src *SourceGeometry, msg *goaixm.Message) ([]SourceDep, error) {
	return NewSourceResolver(msg, o).Deps(src)
}

// SourceResolver resolves the dependencies of source geometries in one
// message, caching the source geometry of every referenced feature.  It
// is not safe for concurrent use.
type SourceResolver struct {
	msg   *goaixm.Message
	opt   Options
	cache map[string]*resolvedDep // role|id
}

type resolvedDep struct {
	kind  string
	slice int
	hash  string
	src   *SourceGeometry
	err   error
}

// NewSourceResolver returns a resolver for the features of msg (nil: no
// dependency resolves), choosing referenced slices by o.At.
func NewSourceResolver(msg *goaixm.Message, o Options) *SourceResolver {
	return &SourceResolver{msg: msg, opt: o, cache: map[string]*resolvedDep{}}
}

// Deps lists the dependencies of src in document order, each feature once
// per role: the GeoBorders of its rings, its contributor airspaces (every
// contributorAirspace, whatever its dependency code and whether the
// volume also has a geometry of its own), and the features named as arc
// centres, then, transitively (Via), those of its contributors and
// borders.  The slice of each is chosen as the conversion chooses it
// with the resolver's options (Options.At).  The members' element trees
// are needed for SrcHash (goaixm.Options.KeepRaw, or generic features).
func (r *SourceResolver) Deps(src *SourceGeometry) ([]SourceDep, error) {
	if src == nil || r == nil || r.msg == nil {
		return nil, nil
	}
	var out []SourceDep
	seen := map[string]bool{}
	var visit func(g *SourceGeometry, via string, depth int) error
	visit = func(g *SourceGeometry, via string, depth int) error {
		var refs []SourceDep
		g.eachRef(func(role, href string) { refs = append(refs, SourceDep{Role: role, Href: href}) })
		for _, d := range refs {
			d.ID, d.Via, d.Slice = goaixm.HrefID(d.Href), via, -1
			if d.ID == "" || strings.HasPrefix(strings.TrimSpace(d.Href), "#") {
				// A gml:id inside some feature (arc centres), not a
				// feature identifier: listed, not resolved.
				if key := d.Role + "|#|" + d.Href; !seen[key] {
					seen[key] = true
					out = append(out, d)
				}
				continue
			}
			key := d.Role + "|" + d.ID
			if seen[key] {
				continue
			}
			seen[key] = true
			rd := r.resolve(d.Role, d.Href, key)
			if rd.err != nil {
				return rd.err
			}
			d.Kind, d.Slice, d.SrcHash = rd.kind, rd.slice, rd.hash
			out = append(out, d)
			if rd.src != nil && (d.Role == "contributor" || d.Role == "border") {
				if depth+1 >= maxSourceDepDepth {
					return fmt.Errorf("geo: dependency nesting exceeds %d levels at %s", maxSourceDepDepth, d.Href)
				}
				if err := visit(rd.src, d.Href, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(src, "", 0); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *SourceResolver) resolve(role, href, key string) *resolvedDep {
	if rd, ok := r.cache[key]; ok {
		return rd
	}
	rd := &resolvedDep{slice: -1}
	r.cache[key] = rd
	f, ok := r.msg.Resolve(href)
	if !ok {
		return rd
	}
	mem, _ := r.msg.Member(f)
	rd.kind = mem.Kind
	root, err := mem.Node()
	if err != nil {
		return rd // no element tree: resolved, but not hashed
	}
	if rd.slice = r.opt.depSlice(role, root); rd.slice < 0 {
		return rd
	}
	if rd.src, err = SourceOf(root, rd.slice); err != nil {
		rd.err = fmt.Errorf("geo: dependency %s: %w", href, err)
		return rd
	}
	rd.hash, rd.err = rd.src.Hash()
	return rd
}

// depSlice is the time slice of a referenced feature the conversion
// uses (see Options.At); -1 when there is none.
func (o Options) depSlice(role string, root *xmlnode.Node) int {
	tss := timeSlices(root)
	hasCurve := func(i int) bool {
		return tss[i] != nil && (role != "border" || tss[i].Find("border/*/segments") != nil)
	}
	if !o.At.IsZero() {
		hs := make([]sliceHeader, len(tss))
		for i, ts := range tss {
			if ts != nil {
				hs[i] = nodeHeader(ts)
			}
		}
		if i := sliceAt(hs, o.At); i >= 0 && hasCurve(i) {
			return i
		}
	}
	if role != "border" {
		if len(tss) > 0 && tss[0] != nil {
			return 0
		}
		return -1
	}
	for i := range tss {
		if hasCurve(i) {
			return i
		}
	}
	return -1
}

// eachRef calls fn for every reference of the geometry in document order.
func (g *SourceGeometry) eachRef(fn func(role, href string)) {
	var shape func(s *SourceShape)
	shape = func(s *SourceShape) {
		if s == nil {
			return
		}
		for _, seg := range s.Segments {
			if seg.CentreHref != "" {
				fn("centre", seg.CentreHref)
			}
		}
		for _, p := range s.Patches {
			rings := p.Interior
			if p.Exterior != nil {
				rings = append([]SourceRing{*p.Exterior}, rings...)
			}
			for _, r := range rings {
				for _, m := range r.Members {
					if m.Curve != nil {
						shape(m.Curve)
					} else if m.Href != "" {
						fn("border", m.Href)
					}
				}
			}
		}
	}
	for _, p := range g.Geoms {
		shape(p.Geom)
	}
	for _, c := range g.Components {
		if c.Volume == nil {
			continue
		}
		shape(c.Volume.HorizontalProjection)
		shape(c.Volume.Centreline)
		if ct := c.Volume.Contributor; ct != nil && ct.Href != "" {
			fn("contributor", ct.Href)
		}
	}
}

// CanonicalDeps is the canonical encoding of a dependency list (as
// SourceGeometry.Canonical); nil and empty lists encode as "[]".
func CanonicalDeps(deps []SourceDep) ([]byte, error) {
	if deps == nil {
		deps = []SourceDep{}
	}
	b, err := json.Marshal(deps)
	if err != nil {
		return nil, err
	}
	return CanonicalJSON(b)
}
