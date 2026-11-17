package aixm45_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"strings"
	"testing"

	"github.com/davidkohl/goaixm/aixm45"
	"github.com/davidkohl/goaixm/xmlnode"
)

const doc = `<?xml version="1.0" encoding="UTF-8"?>
<!-- synthetic -->
<AIXM-Snapshot version="4.5" origin="Example" created="2030-01-01T00:00:00Z" effective="2030-01-24T00:00:00.000+01:00">
  <Org><OrgUid mid="1"><txtName>EXAMPLE AUTHORITY</txtName></OrgUid><codeId>XX</codeId><codeType>S</codeType></Org>
  <Dpn>
    <DpnUid mid="2"><codeId>ALPHA</codeId><geoLat>450000.00N</geoLat><geoLong>0050000.00E</geoLong></DpnUid>
    <AhpUidAssoc><codeId>XA01</codeId></AhpUidAssoc>
    <codeDatum>WGE</codeDatum><codeType>ICAO</codeType><txtName>ALPHA</txtName>
  </Dpn>
  <Rdn><RdnUid><RwyUid><AhpUid><codeId>XA01</codeId></AhpUid><txtDesig>09/27</txtDesig></RwyUid><txtDesig>27</txtDesig></RdnUid><valTrueBrg>270.5</valTrueBrg></Rdn>
</AIXM-Snapshot>
`

