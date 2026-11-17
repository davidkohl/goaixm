package aixm45to51

import (
	"math/big"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/davidkohl/goaixm/xmlnode"
)

// xsdTypes returns the simple types of the vendored AIXM 5.1.1 data types
// schema by name.
func xsdTypes(t *testing.T) map[string]*xmlnode.Node {
	t.Helper()
	b, err := os.ReadFile("../../tools/xsd/aixm-5.1.1/AIXM_DataTypes.xsd")
	if err != nil {
		t.Fatal(err)
	}
	root, err := xmlnode.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]*xmlnode.Node{}
	for _, c := range root.Children {
		if name, ok := c.Attr("name"); ok && c.Local() == "simpleType" {
			m[name] = c
		}
	}
	return m
}

// enumeration returns the enumeration values of a simple type and whether
// it accepts OTHER (the union pattern of the AIXM code types).
func enumeration(st *xmlnode.Node) (map[string]bool, bool) {
	vals, other := map[string]bool{}, false
	st.Walk(func(n *xmlnode.Node) bool {
		v, _ := n.Attr("value")
		switch n.Local() {
		case "enumeration":
			vals[v] = true
		case "pattern":
			other = other || strings.HasPrefix(v, "OTHER")
		}
		return true
	})
	return vals, other
}

func baseType(types map[string]*xmlnode.Node, typ string) *xmlnode.Node {
	if st := types[strings.TrimSuffix(typ, "Type")+"BaseType"]; st != nil {
		return st
	}
	return types[typ]
}

// TestCodeListsAgainstXSD checks every 5.1.1 value of the code table, and
// that a list writing OTHER has a type accepting it, against the 5.1.1
// schema.
func TestCodeListsAgainstXSD(t *testing.T) {
	types := xsdTypes(t)
	for name, l := range codeLists {
		st := baseType(types, l.Type)
		if st == nil {
			t.Errorf("list %s: type %s not in the schema", name, l.Type)
			continue
		}
		vals, other := enumeration(st)
		if l.Other && !other {
			t.Errorf("list %s: the table writes OTHER, %s does not accept it", name, l.Type)
		}
		for from, to := range l.Codes {
			if !vals[to] && !(to == "OTHER" && other) {
				t.Errorf("list %s: %s -> %s is not a %s value", name, from, to, l.Type)
			}
		}
	}
	vals, _ := enumeration(types["CodeVerticalStructureBaseType"])
	vals["OTHER"] = true
	if len(vals) != len(verticalStructureTypes) {
		t.Errorf("vertical structure types: %d in the schema, %d here", len(vals), len(verticalStructureTypes))
	}
	for v := range verticalStructureTypes {
		if !vals[v] {
			t.Errorf("vertical structure type %s not in the schema", v)
		}
	}
}

// facets follows restriction bases from a simple type and returns its
// pattern, length and value facets (the first found of each).
func facets(types map[string]*xmlnode.Node, name string) map[string]string {
	out := map[string]string{}
	for st := types[name]; st != nil; {
		r := st.Child("restriction")
		if r == nil {
			break
		}
		for _, f := range r.Children {
			if v, ok := f.Attr("value"); ok {
				if _, seen := out[f.Local()]; !seen {
					out[f.Local()] = v
				}
			}
		}
		base, _ := r.Attr("base")
		st = types[strings.TrimPrefix(base, "aixm:")]
	}
	return out
}

