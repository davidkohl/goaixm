package aixm45to51_test

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/convert/aixm45to51"
	"github.com/davidkohl/goaixm/geo"
	"github.com/davidkohl/goaixm/internal/testconv"
	"github.com/davidkohl/goaixm/version"
	"github.com/davidkohl/goaixm/xmlnode"
)

// The DONLON comparison: testdata/aixm45/donlon.xml is a hand-made AIXM
// 4.5 rendering of features of the public DONLON 5.1 sample (EUROCONTROL).
// Converting it and comparing the result with the DONLON originals, value
// by value, is the closest available check of the mapping in both
// directions: every value of an original feature must come back equal,
// or the difference must be listed below with its reason (something
// AIXM 4.5 cannot express, a DONLON inconsistency, or an open mapping
// question of docs/goaixm-multiversion.md, "Qn").  Geometry is compared
// separately, as the polygons geo computes from both.

const donlonFile = "../../testdata/donlon/EA_AIP_DS_FULL_20170701.xml"
const donlonObstacles = "../../testdata/donlon/EA_OBS_DS_AERA1_FULL_20191205.xml"

// donlonKinds are the kinds transcribed; donlonKey identifies a feature
// across the two documents (UUIDs differ by design).
var donlonKinds = map[string]bool{
	"OrganisationAuthority": true, "Unit": true, "AirportHeliport": true, "Runway": true, "RunwayDirection": true,
	"DesignatedPoint": true, "Navaid": true, "VOR": true, "DME": true, "NDB": true, "TACAN": true,
	"VerticalStructure": true, "GeoBorder": true, "Airspace": true,
}

func donlonKey(kind string, ts *xmlnode.Node) string {
	text := func(p string) string { return strings.TrimSpace(ts.TextAt(p)) }
	switch kind {
	case "OrganisationAuthority", "Unit", "VerticalStructure", "GeoBorder":
		return text("name")
	case "Airspace":
		typ, _, _ := strings.Cut(text("type"), ":")
		return typ + " " + text("designator")
	case "Navaid":
		fam := "VOR"
		switch typ := text("type"); {
		case strings.Contains(typ, "NDB"):
			fam = "NDB"
		case strings.Contains(typ, "ILS"):
			fam = "ILS"
		}
		return fam + " " + text("designator")
	}
	return text("designator")
}

// The transcription leaves these DONLON features out.
var donlonNotTranscribed = map[string]string{
	"Airspace A ACR001":                    "corridor (centreline and width): no 4.5 boundary form",
	"Airspace OTHER ":                      "DORNBURG: no designator, and 4.5 needs AseUid/codeId",
	"Runway 03/21":                         "FATO: a 4.5 Fto, phase 2",
	"RunwayDirection 03":                   "FATO direction: phase 2",
	"RunwayDirection 21":                   "FATO direction: phase 2",
	"Navaid ILS OXS":                       "ILS: phase 2",
	"VerticalStructure WINDPARK DONLON IV": "several parts: 4.5 Obs is one position",
	"VerticalStructure RODSAND II":         "surface extent: no 4.5 form",
	"VerticalStructure LANGTALERECKHUETTE - HOCHWILDEHAUS":        "cable car of several parts",
	"VerticalStructure MUTTERBERGALM - DRESDNER HUETTE - EISGRAT": "cable car of several parts",
	"VerticalStructure TUBORG IIB":                                "linear extent: no 4.5 form",
	"VerticalStructure OBS-0007":                                  "surface and point parts",
	"VerticalStructure DONLON NORTHEAST I":                        "surface extent: no 4.5 form",
}

// donlonExpected lists every difference the comparison finds, by
// "Kind key|path" (or "Kind *|path" for all features of a kind), with its
// reason.  A difference not listed fails the test, and so does a listed
// one that no longer occurs.
var donlonExpected = map[string]string{}

// Reasons of the expected differences.
const (
	no45     = "AIXM 4.5 has no counterpart"
	phase2   = "4.5 has it, the converter maps it in phase 2"
	donlonQ  = "DONLON inconsistency"
	equiv    = "same meaning in another form"
	property = "DONLON note on a property: 4.5 has only the feature's txtRmk"
)

