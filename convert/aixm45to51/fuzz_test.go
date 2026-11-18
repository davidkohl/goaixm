package aixm45to51_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/convert/aixm45to51"
	"github.com/davidkohl/goaixm/geo"
)

// FuzzConvert verifies that the converter never panics on arbitrary input,
// that a successful conversion satisfies the accounting invariant (Run
// checks it) and that its output decodes as AIXM 5.1.1 without warnings.
// Run with: go test -fuzz=FuzzConvert ./convert/aixm45to51
func FuzzConvert(f *testing.F) {
	for _, name := range []string{"basic.xml", "edge.xml", "airspace.xml", "donlon.xml"} {
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
	benchConvert(b, largeFixture(40000), aixm45to51.Config{Namespace: ns})
}

// BenchmarkConvertAirspaces converts a generated snapshot with the element
// counts of a national 4.5 data set, written compactly (13.5 MB; the
// national file they mimic has 44 MB with indentation and kinds not yet
// converted): 14,000 obstacles and 3,500 designated points (largeFixture)
// plus 5,000 airspaces with every vertex type, circles and derivations,
// and 40 borders of 450 vertices (largeAirspaces).  The variants run the
// ring check and Reference mode.  Besides time and allocations it reports
// the peak heap in use (sampled every 20 ms).
func BenchmarkConvertAirspaces(b *testing.B) {
	doc := mixedFixture(14000, 5000)
	for _, c := range []struct {
		name string
		cfg  aixm45to51.Config
	}{
		{"inline", aixm45to51.Config{Namespace: ns}},
		{"inline+ringcheck", aixm45to51.Config{Namespace: ns, RingCheck: ringCheck}},
		{"reference", aixm45to51.Config{Namespace: ns, BorderMode: aixm45to51.Reference}},
	} {
		b.Run(c.name, func(b *testing.B) { benchConvert(b, doc, c.cfg) })
	}
}

func benchConvert(b *testing.B, doc []byte, cfg aixm45to51.Config) {
	c := aixm45to51.New(cfg)
	b.SetBytes(int64(len(doc)))
	runtime.GC()
	var peak atomic.Uint64
	done := make(chan struct{})
	go func() {
		var m runtime.MemStats
		t := time.NewTicker(20 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				runtime.ReadMemStats(&m)
				if m.HeapInuse > peak.Load() {
					peak.Store(m.HeapInuse)
				}
			}
		}
	}()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := convert.Run(context.Background(), c, convert.BytesSource("bench.xml", doc), convert.XMLWriter(io.Discard)); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	close(done)
	b.ReportMetric(float64(peak.Load())/(1<<20), "peak-heap-MB")
	b.ReportMetric(float64(len(doc))/(1<<20), "input-MB")
}

// mixedFixture is largeFixture(obstacles) with largeAirspaces(airspaces)
// before its end.
func mixedFixture(obstacles, airspaces int) []byte {
	doc := largeFixture(obstacles)
	end := bytes.LastIndex(doc, []byte("</AIXM-Snapshot>"))
	return append(append(append([]byte(nil), doc[:end]...), largeAirspaces(airspaces)...), doc[end:]...)
}

// dms renders a latitude or longitude in the 4.5 layout DDMMSS.ssH
// (DDDMMSS.ssH), as most producers write them: 0.01" (0.3 m) resolution,
// so computed arc ends land a little off their circle, as in real data.
func dms(v float64, lat bool) string {
	h := "N"
	if !lat {
		h = "E"
	}
	if v < 0 {
		v = -v
		if lat {
			h = "S"
		} else {
			h = "W"
		}
	}
	cs := int64(math.Round(v * 360000))
	d, m, sec := cs/360000, cs%360000/6000, cs%6000
	if lat {
		return fmt.Sprintf("%02d%02d%02d.%02d%s", d, m, sec/100, sec%100, h)
	}
	return fmt.Sprintf("%03d%02d%02d.%02d%s", d, m, sec/100, sec%100, h)
}

