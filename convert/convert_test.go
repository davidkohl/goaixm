package convert_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/internal/testconv"
	"github.com/davidkohl/goaixm/version"
	"github.com/davidkohl/goaixm/xmlnode"
)

// message returns a small synthetic AIXM message of version v.
func message(v version.Version) []byte {
	ns := version.NamespaceURI(v)
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<message:AIXMBasicMessage xmlns:message="` + ns + `/message" xmlns:aixm="` + ns + `"
  xmlns:gml="http://www.opengis.net/gml/3.2" xmlns:xlink="http://www.w3.org/1999/xlink" gml:id="M1">
  <gml:boundedBy/>
  <message:hasMember>
    <aixm:DesignatedPoint gml:id="dp1">
      <gml:identifier codeSpace="urn:uuid:">00000000-0000-4000-8000-000000000001</gml:identifier>
      <aixm:timeSlice><aixm:DesignatedPointTimeSlice gml:id="dp1-ts">
        <aixm:interpretation>BASELINE</aixm:interpretation>
        <aixm:designator>ALPHA</aixm:designator>
        <aixm:type>ICAO</aixm:type>
      </aixm:DesignatedPointTimeSlice></aixm:timeSlice>
    </aixm:DesignatedPoint>
  </message:hasMember>
  <message:hasMember>
    <aixm:Navaid gml:id="nav1">
      <aixm:timeSlice><aixm:NavaidTimeSlice gml:id="nav1-ts">
        <aixm:designator>BRV</aixm:designator>
        <aixm:navaidEquipment xlink:href="urn:uuid:00000000-0000-4000-8000-000000000003"/>
      </aixm:NavaidTimeSlice></aixm:timeSlice>
    </aixm:Navaid>
  </message:hasMember>
</message:AIXMBasicMessage>
`)
}

type collector struct {
	header  convert.Header
	records []convert.Record
	ended   bool
}

func (c *collector) Begin(h convert.Header) error { c.header = h; return nil }
func (c *collector) Put(r convert.Record) error   { c.records = append(c.records, r); return nil }
func (c *collector) End() error                   { c.ended = true; return nil }
func src(v version.Version) convert.Source        { return convert.BytesSource("test.xml", message(v)) }
func ctx() context.Context                        { return context.Background() }
func sha(b []byte) string                         { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func run(t *testing.T, c convert.Converter, s convert.Source) (*convert.Report, []byte) {
	t.Helper()
	var buf bytes.Buffer
	rep, err := convert.Run(ctx(), c, s, convert.XMLWriter(&buf))
	if err != nil {
		t.Fatal(err)
	}
	return rep, buf.Bytes()
}

func TestRunReport(t *testing.T) {
	c := testconv.New(version.AIXM51, version.AIXM511)
	var col collector
	var buf bytes.Buffer
	rep, err := convert.Run(ctx(), c, src(version.AIXM51), convert.Tee(&col, convert.XMLWriter(&buf)))
	if err != nil {
		t.Fatal(err)
	}
	if !col.ended || col.header.Version != version.AIXM511 || col.header.GMLID != "M1" || len(col.records) != 2 {
		t.Fatalf("sink got %+v", col)
	}
	if rep.Format != convert.ReportFormat || rep.From != version.AIXM51 || rep.To != version.AIXM511 || rep.Converter != c.ID() {
		t.Errorf("report header: %+v", rep)
	}
	if rep.Source.Name != "test.xml" || rep.Source.SHA256 != sha(message(version.AIXM51)) || rep.Source.Bytes != int64(len(message(version.AIXM51))) {
		t.Errorf("source info: %+v", rep.Source)
	}
	if rep.Source.Root != "AIXMBasicMessage" || rep.Source.Version != version.AIXM51 {
		t.Errorf("source root: %+v", rep.Source)
	}
	if rep.Output.Records != 2 || rep.Output.SHA256 != sha(buf.Bytes()) || rep.Output.Bytes != int64(buf.Len()) {
		t.Errorf("output info %+v, XMLWriter wrote %d bytes", rep.Output, buf.Len())
	}
	if rep.Targets["DesignatedPoint"] != 1 || rep.Targets["Navaid"] != 1 {
		t.Errorf("targets: %v", rep.Targets)
	}
	dp := rep.Kinds["DesignatedPoint"]
	if dp == nil || dp.In != 1 || dp.Out != 1 || dp.Leaves != rep.Count(convert.Mapped)-rep.Kinds["Navaid"].Leaves {
		t.Errorf("kind stats: %+v", dp)
	}
	e := dp.Elements["DesignatedPoint/timeSlice/DesignatedPointTimeSlice/designator"]
	if e == nil || e.Outcomes[convert.Mapped] != 1 || len(e.Samples) != 1 || e.Samples[0].Value != "ALPHA" {
		t.Errorf("designator stats: %+v", e)
	}
	if n := rep.IssueCount(convert.Warning); n != 1 {
		t.Errorf("warnings = %d, want 1 (navaid without identifier)", n)
	}
	if p := col.records[1].Provenance; p.SourceKind != "Navaid" || p.SourceVersion != version.AIXM51 || len(p.SourceIndex) != 1 || p.SourceIndex[0] != 1 {
		t.Errorf("provenance: %+v", p)
	}
	// The converted document is AIXM 5.1.1 and converts no further.
	det, err := version.Detect(bytes.NewReader(buf.Bytes()))
	if err != nil || det.Version != version.AIXM511 || det.Attrs["id"] != "M1" {
		t.Errorf("output detects as %+v, %v", det, err)
	}
	if strings.Contains(buf.String(), version.NamespaceURI(version.AIXM51)+`"`) {
		t.Error("output still contains the 5.1 namespace")
	}
}