func init() {
	add := func(reason, path string, who ...string) {
		for _, w := range who {
			donlonExpected[w+"|"+path] = reason
		}
	}
	const gcv = "geometryComponent/AirspaceGeometryComponent/theAirspaceVolume/AirspaceVolume/"
	const note = "annotation/Note/"
	notes := func(reason string, who ...string) {
		add(reason, note+"*", who...)
	}

	// Aerodromes.
	ahp := []string{"AirportHeliport EADD", "AirportHeliport EADH"}
	add("Q7: 4.5 valGeoidUndulation goes to the ARP's geoidUndulation; DONLON writes it as a note on fieldElevation", "ARP/ElevatedPoint/geoidUndulation/*", ahp...)
	notes("Q7 and "+property+": txtDescrRefPt becomes a DESCRIPTION note on ARP (DONLON: REMARK on \"aRP\"), the geoid note is a value", ahp...)
	add(phase2+" (Aht timesheets, usage)", "availability/*", ahp...)
	add(no45+" (the view has no civil/military code)", "controlType", ahp...)
	add(no45, "magneticVariationAccuracy", ahp...)
	add("Q7: role of the responsible organisation", "responsibleOrganisation/AirportHeliportResponsibilityOrganisation/role", ahp...)
	add("4.5 codeType is mandatory (AD, HP); DONLON has no type", "type", ahp...)

	// Airspaces.
	var all []string
	for _, a := range []string{"ATZ EADH", "CTA_P EADD1", "CTA_P EADD2", "CTR EADD", "D EAD4", "D EAD6", "FIR EAAD", "OTHER EAMTA01",
		"P EAP2", "R EAR1", "R EAR2", "R EAR3", "R EAR5", "TMA EADD", "TMA_P EAMM1", "TMA_P EAMM2"} {
		all = append(all, "Airspace "+a)
	}
	add(equiv+": the converter writes operation BASE and operationSequence 1 for a single component, DONLON omits them",
		"geometryComponent/AirspaceGeometryComponent/operation", all...)
	add(equiv+": the converter writes operation BASE and operationSequence 1 for a single component, DONLON omits them",
		"geometryComponent/AirspaceGeometryComponent/operationSequence", all...)
	gnd := []string{"Airspace ATZ EADH", "Airspace CTR EADD", "Airspace D EAD4", "Airspace D EAD6", "Airspace FIR EAAD", "Airspace P EAP2", "Airspace R EAR1", "Airspace R EAR5"}
	add("Q19: 4.5 has no GND; transcribed as 0 FT HEI, converted 0 FT SFC (same meaning)", gcv+"lowerLimit", gnd...)
	add("Q19: 4.5 has no GND; transcribed as 0 FT HEI, converted 0 FT SFC (same meaning)", gcv+"lowerLimitReference", gnd...)
	unl := []string{"Airspace FIR EAAD", "Airspace P EAP2"}
	add("Q19: 4.5 has no UNL; transcribed as 999 FL STD, converted as such", gcv+"upperLimit", unl...)
	add("Q19: 4.5 has no UNL; transcribed as 999 FL STD, converted as such", gcv+"upperLimit/@uom", unl...)
	add("Q19: 4.5 has no UNL; transcribed as 999 FL STD, converted as such", gcv+"upperLimitReference", unl...)
	notes(property, "Airspace ATZ EADH", "Airspace CTA EADD", "Airspace CTA_P EADD1", "Airspace CTA_P EADD2", "Airspace CTR EADD",
		"Airspace FIR EAAD", "Airspace TMA EADD", "Airspace TMA_P EAMM1", "Airspace TMA_P EAMM2")
	add(property, gcv+note+"*", "Airspace CTR EADD")
	add("GEN-RMK: txtRmk becomes a REMARK note; DONLON's note has no purpose", note+"purpose", "Airspace R EAR5")
	h24 := []string{"Airspace P EAP2", "Airspace R EAR1", "Airspace R EAR3", "Airspace R EAR5"}
	add("Q18: H24 converted as 00:00-24:00, DONLON writes 00:00-00:00", "activation/AirspaceActivation/timeInterval/Timesheet/endTime", h24...)
	add("Q18: the converter writes timeReference UTC for H24, DONLON omits it", "activation/AirspaceActivation/timeInterval/Timesheet/timeReference", h24...)
	add("excluded NO is the default; the converter writes no excluded", "activation/AirspaceActivation/timeInterval/Timesheet/excluded",
		"Airspace D EAD4", "Airspace D EAD6", "Airspace OTHER EAMTA01")
	for _, p := range []string{"day", "dayTil", "daylightSavingAdjust", "endTime", "startTime", "timeReference"} {
		add(equiv+": one Timsh MON to FRI (codeDayTil) against five DONLON timesheets", "activation/AirspaceActivation/timeInterval/Timesheet/"+p, "Airspace D EAD6")
	}
	add(no45+" (status INACTIVE; the transcription has no Att)", "activation/AirspaceActivation/status", "Airspace R EAR2")
	add("Q18: codeWorkHr TIMSH gives status ACTIVE; DONLON AVBL_FOR_ACTIVATION (activation by NOTAM)", "activation/AirspaceActivation/status", "Airspace OTHER EAMTA01")
	add(no45+" (4.5 time references are UTC and UTCW: no UTC-4)", "activation/AirspaceActivation/timeInterval/Timesheet/timeReference", "Airspace OTHER EAMTA01")
	add("the converter writes daylightSavingAdjust only for UTCW (YES)", "activation/AirspaceActivation/timeInterval/Timesheet/daylightSavingAdjust", "Airspace OTHER EAMTA01")
	add(equiv+": txtRmkWorkHr becomes a note on the activation, DONLON has it on the feature (propertyName activation)", "activation/AirspaceActivation/annotation/Note/*", "Airspace OTHER EAMTA01")
	notes(equiv+": txtRmkWorkHr becomes a note on the activation, DONLON has it on the feature (propertyName activation)", "Airspace OTHER EAMTA01")
	add("Q23: Ase/codeMil is not mapped (controlType)", "controlType", "Airspace OTHER EAMTA01")
	add(no45+" (no MOA type: OTHER and txtLocalType)", "type", "Airspace OTHER EAMTA01")
	add(no45+" (one codeClass per Ase: no class layers)", "class/*", "Airspace FIR EAAD")
	add(no45, "designatorICAO", "Airspace FIR EAAD")
	add("Q17: Adg gives contributor airspaces (HORZ_PROJECTION, the Ase's limits on every volume); DONLON has the parts inline with their own limits",
		gcv+"contributorAirspace/*", "Airspace CTA EADD")
	add("Q17: the derived airspace's limits go on every volume; DONLON has FL 30 and FL 50", gcv+"lowerLimit", "Airspace CTA EADD")

	// Navaids.
	add(phase2+" (Vtt, Dtt, Ntt timesheets: GEN-TT)", "availability/*", "DME BOR", "NDB DON", "NDB KL", "NDB SCN", "VOR BOR")
	add(no45+" (vertical datum)", "location/ElevatedPoint/verticalDatum", "DME BOR", "DME DON", "Navaid VOR BOR", "Navaid VOR DON")
	add("GEN-RMK: txtRmk becomes a REMARK note; DONLON's note has no purpose", note+"purpose", "NDB DON", "NDB KL", "NDB SCN")
	add("Q10: the Navaid takes its location from the primary equipment (the VOR); DONLON gives the Navaid the DME's elevation and accuracy",
		"location/ElevatedPoint/*", "Navaid VOR BOR", "Navaid VOR DON")
	add("Q10: the Navaid takes its location from the primary equipment; DONLON's Navaid has no elevation", "location/ElevatedPoint/*", "Navaid VOR LMD", "Navaid VOR WOB")
	add("Q10: the Navaid takes the name of its primary equipment; DONLON's VOR name differs", "name", "Navaid VOR LMD", "Navaid VOR WOB")
	add(donlonQ+": collocationGroup on a single equipment (the converter writes it for collocated equipment only)",
		"navaidEquipment/NavaidComponent/collocationGroup", "Navaid VOR CAA", "Navaid VOR KAV", "Navaid VOR OST")
	add(phase2+" (Mkr: NDB_MKR with its marker)", "navaidEquipment/*", "Navaid NDB SCN")
	add(phase2+" (Mkr: NDB_MKR with its marker)", "type", "Navaid NDB SCN")
	add(donlonQ+": malformed reference urn:uuid.…", "navaidEquipment/NavaidComponent/theNavaidEquipment/@href", "Navaid NDB DON")
	add(donlonQ+": type VOR_DME with a VOR only", "type", "Navaid VOR KAV")
	add(no45+" (no navaid to aerodrome link)", "servedAirport/@href", "Navaid NDB KL", "Navaid NDB SCN", "Navaid VOR BOR")

	// Points.
	add(no45+" (4.5 Dpn types ICAO, ADHP, COORD, OTHER)", "type", "DesignatedPoint BOR10", "DesignatedPoint EADH", "DesignatedPoint TORSA")

	// Organisations and units.
	add(no45+" (no OTHER:… extension codes)", "type", "OrganisationAuthority DONBURG AERO CLUB")
	add(no45+" (contact details not in the 4.5 entities converted)", "contact/*", "OrganisationAuthority DONBURG AERO CLUB", "Unit DONLONRCC")
	add(phase2+" (organisation and unit relations)", "relatedOrganisationAuthority/*", "OrganisationAuthority CIVIL AVIATION ADMINISTRATION")
	add(phase2+" (organisation and unit relations)", "relatedUnit/*", "Unit AKVINSAR")
	add("Q2: Uni/codeClass target (compliantICAO?)", "compliantICAO", "Unit AKVINSAR", "Unit DONLONRCC")
	add(no45+" (Uni has no elevation)", "position/ElevatedPoint/*", "Unit AKVINSAR")

	// Runways.
	add("Q24: the converter writes no Runway type (4.5 Rwy are runways, FATOs are Fto)", "type", "Runway 09L/27R", "Runway 09R/27L")
	add(no45, "lengthAccuracy/*", "Runway 09L/27R")
	add(no45, "widthAccuracy/*", "Runway 09L/27R")
	add(no45, "trueBearingAccuracy", "RunwayDirection 09L", "RunwayDirection 27R")

	// Obstacles.
	obs := []string{"VerticalStructure OBS-0001", "VerticalStructure OBS-0008", "VerticalStructure OBS-0009 (DONLON ARENA)", "VerticalStructure OBS-0010"}
	notes(no45+" (DONLON's notes on marking, owner and resolution; one txtRmk in 4.5)", obs...)
	add("Q6: the converter writes the type on the VerticalStructure only, DONLON also on the part", "part/VerticalStructurePart/type", obs...)
	add(no45+" (vertical datum)", "part/VerticalStructurePart/horizontalProjection_location/ElevatedPoint/verticalDatum", obs...)
	add(no45+" (height accuracy)", "part/VerticalStructurePart/verticalExtentAccuracy/*", obs...)
	add(no45, "radius/*", "VerticalStructure OBS-0009 (DONLON ARENA)", "VerticalStructure OBS-0010")
	add(no45+" (lighting schedule)", "lightingAvailability/*", "VerticalStructure OBS-0001")
	add(no45, "part/VerticalStructurePart/mobile", "VerticalStructure OBS-0001")
	add("Q6: CHIMNEY is not a 5.1.1 code: OTHER with a DESCRIPTION note; DONLON OTHER:CHIMNEY", "type", "VerticalStructure OBS-0008")
}