// largeAirspaces generates n airspaces (Ase with Abd or Adg) and n/125
// borders (Gbr of 450 vertices) in a grid over 40-55N, 0-20E, with the
// vertex mix of a national data set: mostly great circles, some rhumb
// lines, clockwise and counter-clockwise arcs, border stretches, circles
// and unions of two earlier airspaces.
func largeAirspaces(n int) string {
	var b strings.Builder
	nb := max(1, n/125)
	type pt struct{ lat, lon float64 }
	borders := make([][]pt, nb)
	for k := range borders {
		lat0 := 40 + float64(k%15)
		lon0 := float64(k/15) * 6
		fmt.Fprintf(&b, `<Gbr><GbrUid><txtName>BENCH BORDER %d</txtName></GbrUid><codeType>ST</codeType>`, k)
		for j := 0; j < 450; j++ {
			p := pt{lat0 + 0.02*math.Sin(float64(j)/7), lon0 + float64(j)*0.01}
			borders[k] = append(borders[k], p)
			typ := "GRC"
			if j == 449 {
				typ = "END"
			} else if j%50 == 0 {
				typ = "RHL"
			}
			fmt.Fprintf(&b, `<Gbv><codeType>%s</codeType><geoLat>%s</geoLat><geoLong>%s</geoLong><codeDatum>WGE</codeDatum></Gbv>`, typ, dms(p.lat, true), dms(p.lon, false))
		}
		b.WriteString("</Gbr>\n")
	}
	vertex := func(typ string, p pt, extra string) string {
		return fmt.Sprintf(`<Avx><codeType>%s</codeType><geoLat>%s</geoLat><geoLong>%s</geoLong><codeDatum>WGE</codeDatum>%s</Avx>`, typ, dms(p.lat, true), dms(p.lon, false), extra)
	}
	at := func(c pt, brg, r float64) pt {
		la, lo := geo.DestinationPoint(c.lat, c.lon, brg, r)
		return pt{la, lo}
	}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("XB%05d", i)
		fmt.Fprintf(&b, `<Ase><AseUid mid="%d"><codeType>R</codeType><codeId>%s</codeId></AseUid><txtName>BENCH AREA %d</txtName>`+
			`<codeDistVerUpper>STD</codeDistVerUpper><valDistVerUpper>%03d</valDistVerUpper><uomDistVerUpper>FL</uomDistVerUpper>`+
			`<codeDistVerLower>HEI</codeDistVerLower><valDistVerLower>0</valDistVerLower><uomDistVerLower>FT</uomDistVerLower>`, 5000000+i, id, i, 65+i%300)
		if i%3 == 0 {
			b.WriteString(`<Att><codeWorkHr>H24</codeWorkHr></Att>`)
		}
		b.WriteString("</Ase>\n")
		uid := fmt.Sprintf(`<AseUid mid="%d"><codeType>R</codeType><codeId>%s</codeId></AseUid>`, 5000000+i, id)
		c := pt{40.5 + float64(i%60)*0.24, 0.5 + float64(i/60%80)*0.24}
		switch {
		case i%8 == 7:
			fmt.Fprintf(&b, `<Adg><AdgUid>%s</AdgUid><AseUidBase><codeType>R</codeType><codeId>XB%05d</codeId></AseUidBase>`+
				`<codeOpr>UNION</codeOpr><AseUidComponent><codeType>R</codeType><codeId>XB%05d</codeId></AseUidComponent></Adg>`+"\n", uid, i-1, i-2)
		case i%27 == 0:
			fmt.Fprintf(&b, `<Abd><AbdUid>%s</AbdUid><Circle><geoLatCen>%s</geoLatCen><geoLongCen>%s</geoLongCen><codeDatum>WGE</codeDatum><valRadius>%d</valRadius><uomRadius>NM</uomRadius></Circle></Abd>`+"\n",
				uid, dms(c.lat, true), dms(c.lon, false), 3+i%10)
		case i%11 == 0:
			// A stretch of a border: from border vertex k to k+20, then
			// back south of it.
			bd := borders[i%nb]
			k := i % 400
			fmt.Fprintf(&b, `<Abd><AbdUid>%s</AbdUid>`, uid)
			b.WriteString(vertex("FNT", bd[k], fmt.Sprintf(`<GbrUid><txtName>BENCH BORDER %d</txtName></GbrUid>`, i%nb)))
			b.WriteString(vertex("GRC", bd[k+20], ""))
			b.WriteString(vertex("GRC", pt{bd[k+20].lat - 0.1, bd[k+20].lon}, ""))
			b.WriteString(vertex("GRC", pt{bd[k].lat - 0.1, bd[k].lon}, ""))
			b.WriteString("</Abd>\n")
		default:
			// A sector of about 20 km: four or five corners, some edges
			// rhumb lines, some arcs about the centre.
			r := 9000.0 + float64(i%7)*500
			fmt.Fprintf(&b, `<Abd><AbdUid>%s</AbdUid>`, uid)
			brgs := []float64{20, 110, 200, 290}
			if i%2 == 0 {
				brgs = []float64{0, 70, 150, 220, 300}
			}
			// Every 25th ring runs counter-clockwise, with a
			// counter-clockwise arc.
			ccw := i%25 == 0
			if ccw {
				for l, r := 0, len(brgs)-1; l < r; l, r = l+1, r-1 {
					brgs[l], brgs[r] = brgs[r], brgs[l]
				}
			}
			for j, brg := range brgs {
				p := at(c, brg, r)
				typ, extra := "GRC", ""
				switch {
				case j == 1 && ccw:
					typ = "CCA"
				case j == 1 && i%13 == 0:
					typ = "CWA"
				case j == 0 && i%5 == 0:
					typ = "RHL"
				}
				if typ == "CWA" || typ == "CCA" {
					extra = fmt.Sprintf(`<geoLatArc>%s</geoLatArc><geoLongArc>%s</geoLongArc><valRadiusArc>%.1f</valRadiusArc><uomRadiusArc>KM</uomRadiusArc>`,
						dms(c.lat, true), dms(c.lon, false), r/1000)
				}
				b.WriteString(vertex(typ, p, extra))
			}
			b.WriteString("</Abd>\n")
		}
	}
	return b.String()
}

