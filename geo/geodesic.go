package geo

import (
	"fmt"
	"math"
	"strings"
)

// WGS-84 defining parameters.
const (
	wgs84A = 6378137.0             // semi-major axis, metres
	wgs84F = 1 / 298.257223563     // flattening
	wgs84B = wgs84A * (1 - wgs84F) // semi-minor axis, metres
)

// meanEarthRadius is the IUGG mean radius, used only as a fallback when the
// Vincenty inverse solution fails to converge (near-antipodal points).
const meanEarthRadius = 6371008.8

const (
	vincentyMaxIter = 200
	vincentyEps     = 1e-12
)

// DestinationPoint returns the point reached from (lat, lon) by travelling
// distMetres along the geodesic whose initial true bearing is bearingDeg
// (degrees clockwise from north), on the WGS-84 ellipsoid.
//
// This is the Vincenty direct solution.  Unlike the inverse problem it always
// converges, so there is no fallback path.
func DestinationPoint(lat, lon, bearingDeg, distMetres float64) (destLat, destLon float64) {
	if distMetres == 0 {
		return lat, lon
	}

	phi1 := rad(lat)
	lambda1 := rad(lon)
	alpha1 := rad(bearingDeg)

	sinAlpha1, cosAlpha1 := dsincos(alpha1)

	tanU1 := float64((1 - wgs84F) * dtan(phi1))
	cosU1 := 1 / math.Sqrt(1+float64(tanU1*tanU1))
	sinU1 := float64(tanU1 * cosU1)

	// sigma1 is the angular distance on the sphere from the equator to the
	// starting point along the geodesic.
	sigma1 := datan2(tanU1, cosAlpha1)

	// alpha is the azimuth of the geodesic at the equator.
	sinAlpha := float64(cosU1 * sinAlpha1)
	cosSqAlpha := 1 - float64(sinAlpha*sinAlpha)

	uSq := cosSqAlpha * (wgs84A*wgs84A - wgs84B*wgs84B) / (wgs84B * wgs84B) // constants: exact
	aCoef := 1 + float64(uSq/16384*(4096+float64(uSq*(-768+float64(uSq*(320-float64(175*uSq)))))))
	bCoef := float64(uSq / 1024 * (256 + float64(uSq*(-128+float64(uSq*(74-float64(47*uSq)))))))

	// sigma0 is the fixed angular distance term; sigma is refined around it.
	sigma0 := distMetres / (wgs84B * aCoef)
	sigma := sigma0
	var sinSigma, cosSigma, cos2SigmaM float64

	for i := 0; i < vincentyMaxIter; i++ {
		cos2SigmaM = dcos(float64(2*sigma1) + sigma)
		sinSigma, cosSigma = dsincos(sigma)

		deltaSigma := deltaSigmaOf(bCoef, sinSigma, cosSigma, cos2SigmaM)

		next := sigma0 + deltaSigma
		if math.Abs(next-sigma) < vincentyEps {
			sigma = next
			break
		}
		sigma = next
	}

	sinSigma, cosSigma = dsincos(sigma)
	cos2SigmaM = dcos(float64(2*sigma1) + sigma)

	tmp := float64(sinU1*sinSigma) - float64(cosU1*cosSigma*cosAlpha1)
	phi2 := datan2(
		float64(sinU1*cosSigma)+float64(cosU1*sinSigma*cosAlpha1),
		(1-wgs84F)*dhypot(sinAlpha, tmp),
	)

	lambda := datan2(
		sinSigma*sinAlpha1,
		float64(cosU1*cosSigma)-float64(sinU1*sinSigma*cosAlpha1),
	)
	cCoef := float64(wgs84F / 16 * cosSqAlpha * (4 + float64(wgs84F*(4-float64(3*cosSqAlpha)))))
	l := lambda - float64((1-cCoef)*wgs84F*sinAlpha*lambdaSeries(sigma, cCoef, sinSigma, cosSigma, cos2SigmaM))

	return deg(phi2), normaliseLon(deg(lambda1 + l))
}

