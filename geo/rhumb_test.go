package geo

import (
	"math"
	"testing"
)

// constantAzimuthPath follows the rhumb line from a towards b
// independently of the closed form: steps of step metres along geodesics
// that all start with the same azimuth (the rhumb bearing), recording the
// longitude at which each latitude is reached.  It returns the positions.
func constantAzimuthPath(a [2]float64, bearing, length, step float64) [][2]float64 {
	out := [][2]float64{a}
	lat, lon := a[1], a[0]
	for d := 0.0; d < length; d += step {
		lat, lon = DestinationPoint(lat, lon, bearing, math.Min(step, length-d))
		out = append(out, [2]float64{lon, lat})
	}
	return out
}

// TestRhumbPositionIndependent compares RhumbPosition with a path of
// constant azimuth integrated in 1 m steps (a different construction):
// positions agree within 5 cm over 100 km.
func TestRhumbPositionIndependent(t *testing.T) {
	a, b := [2]float64{20.0, 10.5}, [2]float64{20.7, 11.2}
	br := RhumbBearing(a, b)
	// Rhumb length: along a rhumb line ds = M dφ / cos(α); integrate.
	length := 0.0
	const n = 20000
	for i := 0; i < n; i++ {
		phi := rad(a[1] + (b[1]-a[1])*(float64(i)+0.5)/n)
		s := math.Sin(phi)
		m := wgs84A * (1 - wgs84E2) / math.Pow(1-wgs84E2*s*s, 1.5)
		length += m * rad(b[1]-a[1]) / n / math.Cos(rad(br))
	}
	path := constantAzimuthPath(a, br, length, 1)
	end := path[len(path)-1]
	if d := GeodesicDistance(end[1], end[0], b[1], b[0]); d > 0.05 {
		t.Errorf("integrated end %.3f m from b", d)
	}
	for _, frac := range []float64{0.25, 0.5, 0.75} {
		p := RhumbPosition(a, b, frac)
		// The integrated position at the same latitude.
		best := math.Inf(1)
		for i := 1; i < len(path); i++ {
			if (path[i-1][1]-p[1])*(path[i][1]-p[1]) <= 0 {
				t0 := (p[1] - path[i-1][1]) / (path[i][1] - path[i-1][1])
				lon := path[i-1][0] + t0*(path[i][0]-path[i-1][0])
				best = GeodesicDistance(p[1], p[0], p[1], lon)
				break
			}
		}
		t.Logf("t=%.2f: %.4f m from the integrated rhumb line", frac, best)
		if best > 0.05 {
			t.Errorf("t=%.2f: %.3f m from the integrated rhumb line", frac, best)
		}
	}
	if br < 0 || br > 90 {
		t.Errorf("bearing %f", br)
	}
}

// sphericalParallelDeviation is the deviation in metres of the great
// circle between two points on the parallel lat (degrees) dLon apart from
// the parallel, at its middle, on a sphere of radius r: an independent
// approximation of the ellipsoidal value (well within 1 %).
func sphericalParallelDeviation(lat, dLon, r float64) float64 {
	phi := rad(lat)
	return r * (math.Atan(math.Tan(phi)/math.Cos(rad(dLon)/2)) - phi)
}