var probes = []string{"", "A", "Z", "a", "0", "9", "É", "é", " ", "AB CD", "A-B", "A/B", "A+B", "A--B", "-A", "A ", "ABC.", "A,B", `"`, "&", "(", "[", `\`, "^", "|", "{", "~", "`", "\t", "ABCDE", "ABCDEF", "XAAA", "XA1", "LONG NAME WITH (BRACKETS) & SIGNS: 1/2; 3?"}

// TestTextTypesAgainstXSD checks the length and pattern facets of the text
// types against the schema on a set of probe strings.
func TestTextTypesAgainstXSD(t *testing.T) {
	types := xsdTypes(t)
	for _, tt := range []textType{ttName, ttDesig, ttDpnDesig, ttNavDesig, ttAhpDesig, ttOrgDesig, ttICAO, ttIATA, ttNote} {
		f := facets(types, strings.TrimSuffix(tt.name, "Type")+"BaseType")
		var re *regexp.Regexp
		if p, ok := f["pattern"]; ok {
			re = regexp.MustCompile("^(?:" + p + ")$")
		}
		check := func(s string) bool {
			n := len([]rune(s))
			if v, ok := f["minLength"]; ok && n < atoi(v) {
				return false
			}
			if v, ok := f["maxLength"]; ok && n > atoi(v) {
				return false
			}
			if v, ok := f["length"]; ok && n != atoi(v) {
				return false
			}
			return re == nil || re.MatchString(s)
		}
		for _, s := range append(probes, strings.Repeat("A", 61), strings.Repeat("A", 17), strings.Repeat("A", 13)) {
			if tt.valid(s) != check(s) {
				t.Errorf("%s: %q valid %v here, %v in the schema (%v)", tt.name, s, tt.valid(s), check(s), f)
			}
		}
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

// TestNumTypesAgainstXSD checks the range facets of the numeric types.
func TestNumTypesAgainstXSD(t *testing.T) {
	types := xsdTypes(t)
	for _, nt := range []numType{ntDistance, ntSigned, ntFrequency, ntBearing, ntMagVar, ntTemp} {
		f := facets(types, strings.TrimSuffix(nt.name, "Type")+"BaseType")
		for _, s := range []string{"-1000", "-180.1", "-180", "-1", "-0.5", "0", "0.001", "1", "180", "180.5", "360", "360.01", "100000"} {
			v, _ := new(big.Rat).SetString(s)
			want := true
			for facet, cmp := range map[string]func(int) bool{
				"minInclusive": func(c int) bool { return c >= 0 }, "minExclusive": func(c int) bool { return c > 0 },
				"maxInclusive": func(c int) bool { return c <= 0 }, "maxExclusive": func(c int) bool { return c < 0 },
			} {
				if b, ok := f[facet]; ok {
					bound, _ := new(big.Rat).SetString(b)
					want = want && cmp(v.Cmp(bound))
				}
			}
			if nt.valid(s) != want {
				t.Errorf("%s: %s valid %v here, %v in the schema (%v)", nt.name, s, nt.valid(s), want, f)
			}
		}
	}
	for s, want := range map[string]bool{"2029": true, "0999": false, "29": false} {
		if ntYear.valid(s) != want {
			t.Errorf("year %s", s)
		}
	}
	for s, want := range map[string]bool{"50": true, "50.5": true, "1000": false, "5.55": false, "UNL": false} {
		if ntPCN.valid(s) != want {
			t.Errorf("PCN %s", s)
		}
	}
	for s, want := range map[string]bool{"UNL": true, "GND": true, "123456789": false, "1.12345": false, "-10.5": true} {
		if ntVertical.valid(s) != want {
			t.Errorf("vertical %s", s)
		}
	}
}

// TestSchemaOrder checks the property orders against the property groups
// of the 5.1.1 feature schema: the written order must be a subsequence of
// the schema sequence.
func TestSchemaOrder(t *testing.T) {
	b, err := os.ReadFile("../../tools/xsd/aixm-5.1.1/AIXM_Features.xsd")
	if err != nil {
		t.Fatal(err)
	}
	root, err := xmlnode.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string][]string{}
	for _, c := range root.Children {
		name, _ := c.Attr("name")
		if c.Local() != "group" || !strings.HasSuffix(name, "PropertyGroup") {
			continue
		}
		var elems []string
		c.Walk(func(n *xmlnode.Node) bool {
			if n.Local() == "element" {
				if en, ok := n.Attr("name"); ok {
					elems = append(elems, en)
				}
			}
			if n.Local() == "group" && n != c {
				ref, _ := n.Attr("ref")
				elems = append(elems, "group:"+strings.TrimPrefix(ref, "aixm:"))
			}
			return true
		})
		groups[strings.TrimSuffix(name, "PropertyGroup")] = elems
	}
	expand := func(g string) []string {
		var out []string
		for _, e := range groups[g] {
			if strings.HasPrefix(e, "group:") {
				out = append(out, groups[strings.TrimSuffix(strings.TrimPrefix(e, "group:"), "PropertyGroup")]...)
				continue
			}
			out = append(out, e)
		}
		return out
	}
	for g, order := range map[string][]string{
		"OrganisationAuthority": orderOrg, "Unit": orderUnit, "DesignatedPoint": orderDpn, "VOR": orderVOR, "DME": orderDME,
		"NDB": orderNDB, "TACAN": orderTACAN, "Navaid": orderNavaid, "VerticalStructure": orderVS, "VerticalStructurePart": orderVSPart,
		"AirportHeliport": orderAhp, "City": orderCity, "AirportHeliportResponsibilityOrganisation": orderAhpResp,
		"AuthorityForNavaidEquipment": orderAuthority, "NavaidComponent": orderComponent, "Runway": orderRunway,
		"SurfaceCharacteristics": orderSurface, "RunwayDirection": orderRdn,
	} {
		schema := expand(g)
		if strings.Join(schema, ",") != strings.Join(order, ",") {
			t.Errorf("%s: order\n %v\nschema\n %v", g, order, schema)
		}
	}
}
