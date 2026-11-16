package geo

import (
	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/gml"
)

// Options control how curves are turned into coordinates.  The zero value
// keeps the historical behaviour: arcs and circles with DefaultArcPoints
// points per full turn, geodesic segments as their vertices only.
//
// Every source vertex is kept exactly as given; densification only inserts
// computed positions between them.
//
//	opt := geo.Options{ArcTolerance: 1, GeodesicTolerance: 1}
//	comps, err := opt.AirspaceComponents(airspace, msg)
type Options struct {
	// ArcTolerance, in metres, is the largest distance allowed between a
	// discretised arc or circle (ArcByCenterPoint, CircleByCenterPoint,
	// Arc, Circle) and the true curve.  Positions are placed on the
	// geodesic circle of the given radius around the centre (Vincenty
	// direct on WGS-84) at equal bearing steps Δθ, with
	//
	//	r · (1 − cos(Δθ/2)) ≤ ArcTolerance
	//
	// the sagitta of a chord of a plane circle of radius r.  A geodesic
	// circle of radius r ≪ earth radius curves slightly less than a plane
	// circle of radius r, so the sagitta is an upper bound of the chord
	// error.  Arcs keep at least 8 segments.  0 uses DefaultArcPoints.
	ArcTolerance float64

	// GeodesicTolerance, in metres, is the largest distance allowed
	// between a geodesic segment (gml:GeodesicString, and
	// gml:LineStringSegment with interpolation="geodesic") and the planar
	// lon/lat polyline that represents it, as consumers of GeoJSON draw
	// it (RFC 7946 §3.1.1: straight lines in lon/lat).  A segment whose
	// deviation exceeds the tolerance is split into n equal geodesic
	// pieces (positions by Vincenty direct along the segment's geodesic);
	// the deviation of each piece is measured at its geodesic midpoint
	// (perpendicular distance to the straight lon/lat chord, local metric
	// of the WGS-84 ellipsoid), where it peaks, and n grows until every
	// piece is within the tolerance.  Segments crossing the antimeridian
	// are left undensified.  0 leaves geodesic segments as their vertices.
	//
	// GeoBorder curves are made of the same segment types and are
	// densified by the same rule.  LineStringSegment without
	// interpolation="geodesic" (GML default: linear) is never densified.
	GeodesicTolerance float64

	// PreferSourceVertices keeps the explicit source position where a
	// computed arc or circle end coincides with it (within 1e-9°, where the
	// two are joined into one position).  Without it the position first in
	// the ring wins, which after an arc is the computed end (it may differ
	// from the source coordinate in the tenth decimal).
	PreferSourceVertices bool
}

// maxDensifyPoints bounds the positions inserted into one arc, circle or
// geodesic segment, whatever the tolerance.
const maxDensifyPoints = 1 << 16

// conv carries what a conversion needs besides its input: the resolver for
// referenced arc centres and the options.
type conv struct {
	res centreResolver
	opt Options
	// computed holds the end positions of discretised arcs and circles
	// (only with PreferSourceVertices).
	computed map[[2]float64]bool
}

// markComputed records the ends of a discretised curve.
func (cv *conv) markComputed(c [][2]float64) {
	if !cv.opt.PreferSourceVertices || len(c) == 0 {
		return
	}
	if cv.computed == nil {
		cv.computed = map[[2]float64]bool{}
	}
	cv.computed[c[0]], cv.computed[c[len(c)-1]] = true, true
}

// appendDedup is the package appendDedup, keeping a source position
// instead of a coinciding computed one when PreferSourceVertices is set.
func (cv *conv) appendDedup(dst, src [][2]float64) [][2]float64 {
	if cv != nil && len(dst) > 0 && len(src) > 0 && samePoint(dst[len(dst)-1], src[0]) &&
		cv.computed[dst[len(dst)-1]] && !cv.computed[src[0]] {
		dst[len(dst)-1] = src[0]
	}
	return appendDedup(dst, src)
}

func (o Options) conv(msg *goaixm.Message) *conv {
	return &conv{res: centreResolverFor(msg), opt: o}
}

// SegmentCoords is the package-level SegmentCoords with these options.
func (o Options) SegmentCoords(segs *gml.CurveSegments, msg *goaixm.Message) ([][2]float64, error) {
	return segmentsToCoords(segs, o.conv(msg), msg)
}

// CurveCoords is the package-level CurveCoords with these options.
func (o Options) CurveCoords(c *aixm.CurveProperty, msg *goaixm.Message) ([][2]float64, error) {
	return curveCoords(c, o, msg)
}

// SurfaceRings is the package-level SurfaceRings with these options.
func (o Options) SurfaceRings(patches *gml.SurfacePatches, msg *goaixm.Message) ([][][2]float64, error) {
	return extractAllRings(patches, o.conv(msg), msg, nil)
}

// SurfacePolygons is the package-level SurfacePolygons with these options.
func (o Options) SurfacePolygons(patches *gml.SurfacePatches, msg *goaixm.Message) ([][][][2]float64, error) {
	return surfacePolygons(patches, o, msg)
}

// AirspaceComponents is the package-level AirspaceComponents with these
// options.
func (o Options) AirspaceComponents(a *aixm.Airspace, msg *goaixm.Message) ([]AirspaceComponent, error) {
	return airspaceComponents(a, o, msg)
}

// AirspaceToGeoJSON is the package-level AirspaceToGeoJSON with these
// options.
func (o Options) AirspaceToGeoJSON(a *aixm.Airspace) (*FeatureCollection, error) {
	return airspaceToGeoJSON(a, nil, false, o)
}

// AirspaceToGeoJSONWithRefs is the package-level AirspaceToGeoJSONWithRefs
// with these options.
func (o Options) AirspaceToGeoJSONWithRefs(a *aixm.Airspace, msg *goaixm.Message) (*FeatureCollection, error) {
	return airspaceToGeoJSON(a, msg, true, o)
}
