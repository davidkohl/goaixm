package geo_test

import (
	"context"
	"math"
	"strconv"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/geo"
	"github.com/davidkohl/goaixm/gml"
)

// pos formats a GML lat-lon position.
func pos(lat, lon float64) string {
	return strconv.FormatFloat(lat, 'f', -1, 64) + " " + strconv.FormatFloat(lon, 'f', -1, 64)
}

// sagittaOf returns the largest distance, sampled along every chord of the
// polyline (straight in lon/lat), between the chord and the geodesic circle
// of radius r around (lat, lon): r minus the geodesic distance of the sample
// from the centre.
func sagittaOf(line [][2]float64, lat, lon, r float64) float64 {
	worst := 0.0
	for i := 0; i+1 < len(line); i++ {
		a, b := line[i], line[i+1]
		for k := 1; k < 64; k++ {
			t := float64(k) / 64
			p := [2]float64{a[0] + t*(b[0]-a[0]), a[1] + t*(b[1]-a[1])}
			worst = math.Max(worst, math.Abs(r-geo.GeodesicDistance(lat, lon, p[1], p[0])))
		}
	}
	return worst
}

func TestArcToleranceCircle(t *testing.T) {
	const lat, lon = 50.1, 8.6
	for _, nm := range []float64{0.05, 1, 2, 10, 34} {
		r := nm * 1852
		segs := &gml.CurveSegments{CircleByCenterPoint: []gml.CircleByCenterPoint{{
			Pos:    &gml.DirectPosition{Value: pos(lat, lon)},
			Radius: gml.Measure{Value: nm, UOM: "NM"},
		}}}
		legacy, err := geo.SegmentCoords(segs, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(legacy) != geo.DefaultArcPoints+1 {
			t.Errorf("r=%g NM: zero options give %d positions, want %d", nm, len(legacy), geo.DefaultArcPoints+1)
		}
		for _, tol := range []float64{1, 0.25} {
			ring, err := geo.Options{ArcTolerance: tol}.SegmentCoords(segs, nil)
			if err != nil {
				t.Fatal(err)
			}
			if ring[0] != ring[len(ring)-1] {
				t.Errorf("r=%g NM: ring not closed", nm)
			}
			for _, p := range ring {
				if d := math.Abs(geo.GeodesicDistance(lat, lon, p[1], p[0]) - r); d > 0.001 {
					t.Fatalf("r=%g NM: vertex %.4f m off the radius", nm, d)
				}
			}
			s := sagittaOf(ring, lat, lon, r)
			if s > tol {
				t.Errorf("r=%g NM tol=%g: chord error %.3f m", nm, tol, s)
			}
			// Not wastefully dense either: the error uses most of the budget.
			if r > 100*tol && s < tol/2 {
				t.Errorf("r=%g NM tol=%g: chord error only %.3f m with %d positions", nm, tol, s, len(ring))
			}
			t.Logf("r=%5.2f NM tol=%.2f: %4d positions (legacy %d, error %.2f m → %.3f m)",
				nm, tol, len(ring), len(legacy), sagittaOf(legacy, lat, lon, r), s)
		}
	}
}

func TestArcToleranceArc(t *testing.T) {
	const lat, lon, nm = 52.3, 13.5, 18.0
	r := nm * 1852
	for _, sweep := range [][2]float64{{10, 50}, {270, 497}, {90, -40}, {0, 5.6}} {
		segs := &gml.CurveSegments{ArcByCenterPoint: []gml.ArcByCenterPoint{{
			Pos:        &gml.DirectPosition{Value: pos(lat, lon)},
			Radius:     gml.Measure{Value: nm, UOM: "NM"},
			StartAngle: &gml.Angle{Value: sweep[0], UOM: "deg"},
			EndAngle:   &gml.Angle{Value: sweep[1], UOM: "deg"},
		}}}
		line, err := geo.Options{ArcTolerance: 1}.SegmentCoords(segs, nil)
		if err != nil {
			t.Fatal(err)
		}
		legacy, _ := geo.SegmentCoords(segs, nil)
		// The ends sit at the source bearings, as without a tolerance.
		if line[0] != legacy[0] || line[len(line)-1] != legacy[len(legacy)-1] {
			t.Errorf("arc %v: ends moved", sweep)
		}
		if s := sagittaOf(line, lat, lon, r); s > 1 {
			t.Errorf("arc %v: chord error %.3f m", sweep, s)
		}
		t.Logf("arc %v: %d positions (legacy %d, error %.1f m)", sweep, len(line), len(legacy), sagittaOf(legacy, lat, lon, r))
	}
}

// geodesicDeviation samples the geodesic a→b every 50 m and returns the
// largest distance of a sample from the polyline (nearest point found in a
// local equirectangular projection, distance measured geodesically).
func geodesicDeviation(t *testing.T, line [][2]float64, a, b [2]float64) float64 {
	t.Helper()
	d := geo.GeodesicDistance(a[1], a[0], b[1], b[0])
	// Initial azimuth by bisection-free search: walk to b with DestinationPoint.
	az := initialAzimuth(a, b)
	if lat, lon := geo.DestinationPoint(a[1], a[0], az, d); geo.GeodesicDistance(lat, lon, b[1], b[0]) > 0.01 {
		t.Fatalf("azimuth search failed")
	}
	worst := 0.0
	n := int(d/50) + 1
	for k := 0; k <= n; k++ {
		lat, lon := geo.DestinationPoint(a[1], a[0], az, d*float64(k)/float64(n))
		worst = math.Max(worst, distToLine(line, [2]float64{lon, lat}))
	}
	return worst
}

// initialAzimuth finds the azimuth at a of the geodesic to b by minimising
// the miss distance (golden-section search around the spherical bearing).
func initialAzimuth(a, b [2]float64) float64 {
	d := geo.GeodesicDistance(a[1], a[0], b[1], b[0])
	miss := func(az float64) float64 {
		lat, lon := geo.DestinationPoint(a[1], a[0], az, d)
		return geo.GeodesicDistance(lat, lon, b[1], b[0])
	}
	phi1, phi2, dl := a[1]*math.Pi/180, b[1]*math.Pi/180, (b[0]-a[0])*math.Pi/180
	b0 := math.Atan2(math.Sin(dl)*math.Cos(phi2), math.Cos(phi1)*math.Sin(phi2)-math.Sin(phi1)*math.Cos(phi2)*math.Cos(dl)) * 180 / math.Pi
	lo, hi := b0-1, b0+1
	g := (math.Sqrt(5) - 1) / 2
	for range 200 {
		x1, x2 := hi-g*(hi-lo), lo+g*(hi-lo)
		if miss(x1) < miss(x2) {
			hi = x2
		} else {
			lo = x1
		}
	}
	return (lo + hi) / 2
}

func distToLine(line [][2]float64, p [2]float64) float64 {
	k := math.Cos(p[1] * math.Pi / 180)
	best := math.Inf(1)
	for i := 0; i+1 < len(line); i++ {
		a, b := line[i], line[i+1]
		ax, ay := (a[0]-p[0])*k, a[1]-p[1]
		bx, by := (b[0]-p[0])*k, b[1]-p[1]
		dx, dy := bx-ax, by-ay
		t := 0.0
		if l := dx*dx + dy*dy; l > 0 {
			t = math.Max(0, math.Min(1, -(ax*dx+ay*dy)/l))
		}
		q := [2]float64{p[0] + (ax+t*dx)/k, p[1] + ay + t*dy}
		best = math.Min(best, geo.GeodesicDistance(p[1], p[0], q[1], q[0]))
	}
	return best
}

func TestGeodesicTolerance(t *testing.T) {
	// Long east-west and diagonal segments, as on FIR boundaries, plus a
	// short one that needs nothing.
	cases := [][2][2]float64{
		{{6.0, 54.0}, {9.0, 54.0}},   // ≈ 196 km E-W
		{{7.5, 47.5}, {10.0, 50.5}},  // ≈ 390 km diagonal
		{{13.0, 52.0}, {13.3, 52.0}}, // ≈ 20 km
		{{8.0, 50.0}, {8.0, 50.5}},   // meridian: a straight line already
		{{11.0, 48.0}, {11.01, 48.0}},
	}
	for _, c := range cases {
		a, b := c[0], c[1]
		segs := &gml.CurveSegments{GeodesicString: []gml.GeodesicString{{
			PosList: &gml.DirectPositionList{Value: pos(a[1], a[0]) + " " + pos(b[1], b[0])},
		}}}
		legacy, _ := geo.SegmentCoords(segs, nil)
		if len(legacy) != 2 {
			t.Fatalf("zero options: %d positions, want the 2 source vertices", len(legacy))
		}
		before := geodesicDeviation(t, legacy, a, b)
		for _, tol := range []float64{1, 0.1} {
			line, err := geo.Options{GeodesicTolerance: tol}.SegmentCoords(segs, nil)
			if err != nil {
				t.Fatal(err)
			}
			if line[0] != a || line[len(line)-1] != b {
				t.Errorf("%v: source vertices not kept exactly: %v … %v", c, line[0], line[len(line)-1])
			}
			after := geodesicDeviation(t, line, a, b)
			if after > tol*1.01 { // 1 %: the reference measurement's own error
				t.Errorf("%v tol=%g: deviation %.3f m with %d positions", c, tol, after, len(line))
			}
			t.Logf("%v tol=%.1f: %d positions, deviation %.2f m → %.3f m", c, tol, len(line), before, after)
		}
	}
}

func TestGeodesicToleranceLineStringSegment(t *testing.T) {
	a, b := [2]float64{6, 54}, [2]float64{9, 54}
	list := &gml.DirectPositionList{Value: pos(a[1], a[0]) + " " + pos(b[1], b[0])}
	opt := geo.Options{GeodesicTolerance: 1}
	for interp, want := range map[string]bool{"": false, "linear": false, "geodesic": true} {
		segs := &gml.CurveSegments{LineStringSegment: []gml.LineStringSegment{{Interpolation: interp, PosList: list}}}
		line, err := opt.SegmentCoords(segs, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := len(line) > 2; got != want {
			t.Errorf("interpolation %q: densified = %v, want %v", interp, got, want)
		}
	}
}

// TestOptionsDonlon converts every DONLON airspace with tolerances and
// checks that the result is valid, that geodesic densification keeps every
// position of the undensified ring in order, and that arcs get denser.
func TestOptionsDonlon(t *testing.T) {
	msg, err := goaixm.ParseFile(context.Background(), donlonPath, goaixm.Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	var before, geod, both int
	for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
		legacy, err := geo.AirspaceToGeoJSONWithRefs(a, msg)
		if err != nil {
			continue // covered by TestAirspaceToGeoJSONDonlonAll
		}
		g, err := geo.Options{GeodesicTolerance: 1}.AirspaceToGeoJSONWithRefs(a, msg)
		if err != nil {
			t.Fatalf("%s: %v", a.UUID(), err)
		}
		full, err := geo.Options{ArcTolerance: 1, GeodesicTolerance: 1}.AirspaceToGeoJSONWithRefs(a, msg)
		if err != nil {
			t.Fatalf("%s: %v", a.UUID(), err)
		}
		lr, gr, fr := ringsOf(legacy), ringsOf(g), ringsOf(full)
		if len(lr) != len(gr) || len(lr) != len(fr) {
			t.Fatalf("%s: ring count changed: %d, %d, %d", a.UUID(), len(lr), len(gr), len(fr))
		}
		for i := range lr {
			checkRing(t, a.UUID(), fr[i], i == 0)
			before, geod, both = before+len(lr[i]), geod+len(gr[i]), both+len(fr[i])
			// Every position of the undensified ring appears, in order.
			k := 0
			for _, p := range gr[i] {
				if k < len(lr[i]) && p == lr[i][k] {
					k++
				}
			}
			if k != len(lr[i]) {
				t.Errorf("%s ring %d: only %d of %d positions kept", a.UUID(), i, k, len(lr[i]))
			}
		}
	}
	if before == 0 || both <= before {
		t.Errorf("positions: %d undensified, %d densified", before, both)
	}
	t.Logf("DONLON airspace ring positions: %d undensified, %d geodesic densified, %d arcs and geodesics densified", before, geod, both)
}

// A shared edge or arc described in opposite directions by two areas gets
// identical positions, so that their union leaves no sliver.
func TestDensifyDirectionIndependent(t *testing.T) {
	opt := geo.Options{ArcTolerance: 1, GeodesicTolerance: 1}
	reversedEqual := func(name string, a, b [][2]float64) {
		t.Helper()
		if len(a) != len(b) || len(a) < 3 {
			t.Fatalf("%s: %d and %d positions", name, len(a), len(b))
		}
		for i := range a {
			if a[i] != b[len(b)-1-i] {
				t.Fatalf("%s: position %d differs: %v vs %v", name, i, a[i], b[len(b)-1-i])
			}
		}
	}
	p, q := pos(54.0, 6.0), pos(53.7, 9.1)
	fwd, _ := opt.SegmentCoords(&gml.CurveSegments{GeodesicString: []gml.GeodesicString{{PosList: &gml.DirectPositionList{Value: p + " " + q}}}}, nil)
	rev, _ := opt.SegmentCoords(&gml.CurveSegments{GeodesicString: []gml.GeodesicString{{PosList: &gml.DirectPositionList{Value: q + " " + p}}}}, nil)
	reversedEqual("geodesic", fwd, rev)

	arc := func(from, to float64) [][2]float64 {
		c, err := opt.SegmentCoords(&gml.CurveSegments{ArcByCenterPoint: []gml.ArcByCenterPoint{{
			Pos: &gml.DirectPosition{Value: pos(50, 8)}, Radius: gml.Measure{Value: 12, UOM: "NM"},
			StartAngle: &gml.Angle{Value: from}, EndAngle: &gml.Angle{Value: to},
		}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	reversedEqual("arc", arc(10, 50), arc(50, 10))
}