func TestDeterminism(t *testing.T) {
	c := testconv.New(version.AIXM51, version.AIXM511)
	r1, out1 := run(t, c, src(version.AIXM51))
	r2, out2 := run(t, c, src(version.AIXM51))
	if !bytes.Equal(out1, out2) {
		t.Error("outputs differ")
	}
	if r1.Digest() != r2.Digest() || len(r1.Digest()) != 64 {
		t.Errorf("digests differ: %s %s", r1.Digest(), r2.Digest())
	}
	// A two-pass converter gives the same result.
	c2 := testconv.New(version.AIXM51, version.AIXM511)
	c2.Passes = 2
	r3, out3 := run(t, c2, src(version.AIXM51))
	if !bytes.Equal(out1, out3) || r3.Digest() != r1.Digest() {
		t.Error("two-pass conversion differs")
	}
	// The report survives a JSON round trip with the same digest.
	b, _ := r1.JSON()
	var back convert.Report
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Digest() != r1.Digest() {
		t.Error("digest changed after JSON round trip")
	}
}

func TestAccountingViolation(t *testing.T) {
	c := testconv.New(version.AIXM51, version.AIXM511)
	c.LoseLeaf = true
	rep, err := convert.Run(ctx(), c, src(version.AIXM51), convert.XMLWriter(&bytes.Buffer{}))
	if !errors.Is(err, convert.ErrAccounting) || rep == nil {
		t.Fatalf("err = %v, want ErrAccounting with report", err)
	}
	if !strings.Contains(err.Error(), "DesignatedPoint") || !strings.Contains(err.Error(), "Navaid") {
		t.Errorf("error should name both kinds: %v", err)
	}
	r := convert.NewReport()
	k := r.Kind("X")
	k.In, k.Out, k.Merged, k.Dropped = 3, 1, 1, 0
	if err := r.Check(); !errors.Is(err, convert.ErrAccounting) {
		t.Errorf("in/out mismatch not caught: %v", err)
	}
	k.Dropped = 1
	r.Output.Records = 1
	if err := r.Check(); !errors.Is(err, convert.ErrAccounting) {
		t.Errorf("record/target mismatch not caught: %v", err)
	}
	r.Targets["Y"] = 1
	if err := r.Check(); err != nil {
		t.Errorf("valid report rejected: %v", err)
	}
	// A defaulted target value has no source leaf: Default keeps the
	// accounting balanced, Record with Defaulted alone does not.
	k.Default("X/(featureLifetime)", "r", "X/featureLifetime")
	if err := r.Check(); err != nil || k.Defaults != 1 || r.Count(convert.Defaulted) != 1 {
		t.Errorf("Default: defaults %d, err %v", k.Defaults, err)
	}
	k.Record("X/y", convert.Defaulted, "r", "", nil)
	if err := r.Check(); !errors.Is(err, convert.ErrAccounting) {
		t.Errorf("outcome without leaf or default not caught: %v", err)
	}
}