type donlonFeature struct {
	kind, key string
	ts        *xmlnode.Node
	feature   aixm.Feature
}

func donlonFeatures(t *testing.T, msg *goaixm.Message) (map[string]*donlonFeature, map[string]string) {
	t.Helper()
	out := map[string]*donlonFeature{}
	labels := map[string]string{}
	for _, m := range msg.Members() {
		if !donlonKinds[m.Kind] {
			continue
		}
		n, err := m.Node()
		if err != nil {
			t.Fatal(err)
		}
		s := n.Find("timeSlice/*")
		key := donlonKey(m.Kind, s)
		label := m.Kind + " " + key
		if _, dup := out[label]; dup {
			t.Fatalf("two features %s", label)
		}
		out[label] = &donlonFeature{kind: m.Kind, key: key, ts: s, feature: m.Feature}
		if id := n.TextAt("identifier"); id != "" {
			labels[strings.TrimSpace(id)] = label
		}
	}
	return out, labels
}

// Time-slice properties the comparison skips: the time model (section 7)
// differs by design, and geometry is compared as polygons.
var donlonSkip = map[string]bool{
	"validTime": true, "interpretation": true, "sequenceNumber": true, "correctionNumber": true, "featureLifetime": true,
	"horizontalProjection": true, "centreline": true, "border": true, "extension": true,
}

