package geo

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/davidkohl/goaixm/gml"
)

// ---- Arc and circle discretization ------------------------------------------

// arcPointCount returns the number of segments used to approximate an arc of
// the given sweep, scaled so that a full circle uses DefaultArcPoints.
func arcPointCount(sweepDeg float64) int {
	n := int(math.Ceil(float64(DefaultArcPoints) * math.Abs(sweepDeg) / 360))
	if n < arcMinPoints {
		n = arcMinPoints
	}
	return n
}

// discretizeArcByCenter approximates a gml:ArcByCenterPoint as a geodesic
// polyline running clockwise from its start bearing to its end bearing.
func discretizeArcByCenter(a *gml.ArcByCenterPoint, res centreResolver) ([][2]float64, error) {
	lat, lon, err := arcCentre(a.Pos, a.PosList, a.PointProperty, a.PointRep, res)
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

	n := arcPointCount(sweep)
	coords := make([][2]float64, 0, n+1)
	for i := 0; i <= n; i++ {
		bearing := start + sweep*float64(i)/float64(n)
		pLat, pLon := DestinationPoint(lat, lon, bearing, radius)
		coords = append(coords, [2]float64{pLon, pLat})
	}
	return coords, nil
}

// discretizeCircleByCenter approximates a gml:CircleByCenterPoint as a closed
// geodesic ring.  The final position repeats the first exactly.
func discretizeCircleByCenter(c *gml.CircleByCenterPoint, res centreResolver) ([][2]float64, error) {
	lat, lon, err := arcCentre(c.Pos, c.PosList, c.PointProperty, c.PointRep, res)
	if err != nil {
		return nil, err
	}
	radius, err := LengthToMetres(c.Radius.Value, c.Radius.UOM)
	if err != nil {
		return nil, fmt.Errorf("geojson: CircleByCenterPoint radius: %w", err)
	}

	n := DefaultArcPoints
	coords := make([][2]float64, 0, n+1)
	for i := 0; i < n; i++ {
		bearing := 360 * float64(i) / float64(n)
		pLat, pLon := DestinationPoint(lat, lon, bearing, radius)
		coords = append(coords, [2]float64{pLon, pLat})
	}
	// Close the ring on the first position rather than recomputing it, so the
	// coincidence is exact.
	return append(coords, coords[0]), nil
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
func discretizeArc(pos []gml.DirectPosition, posList *gml.DirectPositionList, n int) ([][2]float64, error) {
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
	cross := (lon2-lon1)*(lat3-lat1) - (lat2-lat1)*(lon3-lon1)
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

	// n is the count for a full turn; scale it to the sweep actually covered.
	steps := int(math.Ceil(float64(n) * math.Abs(sweep) / 360))
	if steps < arcMinPoints {
		steps = arcMinPoints
	}

	coords := make([][2]float64, 0, steps+1)
	for i := 0; i <= steps; i++ {
		bearing := b1 + sweep*float64(i)/float64(steps)
		pLat, pLon := DestinationPoint(cy, cx, bearing, radius)
		coords = append(coords, [2]float64{pLon, pLat})
	}
	return coords, nil
}

// discretizeCircle approximates a circle given by three points on its
// circumference as a closed geodesic ring.
func discretizeCircle(pos []gml.DirectPosition, posList *gml.DirectPositionList, n int) ([][2]float64, error) {
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

	coords := make([][2]float64, 0, n+1)
	for i := 0; i < n; i++ {
		bearing := start + 360*float64(i)/float64(n)
		pLat, pLon := DestinationPoint(cy, cx, bearing, radius)
		coords = append(coords, [2]float64{pLon, pLat})
	}
	return append(coords, coords[0]), nil
}

// bearingTo returns the initial true bearing, in degrees clockwise from north,
// from one position to another.
func bearingTo(lat1, lon1, lat2, lon2 float64) float64 {
	phi1, phi2 := rad(lat1), rad(lat2)
	dLambda := rad(lon2 - lon1)
	y := math.Sin(dLambda) * math.Cos(phi2)
	x := math.Cos(phi1)*math.Sin(phi2) - math.Sin(phi1)*math.Cos(phi2)*math.Cos(dLambda)
	b := deg(math.Atan2(y, x))
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
	D := 2 * (ax*by - ay*bx)
	if math.Abs(D) < 1e-12 {
		// Degenerate / collinear points — return midpoint and zero radius.
		return (x1 + x3) / 2, (y1 + y3) / 2, 0
	}
	ux := (by*(ax*ax+ay*ay) - ay*(bx*bx+by*by)) / D
	uy := (ax*(bx*bx+by*by) - bx*(ax*ax+ay*ay)) / D
	cx = x1 + ux
	cy = y1 + uy
	r = math.Hypot(ux, uy)
	return
}