func TestLargeFixture(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	res := convertBytes(t, largeFixture(4000), aixm45to51.Config{})
	if res.rep.Targets["VerticalStructure"] != 4000 || res.rep.Targets["DesignatedPoint"] != 1000 || res.rep.Targets["Airspace"] != 20 {
		t.Errorf("targets %v", res.rep.Targets)
	}
}

// TestLargeAirspaces: the generated airspaces of the benchmark convert
// with every vertex type in use, every ring valid, and the issues the
// 0.01" source resolution gives (arc ends snapped) and nothing else (the
// CTRs of largeFixture have no geometry).
func TestLargeAirspaces(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}
	res := convertBytes(t, mixedFixture(400, 1000), aixm45to51.Config{RingCheck: ringCheck})
	if res.rep.Targets["Airspace"] != 1002 || res.rep.Targets["GeoBorder"] != 8 {
		t.Errorf("targets %v", res.rep.Targets)
	}
	k := res.rep.Kinds["Abd"]
	for _, typ := range []string{"GRC", "RHL", "CWA", "CCA", "FNT"} {
		if e := k.Elements["Abd/Avx[codeType="+typ+"]/codeType"]; e == nil {
			t.Errorf("no %s vertex", typ)
		}
	}
	if k.Elements["Abd/Circle/valRadius"] == nil || res.rep.Kinds["Adg"].Merged == 0 {
		t.Error("no circle or derivation")
	}
	for _, i := range res.rep.Issues {
		if i.Code != "arc_end_snapped" && !(i.Code == "geometry_missing" && strings.Contains(i.Key, "codeType=CTR")) {
			t.Errorf("issue %s %s: %s", i.Code, i.Key, i.Message)
		}
	}
}

