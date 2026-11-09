// Command aixmgen generates the Go model of package aixm from the AIXM 5.1.1
// XML schemas.
//
// Usage (normally via go generate in package aixm):
//
//	go run ../tools/aixmgen -xsd ../tools/xsd/aixm-5.1.1 -out .
//
// It reads AIXM_Features.xsd and AIXM_DataTypes.xsd and writes:
//
//	datatypes_gen.go  one alias of Value per AIXM data type (CodeAirspace, ValDistance, …)
//	features_gen.go   features, their timeslices, Kind/TimeSlices methods and the feature index
//	objects_gen.go    objects (AirspaceLayerClass, Timesheet, …) and choices of abstract objects
//
// Naming: the XSD "…Type" suffix is dropped and element names become Go
// identifiers (pointChoice_fixDesignatedPoint → PointChoiceFixDesignatedPoint).
//
// Properties are mapped as follows:
//
//	data type                    → *CodeX / *ValX / … (alias of Value)
//	reference to a feature       → *Reference (xlink only, as AIXM requires)
//	concrete object              → *Object / []Object, flattened through the property element
//	abstract object              → *Choice with one field per substituting object
//	geometry (Point, Curve, …)   → hand-written *PointProperty, *CurveProperty, …
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/davidkohl/goaixm/xmlnode"
)

const nsAIXM = "http://www.aixm.aero/schema/5.1.1"

// geometry elements and their property types are hand-written in package
// aixm (geometry.go) because they extend GML types.
var geometryElements = map[string]bool{
	"Point": true, "ElevatedPoint": true, "Curve": true, "ElevatedCurve": true,
	"Surface": true, "ElevatedSurface": true,
}

type elemInfo struct {
	name     string
	typ      string // local name of the type
	abstract bool
	subst    string // local name of the substitution group head
}

type schema struct {
	types     map[string]*xmlnode.Node // complexTypes of AIXM_Features.xsd
	elems     map[string]elemInfo
	groups    map[string]*xmlnode.Node
	dataTypes map[string]*xmlnode.Node // simple/complex types of AIXM_DataTypes.xsd
}

func main() {
	xsdDir := flag.String("xsd", "../tools/xsd/aixm-5.1.1", "directory containing the AIXM 5.1.1 schemas")
	outDir := flag.String("out", ".", "output directory")
	pkg := flag.String("pkg", "aixm", "package name")
	flag.Parse()

	s, err := load(*xsdDir)
	if err != nil {
		fatal(err)
	}
	g := newGenerator(s, *pkg)
	if err := g.plan(); err != nil {
		fatal(err)
	}
	for name, src := range map[string]func() []byte{
		"datatypes_gen.go": g.dataTypesFile,
		"features_gen.go":  g.featuresFile,
		"objects_gen.go":   g.objectsFile,
	} {
		raw := src()
		out, err := format.Source(raw)
		if err != nil {
			_ = os.WriteFile(filepath.Join(*outDir, name+".broken"), raw, 0o644)
			fatal(fmt.Errorf("%s: %w (unformatted source written to %s.broken)", name, err, name))
		}
		if err := os.WriteFile(filepath.Join(*outDir, name), out, 0o644); err != nil {
			fatal(err)
		}
	}
	fmt.Printf("aixmgen: %d features, %d objects, %d choices, %d data types\n",
		len(g.features), len(g.objects), len(g.choices), len(g.dataTypeNames))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "aixmgen:", err)
	os.Exit(1)
}

// ---- schema loading ----------------------------------------------------------

