package geo

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/davidkohl/goaixm/gml"
)

// ---- Arc and circle discretization ------------------------------------------

// arcPointCount returns the number of segments used to approximate an arc of
// the given sweep and radius.  Without a tolerance a full circle uses
// DefaultArcPoints, and shorter sweeps proportionally fewer.  With one, the
// bearing step Δθ is the largest whose chord sagitta r·(1 − cos(Δθ/2)) stays
// within tol (see Options.ArcTolerance).  Either way an arc has at least
// arcMinPoints segments.
func arcPointCount(sweepDeg, radius, tol float64) int {
	var n int
	if tol > 0 && radius > tol {
		step := 2 * deg(dacos(1-tol/radius))
		n = int(math.Ceil(math.Abs(sweepDeg)/step - 1e-9))
	} else {
		n = int(math.Ceil(float64(DefaultArcPoints) * math.Abs(sweepDeg) / 360))
	}
	return min(max(n, arcMinPoints), maxDensifyPoints)
}

// circlePointCount is arcPointCount for a full circle, whose count without a
// tolerance is DefaultArcPoints exactly.
func circlePointCount(radius, tol float64) int {
	if tol > 0 && radius > tol {
		return arcPointCount(360, radius, tol)
	}
	return DefaultArcPoints
}

// discretizeArcByCenter approximates a gml:ArcByCenterPoint as a geodesic
// polyline running clockwise from its start bearing to its end bearing.
func discretizeArcByCenter(a *gml.ArcByCenterPoint, cv *conv) ([][2]float64, error) {
	lat, lon, err := arcCentre(a.Pos, a.PosList, a.PointProperty, a.PointRep, cv.res)
	if err != nil {
		return nil, err
	}
	radius, err := LengthToMetres(a.Radius.Value, a.Radius.UOM)
	if err != nil {
		return nil, fmt.Errorf("geojson: ArcByCenterPoint radius: %w", err)
	}

	var startDeg, endDeg float64
	if a.StartAngle != nil {
		startDeg = a.StartAngle.Value
	}
	if a.EndAngle != nil {
		endDeg = a.EndAngle.Value
	}
	start, sweep := arcSweep(startDeg, endDeg)

	n := arcPointCount(sweep, radius, cv.opt.ArcTolerance)
	return arcPositions(lat, lon, radius, start, sweep, n, cv.opt.ArcTolerance), nil
}

// discretizeCircleByCenter approximates a gml:CircleByCenterPoint as a closed
// geodesic ring.  The final position repeats the first exactly.
func discretizeCircleByCenter(c *gml.CircleByCenterPoint, cv *conv) ([][2]float64, error) {
	lat, lon, err := arcCentre(c.Pos, c.PosList, c.PointProperty, c.PointRep, cv.res)
	if err != nil {
		return nil, err
	}
	radius, err := LengthToMetres(c.Radius.Value, c.Radius.UOM)
	if err != nil {
		return nil, fmt.Errorf("geojson: CircleByCenterPoint radius: %w", err)
	}

	n := circlePointCount(radius, cv.opt.ArcTolerance)
	return circlePositions(lat, lon, radius, 0, n, cv.opt.ArcTolerance), nil
}