// FuzzAirspaceGeometry builds boundaries from arbitrary vertex sequences
// (every codeType, arcs of any radius about nearby centres, border
// stretches along a fictional border, duplicate and coincident vertices)
// in both border modes: the conversion never fails or panics, the output
// decodes, and geo converts every airspace without panicking.
// Run with: go test -fuzz=FuzzAirspaceGeometry ./convert/aixm45to51
func FuzzAirspaceGeometry(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0, 0, 0, 255, 0, 0, 0, 0, 255, 255, 0, 0})
	f.Add([]byte{2, 128, 0, 5, 10, 0, 0, 128, 0, 0, 3, 255, 128, 5, 200, 0, 128, 128, 0, 0})
	f.Add([]byte{4, 0, 26, 0, 0, 0, 0, 230, 0, 0, 0, 200, 230, 0, 0, 0, 200, 26, 0, 0})
	f.Add([]byte{1, 0, 0, 0, 0, 1, 255, 0, 0, 0, 1, 255, 255, 0, 0, 1, 0, 255, 0, 0})
	types := []string{"GRC", "RHL", "CWA", "CCA", "FNT", "XYZ"}
	border := `<Gbr><GbrUid><txtName>FUZZ BORDER</txtName></GbrUid><codeType>ST</codeType>` +
		`<Gbv><codeType>GRC</codeType><geoLat>10.5N</geoLat><geoLong>020.0E</geoLong><codeDatum>WGE</codeDatum></Gbv>` +
		`<Gbv><codeType>RHL</codeType><geoLat>10.6N</geoLat><geoLong>020.25E</geoLong><codeDatum>WGE</codeDatum></Gbv>` +
		`<Gbv><codeType>END</codeType><geoLat>10.5N</geoLat><geoLong>020.5E</geoLong><codeDatum>WGE</codeDatum></Gbv></Gbr>`
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 5*64 {
			return
		}
		var b strings.Builder
		b.WriteString(`<AIXM-Snapshot version="4.5" effective="2030-01-24T00:00:00Z">` + border +
			`<Ase><AseUid><codeType>R</codeType><codeId>XF</codeId></AseUid><txtName>F</txtName></Ase><Abd><AbdUid><AseUid><codeType>R</codeType><codeId>XF</codeId></AseUid></AbdUid>`)
		for i := 0; i+5 <= len(data); i += 5 {
			d := data[i : i+5]
			lat, lon := 10+float64(d[1])/255*0.6, 20+float64(d[2])/255*0.6
			fmt.Fprintf(&b, `<Avx><codeType>%s</codeType><geoLat>%.6fN</geoLat><geoLong>%010.6fE</geoLong><codeDatum>WGE</codeDatum>`, types[int(d[0])%len(types)], lat, lon)
			switch types[int(d[0])%len(types)] {
			case "CWA", "CCA":
				fmt.Fprintf(&b, `<geoLatArc>%.6fN</geoLatArc><geoLongArc>%010.6fE</geoLongArc><valRadiusArc>%d</valRadiusArc><uomRadiusArc>KM</uomRadiusArc>`,
					lat+float64(int(d[4])-128)/2000, lon+float64(int(d[3])-128)/2000, 1+int(d[3])%40)
			case "FNT":
				b.WriteString(`<GbrUid><txtName>FUZZ BORDER</txtName></GbrUid>`)
			}
			b.WriteString(`</Avx>`)
		}
		b.WriteString(`</Abd></AIXM-Snapshot>`)
		for _, mode := range []aixm45to51.BorderMode{aixm45to51.Inline, aixm45to51.Reference} {
			var buf bytes.Buffer
			c := aixm45to51.New(aixm45to51.Config{Namespace: ns, BorderMode: mode, RingCheck: ringCheck})
			if _, err := convert.Run(context.Background(), c, convert.BytesSource("fuzz.xml", []byte(b.String())), convert.XMLWriter(&buf)); err != nil {
				t.Fatalf("convert: %v\n%s", err, b.String())
			}
			msg, err := goaixm.Parse(context.Background(), bytes.NewReader(buf.Bytes()), goaixm.Options{Strict: true})
			if err != nil {
				t.Fatalf("output does not decode: %v\n%s", err, buf.Bytes())
			}
			for _, ft := range msg.ByKind("Airspace") {
				_, _ = geo.Options{ArcTolerance: 5, GeodesicTolerance: 5}.AirspaceToGeoJSONWithRefs(ft.(*aixm.Airspace), msg)
			}
		}
	})
}