func load(dir string) (*schema, error) {
	s := &schema{
		types: map[string]*xmlnode.Node{}, elems: map[string]elemInfo{},
		groups: map[string]*xmlnode.Node{}, dataTypes: map[string]*xmlnode.Node{},
	}
	parse := func(file string) (*xmlnode.Node, error) {
		f, err := os.Open(filepath.Join(dir, file))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return xmlnode.ParseReader(f)
	}
	feat, err := parse("AIXM_Features.xsd")
	if err != nil {
		return nil, err
	}
	for _, c := range feat.Children {
		name, _ := c.Attr("name")
		switch c.Local() {
		case "complexType":
			s.types[name] = c
		case "group":
			s.groups[name] = c
		case "element":
			typ, _ := c.Attr("type")
			abs, _ := c.Attr("abstract")
			sg, _ := c.Attr("substitutionGroup")
			s.elems[name] = elemInfo{name: name, typ: local(typ), abstract: abs == "true", subst: local(sg)}
		}
	}
	dt, err := parse("AIXM_DataTypes.xsd")
	if err != nil {
		return nil, err
	}
	for _, c := range dt.Children {
		if c.Local() == "simpleType" || c.Local() == "complexType" {
			name, _ := c.Attr("name")
			s.dataTypes[name] = c
		}
	}
	return s, nil
}

func local(q string) string {
	if i := strings.IndexByte(q, ':'); i >= 0 {
		return q[i+1:]
	}
	return q
}

// base returns the local name of the type a complexType extends or restricts.
func (s *schema) base(typ string) string {
	ct := s.types[typ]
	if ct == nil {
		return ""
	}
	var b string
	ct.Walk(func(n *xmlnode.Node) bool {
		if b != "" {
			return false
		}
		if n.Local() == "extension" || n.Local() == "restriction" {
			v, _ := n.Attr("base")
			b = local(v)
			return false
		}
		return n.Local() != "element" // do not descend into local element types
	})
	return b
}

func (s *schema) derives(typ, root string) bool {
	for i := 0; typ != "" && i < 20; i++ {
		if typ == root {
			return true
		}
		typ = s.base(typ)
	}
	// Bases defined in AIXM_AbstractGML_ObjectTypes.xsd are not loaded; the
	// direct base name is enough to classify.
	return typ == root
}

// ---- planning ----------------------------------------------------------------

type field struct {
	goName  string
	xmlName string
	goType  string
	tag     string
	slice   bool
	doc     string
}

type structDef struct {
	name   string // Go type name
	elem   string // XML element name
	doc    string
	fields []field
	ext    bool // has extension slot
}

type choiceDef struct {
	name     string
	abstract string
	members  []string // concrete element names
}

type generator struct {
	s             *schema
	pkg           string
	features      []structDef // feature structs
	timeSlices    map[string]structDef
	objects       []structDef
	choices       map[string]*choiceDef
	dataTypeNames []string
	names         map[string]string // Go name → what defined it (collision check)
	missing       map[string]bool
}

func newGenerator(s *schema, pkg string) *generator {
	return &generator{s: s, pkg: pkg, timeSlices: map[string]structDef{}, choices: map[string]*choiceDef{},
		names: map[string]string{}, missing: map[string]bool{}}
}

func (g *generator) claim(goName, by string) error {
	if prev, ok := g.names[goName]; ok {
		return fmt.Errorf("name collision: %s from %s and %s", goName, prev, by)
	}
	g.names[goName] = by
	return nil
}

func (g *generator) plan() error {
	// Hand-written names.
	for _, n := range []string{"Value", "Reference", "Extension", "XHTML", "Interpretation",
		"Feature", "TimeSlice", "FeatureCommon", "TimeSliceCommon", "ObjectCommon",
		"PointProperty", "ElevatedPointProperty", "CurveProperty", "ElevatedCurveProperty",
		"SurfaceProperty", "ElevatedSurfaceProperty"} {
		_ = g.claim(n, "hand-written")
	}
	for n := range geometryElements {
		_ = g.claim(n, "hand-written")
	}

	// Data types.
	for name := range g.s.dataTypes {
		if name == "XHTMLType" {
			continue
		}
		goName := strings.TrimSuffix(name, "Type")
		if err := g.claim(goName, "data type "+name); err != nil {
			return err
		}
		g.dataTypeNames = append(g.dataTypeNames, name)
	}
	sort.Strings(g.dataTypeNames)

	var elemNames []string
	for n := range g.s.elems {
		elemNames = append(elemNames, n)
	}
	sort.Strings(elemNames)

	for _, n := range elemNames {
		e := g.s.elems[n]
		if e.abstract || geometryElements[n] || e.typ == "" {
			continue
		}
		switch {
		case g.s.derives(e.typ, "AbstractAIXMFeatureType"):
			if err := g.planFeature(e); err != nil {
				return err
			}
		case g.s.derives(e.typ, "AbstractExtensionType"),
			g.s.derives(e.typ, "AbstractAIXMTimeSliceType"):
			// extensions are kept raw; timeslices are planned with their feature
		case g.s.derives(e.typ, "AbstractAIXMObjectType"):
			sd, err := g.planStruct(e.typ, n)
			if err != nil {
				return err
			}
			sd.doc = fmt.Sprintf("%s is the AIXM %s object.", n, n)
			if err := g.claim(n, "object"); err != nil {
				return err
			}
			g.objects = append(g.objects, sd)
		}
	}
	if len(g.missing) > 0 {
		var m []string
		for k := range g.missing {
			m = append(m, k)
		}
		sort.Strings(m)
		return fmt.Errorf("unmapped property types: %s", strings.Join(m, ", "))
	}
	for _, c := range g.choices {
		if err := g.claim(c.name, "choice of "+c.abstract); err != nil {
			return err
		}
	}
	return nil
}