// arcPositions returns n+1 positions ([lon, lat]) at equal bearing steps
// on the geodesic circle of the given radius around (lat, lon), from start
// through start+sweep.  With tol > 0 the served error of every chord — the
// distance between the curve at the step's middle bearing and the straight
// lon/lat chord — is measured, and n grows until it is within tol: the
// sagitta estimate of arcPointCount does not include the (small) deviation
// of a lon/lat chord from the true one.
//
// With tol > 0 an arc is computed in the clockwise direction (a negative
// sweep from its other end, then reversed), so that two curves describing
// the same arc in opposite directions get identical positions and a union
// of their areas leaves no sliver along it.  Without, positions are as
// they always were.
func arcPositions(lat, lon, radius, start, sweep float64, n int, tol float64) [][2]float64 {
	if tol > 0 && sweep < 0 {
		coords := arcPositions(lat, lon, radius, start+sweep, -sweep, n, tol)
		slices.Reverse(coords)
		return coords
	}
	at := func(bearing float64) [2]float64 {
		pLat, pLon := DestinationPoint(lat, lon, bearing, radius)
		return [2]float64{pLon, pLat}
	}
	for {
		coords := make([][2]float64, 0, n+1)
		for i := 0; i <= n; i++ {
			coords = append(coords, at(start+sweep*float64(i)/float64(n)))
		}
		if tol <= 0 || n >= maxDensifyPoints {
			return coords
		}
		worst := 0.0
		for i := 0; i < n; i++ {
			worst = math.Max(worst, chordDeviation(at(start+sweep*(float64(i)+0.5)/float64(n)), coords[i], coords[i+1]))
		}
		if worst <= tol {
			return coords
		}
		n = min(int(math.Ceil(float64(n)*math.Sqrt(worst/tol)))+1, maxDensifyPoints)
	}
}

// circlePositions is arcPositions for a closed ring of n positions from
// the start bearing; the final position repeats the first exactly.
func circlePositions(lat, lon, radius, start float64, n int, tol float64) [][2]float64 {
	coords := arcPositions(lat, lon, radius, start, 360, n, tol)
	// Close the ring on the first position rather than recomputing it, so the
	// coincidence is exact.
	coords[len(coords)-1] = coords[0]
	return coords
}

// arcCentre extracts the centre of a by-centre arc from whichever of the four
// GML alternatives carries it.
func arcCentre(pos *gml.DirectPosition, posList *gml.DirectPositionList,
	pp, pr *gml.PointProperty, res centreResolver) (lat, lon float64, err error) {

	if pos != nil {
		return firstLatLon(pos.Value)
	}
	if posList != nil {
		return firstLatLon(posList.Value)
	}
	for _, p := range []*gml.PointProperty{pp, pr} {
		if p == nil {
			continue
		}
		if p.Point != nil && p.Point.Pos != nil {
			return firstLatLon(p.Point.Pos.Value)
		}
		if p.Href != "" {
			if res == nil {
				return 0, 0, fmt.Errorf(
					"geojson: arc centre references %s, which needs the surrounding message to resolve",
					p.Href)
			}
			if lat, lon, ok := res(p.Href); ok {
				return lat, lon, nil
			}
			return 0, 0, fmt.Errorf("geojson: cannot resolve arc centre %s", p.Href)
		}
	}
	return 0, 0, fmt.Errorf("geojson: arc has no centre point")
}

// firstLatLon parses the first latitude/longitude pair from a coordinate string.
func firstLatLon(s string) (lat, lon float64, err error) {
	fields := strings.Fields(s)
	if len(fields) < 2 {
		return 0, 0, fmt.Errorf("geojson: position %q has fewer than 2 values", s)
	}
	if lat, err = strconv.ParseFloat(fields[0], 64); err != nil {
		return 0, 0, fmt.Errorf("geojson: invalid lat %q: %w", fields[0], err)
	}
	if lon, err = strconv.ParseFloat(fields[1], 64); err != nil {
		return 0, 0, fmt.Errorf("geojson: invalid lon %q: %w", fields[1], err)
	}
	return lat, lon, nil
}