// GeodesicDistance returns the distance in metres between two points on the
// WGS-84 ellipsoid, using the Vincenty inverse solution.
//
// The inverse solution does not converge for near-antipodal point pairs; in
// that case the great-circle distance on a sphere of mean earth radius is
// returned instead.
func GeodesicDistance(lat1, lon1, lat2, lon2 float64) float64 {
	d, _, ok := geodesicInverse(lat1, lon1, lat2, lon2)
	if !ok {
		// Near-antipodal: fall back to a spherical great-circle distance.
		return haversine(lat1, lon1, lat2, lon2)
	}
	return d
}

// geodesicInverse is the Vincenty inverse solution: the distance in metres
// and the initial azimuth (degrees clockwise from north) of the geodesic
// from the first point to the second.  ok is false when the iteration does
// not converge (near-antipodal points).
func geodesicInverse(lat1, lon1, lat2, lon2 float64) (dist, azimuth float64, ok bool) {
	phi1, phi2 := rad(lat1), rad(lat2)
	l := rad(normaliseLon(lon2 - lon1))

	tanU1 := float64((1 - wgs84F) * dtan(phi1))
	cosU1 := 1 / math.Sqrt(1+float64(tanU1*tanU1))
	sinU1 := float64(tanU1 * cosU1)

	tanU2 := float64((1 - wgs84F) * dtan(phi2))
	cosU2 := 1 / math.Sqrt(1+float64(tanU2*tanU2))
	sinU2 := float64(tanU2 * cosU2)

	lambda := l
	var sinSigma, cosSigma, sigma, cosSqAlpha, cos2SigmaM float64

	converged := false
	for i := 0; i < vincentyMaxIter; i++ {
		sinLambda, cosLambda := dsincos(lambda)

		sinSigma = dhypot(
			cosU2*sinLambda,
			float64(cosU1*sinU2)-float64(sinU1*cosU2*cosLambda),
		)
		if sinSigma == 0 {
			return 0, 0, true // coincident points
		}
		cosSigma = float64(sinU1*sinU2) + float64(cosU1*cosU2*cosLambda)
		sigma = datan2(sinSigma, cosSigma)

		sinAlpha := float64(cosU1 * cosU2 * sinLambda / sinSigma)
		cosSqAlpha = 1 - float64(sinAlpha*sinAlpha)

		if cosSqAlpha == 0 {
			cos2SigmaM = 0 // equatorial line
		} else {
			cos2SigmaM = cosSigma - float64(2*sinU1*sinU2/cosSqAlpha)
		}

		cCoef := float64(wgs84F / 16 * cosSqAlpha * (4 + float64(wgs84F*(4-float64(3*cosSqAlpha)))))
		prev := lambda
		lambda = l + float64((1-cCoef)*wgs84F*sinAlpha*lambdaSeries(sigma, cCoef, sinSigma, cosSigma, cos2SigmaM))

		if math.Abs(lambda-prev) < vincentyEps {
			converged = true
			break
		}
	}

	if !converged {
		return 0, 0, false
	}

	uSq := cosSqAlpha * (wgs84A*wgs84A - wgs84B*wgs84B) / (wgs84B * wgs84B) // constants: exact
	aCoef := 1 + float64(uSq/16384*(4096+float64(uSq*(-768+float64(uSq*(320-float64(175*uSq)))))))
	bCoef := float64(uSq / 1024 * (256 + float64(uSq*(-128+float64(uSq*(74-float64(47*uSq)))))))

	deltaSigma := deltaSigmaOf(bCoef, sinSigma, cosSigma, cos2SigmaM)

	sinLambda, cosLambda := dsincos(lambda)
	az := deg(datan2(cosU2*sinLambda, float64(cosU1*sinU2)-float64(sinU1*cosU2*cosLambda)))
	if az < 0 {
		az += 360
	}
	return wgs84B * aCoef * (sigma - deltaSigma), az, true
}

// deltaSigmaOf is Vincenty's Δσ series (FMA guarded, detmath.go).
func deltaSigmaOf(bCoef, sinSigma, cosSigma, cos2SigmaM float64) float64 {
	c2 := float64(2 * cos2SigmaM * cos2SigmaM)
	inner := float64(cosSigma*(-1+c2)) -
		float64(bCoef/6*cos2SigmaM*(-3+float64(4*sinSigma*sinSigma))*(-3+float64(4*cos2SigmaM*cos2SigmaM)))
	return float64(bCoef * sinSigma * (cos2SigmaM + float64(bCoef/4*inner)))
}