// donlonLeaves flattens a time slice into path → values: element texts
// and the uom and href attributes (references as the label of their
// target); ids, srsName, titles and nil elements are left out.
func donlonLeaves(ts *xmlnode.Node, labels map[string]string) map[string][]string {
	out := map[string][]string{}
	var walk func(n *xmlnode.Node, path string)
	walk = func(n *xmlnode.Node, path string) {
		if n.IsNil() {
			return
		}
		for _, a := range n.Attrs {
			switch a.Name.Local {
			case "uom":
				out[path+"/@uom"] = append(out[path+"/@uom"], a.Value)
			case "href":
				v := a.Value
				if l, ok := labels[goaixm.HrefID(v)]; ok {
					v = l
				}
				out[path+"/@href"] = append(out[path+"/@href"], v)
			}
		}
		if len(n.Children) == 0 {
			if v := strings.Join(strings.Fields(n.Text), " "); v != "" {
				out[path] = append(out[path], v)
			}
			return
		}
		for _, c := range n.Children {
			if donlonSkip[c.Local()] {
				continue
			}
			walk(c, path+"/"+c.Local())
		}
	}
	for _, c := range ts.Children {
		if !donlonSkip[c.Local()] {
			walk(c, c.Local())
		}
	}
	for _, v := range out {
		sort.Strings(v)
	}
	return out
}

