package geo

import "math"

// wgs84E2 is the first eccentricity squared of WGS-84.
const wgs84E2 = wgs84F * (2 - wgs84F)

// densifyGeodesic returns the polyline coords ([lon, lat]) with positions on
// the geodesic inserted between consecutive vertices, so that the straight
// lon/lat line through them deviates from the geodesic by at most tol metres
// (see Options.GeodesicTolerance).  The given vertices are kept exactly.
// With tol ≤ 0 coords is returned unchanged.
func densifyGeodesic(coords [][2]float64, tol float64) [][2]float64 {
	if tol <= 0 || len(coords) < 2 {
		return coords
	}
	out := make([][2]float64, 0, len(coords))
	out = append(out, coords[0])
	for i := 1; i < len(coords); i++ {
		out = append(out, geodesicInterior(coords[i-1], coords[i], tol)...)
		out = append(out, coords[i])
	}
	return out
}

// geodesicInterior returns the positions to insert strictly between a and b
// ([lon, lat]): the n−1 points dividing the geodesic a→b into n pieces of
// equal length, for the smallest n found whose pieces each deviate from
// their straight lon/lat chord by at most tol metres.
func geodesicInterior(a, b [2]float64, tol float64) [][2]float64 {
	if math.Abs(a[0]-b[0]) > 180 {
		return nil // crosses the antimeridian: no straight lon/lat chord
	}
	dist, az, ok := geodesicInverse(a[1], a[0], b[1], b[0])
	if !ok || dist == 0 {
		return nil
	}
	at := func(s float64) [2]float64 {
		lat, lon := DestinationPoint(a[1], a[0], az, s)
		return [2]float64{lon, lat}
	}
	dev := chordDeviation(at(dist/2), a, b)
	if dev <= tol {
		return nil
	}
	// The deviation of a short piece grows with the square of its length.
	n := int(math.Ceil(math.Sqrt(dev / tol)))
	for range 32 {
		n = min(max(n, 2), maxDensifyPoints)
		pts := make([][2]float64, n+1)
		pts[0], pts[n] = a, b
		for i := 1; i < n; i++ {
			pts[i] = at(dist * float64(i) / float64(n))
		}
		worst := 0.0
		for i := 0; i < n; i++ {
			worst = math.Max(worst, chordDeviation(at(dist*(float64(i)+0.5)/float64(n)), pts[i], pts[i+1]))
		}
		if worst <= tol || n == maxDensifyPoints {
			return pts[1:n]
		}
		n = int(math.Ceil(float64(n)*math.Sqrt(worst/tol))) + 1
	}
	return nil // not reached: n grows to maxDensifyPoints at most
}

// chordDeviation returns the distance in metres from m to the straight
// lon/lat segment p–q, measured in the local metric of the WGS-84 ellipsoid
// at m (meridian and prime-vertical radii of curvature).  It is accurate
// for m near the segment, which is the case it is used for.
func chordDeviation(m, p, q [2]float64) float64 {
	phi := rad(m[1])
	s := math.Sin(phi)
	w := math.Sqrt(1 - wgs84E2*s*s)
	ky := rad(wgs84A * (1 - wgs84E2) / (w * w * w)) // metres per degree of latitude
	kx := rad(wgs84A / w * math.Cos(phi))           // metres per degree of longitude
	px, py := (p[0]-m[0])*kx, (p[1]-m[1])*ky
	qx, qy := (q[0]-m[0])*kx, (q[1]-m[1])*ky
	dx, dy := qx-px, qy-py
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = math.Max(0, math.Min(1, -(px*dx+py*dy)/l))
	}
	return math.Hypot(px+t*dx, py+t*dy)
}
