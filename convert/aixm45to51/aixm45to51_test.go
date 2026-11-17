package aixm45to51_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm45"
	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/convert/aixm45to51"
	"github.com/davidkohl/goaixm/version"
	"github.com/davidkohl/goaixm/xmlnode"
)

var update = flag.Bool("update", false, "rewrite the golden reports")

// ns is a test identity namespace.
var ns = convert.NewV5(convert.UUID{}, "goaixm test namespace")

const fixtures = "../../testdata/aixm45"

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixtures, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type result struct {
	rep     *convert.Report
	xml     []byte
	members []goaixm.Member
	byKey   map[string]goaixm.Member // by kind and provenance SourceKey
}

// convertBytes converts b, writes the XMLWriter form and decodes every
// record strictly (any decoding problem fails).
func convertBytes(t testing.TB, b []byte, cfg aixm45to51.Config) result {
	t.Helper()
	if cfg.Namespace.IsZero() {
		cfg.Namespace = ns
	}
	var res result
	res.byKey = map[string]goaixm.Member{}
	var buf bytes.Buffer
	sink := goaixm.MemberSink(goaixm.Options{Fallback: goaixm.FallbackSkip, Strict: true}, func(m goaixm.Member) error {
		res.members = append(res.members, m)
		res.byKey[m.Kind+" "+m.Provenance.SourceKey] = m
		return nil
	})
	rep, err := convert.Run(context.Background(), aixm45to51.New(cfg), convert.BytesSource("in.xml", b), convert.Tee(convert.XMLWriter(&buf), sink))
	if err != nil {
		t.Fatal(err)
	}
	res.rep, res.xml = rep, buf.Bytes()
	return res
}

func uuidOf(key string) string { return convert.NewV5(ns, "aixm45:"+key).String() }

func ts(t *testing.T, m goaixm.Member) *xmlnode.Node {
	t.Helper()
	n, err := m.Node()
	if err != nil {
		t.Fatal(err)
	}
	s := n.Find("timeSlice/*")
	if s == nil {
		t.Fatalf("%s: no time slice", m.Kind)
	}
	return s
}