// discretizeArc approximates a three-point circular arc as a geodesic
// polyline.  Points are in GML lat-lon order; returned coords are [lon,lat].
//
// The centre and radius are found planar, which is a good approximation at the
// scale of an airspace boundary; the arc itself is then walked geodesically so
// that the result is consistent with the by-centre forms.
func discretizeArc(pos []gml.DirectPosition, posList *gml.DirectPositionList, tol float64) ([][2]float64, error) {
	pts, err := arcPoints(pos, posList)
	if err != nil {
		return nil, err
	}
	if len(pts) != 3 {
		return nil, fmt.Errorf("geojson: Arc requires exactly 3 points, got %d", len(pts))
	}
	lat1, lon1 := pts[0][0], pts[0][1]
	lat2, lon2 := pts[1][0], pts[1][1]
	lat3, lon3 := pts[2][0], pts[2][1]

	cx, cy, _ := circumcenter(lon1, lat1, lon2, lat2, lon3, lat3)

	// Bearings from the centre to the three defining points.
	b1 := bearingTo(cy, cx, lat1, lon1)
	b3 := bearingTo(cy, cx, lat3, lon3)

	// The middle point disambiguates which way round the arc runs.
	cross := float64((lon2-lon1)*(lat3-lat1)) - float64((lat2-lat1)*(lon3-lon1))
	sweep := b3 - b1
	if cross > 0 { // counter-clockwise in lon/lat space
		for sweep > 0 {
			sweep -= 360
		}
	} else {
		for sweep < 0 {
			sweep += 360
		}
	}

	radius := GeodesicDistance(cy, cx, lat1, lon1)

	steps := arcPointCount(sweep, radius, tol)
	return arcPositions(cy, cx, radius, b1, sweep, steps, tol), nil
}

// discretizeCircle approximates a circle given by three points on its
// circumference as a closed geodesic ring.
func discretizeCircle(pos []gml.DirectPosition, posList *gml.DirectPositionList, tol float64) ([][2]float64, error) {
	pts, err := arcPoints(pos, posList)
	if err != nil {
		return nil, err
	}
	if len(pts) != 3 {
		return nil, fmt.Errorf("geojson: Circle requires exactly 3 points, got %d", len(pts))
	}
	lat1, lon1 := pts[0][0], pts[0][1]
	lat2, lon2 := pts[1][0], pts[1][1]
	lat3, lon3 := pts[2][0], pts[2][1]

	cx, cy, _ := circumcenter(lon1, lat1, lon2, lat2, lon3, lat3)
	radius := GeodesicDistance(cy, cx, lat1, lon1)
	start := bearingTo(cy, cx, lat1, lon1)

	n := circlePointCount(radius, tol)
	return circlePositions(cy, cx, radius, start, n, tol), nil
}

// bearingTo returns the initial true bearing, in degrees clockwise from north,
// from one position to another.
func bearingTo(lat1, lon1, lat2, lon2 float64) float64 {
	phi1, phi2 := rad(lat1), rad(lat2)
	dLambda := rad(lon2 - lon1)
	y := float64(dsin(dLambda) * dcos(phi2))
	x := float64(dcos(phi1)*dsin(phi2)) - float64(dsin(phi1)*dcos(phi2)*dcos(dLambda))
	b := deg(datan2(y, x))
	if b < 0 {
		b += 360
	}
	return b
}

// arcPoints extracts lat-lon pairs from pos/posList, returning [][lat,lon].
func arcPoints(pos []gml.DirectPosition, posList *gml.DirectPositionList) ([][2]float64, error) {
	if posList != nil {
		return posListToLatLon(posList.Value)
	}
	if len(pos) > 0 {
		return posSliceToLatLon(pos)
	}
	return nil, fmt.Errorf("geojson: arc has no positions")
}

// circumcenter computes the circumcenter (cx,cy) and radius r of the circle
// passing through (x1,y1), (x2,y2), (x3,y3).
func circumcenter(x1, y1, x2, y2, x3, y3 float64) (cx, cy, r float64) {
	ax, ay := x2-x1, y2-y1
	bx, by := x3-x1, y3-y1
	D := 2 * (float64(ax*by) - float64(ay*bx))
	if math.Abs(D) < 1e-12 {
		// Degenerate / collinear points — return midpoint and zero radius.
		return (x1 + x3) / 2, (y1 + y3) / 2, 0
	}
	ux := (float64(by*(float64(ax*ax)+float64(ay*ay))) - float64(ay*(float64(bx*bx)+float64(by*by)))) / D
	uy := (float64(ax*(float64(bx*bx)+float64(by*by))) - float64(bx*(float64(ax*ax)+float64(ay*ay)))) / D
	cx = x1 + ux
	cy = y1 + uy
	r = dhypot(ux, uy)
	return
}