func TestRunVersionMismatch(t *testing.T) {
	c := testconv.New(version.AIXM51, version.AIXM511)
	if _, err := convert.Run(ctx(), c, src(version.AIXM52), &collector{}); !errors.Is(err, convert.ErrVersionMismatch) {
		t.Errorf("err = %v", err)
	}
	if _, err := convert.Run(ctx(), c, convert.BytesSource("x", []byte("<html/>")), &collector{}); !errors.Is(err, version.ErrUnknownVersion) {
		t.Errorf("err = %v", err)
	}
}

// badConverter misuses its sink.
type badConverter struct {
	*testconv.Relabel
	mode string
}

func (b badConverter) Convert(_ context.Context, _ convert.Source, dst convert.Sink) (*convert.Report, error) {
	h := convert.Header{Version: b.To()}
	switch b.mode {
	case "put-first":
		return nil, dst.Put(convert.Record{Kind: "X", XML: []byte("<X/>")})
	case "no-end":
		return nil, dst.Begin(h)
	case "wrong-version":
		return nil, dst.Begin(convert.Header{Version: version.AIXM52})
	case "empty-record":
		if err := dst.Begin(h); err != nil {
			return nil, err
		}
		return nil, dst.Put(convert.Record{Kind: "X"})
	}
	return nil, nil
}

func TestRunSinkOrder(t *testing.T) {
	base := testconv.New(version.AIXM51, version.AIXM511)
	for mode, want := range map[string]error{
		"put-first":     convert.ErrSinkOrder,
		"no-end":        convert.ErrSinkOrder,
		"wrong-version": convert.ErrVersionMismatch,
		"none":          convert.ErrSinkOrder,
	} {
		_, err := convert.Run(ctx(), badConverter{base, mode}, src(version.AIXM51), &collector{})
		if !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", mode, err, want)
		}
	}
	if _, err := convert.Run(ctx(), badConverter{base, "empty-record"}, src(version.AIXM51), &collector{}); err == nil {
		t.Error("empty record accepted")
	}
}

