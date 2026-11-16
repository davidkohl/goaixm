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

	sinAlpha1, cosAlpha1 := math.Sincos(alpha1)

	tanU1 := (1 - wgs84F) * math.Tan(phi1)
	cosU1 := 1 / math.Sqrt(1+tanU1*tanU1)
	sinU1 := tanU1 * cosU1

	// sigma1 is the angular distance on the sphere from the equator to the
	// starting point along the geodesic.
	sigma1 := math.Atan2(tanU1, cosAlpha1)

	// alpha is the azimuth of the geodesic at the equator.
	sinAlpha := cosU1 * sinAlpha1
	cosSqAlpha := 1 - sinAlpha*sinAlpha

	uSq := cosSqAlpha * (wgs84A*wgs84A - wgs84B*wgs84B) / (wgs84B * wgs84B)
	aCoef := 1 + uSq/16384*(4096+uSq*(-768+uSq*(320-175*uSq)))
	bCoef := uSq / 1024 * (256 + uSq*(-128+uSq*(74-47*uSq)))

	// sigma0 is the fixed angular distance term; sigma is refined around it.
	sigma0 := distMetres / (wgs84B * aCoef)
	sigma := sigma0
	var sinSigma, cosSigma, cos2SigmaM float64

	for i := 0; i < vincentyMaxIter; i++ {
		cos2SigmaM = math.Cos(2*sigma1 + sigma)
		sinSigma, cosSigma = math.Sincos(sigma)

		deltaSigma := bCoef * sinSigma * (cos2SigmaM + bCoef/4*
			(cosSigma*(-1+2*cos2SigmaM*cos2SigmaM)-
				bCoef/6*cos2SigmaM*(-3+4*sinSigma*sinSigma)*(-3+4*cos2SigmaM*cos2SigmaM)))

		next := sigma0 + deltaSigma
		if math.Abs(next-sigma) < vincentyEps {
			sigma = next
			break
		}
		sigma = next
	}

	sinSigma, cosSigma = math.Sincos(sigma)
	cos2SigmaM = math.Cos(2*sigma1 + sigma)

	tmp := sinU1*sinSigma - cosU1*cosSigma*cosAlpha1
	phi2 := math.Atan2(
		sinU1*cosSigma+cosU1*sinSigma*cosAlpha1,
		(1-wgs84F)*math.Hypot(sinAlpha, tmp),
	)

	lambda := math.Atan2(
		sinSigma*sinAlpha1,
		cosU1*cosSigma-sinU1*sinSigma*cosAlpha1,
	)
	cCoef := wgs84F / 16 * cosSqAlpha * (4 + wgs84F*(4-3*cosSqAlpha))
	l := lambda - (1-cCoef)*wgs84F*sinAlpha*
		(sigma+cCoef*sinSigma*(cos2SigmaM+cCoef*cosSigma*(-1+2*cos2SigmaM*cos2SigmaM)))

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

	tanU1 := (1 - wgs84F) * math.Tan(phi1)
	cosU1 := 1 / math.Sqrt(1+tanU1*tanU1)
	sinU1 := tanU1 * cosU1

	tanU2 := (1 - wgs84F) * math.Tan(phi2)
	cosU2 := 1 / math.Sqrt(1+tanU2*tanU2)
	sinU2 := tanU2 * cosU2

	lambda := l
	var sinSigma, cosSigma, sigma, cosSqAlpha, cos2SigmaM float64

	converged := false
	for i := 0; i < vincentyMaxIter; i++ {
		sinLambda, cosLambda := math.Sincos(lambda)

		sinSigma = math.Hypot(
			cosU2*sinLambda,
			cosU1*sinU2-sinU1*cosU2*cosLambda,
		)
		if sinSigma == 0 {
			return 0, 0, true // coincident points
		}
		cosSigma = sinU1*sinU2 + cosU1*cosU2*cosLambda
		sigma = math.Atan2(sinSigma, cosSigma)

		sinAlpha := cosU1 * cosU2 * sinLambda / sinSigma
		cosSqAlpha = 1 - sinAlpha*sinAlpha

		if cosSqAlpha == 0 {
			cos2SigmaM = 0 // equatorial line
		} else {
			cos2SigmaM = cosSigma - 2*sinU1*sinU2/cosSqAlpha
		}

		cCoef := wgs84F / 16 * cosSqAlpha * (4 + wgs84F*(4-3*cosSqAlpha))
		prev := lambda
		lambda = l + (1-cCoef)*wgs84F*sinAlpha*
			(sigma+cCoef*sinSigma*(cos2SigmaM+cCoef*cosSigma*(-1+2*cos2SigmaM*cos2SigmaM)))

		if math.Abs(lambda-prev) < vincentyEps {
			converged = true
			break
		}
	}

	if !converged {
		return 0, 0, false
	}

	uSq := cosSqAlpha * (wgs84A*wgs84A - wgs84B*wgs84B) / (wgs84B * wgs84B)
	aCoef := 1 + uSq/16384*(4096+uSq*(-768+uSq*(320-175*uSq)))
	bCoef := uSq / 1024 * (256 + uSq*(-128+uSq*(74-47*uSq)))

	deltaSigma := bCoef * sinSigma * (cos2SigmaM + bCoef/4*
		(cosSigma*(-1+2*cos2SigmaM*cos2SigmaM)-
			bCoef/6*cos2SigmaM*(-3+4*sinSigma*sinSigma)*(-3+4*cos2SigmaM*cos2SigmaM)))

	sinLambda, cosLambda := math.Sincos(lambda)
	az := deg(math.Atan2(cosU2*sinLambda, cosU1*sinU2-sinU1*cosU2*cosLambda))
	if az < 0 {
		az += 360
	}
	return wgs84B * aCoef * (sigma - deltaSigma), az, true
}

// haversine returns the great-circle distance in metres on a sphere of mean
// earth radius.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	phi1, phi2 := rad(lat1), rad(lat2)
	dPhi := rad(lat2 - lat1)
	dLambda := rad(lon2 - lon1)

	a := math.Sin(dPhi/2)*math.Sin(dPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(dLambda/2)*math.Sin(dLambda/2)
	return meanEarthRadius * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
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
