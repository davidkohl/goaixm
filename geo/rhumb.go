package geo

import (
	"math"
	"slices"
)

// wgs84E is the first eccentricity of WGS-84.
var wgs84E = math.Sqrt(wgs84E2)

// isometricLat returns the isometric latitude ψ of a geodetic latitude
// (radians) on WGS-84: the Mercator ordinate, linear in longitude along a
// rhumb line.
func isometricLat(phi float64) float64 {
	s := math.Sin(phi)
	return math.Atanh(s) - wgs84E*math.Atanh(wgs84E*s)
}

// geodeticLat inverts isometricLat (fixed-point iteration, converging to
// full precision in a few steps away from the poles).
func geodeticLat(psi float64) float64 {
	phi := math.Atan(math.Sinh(psi))
	for range 20 {
		next := math.Atan(math.Sinh(psi + wgs84E*math.Atanh(wgs84E*math.Sin(phi))))
		if next == phi {
			break
		}
		phi = next
	}
	return phi
}

// maxRhumbLat bounds the latitudes of rhumb lines (degrees): the isometric
// latitude diverges at the poles.
const maxRhumbLat = 89.9

// RhumbPosition returns the position ([lon, lat]) at fraction t (0 to 1) of
// the rhumb line (loxodrome: the line of constant true bearing) from a to b
// on the WGS-84 ellipsoid, taking the shorter way in longitude.  Along a
// rhumb line the longitude is linear in the isometric latitude, so t
// divides both linearly; positions are not evenly spaced in distance.
func RhumbPosition(a, b [2]float64, t float64) [2]float64 {
	psi1, psi2 := isometricLat(rad(a[1])), isometricLat(rad(b[1]))
	dLon := normaliseLon(b[0] - a[0])
	return [2]float64{normaliseLon(a[0] + t*dLon), deg(geodeticLat(psi1 + t*(psi2-psi1)))}
}

// RhumbBearing returns the constant true bearing in degrees (0 to 360) of
// the rhumb line from a to b ([lon, lat]) on WGS-84.
func RhumbBearing(a, b [2]float64) float64 {
	dPsi := isometricLat(rad(b[1])) - isometricLat(rad(a[1]))
	br := deg(math.Atan2(rad(normaliseLon(b[0]-a[0])), dPsi))
	if br < 0 {
		br += 360
	}
	return br
}

// RhumbLine returns the positions to insert strictly between a and b
// ([lon, lat]) so that the polyline a, positions…, b, each piece read as a
// geodesic (gml:GeodesicString), deviates from the rhumb line from a to b
// by at most tol metres.  The positions lie on the rhumb line at equal
// fractions (RhumbPosition); the deviation of a piece is measured at the
// rhumb position halfway along it (cross-track distance to the piece's
// geodesic) and the number of pieces grows until every piece is within
// tol.  The result does not depend on the direction: b→a gives the same
// positions in reverse order, bit for bit.
//
// nil is returned when no position is needed (meridians and the equator
// are both rhumb lines and geodesics), for tol ≤ 0, for coincident ends and
// for lines reaching beyond ±89.9° latitude.
func RhumbLine(a, b [2]float64, tol float64) [][2]float64 {
	if b[0] < a[0] || (b[0] == a[0] && b[1] < a[1]) {
		pts := RhumbLine(b, a, tol)
		slices.Reverse(pts)
		return pts
	}
	if tol <= 0 || a == b || math.Abs(a[1]) > maxRhumbLat || math.Abs(b[1]) > maxRhumbLat {
		return nil
	}
	at := func(t float64) [2]float64 { return RhumbPosition(a, b, t) }
	dev := crossTrack(at(0.5), a, b)
	if dev <= tol {
		return nil
	}
	n := int(math.Ceil(math.Sqrt(dev / tol)))
	for range 32 {
		n = min(max(n, 2), maxDensifyPoints)
		pts := make([][2]float64, n+1)
		pts[0], pts[n] = a, b
		for i := 1; i < n; i++ {
			pts[i] = at(float64(i) / float64(n))
		}
		worst := 0.0
		for i := 0; i < n; i++ {
			worst = math.Max(worst, crossTrack(at((float64(i)+0.5)/float64(n)), pts[i], pts[i+1]))
		}
		if worst <= tol || n == maxDensifyPoints {
			return pts[1:n]
		}
		n = int(math.Ceil(float64(n)*math.Sqrt(worst/tol))) + 1
	}
	return nil // not reached
}

// crossTrack returns the distance in metres of m from the geodesic p→q,
// for m near its middle: d(p, m) · |sin(az(p→m) − az(p→q))|, adequate
// when the angle is small (it is used for deviations of metres over
// kilometres).
func crossTrack(m, p, q [2]float64) float64 {
	d, azm, ok1 := geodesicInverse(p[1], p[0], m[1], m[0])
	_, azq, ok2 := geodesicInverse(p[1], p[0], q[1], q[0])
	if !ok1 || !ok2 {
		return math.Inf(1)
	}
	return math.Abs(d * math.Sin(rad(azm-azq)))
}
