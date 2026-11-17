package geo

import (
	"fmt"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/gml"
)

// The building blocks below work on any AIXM geometry, so callers using
// their own (extension) types can convert it too.  Coordinates are [lon,
// lat].  msg may be nil; it is only needed to resolve geometry given by
// reference (GeoBorders, arc centres held in other features).

// FromPoint extracts the coordinate of an aixm:Point.
func FromPoint(p *aixm.Point) (Coordinate, error) {
	if p == nil {
		return Coordinate{}, fmt.Errorf("geo: nil Point")
	}
	if p.Pos == nil {
		return Coordinate{}, fmt.Errorf("geo: Point has no pos")
	}
	coords, err := parsePosList(p.Pos.Value, p.SrsDimension)
	if err != nil || len(coords) < 2 {
		return Coordinate{}, fmt.Errorf("geo: parse pos: %w", err)
	}
	return Coordinate{Lat: coords[0], Lon: coords[1]}, nil
}

// FromPointProperty extracts the coordinate of whichever point form a
// PointProperty holds.
func FromPointProperty(p *aixm.PointProperty) (Coordinate, error) {
	switch {
	case p == nil:
		return Coordinate{}, fmt.Errorf("geo: nil PointProperty")
	case p.ElevatedPoint != nil:
		return FromElevatedPoint(p.ElevatedPoint)
	default:
		return FromPoint(p.Point)
	}
}

// SegmentCoords flattens GML curve segments (LineStringSegment,
// GeodesicString, Arc, Circle, ArcByCenterPoint, CircleByCenterPoint) into a
// polyline.  Arcs are discretised with DefaultArcPoints resolution; see
// Options for densification by tolerance.
func SegmentCoords(segs *gml.CurveSegments, msg *goaixm.Message) ([][2]float64, error) {
	return Options{}.SegmentCoords(segs, msg)
}

// CurveCoords flattens whichever curve form a CurveProperty holds.
func CurveCoords(c *aixm.CurveProperty, msg *goaixm.Message) ([][2]float64, error) {
	return curveCoords(c, Options{}, msg)
}

func curveCoords(c *aixm.CurveProperty, o Options, msg *goaixm.Message) ([][2]float64, error) {
	segs := c.Segments()
	if segs == nil {
		return nil, fmt.Errorf("geo: curve property has no inline curve")
	}
	return o.SegmentCoords(segs, msg)
}

// SurfaceRings returns the closed, RFC 7946 oriented rings (exterior first,
// then holes) of the first polygon patch of a surface.
func SurfaceRings(patches *gml.SurfacePatches, msg *goaixm.Message) ([][][2]float64, error) {
	return Options{}.SurfaceRings(patches, msg)
}

// SurfacePolygons returns one polygon (list of rings) per polygon patch.
func SurfacePolygons(patches *gml.SurfacePatches, msg *goaixm.Message) ([][][][2]float64, error) {
	return surfacePolygons(patches, Options{}, msg)
}

func surfacePolygons(patches *gml.SurfacePatches, o Options, msg *goaixm.Message) ([][][][2]float64, error) {
	if patches == nil || len(patches.PolygonPatch) == 0 {
		return nil, fmt.Errorf("geo: surface has no patches")
	}
	cv := o.conv(msg)
	var out [][][][2]float64
	for i := range patches.PolygonPatch {
		cv.ringBase = 0
		for _, pp := range patches.PolygonPatch[:i] {
			cv.ringBase += 1 + len(pp.Interior)
		}
		single := &gml.SurfacePatches{PolygonPatch: patches.PolygonPatch[i : i+1]}
		rings, err := extractAllRings(single, cv, msg, nil)
		if err != nil {
			return nil, fmt.Errorf("geo: patch %d: %w", i, err)
		}
		out = append(out, rings)
	}
	return out, nil
}

// AirspaceComponent is one geometry component of an airspace, with the
// aggregation operation that combines it with the components before it.
type AirspaceComponent struct {
	// Operation is BASE, UNION, INTERS or SUBTR ("" when unspecified).
	Operation string
	// Sequence is the operationSequence (0 when unspecified).
	Sequence int
	// Volume is the component's volume.
	Volume *aixm.AirspaceVolume
	// Geometry is Polygon, MultiPolygon, LineString (corridor centreline)
	// or nil.
	Geometry any
	// Warnings describe problems that make Geometry deviate from the source,
	// e.g. ring members that do not connect.  The geometry is still usable.
	Warnings []string
	// Unresolved lists the contributor airspaces (contributorAirspace /
	// theAirspace) the message does not contain: their geometry is missing
	// from Geometry, so the component is incomplete.  The references keep
	// href and xlink title as delivered.  Empty when msg is nil (nothing
	// can be resolved then, and nothing is reported).
	Unresolved []aixm.Reference
}

// AirspaceComponents returns the components of the airspace's first time
// slice in operationSequence order, each converted to geometry, without
// combining them.  Unlike AirspaceToGeoJSONWithRefs it accepts INTERS and
// SUBTR components; callers evaluate the operations themselves, e.g. with
// PostGIS ST_Intersection / ST_Difference.
func AirspaceComponents(a *aixm.Airspace, msg *goaixm.Message) ([]AirspaceComponent, error) {
	return airspaceComponents(a, Options{}, msg)
}

func airspaceComponents(a *aixm.Airspace, o Options, msg *goaixm.Message) ([]AirspaceComponent, error) {
	if a == nil || len(a.TimeSlice) == 0 {
		return nil, fmt.Errorf("geo: airspace has no time slices")
	}
	cv := o.conv(msg)
	var out []AirspaceComponent
	for _, gc := range orderedComponents(a.TimeSlice[0].GeometryComponent) {
		c := AirspaceComponent{
			Operation: codeValue(gc.Operation),
			Sequence:  componentSequence(gc),
			Volume:    gc.TheAirspaceVolume,
		}
		if c.Volume != nil {
			cv.ringBase = ringBase(a.TimeSlice[0].GeometryComponent, c.Volume)
			g, err := volumeGeometry(c.Volume, cv, msg, 0, func(w string) { c.Warnings = append(c.Warnings, w) })
			if err != nil {
				return nil, err
			}
			c.Geometry = g
			if ref := unresolvedContributor(c.Volume, msg); ref != nil {
				c.Unresolved = append(c.Unresolved, *ref)
			}
		}
		out = append(out, c)
	}
	return out, nil
}

// unresolvedContributor returns the contributor airspace reference of vol
// when it contributes horizontal geometry (dependency FULL_GEOMETRY,
// HORZ_PROJECTION or unspecified) but msg holds no airspace under its href.
func unresolvedContributor(vol *aixm.AirspaceVolume, msg *goaixm.Message) *aixm.Reference {
	dep := vol.ContributorAirspace
	if msg == nil || dep == nil || dep.TheAirspace == nil || dep.TheAirspace.Href == "" {
		return nil
	}
	if d := codeValue(dep.Dependency); d != "" && d != aixm.DependencyFullGeometry && d != aixm.DependencyHorzProjection {
		return nil
	}
	if resolved, ok := msg.Resolve(dep.TheAirspace.Href); ok {
		if _, ok := resolved.(*aixm.Airspace); ok {
			return nil
		}
	}
	return dep.TheAirspace
}