// sameValue compares two values as text, or as numbers (each
// space-separated token within 1e-7, about 1 cm for degrees).
func sameValue(a, b string) bool {
	if a == b {
		return true
	}
	fa, fb := strings.Fields(a), strings.Fields(b)
	if len(fa) != len(fb) {
		return false
	}
	for i := range fa {
		x, err1 := strconv.ParseFloat(fa[i], 64)
		y, err2 := strconv.ParseFloat(fb[i], 64)
		if err1 != nil || err2 != nil || math.Abs(x-y) > 1e-7 {
			return false
		}
	}
	return true
}

func sameValues(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// Numeric values sort as text; compare in order after sorting both
	// numerically where possible.
	key := func(s string) string {
		if f, err := strconv.ParseFloat(strings.Fields(s)[0], 64); err == nil {
			return fmt.Sprintf("%020.9f", f+1e9)
		}
		return s
	}
	a, b = append([]string(nil), a...), append([]string(nil), b...)
	sort.Slice(a, func(i, j int) bool { return key(a[i]) < key(a[j]) })
	sort.Slice(b, func(i, j int) bool { return key(b[i]) < key(b[j]) })
	for i := range a {
		if !sameValue(a[i], b[i]) {
			return false
		}
	}
	return true
}

func donlonConverted(t *testing.T) (*goaixm.Message, result) {
	t.Helper()
	res := convertBytes(t, fixture(t, "donlon.xml"), aixm45to51.Config{
		// DONLON's obstacle types are 5.1.1 codes already; the hook takes
		// the ones the list has.
		ObstacleType: func(s string) string { return s },
	})
	msg, err := goaixm.Parse(context.Background(), bytes.NewReader(res.xml), goaixm.Options{Strict: true, KeepRaw: true})
	if err != nil {
		t.Fatal(err)
	}
	return msg, res
}