func TestRunCancelled(t *testing.T) {
	c, cancel := context.WithCancel(ctx())
	cancel()
	_, err := convert.Run(c, testconv.New(version.AIXM51, version.AIXM511), src(version.AIXM51), &collector{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
}

func TestRegistry(t *testing.T) {
	a := testconv.New(version.AIXM52, version.AIXM51)
	b := testconv.New(version.AIXM51, version.AIXM511)
	reg := convert.NewRegistry(a, b)
	if err := reg.Add(testconv.New(version.AIXM51, version.AIXM511)); err == nil {
		t.Error("duplicate pair accepted")
	}
	if err := reg.Add(testconv.New(version.AIXM51, version.AIXM51)); err == nil {
		t.Error("identity converter accepted")
	}
	if err := reg.Add(nil); err == nil {
		t.Error("nil converter accepted")
	}
	path, err := reg.Path(version.AIXM52, version.AIXM511)
	if err != nil || len(path) != 2 || path[0] != convert.Converter(a) || path[1] != convert.Converter(b) {
		t.Fatalf("path = %v, %v", path, err)
	}
	if p, err := reg.Path(version.AIXM511, version.AIXM511); err != nil || len(p) != 0 {
		t.Errorf("same version path = %v, %v", p, err)
	}
	if _, err := reg.Path(version.AIXM45, version.AIXM511); !errors.Is(err, convert.ErrNoPath) {
		t.Errorf("err = %v", err)
	}
	if _, err := reg.Converter(version.AIXM511, version.AIXM511); !errors.Is(err, convert.ErrNoPath) {
		t.Errorf("err = %v", err)
	}
	if c, err := reg.Converter(version.AIXM51, version.AIXM511); err != nil || c != convert.Converter(b) {
		t.Errorf("single step should return the converter itself: %v %v", c, err)
	}
	// A direct converter is shorter than the chain.
	direct := testconv.New(version.AIXM52, version.AIXM511)
	if err := reg.Add(direct); err != nil {
		t.Fatal(err)
	}
	if path, _ := reg.Path(version.AIXM52, version.AIXM511); len(path) != 1 || path[0] != convert.Converter(direct) {
		t.Errorf("shortest path not chosen: %v", path)
	}
	if got := len(reg.Converters()); got != 3 {
		t.Errorf("Converters() = %d", got)
	}
	// Deterministic among equal lengths: via the lower intermediate version.
	v53 := version.Version{Major: 5, Minor: 3}
	r2 := convert.NewRegistry(
		testconv.New(version.AIXM52, v53), testconv.New(v53, version.AIXM511),
		testconv.New(version.AIXM52, version.AIXM51), testconv.New(version.AIXM51, version.AIXM511),
	)
	for i := 0; i < 20; i++ {
		p, err := r2.Path(version.AIXM52, version.AIXM511)
		if err != nil || p[0].To() != version.AIXM51 {
			t.Fatalf("path via %v, %v", p[0].To(), err)
		}
	}
}

func TestChain(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	a := testconv.New(version.AIXM52, version.AIXM51)
	a.Passes = 2
	b := testconv.New(version.AIXM51, version.AIXM511)
	ch, err := convert.Chain(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if ch.From() != version.AIXM52 || ch.To() != version.AIXM511 || !strings.HasPrefix(ch.ID().Name, "chain(") {
		t.Errorf("chain id %v %v %v", ch.From(), ch.To(), ch.ID())
	}
	rep, out := run(t, ch, src(version.AIXM52))
	if len(rep.Steps) != 2 || rep.Steps[0].To != version.AIXM51 || rep.Steps[1].From != version.AIXM51 {
		t.Fatalf("steps: %+v", rep.Steps)
	}
	if rep.Output.Records != 2 || rep.Steps[1].Output.SHA256 != rep.Output.SHA256 {
		t.Errorf("output %+v, last step %+v", rep.Output, rep.Steps[1].Output)
	}
	if rep.Steps[1].Source.Name != "test.xml" {
		t.Errorf("intermediate source name = %q", rep.Steps[1].Source.Name)
	}
	if rep.Count(convert.Mapped) != rep.Steps[0].Count(convert.Mapped)+rep.Steps[1].Count(convert.Mapped) {
		t.Error("Count does not sum the steps")
	}
	// Same result as the direct conversion.
	_, direct := run(t, testconv.New(version.AIXM52, version.AIXM511), src(version.AIXM52))
	if !bytes.Equal(out, direct) {
		t.Errorf("chain output differs from direct conversion\n%s\n%s", out, direct)
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
	if _, err := convert.Chain(b, a); err == nil {
		t.Error("mismatched chain accepted")
	}
	if _, err := convert.Chain(); err == nil {
		t.Error("empty chain accepted")
	}
}

func TestXMLWriter(t *testing.T) {
	var buf bytes.Buffer
	w := convert.XMLWriter(&buf)
	h := convert.Header{Version: version.AIXM511, GMLID: `a"b`, Attrs: map[string]string{"origin": "x--y", "effective": "2020-01-01"}}
	if err := w.Begin(h); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(convert.Record{Kind: "X", XML: []byte(`<?xml version="1.0"?><X/>`)}); err == nil {
		t.Error("record with XML declaration accepted")
	}
	if err := w.Put(convert.Record{Kind: "X", XML: []byte(" <X xmlns=\"" + xmlnode.NSAIXM + "\"/>\n")}); err != nil {
		t.Fatal(err)
	}
	if err := w.End(); err != nil {
		t.Fatal(err)
	}
	want := `<?xml version="1.0" encoding="UTF-8"?>
<!-- source effective="2020-01-01" origin="x- -y" -->
<message:AIXMBasicMessage xmlns:message="http://www.aixm.aero/schema/5.1.1/message" xmlns:aixm="http://www.aixm.aero/schema/5.1.1" xmlns:gml="http://www.opengis.net/gml/3.2" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" gml:id="a&#34;b">
<message:hasMember><X xmlns="http://www.aixm.aero/schema/5.1.1"/></message:hasMember>
</message:AIXMBasicMessage>
`
	if buf.String() != want {
		t.Errorf("got\n%s\nwant\n%s", buf.String(), want)
	}
	if err := convert.XMLWriter(&buf).Begin(convert.Header{Version: version.AIXM45}); err == nil {
		t.Error("4.5 output accepted")
	}
}

func TestSpool(t *testing.T) {
	dir := t.TempDir()
	s, cleanup, err := convert.Spool(strings.NewReader("hello"), dir, 5)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // re-openable
		rc, err := s.Open()
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		b.ReadFrom(rc)
		rc.Close()
		if b.String() != "hello" {
			t.Errorf("read %q", b.String())
		}
	}
	cleanup()
	cleanup()
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Errorf("spool file left: %v", left)
	}
	if _, _, err := convert.Spool(strings.NewReader("hello!"), dir, 5); !errors.Is(err, convert.ErrTooLarge) {
		t.Errorf("err = %v", err)
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Errorf("spool file left after error: %v", left)
	}
}

func TestIssues(t *testing.T) {
	r := convert.NewReport()
	r.AddIssue(convert.Issue{Code: "b", Severity: convert.Error, Key: "1"})
	r.AddIssue(convert.Issue{Code: "b", Severity: convert.Error, Key: "1", Count: 2})
	for i := 0; i < convert.MaxIssuesPerCode+10; i++ {
		r.AddIssue(convert.Issue{Code: "a", Severity: convert.Warning, Key: strings.Repeat("k", i+1)})
	}
	if r.IssueCount("") != 3+convert.MaxIssuesPerCode+10 {
		t.Errorf("counts not exact: %d", r.IssueCount(""))
	}
	if r.IssueCount(convert.Error) != 3 {
		t.Errorf("errors = %d", r.IssueCount(convert.Error))
	}
	if len(r.Issues) != 1+convert.MaxIssuesPerCode+1 {
		t.Errorf("entries = %d", len(r.Issues))
	}
}

func TestSamplesCapped(t *testing.T) {
	r := convert.NewReport()
	k := r.Kind("K")
	for i := 0; i < convert.MaxSamples+3; i++ {
		k.Record("K/x", convert.Lossy, "r1", "T/x", &convert.Sample{Value: "v"})
	}
	e := k.Elements["K/x"]
	if len(e.Samples) != convert.MaxSamples || e.Outcomes[convert.Lossy] != convert.MaxSamples+3 || e.Rule != "r1" {
		t.Errorf("element stats: %+v", e)
	}
}

func TestWalkLeaves(t *testing.T) {
	n, err := xmlnode.Parse([]byte(`<Ase mid="7"><AseUid><codeType>CTR</codeType><codeId>X</codeId></AseUid><txtName/></Ase>`))
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	convert.WalkLeaves(n, func(l convert.Leaf) { paths = append(paths, l.Path+"="+l.Value) })
	want := "Ase/@mid=7 Ase/AseUid/codeType=CTR Ase/AseUid/codeId=X Ase/txtName="
	if got := strings.Join(paths, " "); got != want {
		t.Errorf("leaves %q, want %q", got, want)
	}
	if convert.CountLeaves(n) != 4 {
		t.Error("CountLeaves")
	}
}

func TestUUID(t *testing.T) {
	dns, err := convert.ParseUUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // RFC 4122 DNS namespace
	if err != nil {
		t.Fatal(err)
	}
	if got := convert.NewV5(dns, "www.example.com").String(); got != "2ed6657d-e927-568b-95e1-2665a8aea6a2" {
		t.Errorf("NewV5 = %s", got)
	}
	url, _ := convert.ParseUUID("urn:uuid:6BA7B811-9DAD-11D1-80B4-00C04FD430C8")
	if got := convert.NewV5(url, "aixm45:test").String(); got != "2828f2a3-9a74-53c0-b2b8-c180743cd957" {
		t.Errorf("NewV5 = %s", got)
	}
	for _, bad := range []string{"", "6ba7b810-9dad-11d1-80b4-00c04fd430c", "6ba7b810x9dad-11d1-80b4-00c04fd430c8", "zba7b810-9dad-11d1-80b4-00c04fd430c8"} {
		if _, err := convert.ParseUUID(bad); err == nil {
			t.Errorf("ParseUUID(%q) accepted", bad)
		}
	}
	b, _ := json.Marshal(dns)
	var back convert.UUID
	if err := json.Unmarshal(b, &back); err != nil || back != dns {
		t.Errorf("JSON round trip: %s %v", b, err)
	}
}

func TestNewID(t *testing.T) {
	a := convert.NewID("n", "1.0.0", []byte("ab"), []byte("c"))
	b := convert.NewID("n", "1.0.0", []byte("a"), []byte("bc"))
	if a.MappingDigest == b.MappingDigest {
		t.Error("table boundaries not part of the digest")
	}
	if a.Module == "" {
		t.Error("module version empty")
	}
}