func (g *generator) planFeature(e elemInfo) error {
	// The timeSlice element's property type wraps the XxxTimeSlice element.
	var tsElem string
	g.s.types[e.typ].Walk(func(n *xmlnode.Node) bool {
		if name, _ := n.Attr("name"); n.Local() == "element" && name == "timeSlice" {
			pt, _ := n.Attr("type")
			g.s.types[local(pt)].Walk(func(m *xmlnode.Node) bool {
				if ref, ok := m.Attr("ref"); ok && m.Local() == "element" {
					tsElem = local(ref)
				}
				return true
			})
			return false
		}
		return true
	})
	if tsElem == "" {
		return fmt.Errorf("feature %s: no timeslice", e.name)
	}
	ts, err := g.planStruct(g.s.elems[tsElem].typ, tsElem)
	if err != nil {
		return err
	}
	ts.doc = fmt.Sprintf("%s holds the properties of %s valid during one time slice.", tsElem, e.name)
	if err := g.claim(e.name, "feature"); err != nil {
		return err
	}
	if err := g.claim(tsElem, "timeslice"); err != nil {
		return err
	}
	g.timeSlices[e.name] = ts
	g.features = append(g.features, structDef{name: e.name, elem: e.name,
		doc: fmt.Sprintf("%s is the AIXM %s feature.", e.name, e.name)})
	return nil
}