func donlonOriginal(t *testing.T) *goaixm.Message {
	t.Helper()
	if _, err := os.Stat(filepath.FromSlash(donlonFile)); err != nil {
		t.Skip("DONLON reference data missing (git submodule update --init)")
	}
	msg, err := goaixm.ParseFile(context.Background(), filepath.FromSlash(donlonFile), goaixm.Options{KeepRaw: true})
	if err != nil {
		t.Fatal(err)
	}
	// The obstacle set is AIXM 5.1: relabelled to 5.1.1 (the
	// VerticalStructure schema is the same).
	obs, err := goaixm.ParseFile(context.Background(), filepath.FromSlash(donlonObstacles), goaixm.Options{KeepRaw: true,
		Converters: convert.NewRegistry(testconv.New(version.AIXM51, version.AIXM511))})
	if err != nil {
		t.Fatal(err)
	}
	// One message of both: the obstacles reference nothing of the AIP set.
	var buf bytes.Buffer
	buf.WriteString(`<message:AIXMBasicMessage xmlns:message="http://www.aixm.aero/schema/5.1.1/message" xmlns:gml="http://www.opengis.net/gml/3.2" gml:id="donlon">`)
	for _, m := range append(msg.Members(), obs.Members()...) {
		buf.WriteString("<message:hasMember>")
		buf.Write(m.Raw)
		buf.WriteString("</message:hasMember>")
	}
	buf.WriteString(`</message:AIXMBasicMessage>`)
	all, err := goaixm.Parse(context.Background(), &buf, goaixm.Options{KeepRaw: true})
	if err != nil {
		t.Fatal(err)
	}
	return all
}

func TestDonlonComparison(t *testing.T) {
	conv, res := donlonConverted(t)
	orig := donlonOriginal(t)
	of, olabels := donlonFeatures(t, orig)
	cf, clabels := donlonFeatures(t, conv)

	var labels []string
	for l := range of {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	hit := map[string]bool{}
	expected := func(f *donlonFeature, path string) (string, bool) {
		for _, k := range []string{f.kind + " " + f.key + "|" + path, f.kind + " *|" + path} {
			if r, ok := donlonExpected[k]; ok {
				hit[k] = true
				return r, true
			}
		}
		// "path/*" covers everything below path.
		for k, r := range donlonExpected {
			who, pre, _ := strings.Cut(k, "|")
			if (who == f.kind+" "+f.key || who == f.kind+" *") && strings.HasSuffix(pre, "/*") && strings.HasPrefix(path+"/", strings.TrimSuffix(pre, "*")) {
				hit[k] = true
				return r, true
			}
		}
		return "", false
	}
	type tally struct{ features, equal, expected int }
	tallies := map[string]*tally{}
	for _, l := range labels {
		o := of[l]
		c, ok := cf[l]
		if !ok {
			if _, skip := donlonNotTranscribed[l]; !skip {
				t.Errorf("%s: not in the converted document", l)
			}
			continue
		}
		if r, skip := donlonNotTranscribed[l]; skip {
			t.Errorf("%s: transcribed although listed as not transcribed (%s)", l, r)
		}
		ty := tallies[o.kind]
		if ty == nil {
			ty = &tally{}
			tallies[o.kind] = ty
		}
		ty.features++
		ol, cl := donlonLeaves(o.ts, olabels), donlonLeaves(c.ts, clabels)
		paths := map[string]bool{}
		for p := range ol {
			paths[p] = true
		}
		for p := range cl {
			paths[p] = true
		}
		var ps []string
		for p := range paths {
			ps = append(ps, p)
		}
		sort.Strings(ps)
		for _, p := range ps {
			if sameValues(ol[p], cl[p]) {
				ty.equal += len(ol[p])
				continue
			}
			if _, ok := expected(o, p); ok {
				ty.expected++
				continue
			}
			t.Errorf("%s|%s: DONLON %q, converted %q", l, p, ol[p], cl[p])
		}
	}
	for l := range cf {
		if _, ok := of[l]; !ok {
			t.Errorf("%s: converted, but not in DONLON", l)
		}
	}
	for k, r := range donlonExpected {
		if !hit[k] {
			t.Errorf("expected difference %s (%s) did not occur", k, r)
		}
	}
	var kinds []string
	for k := range tallies {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		ty := tallies[k]
		t.Logf("%-22s %2d features, %3d values equal, %2d paths differ as listed", k, ty.features, ty.equal, ty.expected)
	}
	// The conversion itself reports only the inconsistency DONLON has and
	// the transcription keeps (FL with a height reference), plus the kinds
	// it does not convert yet.
	var got []string
	for _, i := range res.rep.Issues {
		got = append(got, i.Code+" "+i.Key)
	}
	if want := []string{"limit_inconsistent Ase(codeId=EADD1;codeType=CTA-P)"}; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("issues %q, want %q", got, want)
	}
	t.Logf("report: mapped %d, derived %d, defaulted %d, lossy %d, unmapped %d, invalid %d",
		res.rep.Count(convert.Mapped), res.rep.Count(convert.Derived), res.rep.Count(convert.Defaulted),
		res.rep.Count(convert.Lossy), res.rep.Count(convert.Unmapped), res.rep.Count(convert.Invalid))
}