// lambdaSeries is the bracket of Vincenty's λ − L term (FMA guarded).
func lambdaSeries(sigma, cCoef, sinSigma, cosSigma, cos2SigmaM float64) float64 {
	c2 := float64(2 * cos2SigmaM * cos2SigmaM)
	return sigma + float64(cCoef*sinSigma*(cos2SigmaM+float64(cCoef*cosSigma*(-1+c2))))
}

// haversine returns the great-circle distance in metres on a sphere of mean
// earth radius.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	phi1, phi2 := rad(lat1), rad(lat2)
	dPhi := rad(lat2 - lat1)
	dLambda := rad(lon2 - lon1)

	a := float64(dsin(dPhi/2)*dsin(dPhi/2)) +
		float64(dcos(phi1)*dcos(phi2)*dsin(dLambda/2)*dsin(dLambda/2))
	return meanEarthRadius * 2 * datan2(math.Sqrt(a), math.Sqrt(1-a))
}

// lengthUnits maps a unit of measure to its length in metres.
//
// AIXM declares the enumeration NM|KM|M|FT|MI|CM (AIXM_DataTypes.xsd), but
// gml:radius is a gml:LengthType whose uom is a free-form UCUM symbol, and
// published datasets use both spellings, some writing every radius as
// "[nmi_i]".  Keys here are lower-cased before lookup.
var lengthUnits = map[string]float64{
	"nm":      1852,
	"[nmi_i]": 1852,
	"km":      1000,
	"m":       1,
	"ft":      0.3048,
	"[ft_i]":  0.3048,
	"mi":      1609.344,
	"[mi_i]":  1609.344,
	"cm":      0.01,
}

// LengthToMetres converts value, expressed in the unit of measure uom, to
// metres.  Recognised units are NM, KM, M, FT, MI and CM together with the
// UCUM spellings [nmi_i], km, m, [ft_i] and [mi_i]; matching is
// case-insensitive.
//
// An unrecognised or empty uom is an error rather than a default: silently
// treating "[nmi_i]" as metres would shrink a circle by a factor of 1852, and
// a plausible-looking wrong polygon is more dangerous than a failed conversion.
func LengthToMetres(value float64, uom string) (float64, error) {
	key := strings.ToLower(strings.TrimSpace(uom))
	factor, ok := lengthUnits[key]
	if !ok {
		return 0, fmt.Errorf("geo: unrecognised unit of measure %q", uom)
	}
	return value * factor, nil
}

// arcSweep returns the start bearing and the signed sweep, in degrees, of a
// GML arc running from startDeg to endDeg.
//
// GML start/end angles are true bearings measured clockwise from north.  They
// may run backwards and may exceed 360: DONLON airspace EAR1 has
// startAngle=270, endAngle=497, describing a +227° clockwise sweep.  The sweep
// is therefore the raw difference, not a modular one — reducing 497 to 137
// would yield a wrong −133° sweep.
//
// Sweeps are clamped to (-360, 360].  A zero sweep denotes a full circle.
func arcSweep(startDeg, endDeg float64) (start, sweep float64) {
	sweep = endDeg - startDeg

	switch {
	case sweep == 0:
		sweep = 360
	case sweep > 360:
		sweep = math.Mod(sweep, 360)
		if sweep == 0 {
			sweep = 360
		}
	case sweep < -360:
		sweep = math.Mod(sweep, 360)
		if sweep == 0 {
			sweep = -360
		}
	}
	return startDeg, sweep
}

func rad(d float64) float64 { return d * math.Pi / 180 }
func deg(r float64) float64 { return r * 180 / math.Pi }

// normaliseLon wraps a longitude into [-180, 180).
func normaliseLon(lon float64) float64 {
	if lon >= -180 && lon < 180 {
		return lon
	}
	lon = math.Mod(lon+180, 360)
	if lon < 0 {
		lon += 360
	}
	return lon - 180
}
