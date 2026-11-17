package aixm45to51_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/convert/aixm45to51"
)

// FuzzConvert verifies that the converter never panics on arbitrary input,
// that a successful conversion satisfies the accounting invariant (Run
// checks it) and that its output decodes as AIXM 5.1.1 without warnings.
// Run with: go test -fuzz=FuzzConvert ./convert/aixm45to51
func FuzzConvert(f *testing.F) {
	for _, name := range []string{"basic.xml", "edge.xml"} {
		b, err := os.ReadFile(filepath.Join(fixtures, name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	f.Add([]byte(`<AIXM-Snapshot version="4.5" effective="2030-01-24T00:00:00Z"><Dpn><DpnUid><codeId>A</codeId><geoLat>10N</geoLat><geoLong>010E</geoLong></DpnUid><codeType>ICAO</codeType></Dpn></AIXM-Snapshot>`))
	f.Add([]byte(`<AIXM-Snapshot version="4.5" effective="2030-01-24T00:00:00Z"><Vor><VorUid><codeId>V</codeId></VorUid></Vor><Dme><DmeUid><codeId>V</codeId></DmeUid><VorUid><codeId>V</codeId></VorUid></Dme></AIXM-Snapshot>`))
	f.Add([]byte(``))
	c := aixm45to51.New(aixm45to51.Config{Namespace: ns})
	f.Fuzz(func(t *testing.T, data []byte) {
		var buf bytes.Buffer
		rep, err := convert.Run(context.Background(), c, convert.BytesSource("fuzz.xml", data), convert.XMLWriter(&buf))
		if err != nil {
			if rep != nil {
				t.Fatalf("accounting: %v", err)
			}
			return
		}
		msg, err := goaixm.Parse(context.Background(), bytes.NewReader(buf.Bytes()), goaixm.Options{Strict: true})
		if err != nil {
			t.Fatalf("output does not decode: %v\n%s", err, buf.Bytes())
		}
		if len(msg.Members()) != rep.Output.Records {
			t.Fatalf("%d members, %d records", len(msg.Members()), rep.Output.Records)
		}
	})
}

// largeFixture generates a synthetic snapshot with n obstacles and n/4
// designated points, plus a few of every other converted kind.
func largeFixture(n int) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<AIXM-Snapshot version="4.5" origin="Benchmark" created="2030-01-01T00:00:00Z" effective="2030-01-24T00:00:00.000+01:00">` + "\n")
	b.WriteString(`<Org><OrgUid mid="1"><txtName>BENCH STATE</txtName></OrgUid><codeId>XX</codeId><codeType>S</codeType></Org>` + "\n")
	for i := 0; i < n; i++ {
		lat, long := fmt.Sprintf("%02d%02d%02d.%02dN", 10+i%40, i/40%60, i%60, i%100), fmt.Sprintf("%03d%02d%02d.%02dE", 20+i/2400%100, i%60, i/60%60, i%97)
		fmt.Fprintf(&b, `<Obs><ObsUid mid="%d"><geoLat>%s</geoLat><geoLong>%s</geoLong></ObsUid><txtName>OBSTACLE %d</txtName><txtDescrType>Mast</txtDescrType><codeGroup>N</codeGroup><codeLgt>Y</codeLgt><txtDescrLgt>Red.</txtDescrLgt><codeDatum>WGE</codeDatum><valElev>%d</valElev><valElevAccuracy>3</valElevAccuracy><valHgt>%d</valHgt><uomDistVer>FT</uomDistVer><valGeoAccuracy>5</valGeoAccuracy><uomGeoAccuracy>M</uomGeoAccuracy><valCrc>01234567</valCrc></Obs>`+"\n",
			1000+i, lat, long, i, 500+i%300, 100+i%200)
		if i%4 == 0 {
			fmt.Fprintf(&b, `<Dpn><DpnUid mid="%d"><codeId>P%04d</codeId><geoLat>%s</geoLat><geoLong>%s</geoLong></DpnUid><codeDatum>WGE</codeDatum><codeType>ICAO</codeType><txtName>P%04d</txtName></Dpn>`+"\n", 900000+i, i/4%10000, lat, long, i/4%10000)
		}
		if i%200 == 0 {
			fmt.Fprintf(&b, `<Ahp><AhpUid><codeId>XB%02d</codeId></AhpUid><OrgUid><txtName>BENCH STATE</txtName></OrgUid><txtName>FIELD %d</txtName><codeType>AD</codeType><geoLat>%s</geoLat><geoLong>%s</geoLong><codeDatum>WGE</codeDatum><valElev>100</valElev><uomDistVer>FT</uomDistVer></Ahp>`+"\n", i/200%100, i, lat, long)
			fmt.Fprintf(&b, `<Ase><AseUid><codeType>CTR</codeType><codeId>XB%02d</codeId></AseUid><txtName>CTR %d</txtName></Ase>`+"\n", i/200%100, i)
		}
	}
	b.WriteString("</AIXM-Snapshot>\n")
	return []byte(b.String())
}

// BenchmarkConvert converts a generated snapshot of about 15 MB (design
// target: 44 MB in under 10 s, peak heap under 200 MB).
func BenchmarkConvert(b *testing.B) {
	doc := largeFixture(40000)
	c := aixm45to51.New(aixm45to51.Config{Namespace: ns})
	b.SetBytes(int64(len(doc)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := convert.Run(context.Background(), c, convert.BytesSource("bench.xml", doc), convert.XMLWriter(io.Discard)); err != nil {
			b.Fatal(err)
		}
	}
}

func TestLargeFixture(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	res := convertBytes(t, largeFixture(4000), aixm45to51.Config{})
	if res.rep.Targets["VerticalStructure"] != 4000 || res.rep.Targets["DesignatedPoint"] != 1000 || res.rep.Kinds["Ase"].Dropped != 20 {
		t.Errorf("targets %v", res.rep.Targets)
	}
}