// ---- geometry ----

// local projects [lon, lat] to metres around an origin (equirectangular on
// the WGS-84 radii of curvature: below 0.1 % error over a few hundred km,
// enough for deviations of metres).
type local struct{ lon0, lat0, kx, ky float64 }

func newLocal(p [2]float64) local {
	lat := p[1] * math.Pi / 180
	s := math.Sin(lat)
	w := math.Sqrt(1 - wgsE2*s*s)
	n := wgsA / w
	m := wgsA * (1 - wgsE2) / (w * w * w)
	return local{p[0], p[1], n * math.Cos(lat) * math.Pi / 180, m * math.Pi / 180}
}

func (l local) xy(p [2]float64) [2]float64 {
	return [2]float64{(p[0] - l.lon0) * l.kx, (p[1] - l.lat0) * l.ky}
}

func segDist(p, a, b [2]float64) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	t := 0.0
	if d := dx*dx + dy*dy; d > 0 {
		t = math.Max(0, math.Min(1, ((p[0]-a[0])*dx+(p[1]-a[1])*dy)/d))
	}
	return math.Hypot(p[0]-a[0]-t*dx, p[1]-a[1]-t*dy)
}

// hausdorff is the symmetric Hausdorff distance in metres between two
// polylines (vertices to the other polyline).
func hausdorff(a, b [][2]float64) float64 {
	l := newLocal(a[0])
	pa, pb := make([][2]float64, len(a)), make([][2]float64, len(b))
	for i, p := range a {
		pa[i] = l.xy(p)
	}
	for i, p := range b {
		pb[i] = l.xy(p)
	}
	one := func(x, y [][2]float64) float64 {
		worst := 0.0
		for _, p := range x {
			best := math.Inf(1)
			for i := 0; i+1 < len(y); i++ {
				best = math.Min(best, segDist(p, y[i], y[i+1]))
			}
			if len(y) == 1 {
				best = math.Hypot(p[0]-y[0][0], p[1]-y[0][1])
			}
			worst = math.Max(worst, best)
		}
		return worst
	}
	return math.Max(one(pa, pb), one(pb, pa))
}

var donlonGeo = geo.Options{ArcTolerance: 1, GeodesicTolerance: 1}

func airspaceRings(msg *goaixm.Message, a *aixm.Airspace) ([][][2]float64, error) {
	fc, err := donlonGeo.AirspaceToGeoJSONWithRefs(a, msg)
	if err != nil {
		return nil, err
	}
	var out [][][2]float64
	for _, ft := range fc.Features {
		switch g := ft.Geometry.(type) {
		case geo.Polygon:
			out = append(out, g.Coordinates[0])
		case geo.MultiPolygon:
			for _, p := range g.Coordinates {
				out = append(out, p[0])
			}
		}
	}
	return out, nil
}

