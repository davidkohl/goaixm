package version_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/davidkohl/goaixm/version"
)

func TestParseAndString(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want version.Version
		str  string
	}{
		{"4.5", version.AIXM45, "4.5"},
		{"5.1", version.AIXM51, "5.1"},
		{"5.1.1", version.AIXM511, "5.1.1"},
		{" 5.2 ", version.AIXM52, "5.2"},
		{"5.1.0", version.AIXM51, "5.1"},
	} {
		got, err := version.Parse(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("Parse(%q) = %v, %v; want %v", tc.in, got, err, tc.want)
		}
		if got.String() != tc.str {
			t.Errorf("String(%v) = %q, want %q", got, got.String(), tc.str)
		}
	}
	for _, bad := range []string{"", "5", "5.", "a.b", "5.1.1.1", "-1.2", "5.+1", "99999.1"} {
		if _, err := version.Parse(bad); err == nil {
			t.Errorf("Parse(%q): expected error", bad)
		}
	}
	if (version.Version{}).String() != "unknown" {
		t.Error("zero version should format as unknown")
	}
}

func TestCompare(t *testing.T) {
	order := []version.Version{version.AIXM40, version.AIXM45, version.AIXM51, version.AIXM511, version.AIXM52}
	for i := range order {
		for j := range order {
			if got, want := order[i].Less(order[j]), i < j; got != want {
				t.Errorf("%v.Less(%v) = %v", order[i], order[j], got)
			}
		}
	}
}

func TestJSON(t *testing.T) {
	b, err := json.Marshal(struct{ V version.Version }{version.AIXM511})
	if err != nil || string(b) != `{"V":"5.1.1"}` {
		t.Fatalf("marshal: %s %v", b, err)
	}
	var back struct{ V version.Version }
	if err := json.Unmarshal(b, &back); err != nil || back.V != version.AIXM511 {
		t.Fatalf("unmarshal: %v %v", back, err)
	}
}

