package aixm45to51_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/davidkohl/goaixm/convert/aixm45to51"
	"github.com/davidkohl/goaixm/geo"
)

var ringCheck = &geo.Options{ArcTolerance: 1, GeodesicTolerance: 1}

// boundary renders an Ase with an Abd of GRC vertices ("lat long" in
// decimal degrees).
func boundary(id string, vertices ...[2]float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<Ase><AseUid><codeType>R</codeType><codeId>%s</codeId></AseUid></Ase>`, id)
	fmt.Fprintf(&b, `<Abd><AbdUid><AseUid><codeType>R</codeType><codeId>%s</codeId></AseUid></AbdUid>`, id)
	for _, v := range vertices {
		fmt.Fprintf(&b, `<Avx><codeType>GRC</codeType><geoLat>%s</geoLat><geoLong>%s</geoLong><codeDatum>WGE</codeDatum></Avx>`,
			dd(v[0], "N", "S"), dd(v[1], "E", "W"))
	}
	b.WriteString(`</Abd>`)
	return b.String()
}

// TestRingCheck: Config.RingCheck reports rings that cross or touch
// themselves and spikes, with the place, and leaves the geometry as the
// source gives it; valid rings give nothing, and without RingCheck nothing
// is checked.
func TestRingCheck(t *testing.T) {
	// Edges along the equator and meridians are straight in lon/lat even
	// when densified, so the expected places are exact.
	body := boundary("XOK", [2]float64{0, 20}, [2]float64{0, 21}, [2]float64{1, 21}, [2]float64{1, 20}) +
		// Bow tie: the edges 1-2 and 3-4 cross near 0.5 20.5 (the
		// diagonals are geodesics, densified: the crossing is 5.7 m north).
		boundary("XBOW", [2]float64{0, 20}, [2]float64{1, 21}, [2]float64{0, 21}, [2]float64{1, 20}) +
		// Spike: vertex 2 (0 22) runs out along the equator and back.
		boundary("XSPIKE", [2]float64{0, 20}, [2]float64{0, 22}, [2]float64{0, 21}, [2]float64{1, 21}, [2]float64{1, 20}) +
		// Figure eight: vertex 3 and 6 are one position.
		boundary("XEIGHT", [2]float64{0, 20}, [2]float64{2, 20}, [2]float64{1, 21}, [2]float64{2, 22}, [2]float64{0, 22}, [2]float64{1, 21})
	in := snapshot(body)

	off := convertBytes(t, in, aixm45to51.Config{})
	if n := len(issues(off.rep, "ring_invalid")); n != 0 {
		t.Errorf("without RingCheck: %d ring_invalid issues", n)
	}
	res := convertBytes(t, in, aixm45to51.Config{RingCheck: ringCheck})
	got := map[string]string{}
	for _, i := range issues(res.rep, "ring_invalid") {
		got[i.Key] = i.Message
	}
	want := map[string][]string{
		aseKey("R", "XBOW"):   {"edges cross or touch at 0.5000", " 20.500000", "m from boundary vertex", "written as the source gives it"},
		aseKey("R", "XSPIKE"): {"spike", "at 0.000000 22.000000", "0 m from boundary vertex 2", "places in all"},
		aseKey("R", "XEIGHT"): {"touches itself at a repeated position", "at 1.000000 21.000000", "boundary vertex 3"},
	}
	for k, parts := range want {
		for _, p := range parts {
			if !strings.Contains(got[k], p) {
				t.Errorf("%s: %q lacks %q", k, got[k], p)
			}
		}
	}
	if m, ok := got[aseKey("R", "XOK")]; ok || len(got) != len(want) {
		t.Errorf("issues %v (XOK: %q)", got, m)
	}
	// Reported, not repaired: the records are the same with and without
	// the check.
	if string(off.xml) != string(res.xml) {
		t.Error("RingCheck changed the output")
	}
	for k, m := range got {
		t.Logf("%s: %s", k, m)
	}
}

// TestRingCheckFixtures: the rings of the synthetic airspace fixture (both
// border modes) and of the DONLON transcription are valid.
func TestRingCheckFixtures(t *testing.T) {
	for _, c := range []struct {
		name string
		mode aixm45to51.BorderMode
	}{{"airspace.xml", aixm45to51.Inline}, {"airspace.xml", aixm45to51.Reference}, {"donlon.xml", aixm45to51.Inline}, {"donlon.xml", aixm45to51.Reference}} {
		res := convertBytes(t, fixture(t, c.name), aixm45to51.Config{RingCheck: ringCheck, BorderMode: c.mode})
		for _, i := range issues(res.rep, "ring_invalid") {
			t.Errorf("%s (mode %d): %s %s", c.name, c.mode, i.Key, i.Message)
		}
	}
}