// donlonGeometry lists the geometry the comparison expects to differ, by
// label, with the reason; the others must agree within donlonTolerance.
var donlonGeometry = map[string]string{
	"Airspace R EAR5": donlonQ + ": the third curve member repeats the closing edge (a spike); the transcription has the five distinct vertices (same outline, Hausdorff 0)",
}

// donlonTolerance is the largest Hausdorff distance between a DONLON
// polygon and its conversion (metres), and the largest relative area
// difference.
const (
	donlonTolerance     = 1.0
	donlonAreaTolerance = 1e-6
)

func TestDonlonGeometry(t *testing.T) {
	conv, _ := donlonConverted(t)
	orig := donlonOriginal(t)
	of, _ := donlonFeatures(t, orig)
	cf, _ := donlonFeatures(t, conv)
	var labels []string
	for l, f := range of {
		if _, ok := cf[l]; ok && (f.kind == "Airspace" || f.kind == "GeoBorder") {
			labels = append(labels, l)
		}
	}
	sort.Strings(labels)
	hit := map[string]bool{}
	for _, l := range labels {
		o, c := of[l], cf[l]
		if o.kind == "GeoBorder" {
			ob, err1 := donlonGeo.CurveCoords(o.feature.(*aixm.GeoBorder).TimeSlice[0].Border, orig)
			cb, err2 := donlonGeo.CurveCoords(c.feature.(*aixm.GeoBorder).TimeSlice[0].Border, conv)
			if err1 != nil || err2 != nil {
				t.Fatalf("%s: %v %v", l, err1, err2)
			}
			h := hausdorff(ob, cb)
			t.Logf("%-28s border %d/%d positions, Hausdorff %.6f m", l, len(ob), len(cb), h)
			if h > donlonTolerance {
				t.Errorf("%s: Hausdorff %.3f m", l, h)
			}
			continue
		}
		or, err1 := airspaceRings(orig, o.feature.(*aixm.Airspace))
		cr, err2 := airspaceRings(conv, c.feature.(*aixm.Airspace))
		msg := ""
		switch {
		case err1 != nil:
			msg = "DONLON geometry not readable: " + err1.Error()
		case err2 != nil:
			msg = "converted geometry not readable: " + err2.Error()
		case len(or) != len(cr):
			msg = fmt.Sprintf("%d polygons in DONLON, %d converted", len(or), len(cr))
		}
		worst, worstArea := 0.0, 0.0
		if msg == "" {
			used := map[int]bool{}
			for _, r := range or {
				best, bi := math.Inf(1), -1
				for j, s := range cr {
					if used[j] {
						continue
					}
					if h := hausdorff(r, s); h < best {
						best, bi = h, j
					}
				}
				used[bi] = true
				worst = math.Max(worst, best)
				ao, ac := ellipsoidArea(r), ellipsoidArea(cr[bi])
				worstArea = math.Max(worstArea, math.Abs(ac-ao)/ao)
			}
			t.Logf("%-28s %d polygon(s), Hausdorff %.6f m, area %+.2e", l, len(or), worst, worstArea)
			if worst > donlonTolerance || worstArea > donlonAreaTolerance {
				msg = fmt.Sprintf("Hausdorff %.3f m, relative area difference %.2e", worst, worstArea)
			}
		}
		if r, ok := donlonGeometry[l]; ok {
			hit[l] = true
			if msg == "" {
				t.Errorf("%s: expected a geometry difference (%s), found none", l, r)
			} else {
				t.Logf("%-28s expected: %s (%s)", l, msg, r)
			}
			continue
		}
		if msg != "" {
			t.Errorf("%s: %s", l, msg)
		}
	}
	for l, r := range donlonGeometry {
		if !hit[l] {
			t.Errorf("expected geometry difference %s (%s) not checked", l, r)
		}
	}
}