// TestRhumbLineParallel: along a parallel the rhumb line keeps its
// latitude; the inserted positions keep every geodesic piece within the
// tolerance of it, and not much more densely than needed.
func TestRhumbLineParallel(t *testing.T) {
	const tol = 1.0
	a, b := [2]float64{20, 10.5}, [2]float64{21, 10.5}
	pts := RhumbLine(a, b, tol)
	if len(pts) < 2 {
		t.Fatalf("%d positions", len(pts))
	}
	all := append(append([][2]float64{a}, pts...), b)
	r := math.Sqrt(wgs84A * wgs84A * (1 - wgs84E2)) // ≈ √(MN), local radius
	worst := 0.0
	for i := 1; i < len(all); i++ {
		if math.Abs(all[i][1]-10.5) > 1e-9 {
			t.Fatalf("position %d off the parallel: %v", i, all[i])
		}
		worst = math.Max(worst, sphericalParallelDeviation(10.5, all[i][0]-all[i-1][0], r))
	}
	t.Logf("%d positions inserted, worst piece %.4f m (spherical model)", len(pts), worst)
	if worst > tol*1.01 {
		t.Errorf("worst piece deviates %.3f m (tolerance %.1f m)", worst, tol)
	}
	// One piece fewer would exceed the tolerance (by the same model).
	if d := sphericalParallelDeviation(10.5, 1/float64(len(all)-2), r); d <= tol {
		t.Errorf("%d pieces where %d would do (%.3f m)", len(all)-1, len(all)-2, d)
	}
	// The whole parallel deviates by tens of metres from its geodesic.
	if d := sphericalParallelDeviation(10.5, 1, r); d < 30 {
		t.Errorf("undensified deviation %.1f m", d)
	}
}

func TestRhumbLineSpecial(t *testing.T) {
	// Meridians and the equator are geodesics: nothing to insert.
	if p := RhumbLine([2]float64{20, 10}, [2]float64{20, 12}, 1); p != nil {
		t.Errorf("meridian: %v", p)
	}
	if p := RhumbLine([2]float64{20, 0}, [2]float64{22, 0}, 1); p != nil {
		t.Errorf("equator: %v", p)
	}
	if p := RhumbLine([2]float64{20, 10}, [2]float64{21, 10}, 0); p != nil {
		t.Error("tolerance 0")
	}
	// Direction independence, bit for bit.
	a, b := [2]float64{20, 10.5}, [2]float64{21.3, 11.1}
	f, r := RhumbLine(a, b, 0.5), RhumbLine(b, a, 0.5)
	if len(f) == 0 || len(f) != len(r) {
		t.Fatalf("%d and %d positions", len(f), len(r))
	}
	for i := range f {
		if f[i] != r[len(r)-1-i] {
			t.Fatalf("position %d differs: %v %v", i, f[i], r[len(r)-1-i])
		}
	}
	// The inserted positions keep the bearing.
	br := RhumbBearing(a, b)
	for _, p := range f {
		if d := math.Abs(RhumbBearing(a, p) - br); d > 1e-9 {
			t.Errorf("bearing differs by %g°", d)
		}
	}
	// Positions at the ends of the parametrisation.
	if p := RhumbPosition(a, b, 0); math.Abs(p[0]-a[0]) > 1e-12 || math.Abs(p[1]-a[1]) > 1e-12 {
		t.Errorf("t=0: %v", p)
	}
	if p := RhumbPosition(a, b, 1); math.Abs(p[0]-b[0]) > 1e-12 || math.Abs(p[1]-b[1]) > 1e-12 {
		t.Errorf("t=1: %v", p)
	}
}

func FuzzRhumbLine(f *testing.F) {
	f.Add(20.0, 10.5, 21.0, 10.5, 1.0)
	f.Add(-179.5, -60.0, 179.5, 60.0, 0.1)
	f.Fuzz(func(t *testing.T, x1, y1, x2, y2, tol float64) {
		for _, v := range []float64{x1, y1, x2, y2, tol} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return
			}
		}
		if math.Abs(x1) > 180 || math.Abs(x2) > 180 || math.Abs(y1) > 90 || math.Abs(y2) > 90 || tol < 0.01 {
			return
		}
		pts := RhumbLine([2]float64{x1, y1}, [2]float64{x2, y2}, tol)
		if len(pts) > maxDensifyPoints {
			t.Fatalf("%d positions", len(pts))
		}
		for _, p := range pts {
			if math.IsNaN(p[0]) || math.IsNaN(p[1]) || math.Abs(p[1]) > 90 || math.Abs(p[0]) > 180 {
				t.Fatalf("position %v", p)
			}
		}
	})
}