// planStruct collects the fields of a complexType from its explicit
// sequences and group references.  AIXM repeats inherited property groups
// in every concrete type, so bases need not be followed.
func (g *generator) planStruct(typ, elem string) (structDef, error) {
	sd := structDef{name: elem, elem: elem}
	ct := g.s.types[typ]
	if ct == nil {
		return sd, fmt.Errorf("type %s of %s not found", typ, elem)
	}
	seen := map[string]bool{}
	var walk func(n *xmlnode.Node) error
	walk = func(n *xmlnode.Node) error {
		for _, c := range n.Children {
			switch c.Local() {
			case "element":
				name, ok := c.Attr("name")
				if !ok {
					continue // element refs only occur in abstract bases
				}
				if name == "extension" {
					sd.ext = true
					continue
				}
				if seen[name] {
					continue
				}
				seen[name] = true
				f, err := g.planField(c, name)
				if err != nil {
					return fmt.Errorf("%s.%s: %w", elem, name, err)
				}
				if f.goName != "" {
					sd.fields = append(sd.fields, f)
				}
			case "group":
				ref, _ := c.Attr("ref")
				grp := g.s.groups[local(ref)]
				if grp == nil {
					return fmt.Errorf("%s: group %s not found", elem, ref)
				}
				if err := walk(grp); err != nil {
					return err
				}
			case "sequence", "choice", "complexContent", "extension", "restriction":
				if err := walk(c); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return sd, walk(ct)
}

func (g *generator) planField(el *xmlnode.Node, name string) (field, error) {
	typQ, _ := el.Attr("type")
	typ := local(typQ)
	maxOcc, _ := el.Attr("maxOccurs")
	f := field{goName: goIdent(name), xmlName: name, slice: maxOcc == "unbounded" || (maxOcc != "" && maxOcc != "1")}
	plain := nsAIXM + " " + name

	switch {
	case typ == "XHTMLType":
		f.goType, f.tag = "XHTML", plain
	case g.s.dataTypes[typ] != nil:
		f.goType, f.tag = strings.TrimSuffix(typ, "Type"), plain
	case strings.HasSuffix(typ, "PropertyType"):
		target := strings.TrimSuffix(typ, "PropertyType")
		if geometryElements[target] {
			f.goType, f.tag = target+"Property", plain
			break
		}
		pt := g.s.types[typ]
		if pt == nil {
			g.missing[typ] = true
			return f, nil
		}
		ref := ""
		pt.Walk(func(n *xmlnode.Node) bool {
			if r, ok := n.Attr("ref"); ok && n.Local() == "element" {
				ref = local(r)
				return false
			}
			return true
		})
		switch {
		case ref == "":
			// xlink-only association: a reference to a feature.
			f.goType, f.tag = "Reference", plain
			f.doc = "reference to " + target
		case g.s.elems[ref].abstract:
			c := g.choice(ref)
			f.goType, f.tag = c.name, plain
		default:
			f.goType, f.tag = ref, plain+">"+ref
		}
	default:
		g.missing[typ] = true
	}
	return f, nil
}

// choice registers (once) the choice type standing for an abstract object.
func (g *generator) choice(abstract string) *choiceDef {
	if c, ok := g.choices[abstract]; ok {
		return c
	}
	name := strings.TrimPrefix(abstract, "Abstract")
	if _, taken := g.s.elems[name]; taken {
		name += "Choice"
	}
	c := &choiceDef{name: name, abstract: abstract}
	var collect func(head string)
	collect = func(head string) {
		var subs []string
		for n, e := range g.s.elems {
			if e.subst == head {
				subs = append(subs, n)
			}
		}
		sort.Strings(subs)
		for _, n := range subs {
			if g.s.elems[n].abstract {
				collect(n)
			} else {
				c.members = append(c.members, n)
			}
		}
	}
	collect(abstract)
	g.choices[abstract] = c
	return c
}

// goIdent converts an XML name to an exported Go identifier.
func goIdent(name string) string {
	var b strings.Builder
	parts := strings.FieldsFunc(name, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	for _, part := range parts {
		if part == "" {
			continue
		}
		r := []rune(part)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	return b.String()
}

// ---- output ------------------------------------------------------------------

func (g *generator) header(b *bytes.Buffer, imports ...string) {
	fmt.Fprintf(b, "// Code generated by aixmgen from the AIXM 5.1.1 schemas. DO NOT EDIT.\n\npackage %s\n\n", g.pkg)
	if len(imports) > 0 {
		b.WriteString("import (\n")
		for _, i := range imports {
			fmt.Fprintf(b, "\t%q\n", i)
		}
		b.WriteString(")\n\n")
	}
}

func (g *generator) dataTypesFile() []byte {
	var b bytes.Buffer
	g.header(&b)
	for _, name := range g.dataTypeNames {
		goName := strings.TrimSuffix(name, "Type")
		doc := fmt.Sprintf("// %s is the AIXM data type %s.", goName, name)
		if vals := enumValues(g.s.dataTypes[name]); len(vals) > 0 {
			doc += "\n// Values: " + wrapList(vals)
		}
		fmt.Fprintf(&b, "%s\ntype %s = Value\n\n", doc, goName)
	}
	return b.Bytes()
}

func enumValues(n *xmlnode.Node) []string {
	var out []string
	n.Walk(func(m *xmlnode.Node) bool {
		if m.Local() == "enumeration" {
			if v, ok := m.Attr("value"); ok && !strings.HasPrefix(v, "OTHER") {
				out = append(out, v)
			}
		}
		return true
	})
	return out
}

func wrapList(vals []string) string {
	const max = 40
	if len(vals) > max {
		vals = append(vals[:max:max], "…")
	}
	var b strings.Builder
	line := 0
	for i, v := range vals {
		if i > 0 {
			b.WriteString(", ")
			line += 2
		}
		if line > 66 {
			b.WriteString("\n//   ")
			line = 0
		}
		b.WriteString(v)
		line += len(v)
	}
	return b.String() + "; OTHER:… allowed where the schema permits."
}

func (g *generator) writeStruct(b *bytes.Buffer, sd structDef, embed string, xmlName bool) {
	fmt.Fprintf(b, "// %s\ntype %s struct {\n", sd.doc, sd.name)
	if xmlName {
		fmt.Fprintf(b, "\tXMLName xml.Name `xml:\"%s %s\"`\n", nsAIXM, sd.elem)
	}
	if embed != "" {
		fmt.Fprintf(b, "\t%s\n", embed)
	}
	for _, f := range sd.fields {
		t := "*" + f.goType
		if f.slice {
			t = "[]" + f.goType
		}
		comment := ""
		if f.doc != "" {
			comment = " // " + f.doc
		}
		fmt.Fprintf(b, "\t%s %s `xml:\"%s\"`%s\n", f.goName, t, f.tag, comment)
	}
	if sd.ext {
		fmt.Fprintf(b, "\tExtension []Extension `xml:\"%s extension\"`\n", nsAIXM)
	}
	b.WriteString("}\n\n")
}

func (g *generator) featuresFile() []byte {
	var b bytes.Buffer
	g.header(&b, "encoding/xml")
	for _, f := range g.features {
		ts := g.timeSlices[f.name]
		fmt.Fprintf(&b, "// %s\ntype %s struct {\n\tXMLName xml.Name `xml:\"%s %s\"`\n\tFeatureCommon\n",
			f.doc, f.name, nsAIXM, f.elem)
		fmt.Fprintf(&b, "\tTimeSlice []%s `xml:\"%s timeSlice>%s\"`\n}\n\n", ts.name, nsAIXM, ts.elem)
		fmt.Fprintf(&b, "// Kind returns %q.\nfunc (*%s) Kind() string { return %q }\n\n", f.name, f.name, f.name)
		fmt.Fprintf(&b, "// TimeSlices returns the feature's time slices.\nfunc (f *%s) TimeSlices() []TimeSlice {\n"+
			"\tout := make([]TimeSlice, len(f.TimeSlice))\n\tfor i := range f.TimeSlice {\n\t\tout[i] = &f.TimeSlice[i]\n\t}\n\treturn out\n}\n\n",
			f.name)
		g.writeStruct(&b, ts, "TimeSliceCommon", false)
	}
	b.WriteString("// featureFactories creates an empty feature for each XML local name.\nvar featureFactories = map[string]func() Feature{\n")
	for _, f := range g.features {
		fmt.Fprintf(&b, "\t%q: func() Feature { return new(%s) },\n", f.elem, f.name)
	}
	b.WriteString("}\n")
	return b.Bytes()
}

func (g *generator) objectsFile() []byte {
	var b bytes.Buffer
	g.header(&b)
	for _, o := range g.objects {
		g.writeStruct(&b, o, "ObjectCommon", false)
	}
	var abstracts []string
	for a := range g.choices {
		abstracts = append(abstracts, a)
	}
	sort.Strings(abstracts)
	for _, a := range abstracts {
		c := g.choices[a]
		fmt.Fprintf(&b, "// %s holds one of the objects substituting for the abstract %s.\n// At most one field is set.\ntype %s struct {\n", c.name, a, c.name)
		for _, m := range c.members {
			fmt.Fprintf(&b, "\t%s *%s `xml:\"%s %s\"`\n", m, m, nsAIXM, m)
		}
		b.WriteString("\tNilReason string `xml:\"nilReason,attr,omitempty\"`\n}\n\n")
		fmt.Fprintf(&b, "// Object returns the object that is set, or nil.\nfunc (c *%s) Object() any {\n\tswitch {\n\tcase c == nil:\n\t\treturn nil\n", c.name)
		for _, m := range c.members {
			fmt.Fprintf(&b, "\tcase c.%s != nil:\n\t\treturn c.%s\n", m, m)
		}
		b.WriteString("\t}\n\treturn nil\n}\n\n")
	}
	return b.Bytes()
}
