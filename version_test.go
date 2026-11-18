package goaixm_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/geo"
	"github.com/davidkohl/goaixm/internal/testconv"
	"github.com/davidkohl/goaixm/version"
)

// snapshot45 is a minimal synthetic AIXM 4.5 document.
const snapshot45 = `<?xml version="1.0" encoding="UTF-8"?>
<AIXM-Snapshot version="4.5" origin="test" created="2020-01-01T00:00:00Z" effective="2020-01-02T00:00:00Z">
  <Dpn><DpnUid><codeId>ALPHA</codeId><geoLat>450000.00N</geoLat><geoLong>0010000.00E</geoLong></DpnUid><codeType>ICAO</codeType></Dpn>
</AIXM-Snapshot>`

// relabelled returns DONLON with its AIXM namespaces moved to version v.
func relabelled(t testing.TB, v version.Version) []byte {
	t.Helper()
	requireDonlon(t)
	b, err := os.ReadFile(donlonFile)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.ReplaceAll(b, []byte(version.NamespaceURI(version.AIXM511)), []byte(version.NamespaceURI(v)))
}

func TestUnsupportedVersion(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "s45.xml")
	if err := os.WriteFile(path, []byte(snapshot45), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func(goaixm.Options) error{
		"Parse": func(o goaixm.Options) error {
			_, err := goaixm.Parse(ctx, strings.NewReader(snapshot45), o)
			return err
		},
		"Stream": func(o goaixm.Options) error {
			return goaixm.Stream(ctx, strings.NewReader(snapshot45), o, func(goaixm.Member) error { return nil })
		},
		"ParseFile": func(o goaixm.Options) error {
			_, err := goaixm.ParseFile(ctx, path, o)
			return err
		},
		"StreamSource": func(o goaixm.Options) error {
			return goaixm.StreamSource(ctx, convert.FileSource(path), o, func(goaixm.Member) error { return nil })
		},
	} {
		// Without converters, and with a registry that has no 4.5 path.
		for _, reg := range []*convert.Registry{nil, convert.NewRegistry(testconv.New(version.AIXM51, version.AIXM511))} {
			if err := call(goaixm.Options{Converters: reg}); !errors.Is(err, goaixm.ErrUnsupportedVersion) {
				t.Errorf("%s (registry %v): err = %v, want ErrUnsupportedVersion", name, reg != nil, err)
			}
		}
	}
	for _, doc := range []string{
		`<AIXM-Snapshot origin="x"/>`, // 4.x without version
		`<AIXM-Update version="4.5"/>`,
		string(relabelledMini(version.AIXM51)),
		string(relabelledMini(version.AIXM52)),
	} {
		if _, err := goaixm.Parse(ctx, strings.NewReader(doc), goaixm.Options{}); !errors.Is(err, goaixm.ErrUnsupportedVersion) {
			t.Errorf("%.40s: err = %v", doc, err)
		}
	}
}

func relabelledMini(v version.Version) []byte {
	ns := version.NamespaceURI(v)
	return []byte(`<message:AIXMBasicMessage xmlns:message="` + ns + `/message" xmlns:aixm="` + ns + `" xmlns:gml="http://www.opengis.net/gml/3.2" gml:id="m"/>`)
}

// TestUnknownRootUnchanged: documents naming no AIXM version keep the
// previous behaviour (an empty message, no error).
func TestUnknownRootUnchanged(t *testing.T) {
	for _, reg := range []*convert.Registry{nil, convert.NewRegistry(testconv.New(version.AIXM51, version.AIXM511))} {
		msg, err := goaixm.Parse(context.Background(), strings.NewReader(`<root><hasMember/></root>`), goaixm.Options{Converters: reg})
		if err != nil || msg.Kind != "root" || len(msg.Members()) != 0 || !msg.Source.Version.IsZero() {
			t.Errorf("got %+v, %v", msg, err)
		}
	}
}