func TestDetect(t *testing.T) {
	for _, tc := range []struct {
		name, doc string
		want      version.Version
		root      string
		err       bool
	}{
		{
			name: "aixm 4.5 snapshot",
			doc:  `<?xml version="1.0"?><!-- c --><AIXM-Snapshot version="4.5" origin="test" effective="2020-01-01T00:00:00Z"><Ase/></AIXM-Snapshot>`,
			want: version.AIXM45, root: "AIXM-Snapshot",
		},
		{
			name: "aixm 4.5 update",
			doc:  `<AIXM-Update version="4.5"/>`,
			want: version.AIXM45, root: "AIXM-Update",
		},
		{
			name: "aixm 4.x without version",
			doc:  `<AIXM-Snapshot origin="x"/>`,
			root: "AIXM-Snapshot", err: true,
		},
		{
			name: "basic message 5.1.1",
			doc: `<message:AIXMBasicMessage xmlns:message="http://www.aixm.aero/schema/5.1.1/message"
				xmlns:gml="http://www.opengis.net/gml/3.2" gml:id="m"/>`,
			want: version.AIXM511, root: "AIXMBasicMessage",
		},
		{
			name: "basic message 5.1",
			doc:  `<AIXMBasicMessage xmlns="http://www.aixm.aero/schema/5.1/message"/>`,
			want: version.AIXM51, root: "AIXMBasicMessage",
		},
		{
			name: "basic message 5.2",
			doc:  `<m:AIXMBasicMessage xmlns:m="http://www.aixm.aero/schema/5.2/message"/>`,
			want: version.AIXM52, root: "AIXMBasicMessage",
		},
		{
			name: "adr message decided by aixm prefix",
			doc: `<adrmsg:ADRMessage xmlns:adrmsg="http://www.eurocontrol.int/cfmu/b2b/ADRMessage"
				xmlns:adr="http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR"
				xmlns:aixm="http://www.aixm.aero/schema/5.1.1"/>`,
			want: version.AIXM511, root: "ADRMessage",
		},
		{
			name: "aixm prefix wins over other declarations",
			doc: `<x:Root xmlns:x="urn:other" xmlns:old="http://www.aixm.aero/schema/5.1"
				xmlns:aixm="http://www.aixm.aero/schema/5.1.1"/>`,
			want: version.AIXM511, root: "Root",
		},
		{
			name: "extension namespace alone is not a version",
			doc:  `<x:Root xmlns:x="urn:other" xmlns:adr="http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR"/>`,
			root: "Root", err: true,
		},
		{name: "unknown root", doc: `<html><body/></html>`, root: "html", err: true},
		{name: "empty", doc: ``, err: true},
		{name: "garbage", doc: `<<<`, err: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := version.Detect(strings.NewReader(tc.doc))
			if tc.err {
				if !errors.Is(err, version.ErrUnknownVersion) {
					t.Fatalf("err = %v, want ErrUnknownVersion", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if d.Version != tc.want || d.Root != tc.root {
				t.Errorf("got %v %q, want %v %q", d.Version, d.Root, tc.want, tc.root)
			}
		})
	}
}

func TestDetectAttrs(t *testing.T) {
	d, err := version.Detect(strings.NewReader(`<AIXM-Snapshot xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" version="4.5" origin="test" created="2020-01-01" effective="2020-02-01T00:00:00+01:00"/>`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"version": "4.5", "origin": "test", "created": "2020-01-01", "effective": "2020-02-01T00:00:00+01:00"}
	if len(d.Attrs) != len(want) {
		t.Fatalf("attrs = %v", d.Attrs)
	}
	for k, v := range want {
		if d.Attrs[k] != v {
			t.Errorf("attr %s = %q, want %q", k, d.Attrs[k], v)
		}
	}
	if d.Namespace != "" {
		t.Errorf("namespace = %q", d.Namespace)
	}
}

func TestDetectDonlon(t *testing.T) {
	d, err := version.DetectFile("../testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		if strings.Contains(err.Error(), "no such file") {
			t.Skip("DONLON reference data missing")
		}
		t.Fatal(err)
	}
	if d.Version != version.AIXM511 || d.Root != "AIXMBasicMessage" {
		t.Errorf("got %+v", d)
	}
}

func TestNamespace(t *testing.T) {
	if v, ok := version.Namespace(version.MessageNamespaceURI(version.AIXM511)); !ok || v != version.AIXM511 {
		t.Errorf("message namespace: %v %v", v, ok)
	}
	if v, ok := version.Namespace(version.NamespaceURI(version.AIXM51)); !ok || v != version.AIXM51 {
		t.Errorf("namespace: %v %v", v, ok)
	}
	for _, bad := range []string{"", "http://www.aixm.aero/schema/", "http://www.aixm.aero/schema/4.5", "http://example.com/5.1.1"} {
		if _, ok := version.Namespace(bad); ok {
			t.Errorf("Namespace(%q) accepted", bad)
		}
	}
}

// FuzzDetect verifies that Detect never panics and that every success
// carries a non-zero version and a root name.
// Run with: go test -fuzz=FuzzDetect ./version
func FuzzDetect(f *testing.F) {
	f.Add([]byte(`<AIXM-Snapshot version="4.5"/>`))
	f.Add([]byte(`<message:AIXMBasicMessage xmlns:message="http://www.aixm.aero/schema/5.1.1/message"/>`))
	f.Add([]byte(`<a:ADRMessage xmlns:a="urn:x" xmlns:aixm="http://www.aixm.aero/schema/5.1.1"/>`))
	f.Add([]byte(`<?xml version="1.0"?><!DOCTYPE x><x/>`))
	f.Add([]byte(``))
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := version.Detect(strings.NewReader(string(data)))
		if err == nil && (d.Version.IsZero() || d.Root == "") {
			t.Fatalf("success without version or root: %+v", d)
		}
		if err != nil && !errors.Is(err, version.ErrUnknownVersion) {
			t.Fatalf("error does not wrap ErrUnknownVersion: %v", err)
		}
	})
}