func TestReader(t *testing.T) {
	var kinds []string
	h, err := aixm45.Stream(context.Background(), strings.NewReader(doc), func(e aixm45.Element) error {
		kinds = append(kinds, fmt.Sprintf("%d:%s", e.Index, e.Kind()))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.Root != aixm45.RootSnapshot || h.Version != "4.5" || h.Origin != "Example" || h.Effective != "2030-01-24T00:00:00.000+01:00" || len(h.Node.Attrs) != 4 {
		t.Errorf("header %+v", h)
	}
	if got := strings.Join(kinds, " "); got != "0:Org 1:Dpn 2:Rdn" {
		t.Errorf("elements %s", got)
	}
}

func TestReaderErrors(t *testing.T) {
	for _, in := range []string{
		``,
		`<AIXM-Snapshot effective="x"/>`,
		`<AIXM-Snapshot version="5.1"/>`,
		`<AIXMBasicMessage version="4.5"/>`,
		`<x:AIXM-Snapshot xmlns:x="urn:x" version="4.5"/>`,
	} {
		if _, err := aixm45.NewReader(strings.NewReader(in)); !errors.Is(err, aixm45.ErrNotAIXM45) {
			t.Errorf("%q: err %v", in, err)
		}
	}
	r, err := aixm45.NewReader(strings.NewReader(`<AIXM-Update version="4.5"><Org><OrgUid>`))
	if err != nil || r.Header().Root != aixm45.RootUpdate {
		t.Fatal(err)
	}
	if _, err := r.Next(); err == nil || errors.Is(err, io.EOF) {
		t.Errorf("truncated element: %v", err)
	}
	if _, err := r.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("after error: %v", err)
	}
	r, _ = aixm45.NewReader(strings.NewReader(`<AIXM-Snapshot version="4.5"><Org/>`))
	r.Next()
	if _, err := r.Next(); err == nil || errors.Is(err, io.EOF) {
		t.Errorf("missing end of root: %v", err)
	}
	stop := errors.New("stop")
	if _, err := aixm45.Stream(context.Background(), strings.NewReader(doc), func(aixm45.Element) error { return stop }); !errors.Is(err, stop) {
		t.Errorf("callback error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := aixm45.Stream(ctx, strings.NewReader(doc), func(aixm45.Element) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
}

func TestParseCoord(t *testing.T) {
	for _, c := range []struct {
		in    string
		long  bool
		want  string // Decimal(10)
		f     aixm45.Format
		frac  int
		canon string
	}{
		{"450000N", false, "45.0000000000", aixm45.FormatDMS, 0, "45"},
		{"484207.80N", false, "48.7021666667", aixm45.FormatDMS, 2, "48.7021666667"},
		{"484207.8N", false, "48.7021666667", aixm45.FormatDMS, 1, "48.7021666667"},
		{"4842.13N", false, "48.7021666667", aixm45.FormatDM, 2, "48.7021666667"},
		{"48.70216666667N", false, "48.7021666667", aixm45.FormatD, 11, "48.7021666667"},
		{"123015.1234S", false, "-12.5042009444", aixm45.FormatDMS, 4, "-12.5042009444"},
		{"0050000.00E", true, "5.0000000000", aixm45.FormatDMS, 2, "5"},
		{"0012030W", true, "-1.3416666667", aixm45.FormatDMS, 0, "-1.3416666667"},
		{"179.5W", true, "-179.5000000000", aixm45.FormatD, 1, "-179.5"},
		{"00000W", true, "0.0000000000", aixm45.FormatDM, 0, "0"},
		{" 900000.00N ", false, "90.0000000000", aixm45.FormatDMS, 2, "90"},
	} {
		parse := aixm45.ParseLat
		if c.long {
			parse = aixm45.ParseLong
		}
		got, err := parse(c.in)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if got.Decimal(10) != c.want || got.Format != c.f || got.Fraction != c.frac || got.Canonical() != c.canon {
			t.Errorf("%q: %s %v %d %s", c.in, got.Decimal(10), got.Format, got.Fraction, got.Canonical())
		}
	}
	for _, bad := range []string{"", "N", "4N", "450000", "450000E", "4500000N", "456000N", "450060N", "910000N", "1810000E", "45.N", "4a0000N", "450000.1234567890123N", "-450000N"} {
		parse := aixm45.ParseLat
		if strings.HasSuffix(bad, "E") {
			parse = aixm45.ParseLong
		}
		if c, err := parse(bad); !errors.Is(err, aixm45.ErrCoordinate) {
			t.Errorf("%q accepted: %+v", bad, c)
		}
	}
	if _, err := aixm45.ParseLong("0050000N"); err == nil {
		t.Error("latitude hemisphere accepted for a longitude")
	}
}

// TestCoordRoundTrip is the property of section 6 of the design: the
// decimal degrees written with Decimals() fraction digits, put back into the
// source layout, reproduce the source string, for every layout and random
// values.
func TestCoordRoundTrip(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		axis := aixm45.Axis(rnd.Intn(2))
		f := aixm45.Format(rnd.Intn(3))
		frac := rnd.Intn(7)
		s := randomCoord(rnd, axis, f, frac)
		parse := aixm45.ParseLat
		if axis == aixm45.Long {
			parse = aixm45.ParseLong
		}
		c, err := parse(s)
		if err != nil {
			t.Fatalf("%q: %v", s, err)
		}
		if back := c.Layout(f, frac); back != s {
			t.Fatalf("%q: Layout gives %q", s, back)
		}
		dec := c.Decimal(c.Decimals())
		r, _ := new(big.Rat).SetString(dec)
		// Rebuild a coordinate from the decimal output in the D layout and
		// render it in the source layout.
		hemi := map[bool]string{false: "N", true: "S"}
		if axis == aixm45.Long {
			hemi = map[bool]string{false: "E", true: "W"}
		}
		neg := r.Sign() < 0
		abs := new(big.Rat).Abs(r)
		width := 2 + c.Decimals() + 1
		if axis == aixm45.Long {
			width++
		}
		dtext := fmt.Sprintf("%0*s", width, abs.FloatString(c.Decimals())) + hemi[neg]
		d, err := parse(dtext)
		if err != nil {
			t.Fatalf("%q: decimal %q: %v", s, dtext, err)
		}
		if back := d.Layout(f, frac); back != s && !zeroEquivalent(back, s) {
			t.Fatalf("%q: decimal %s gives %q", s, dec, back)
		}
	}
}

// zeroEquivalent accepts N/S or E/W flips of an exact zero.
func zeroEquivalent(a, b string) bool {
	return strings.Trim(a[:len(a)-1], "0.") == "" && a[:len(a)-1] == b[:len(b)-1]
}

func randomCoord(rnd *rand.Rand, axis aixm45.Axis, f aixm45.Format, frac int) string {
	dd, limit := 2, 90
	hemi := []string{"N", "S"}
	if axis == aixm45.Long {
		dd, limit = 3, 180
		hemi = []string{"E", "W"}
	}
	deg := rnd.Intn(limit)
	var b strings.Builder
	fmt.Fprintf(&b, "%0*d", dd, deg)
	if f >= aixm45.FormatDM {
		fmt.Fprintf(&b, "%02d", rnd.Intn(60))
	}
	if f == aixm45.FormatDMS {
		fmt.Fprintf(&b, "%02d", rnd.Intn(60))
	}
	if frac > 0 {
		b.WriteByte('.')
		for i := 0; i < frac; i++ {
			b.WriteByte(byte('0' + rnd.Intn(10)))
		}
	}
	return b.String() + hemi[rnd.Intn(2)]
}

func node(t *testing.T, s string) *xmlnode.Node {
	t.Helper()
	n, err := xmlnode.Parse([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestKey(t *testing.T) {
	rdn := node(t, `<RdnUid mid="9"><txtDesig>27</txtDesig><RwyUid><txtDesig>09/27</txtDesig><AhpUid><codeId>XA01</codeId></AhpUid></RwyUid></RdnUid>`)
	if got := aixm45.Key(rdn); got != "Rdn(RwyUid(AhpUid(codeId=XA01);txtDesig=09/27);txtDesig=27)" {
		t.Errorf("Key = %s", got)
	}
	a := aixm45.Key(node(t, `<DpnUid><codeId>ALPHA</codeId><geoLat>484207.80N</geoLat><geoLong>0021434.70E</geoLong></DpnUid>`))
	b := aixm45.Key(node(t, `<DpnUid mid="x"><geoLong> 0021434.7E</geoLong><codeId>ALPHA</codeId><geoLat>4842.13N</geoLat></DpnUid>`))
	if a != b || a != "Dpn(codeId=ALPHA;geoLat=48.7021666667;geoLong=2.2429722222)" {
		t.Errorf("coordinate keys %s / %s", a, b)
	}
	if got := aixm45.Key(node(t, `<AhpUidAssoc><codeId>XA01</codeId></AhpUidAssoc>`)); got != "Ahp(codeId=XA01)" {
		t.Errorf("reference key %s", got)
	}
	if got := aixm45.Key(node(t, `<UniUid><txtName>A  (B);C=D\E</txtName></UniUid>`)); got != `Uni(txtName=A \(B\)\;C\=D\\E)` {
		t.Errorf("escaped key %s", got)
	}
	if got := aixm45.Key(node(t, `<ObsUid><geoLat>bad</geoLat><geoLong>0050000E</geoLong></ObsUid>`)); got != "Obs(geoLat=bad;geoLong=5)" {
		t.Errorf("bad coordinate kept: %s", got)
	}
	if aixm45.Key(nil) != "" || aixm45.KindOf("codeId") != "" || aixm45.KindOf("Uid") != "" || aixm45.KindOf("AhpUidAssoc") != "Ahp" {
		t.Error("KindOf / nil")
	}
}

func TestViews(t *testing.T) {
	var dpn aixm45.Dpn
	var rdn aixm45.Rdn
	_, err := aixm45.Stream(context.Background(), strings.NewReader(doc), func(e aixm45.Element) error {
		switch e.Kind() {
		case "Dpn":
			dpn = aixm45.NewDpn(e.Node)
		case "Rdn":
			rdn = aixm45.NewRdn(e.Node)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if dpn.CodeID.Text != "ALPHA" || dpn.Uid.MID != "2" || dpn.AhpAssoc.Key != "Ahp(codeId=XA01)" || dpn.Ahp.OK() || !dpn.Position.Datum.OK() || dpn.Remark.OK() {
		t.Errorf("Dpn view %+v", dpn)
	}
	if rdn.Rwy.Key != "Rwy(AhpUid(codeId=XA01);txtDesig=09/27)" || rdn.Rwy.Kind != "Rwy" || rdn.TrueBrg.Text != "270.5" || rdn.Desig.Text != "27" {
		t.Errorf("Rdn view %+v", rdn)
	}
}