// TestDonlonNativeUnchanged: registering converters changes nothing for
// AIXM 5.1.1 input, through every entry point.
func TestDonlonNativeUnchanged(t *testing.T) {
	ctx := context.Background()
	base := parseDonlon(t, goaixm.Options{KeepRaw: true})
	if base.Source.Version != version.AIXM511 || base.Source.Root != "AIXMBasicMessage" || base.Conversion != nil {
		t.Errorf("source %+v, conversion %v", base.Source, base.Conversion)
	}
	reg := convert.NewRegistry(testconv.New(version.AIXM51, version.AIXM511))
	opts := goaixm.Options{KeepRaw: true, Converters: reg, OnConverted: func(*convert.Report) error {
		t.Error("OnConverted called for native input")
		return nil
	}}
	f, err := os.Open(donlonFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	viaReader, err := goaixm.Parse(ctx, f, opts)
	if err != nil {
		t.Fatal(err)
	}
	viaFile := parseFile(t, donlonFile, opts)
	b, _ := os.ReadFile(donlonFile)
	viaSource, err := goaixm.ParseSource(ctx, convert.BytesSource("donlon", b), opts)
	if err != nil {
		t.Fatal(err)
	}
	for name, msg := range map[string]*goaixm.Message{"Parse": viaReader, "ParseFile": viaFile, "ParseSource": viaSource} {
		if msg.Kind != base.Kind || msg.GMLID != base.GMLID || msg.Conversion != nil {
			t.Errorf("%s: message header differs", name)
		}
		got, want := msg.Members(), base.Members()
		if len(got) != len(want) {
			t.Fatalf("%s: %d members, want %d", name, len(got), len(want))
		}
		for i := range got {
			if got[i].Kind != want[i].Kind || got[i].Index != want[i].Index || !bytes.Equal(got[i].Raw, want[i].Raw) || got[i].Provenance != nil {
				t.Fatalf("%s: member %d differs", name, i)
			}
		}
	}
}

// TestDonlonConverted converts a DONLON copy relabelled to 5.1 back to
// 5.1.1 with the test converter: the members, their trees and the derived
// GeoJSON equal those of the native parse.
func TestDonlonConverted(t *testing.T) {
	ctx := context.Background()
	base := parseDonlon(t, goaixm.Options{KeepRaw: true})
	in := relabelled(t, version.AIXM51)
	two := testconv.New(version.AIXM51, version.AIXM511)
	two.Passes = 2
	reg := convert.NewRegistry(two)

	if _, err := goaixm.Parse(ctx, bytes.NewReader(in), goaixm.Options{}); !errors.Is(err, goaixm.ErrUnsupportedVersion) {
		t.Fatalf("without converters: err = %v", err)
	}

	var reports []*convert.Report
	opts := goaixm.Options{Converters: reg, SpoolDir: t.TempDir(), OnConverted: func(r *convert.Report) error {
		reports = append(reports, r)
		return nil
	}}
	viaReader, err := goaixm.Parse(ctx, bytes.NewReader(in), opts)
	if err != nil {
		t.Fatal(err)
	}
	if left, _ := os.ReadDir(opts.SpoolDir); len(left) != 0 {
		t.Errorf("spool files left: %v", left)
	}
	viaSource, err := goaixm.ParseSource(ctx, convert.BytesSource("donlon-5.1.xml", in), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 2 {
		t.Fatalf("OnConverted called %d times", len(reports))
	}
	for name, msg := range map[string]*goaixm.Message{"Parse": viaReader, "ParseSource": viaSource} {
		compareConverted(t, name, base, msg, version.AIXM51)
	}
	if viaSource.Conversion.Digest() != reports[1].Digest() {
		t.Error("Message.Conversion is not the reported one")
	}
	// The reader path names the spool, the source path the source; the
	// output is the same.
	if reports[0].Output != reports[1].Output {
		t.Errorf("outputs differ: %+v %+v", reports[0].Output, reports[1].Output)
	}

	// Streaming gives the same members in the same order.
	var kinds []string
	err = goaixm.StreamSource(ctx, convert.BytesSource("d", in), goaixm.Options{Converters: reg}, func(m goaixm.Member) error {
		if m.Provenance == nil || len(m.Raw) == 0 {
			t.Fatalf("member %d without provenance or raw", m.Index)
		}
		kinds = append(kinds, m.Kind)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(kinds) != len(base.Members()) {
		t.Errorf("streamed %d members", len(kinds))
	}
}

func compareConverted(t *testing.T, name string, base, msg *goaixm.Message, from version.Version) {
	t.Helper()
	if msg.Source.Version != from || msg.Kind != "AIXMBasicMessage" || msg.GMLID != base.GMLID {
		t.Errorf("%s: source %+v kind %q id %q", name, msg.Source, msg.Kind, msg.GMLID)
	}
	rep := msg.Conversion
	if rep == nil || rep.Check() != nil || rep.Output.Records != len(base.Members()) {
		t.Fatalf("%s: report %+v", name, rep)
	}
	if len(msg.Warnings()) != 0 {
		t.Errorf("%s: warnings %v", name, msg.Warnings())
	}
	got, want := msg.Members(), base.Members()
	if len(got) != len(want) {
		t.Fatalf("%s: %d members, want %d", name, len(got), len(want))
	}
	for i := range got {
		g, w := got[i], want[i]
		if g.Kind != w.Kind || g.Index != i || g.Feature.Common().UUID() != w.Feature.Common().UUID() {
			t.Fatalf("%s: member %d: %s %s, want %s %s", name, i, g.Kind, g.Feature.Common().UUID(), w.Kind, w.Feature.Common().UUID())
		}
		if g.Provenance == nil || g.Provenance.SourceVersion != from || g.Provenance.SourceKind != w.Kind {
			t.Fatalf("%s: member %d provenance %+v", name, i, g.Provenance)
		}
		gn, err1 := g.Node()
		wn, err2 := w.Node()
		if err1 != nil || err2 != nil {
			t.Fatal(err1, err2)
		}
		gj, _ := json.Marshal(gn)
		wj, _ := json.Marshal(wn)
		if !bytes.Equal(gj, wj) {
			t.Fatalf("%s: member %d (%s): element tree differs", name, i, g.Kind)
		}
	}
	// Consumers of the decoded model work unchanged.
	ga, wa := goaixm.FeaturesOf[*aixm.Airspace](msg), goaixm.FeaturesOf[*aixm.Airspace](base)
	if len(ga) != 20 || len(ga) != len(wa) {
		t.Fatalf("%s: %d airspaces", name, len(ga))
	}
	for i := range ga {
		gf, gerr := geo.AirspaceToGeoJSONWithRefs(ga[i], msg)
		wf, werr := geo.AirspaceToGeoJSONWithRefs(wa[i], base)
		if (gerr == nil) != (werr == nil) {
			t.Fatalf("%s: airspace %d: errors %v / %v", name, i, gerr, werr)
		}
		gj, _ := geo.MarshalGeoJSON(gf)
		wj, _ := geo.MarshalGeoJSON(wf)
		if !bytes.Equal(gj, wj) {
			t.Errorf("%s: airspace %s: GeoJSON differs", name, ga[i].UUID())
		}
	}
}

// TestDonlonChain converts DONLON relabelled to 5.2 through 5.2 > 5.1 >
// 5.1.1.
func TestDonlonChain(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	base := parseDonlon(t, goaixm.Options{KeepRaw: true})
	reg := convert.NewRegistry(testconv.New(version.AIXM52, version.AIXM51), testconv.New(version.AIXM51, version.AIXM511))
	msg, err := goaixm.ParseSource(context.Background(), convert.BytesSource("d52", relabelled(t, version.AIXM52)), goaixm.Options{Converters: reg})
	if err != nil {
		t.Fatal(err)
	}
	compareConverted(t, "chain", base, msg, version.AIXM52)
	if len(msg.Conversion.Steps) != 2 {
		t.Errorf("steps = %d", len(msg.Conversion.Steps))
	}
	// Provenance names the original source, not the intermediate.
	for i, m := range msg.Members() {
		if p := m.Provenance; p.SourceVersion != version.AIXM52 || len(p.SourceIndex) != 1 || p.SourceIndex[0] != i {
			t.Fatalf("member %d provenance %+v", i, p)
		}
	}
}

func TestConvertedMaxBytes(t *testing.T) {
	ctx := context.Background()
	in := relabelled(t, version.AIXM51)
	opts := goaixm.Options{Converters: convert.NewRegistry(testconv.New(version.AIXM51, version.AIXM511)), MaxBytes: int64(len(in) - 1), SpoolDir: t.TempDir()}
	if _, err := goaixm.Parse(ctx, bytes.NewReader(in), opts); !errors.Is(err, goaixm.ErrTooLarge) {
		t.Errorf("Parse: err = %v", err)
	}
	if _, err := goaixm.ParseSource(ctx, convert.BytesSource("d", in), opts); !errors.Is(err, goaixm.ErrTooLarge) {
		t.Errorf("ParseSource: err = %v", err)
	}
	opts.MaxBytes = int64(len(in))
	if _, err := goaixm.ParseSource(ctx, convert.BytesSource("d", in), opts); err != nil {
		t.Errorf("exact limit: %v", err)
	}
}

func TestConvertedCallbackErrors(t *testing.T) {
	ctx := context.Background()
	in := relabelled(t, version.AIXM51)
	reg := convert.NewRegistry(testconv.New(version.AIXM51, version.AIXM511))
	stop := errors.New("stop")
	n := 0
	err := goaixm.StreamSource(ctx, convert.BytesSource("d", in), goaixm.Options{Converters: reg}, func(goaixm.Member) error {
		n++
		if n == 3 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) || n != 3 {
		t.Errorf("fn error: %v after %d", err, n)
	}
	_, err = goaixm.ParseSource(ctx, convert.BytesSource("d", in), goaixm.Options{Converters: reg, OnConverted: func(*convert.Report) error { return stop }})
	if !errors.Is(err, stop) {
		t.Errorf("OnConverted error: %v", err)
	}
}

func TestMemberSink(t *testing.T) {
	var got []goaixm.Member
	s := goaixm.MemberSink(goaixm.Options{Fallback: goaixm.FallbackGeneric}, func(m goaixm.Member) error {
		got = append(got, m)
		return nil
	})
	if err := s.Begin(convert.Header{Version: version.AIXM51}); !errors.Is(err, goaixm.ErrUnsupportedVersion) {
		t.Errorf("Begin 5.1: %v", err)
	}
	if err := s.Begin(convert.Header{Version: version.AIXM511}); err != nil {
		t.Fatal(err)
	}
	rec := convert.Record{Kind: "DesignatedPoint", XML: []byte(`<DesignatedPoint xmlns="http://www.aixm.aero/schema/5.1.1"><identifier xmlns="http://www.opengis.net/gml/3.2">00000000-0000-4000-8000-000000000001</identifier></DesignatedPoint>`),
		Provenance: convert.Provenance{SourceVersion: version.AIXM45, SourceKind: "Dpn", SourceKey: "Dpn(codeId=A)", SourceIndex: []int{4}}}
	if err := s.Put(rec); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(convert.Record{Kind: "Unknownish", XML: []byte(`<Unknownish/>`)}); err != nil {
		t.Fatal(err) // generic fallback
	}
	if err := s.Put(convert.Record{Kind: "Navaid", XML: []byte(`<DesignatedPoint/>`)}); err == nil {
		t.Error("kind mismatch accepted")
	}
	if err := s.End(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Feature.Common().UUID() != "00000000-0000-4000-8000-000000000001" || got[0].Provenance.SourceKey != "Dpn(codeId=A)" || len(got[0].Raw) == 0 {
		t.Fatalf("members %+v", got)
	}
	if got[1].Index != 1 || got[1].Kind != "Unknownish" {
		t.Errorf("generic member %+v", got[1])
	}
}

// FuzzParseConverted verifies that Parse with converters never panics.
// Run with: go test -fuzz=FuzzParseConverted .
func FuzzParseConverted(f *testing.F) {
	f.Add(relabelledMini(version.AIXM51))
	f.Add([]byte(snapshot45))
	f.Add([]byte(`<message:AIXMBasicMessage xmlns:message="http://www.aixm.aero/schema/5.1/message"><message:hasMember><x/></message:hasMember></message:AIXMBasicMessage>`))
	reg := convert.NewRegistry(testconv.New(version.AIXM51, version.AIXM511), testconv.New(version.AIXM52, version.AIXM51))
	f.Fuzz(func(t *testing.T, data []byte) {
		msg, err := goaixm.Parse(context.Background(), bytes.NewReader(data), goaixm.Options{Converters: reg, Fallback: goaixm.FallbackGeneric, SpoolDir: t.TempDir()})
		if err == nil && msg.Conversion != nil && msg.Conversion.Check() != nil {
			t.Fatalf("accounting: %v", msg.Conversion.Check())
		}
	})
}