func member(t *testing.T, res result, kind, key string) goaixm.Member {
	t.Helper()
	m, ok := res.byKey[kind+" "+key]
	if !ok {
		var keys []string
		for k := range res.byKey {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		t.Fatalf("no %s record for %s; have %s", kind, key, strings.Join(keys, "\n"))
	}
	return m
}

func hrefOf(n *xmlnode.Node, path string) string {
	c := n.Find(path)
	if c == nil {
		return ""
	}
	return strings.TrimPrefix(c.Href(), "urn:uuid:")
}

func issues(rep *convert.Report, code string) []convert.Issue {
	var out []convert.Issue
	for _, i := range rep.Issues {
		if i.Code == code {
			out = append(out, i)
		}
	}
	return out
}

func TestConvertBasic(t *testing.T) {
	res := convertBytes(t, fixture(t, "basic.xml"), aixm45to51.Config{})
	rep := res.rep
	wantTargets := map[string]int{
		"OrganisationAuthority": 1, "AirportHeliport": 1, "Runway": 2, "RunwayDirection": 1, "Unit": 2,
		"DesignatedPoint": 3, "VOR": 2, "DME": 2, "TACAN": 2, "NDB": 1, "Navaid": 5, "VerticalStructure": 3,
		"Airspace": 1, "GeoBorder": 1,
	}
	if got, _ := json.Marshal(rep.Targets); string(got) != mustJSON(wantTargets) {
		t.Errorf("targets %s", got)
	}
	if rep.Output.Records != len(res.members) {
		t.Errorf("%d records, %d members", rep.Output.Records, len(res.members))
	}

	// Identity, time slice and provenance.
	ahpKey := "Ahp(codeId=XA01)"
	ahp := member(t, res, "AirportHeliport", ahpKey)
	n, _ := ahp.Node()
	if n.TextAt("identifier") != uuidOf(ahpKey) || ahp.Provenance.SourceKind != "Ahp" || ahp.Provenance.SourceIDs[0] != "200" || ahp.Provenance.SourceIndex[0] != 1 {
		t.Errorf("AirportHeliport identity/provenance: %s %+v", n.TextAt("identifier"), ahp.Provenance)
	}
	a := ts(t, ahp)
	for path, want := range map[string]string{
		"validTime/TimePeriod/beginPosition": "2030-01-24T00:00:00.000+01:00", "interpretation": "BASELINE", "sequenceNumber": "1", "correctionNumber": "0",
		"featureLifetime/TimePeriod/beginPosition": "2030-01-24T00:00:00.000+01:00",
		"designator": "XA01", "name": "EXAMPLE FIELD", "locationIndicatorICAO": "XAAA", "designatorIATA": "XAA", "type": "AD",
		"fieldElevation": "320", "magneticVariation": "-1.5", "dateMagneticVariation": "2029", "magneticVariationChange": "0.1",
		"referenceTemperature": "28", "transitionAltitude": "5000", "servedCity/City/name": "EXAMPLE TOWN",
		"ARP/ElevatedPoint/pos": "10.25000000 20.25000000", "ARP/ElevatedPoint/geoidUndulation": "12.5",
	} {
		if got := a.TextAt(path); got != want {
			t.Errorf("AirportHeliport %s = %q, want %q", path, got, want)
		}
	}
	if hrefOf(a, "responsibleOrganisation/AirportHeliportResponsibilityOrganisation/theOrganisationAuthority") != uuidOf("Org(txtName=EXAMPLE STATE)") {
		t.Error("responsibleOrganisation href")
	}

	// References by natural key, nested.
	rdn := ts(t, member(t, res, "RunwayDirection", "Rdn(RwyUid(AhpUid(codeId=XA01);txtDesig=09/27);txtDesig=09)"))
	if hrefOf(rdn, "usedRunway") != uuidOf("Rwy(AhpUid(codeId=XA01);txtDesig=09/27)") || rdn.TextAt("trueBearing") != "089.5" || rdn.TextAt("elevationTDZ") != "310" {
		t.Errorf("RunwayDirection %v", rdn)
	}
	rwy := ts(t, member(t, res, "Runway", "Rwy(AhpUid(codeId=XA01);txtDesig=09/27)"))
	if rwy.TextAt("surfaceProperties/SurfaceCharacteristics/composition") != "CONC_ASPH" || rwy.TextAt("surfaceProperties/SurfaceCharacteristics/pavementTypePCN") != "FLEXIBLE" ||
		rwy.TextAt("surfaceProperties/SurfaceCharacteristics/evaluationMethodPCN") != "TECH" || rwy.TextAt("nominalLength") != "2400" || hrefOf(rwy, "associatedAirportHeliport") != uuidOf(ahpKey) {
		t.Errorf("Runway %v", rwy)
	}

	// Coordinates in every layout, and DM/D precision.
	dp := ts(t, member(t, res, "DesignatedPoint", "Dpn(codeId=XA012;geoLat=10.25;geoLong=20.25)"))
	if dp.TextAt("location/Point/pos") != "10.25000000 20.25000000" || dp.TextAt("type") != "TERMINAL" || hrefOf(dp, "airportHeliport") != uuidOf(ahpKey) {
		t.Errorf("DesignatedPoint XA012 %v", dp)
	}
	uni := ts(t, member(t, res, "Unit", "Uni(txtName=EXAMPLE TOWER)"))
	if uni.TextAt("position/ElevatedPoint/pos") != "10.25166667 20.25166667" || hrefOf(uni, "airportLocation") != uuidOf(ahpKey) {
		t.Errorf("Unit position %q", uni.TextAt("position/ElevatedPoint/pos"))
	}
	bravo := ts(t, member(t, res, "DesignatedPoint", "Dpn(codeId=BRAVOX;geoLat=-10.7501388889;geoLong=-20.7501388889)"))
	if bravo.TextAt("location/Point/pos") != "-10.75013889 -20.75013889" || bravo.Find("designator") != nil || bravo.TextAt("annotation/Note/translatedNote/LinguisticNote/note") != "BRAVOX" {
		t.Errorf("DesignatedPoint BRAVOX: invalid designator must become a note: %v", bravo)
	}

	// Navaids from collocation.
	vorKey := "Vor(codeId=XAV;geoLat=10.1666666667;geoLong=20.1666666667)"
	nav := ts(t, member(t, res, "Navaid", vorKey+"+Dme(codeId=XAV;geoLat=10.1666666667;geoLong=20.1666666667)"))
	comps := nav.FindAll("navaidEquipment/NavaidComponent/theNavaidEquipment")
	if nav.TextAt("type") != "VOR_DME" || len(comps) != 2 || nav.TextAt("designator") != "XAV" || nav.TextAt("location/ElevatedPoint/elevation") != "300" {
		t.Errorf("VOR_DME navaid %v", nav)
	}
	if strings.TrimPrefix(comps[0].Href(), "urn:uuid:") != uuidOf(vorKey) || strings.TrimPrefix(comps[1].Href(), "urn:uuid:") != uuidOf("Dme(codeId=XAV;geoLat=10.1666666667;geoLong=20.1666666667)") {
		t.Error("VOR_DME components")
	}
	if p := member(t, res, "Navaid", vorKey+"+Dme(codeId=XAV;geoLat=10.1666666667;geoLong=20.1666666667)").Provenance; p.SourceKind != "Vor+Dme" || len(p.SourceIndex) != 2 || len(p.SourceIDs) != 2 {
		t.Errorf("navaid provenance %+v", p)
	}
	types := map[string]string{}
	for _, m := range res.members {
		if m.Kind == "Navaid" {
			types[ts(t, m).TextAt("designator")] = ts(t, m).TextAt("type")
		}
	}
	if mustJSON(types) != `{"XAV":"VOR_DME","XCV":"VORTAC","XD":"DME","XN":"NDB","XT":"TACAN"}` {
		t.Errorf("navaid types %v", types)
	}
	vor := ts(t, member(t, res, "VOR", vorKey))
	if vor.TextAt("type") != "DVOR" || vor.TextAt("frequency") != "114.0" || vor.Find("frequency").Attrs[0].Value != "MHZ" || vor.TextAt("zeroBearingDirection") != "MAG" || vor.TextAt("declination") != "-2" ||
		hrefOf(vor, "authority/AuthorityForNavaidEquipment/theOrganisationAuthority") != uuidOf("Org(txtName=EXAMPLE STATE)") {
		t.Errorf("VOR %v", vor)
	}

	// Obstacles: duplicate natural key (equal position in two layouts),
	// unknown datum.
	obsKey := "Obs(geoLat=10.2084722222;geoLong=20.2084722222)"
	o1, o2 := ts(t, member(t, res, "VerticalStructure", obsKey)), ts(t, member(t, res, "VerticalStructure", obsKey+"#2"))
	if o1.TextAt("part/VerticalStructurePart/verticalExtent") != "330" || o2.TextAt("part/VerticalStructurePart/verticalExtent") != "320" {
		t.Error("duplicate obstacles")
	}
	if o1.TextAt("type") != "OTHER" || o1.TextAt("lighted") != "YES" || o1.TextAt("group") != "NO" ||
		o1.TextAt("part/VerticalStructurePart/horizontalProjection_location/ElevatedPoint/horizontalAccuracy") != "5" ||
		o1.TextAt("part/VerticalStructurePart/horizontalProjection_location/ElevatedPoint/verticalAccuracy") != "3" {
		t.Errorf("obstacle %v", o1)
	}
	if d := issues(rep, "duplicate_key"); len(d) != 1 || d[0].Key != obsKey || d[0].Count != 1 || d[0].Severity != convert.Warning {
		t.Errorf("duplicate_key %+v", d)
	}
	if d := issues(rep, "datum_unknown"); len(d) != 2 || d[0].Severity != convert.Error {
		t.Errorf("datum_unknown %+v", d)
	}
	o3 := ts(t, member(t, res, "VerticalStructure", "Obs(geoLat=10.3;geoLong=20.3)"))
	warned := false
	for _, note := range o3.FindAll("annotation/Note") {
		warned = warned || note.TextAt("purpose") == "WARNING"
	}
	if !warned || o3.TextAt("part/VerticalStructurePart/horizontalProjection_location/ElevatedPoint/pos") != "10.30000000 20.30000000" {
		t.Errorf("unknown datum: coordinates carried and flagged: %v", o3)
	}

	// Data problems become issues, never silent.
	for code, n := range map[string]int{"dangling_ref": 2, "code_unknown": 3, "value_invalid": 2, "kind_unmapped": 1, "datum_missing": 0} {
		got := 0
		for _, i := range issues(rep, code) {
			got += i.Count
		}
		if got != n {
			t.Errorf("%s: %d, want %d: %+v", code, got, n, issues(rep, code))
		}
	}

	// Accounting: later kinds are dropped, all their leaves unmapped;
	// airspaces and borders are converted, their boundaries merged; every
	// element is accounted for.
	if k := rep.Kinds["Ser"]; k == nil || k.Dropped != 1 || k.Out != 0 {
		t.Errorf("Ser: %+v", k)
	} else {
		for p, e := range k.Elements {
			if e.Rule != "LATER" || e.Outcomes[convert.Unmapped] == 0 || len(e.Outcomes) != 1 {
				t.Errorf("Ser %s: %+v", p, e)
			}
		}
	}
	for kind, want := range map[string][3]int{"Ase": {1, 0, 0}, "Gbr": {1, 0, 0}, "Abd": {0, 1, 0}} {
		if k := rep.Kinds[kind]; k == nil || k.Out != want[0] || k.Merged != want[1] || k.Dropped != want[2] {
			t.Errorf("%s: %+v", kind, k)
		}
	}
	if k := rep.Kinds["Obs"]; k.In != 3 || k.Out != 3 || k.Defaults != 3 {
		t.Errorf("Obs stats %+v", k)
	}
	if e := rep.Kinds["Ahp"].Elements["Ahp/txtNameAdmin"]; e == nil || e.Rule != "AHP-ADMIN" || e.Outcomes[convert.Unmapped] != 1 {
		t.Errorf("explicitly unmapped: %+v", e)
	}
	if e := rep.Kinds["Rdn"].Elements["Rdn/codeTypeVasis"]; e == nil || e.Rule != "UNMAPPED" {
		t.Errorf("unnamed leaf: %+v", e)
	}
	if e := rep.Kinds["Rwy"].Elements["Rwy/valLen"]; e.Outcomes[convert.Invalid] != 1 || e.Outcomes[convert.Mapped] != 1 {
		t.Errorf("Rwy/valLen %+v", e.Outcomes)
	}
	if rep.Kinds[aixm45.RootSnapshot].Merged != 1 || rep.Kinds[aixm45.RootSnapshot].Elements["AIXM-Snapshot/@noNamespaceSchemaLocation"].Rule != "HDR-OTHER" {
		t.Errorf("root accounting %+v", rep.Kinds[aixm45.RootSnapshot])
	}

	// The XMLWriter form parses as a native 5.1.1 message with the same
	// members.
	msg, err := goaixm.Parse(context.Background(), bytes.NewReader(res.xml), goaixm.Options{Strict: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(msg.Members()) != len(res.members) || msg.Source.Version != version.AIXM511 {
		t.Errorf("reparsed %d members of %v", len(msg.Members()), msg.Source.Version)
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestObstacleTypeHook(t *testing.T) {
	res := convertBytes(t, fixture(t, "basic.xml"), aixm45to51.Config{ObstacleType: func(s string) string {
		return map[string]string{"Mast": "POLE", "Wind turbine": "NOT_A_CODE"}[s]
	}})
	o1 := ts(t, member(t, res, "VerticalStructure", "Obs(geoLat=10.2084722222;geoLong=20.2084722222)"))
	o3 := ts(t, member(t, res, "VerticalStructure", "Obs(geoLat=10.3;geoLong=20.3)"))
	if o1.TextAt("type") != "POLE" || o3.TextAt("type") != "OTHER" {
		t.Errorf("hook: %s %s", o1.TextAt("type"), o3.TextAt("type"))
	}
	e := res.rep.Kinds["Obs"].Elements["Obs/txtDescrType"]
	if e.Outcomes[convert.Mapped] != 2 || e.Outcomes[convert.Lossy] != 1 {
		t.Errorf("outcomes %+v", e.Outcomes)
	}
}

func TestSequence(t *testing.T) {
	res := convertBytes(t, fixture(t, "basic.xml"), aixm45to51.Config{Sequence: func(h aixm45.Header) uint32 {
		if h.Origin != "Example producer" {
			t.Errorf("header %+v", h)
		}
		return 7
	}})
	if ts(t, res.members[0]).TextAt("sequenceNumber") != "7" {
		t.Error("Config.Sequence ignored")
	}
}

func TestDeterminism(t *testing.T) {
	a := convertBytes(t, fixture(t, "basic.xml"), aixm45to51.Config{})
	b := convertBytes(t, fixture(t, "basic.xml"), aixm45to51.Config{})
	if !bytes.Equal(a.xml, b.xml) || a.rep.Digest() != b.rep.Digest() || a.rep.Output.SHA256 != b.rep.Output.SHA256 {
		t.Error("two runs differ")
	}
	other := convertBytes(t, fixture(t, "basic.xml"), aixm45to51.Config{Namespace: convert.NewV5(ns, "other")})
	if other.members[0].Provenance.SourceKey != a.members[0].Provenance.SourceKey || bytes.Equal(other.xml, a.xml) {
		t.Error("namespace not used")
	}
}

// TestShuffled: another element order gives the same records, except for
// the numbering of duplicate keys.
func TestShuffled(t *testing.T) {
	b := fixture(t, "basic.xml")
	h, elems := split(t, b)
	rnd := rand.New(rand.NewSource(3))
	rnd.Shuffle(len(elems), func(i, j int) { elems[i], elems[j] = elems[j], elems[i] })
	shuffled := []byte(h + strings.Join(elems, "\n") + "\n</AIXM-Snapshot>\n")
	a, s := convertBytes(t, b, aixm45to51.Config{}), convertBytes(t, shuffled, aixm45to51.Config{})
	records := func(r result) map[string]string {
		out := map[string]string{}
		for _, m := range r.members {
			if !strings.HasPrefix(m.Provenance.SourceKey, "Obs(geoLat=10.2084722222") {
				out[m.Kind+" "+m.Provenance.SourceKey] = string(m.Raw)
			}
		}
		return out
	}
	ra, rs := records(a), records(s)
	if len(ra) != len(rs) {
		t.Fatalf("%d vs %d records", len(ra), len(rs))
	}
	for k, x := range ra {
		if rs[k] != x {
			t.Errorf("%s differs after shuffling", k)
		}
	}
	if a.rep.Count(convert.Mapped) != s.rep.Count(convert.Mapped) || a.rep.Count(convert.Unmapped) != s.rep.Count(convert.Unmapped) {
		t.Error("outcome counts differ after shuffling")
	}
}

// split returns the document up to the root start tag and the top-level
// elements as text.
func split(t *testing.T, b []byte) (string, []string) {
	t.Helper()
	s := string(b)
	i := strings.Index(s, "<AIXM-Snapshot")
	j := i + strings.Index(s[i:], ">") + 1
	var elems []string
	_, err := aixm45.Stream(context.Background(), bytes.NewReader(b), func(e aixm45.Element) error {
		var buf bytes.Buffer
		if err := e.Node.Encode(&buf); err != nil {
			return err
		}
		elems = append(elems, buf.String())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return s[:j] + "\n", elems
}

func TestErrors(t *testing.T) {
	c := aixm45to51.New(aixm45to51.Config{})
	if _, err := convert.Run(context.Background(), c, convert.BytesSource("x", fixture(t, "basic.xml")), convert.XMLWriter(&bytes.Buffer{})); !errors.Is(err, aixm45to51.ErrNoNamespace) {
		t.Errorf("no namespace: %v", err)
	}
	c = aixm45to51.New(aixm45to51.Config{Namespace: ns})
	for in, want := range map[string]error{
		`<AIXM-Update version="4.5" effective="2030-01-24T00:00:00Z"><Org/></AIXM-Update>`: aixm45to51.ErrUpdate,
		`<AIXM-Snapshot version="4.5"><Org/></AIXM-Snapshot>`:                              aixm45to51.ErrNoEffective,
		`<AIXM-Snapshot version="4.5" effective="soon"></AIXM-Snapshot>`:                   aixm45to51.ErrNoEffective,
	} {
		if _, err := convert.Run(context.Background(), c, convert.BytesSource("x", []byte(in)), convert.XMLWriter(&bytes.Buffer{})); !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", in, err, want)
		}
	}
	if _, err := convert.Run(context.Background(), c, convert.BytesSource("x", []byte(`<AIXM-Snapshot version="4.5" effective="2030-01-24T00:00:00Z"><Org>`)), convert.XMLWriter(&bytes.Buffer{})); err == nil {
		t.Error("truncated document accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := convert.Run(ctx, c, convert.BytesSource("x", fixture(t, "basic.xml")), convert.XMLWriter(&bytes.Buffer{})); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
	if c.From() != version.AIXM45 || c.To() != version.AIXM511 || c.ID().Name != aixm45to51.Name || len(c.ID().MappingDigest) != 64 {
		t.Errorf("identity %+v", c.ID())
	}
}

// TestEdgeCases: elements without their Uid, empty and odd values, an
// empty snapshot.
func TestEdgeCases(t *testing.T) {
	res := convertBytes(t, fixture(t, "edge.xml"), aixm45to51.Config{})
	if k := res.rep.Kinds["Org"]; k.Dropped != 1 || k.Out != 1 {
		t.Errorf("Org without OrgUid: %+v", k)
	}
	if len(issues(res.rep, "no_uid")) != 1 || len(issues(res.rep, "coordinate_invalid")) != 1 || len(issues(res.rep, "datum_missing")) != 3 {
		t.Errorf("issues %+v", res.rep.Issues)
	}
	empty := convertBytes(t, []byte(`<AIXM-Snapshot version="4.5" effective="2030-01-24T00:00:00Z"/>`), aixm45to51.Config{})
	if empty.rep.Output.Records != 0 || empty.rep.Kinds["AIXM-Snapshot"].Leaves != 2 || empty.rep.Kinds["AIXM-Snapshot"].Elements["AIXM-Snapshot/@effective"].Rule != "HDR-EFF" {
		t.Errorf("empty snapshot %+v", empty.rep)
	}
}

// TestGoaixmOptions: Options.Converters reads a 4.5 file directly.
func TestGoaixmOptions(t *testing.T) {
	var rep *convert.Report
	msg, err := goaixm.ParseFile(context.Background(), filepath.Join(fixtures, "basic.xml"), goaixm.Options{
		Converters:  convert.NewRegistry(aixm45to51.New(aixm45to51.Config{Namespace: ns})),
		OnConverted: func(r *convert.Report) error { rep = r; return nil },
		Strict:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep == nil || msg.Source.Version != version.AIXM45 || len(msg.Members()) != rep.Output.Records || rep.Source.Name != "basic.xml" {
		t.Errorf("message %v, report %v", msg.Source, rep)
	}
}

// TestRulesUsed: every rule of the table is exercised by a fixture, except
// the policy rules that have no leaf of their own.
func TestRulesUsed(t *testing.T) {
	policy := map[string]bool{"ID-KEY": true, "TS-SEQ": true, "NAV-NAVAID": true, "GEN-VALUE": true, "GEN-CODE": true, "ASE-GEOM": true}
	used := map[string]bool{}
	for _, f := range []string{"basic.xml", "edge.xml", "airspace.xml"} {
		res := convertBytes(t, fixture(t, f), aixm45to51.Config{})
		for _, k := range res.rep.Kinds {
			for _, e := range k.Elements {
				used[e.Rule] = true
			}
		}
	}
	for _, r := range aixm45to51.Rules() {
		if !used[r.ID] && !policy[r.ID] {
			t.Errorf("rule %s not exercised by any fixture", r.ID)
		}
		if r.Ref == "" || r.Target == "" || r.Source == "" {
			t.Errorf("rule %s incomplete: %+v", r.ID, r)
		}
	}
}

// TestGoldenReport compares the reports of basic.xml and airspace.xml
// with testdata/aixm45/*.report.json (go test -update rewrites them).  The
// module version is not part of the comparison.
func TestGoldenReport(t *testing.T) {
	for _, name := range []string{"basic", "airspace"} {
		res := convertBytes(t, fixture(t, name+".xml"), aixm45to51.Config{})
		res.rep.Converter.Module = ""
		got, err := json.MarshalIndent(res.rep, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, '\n')
		path := filepath.Join(fixtures, name+".report.json")
		if *update {
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("report differs from %s (run go test -update and review the diff)", path)
		}
	}
}
