package geo

import (
	"fmt"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/gml"
	"gitlab.com/davidkohl/goaixm/parse"
)

// This file exposes the geometry building blocks used by the GeoJSON
// conversion, so callers working with their own (extension) types can turn
// any AIXM geometry into coordinates.  All coordinates are [lon, lat].
// msg may be nil; it is only needed to resolve geometry given by reference
// (GeoBorders, arc centres held in other features).

// FromPointType extracts the coordinate from an aixm:Point.
func FromPointType(p *features.PointType) (Coordinate, error) {
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

// SegmentCoords flattens GML curve segments (LineStringSegment,
// GeodesicString, Arc, Circle, ArcByCenterPoint, CircleByCenterPoint) into a
// polyline.  Arcs are discretised with DefaultArcPoints resolution.
func SegmentCoords(segs *gml.CurveSegmentsType, msg *parse.Message) ([][2]float64, error) {
	return segmentsToCoords(segs, centreResolverFor(msg), msg)
}

// CurveCoords flattens an aixm:Curve into a polyline.
func CurveCoords(c *features.CurveType, msg *parse.Message) ([][2]float64, error) {
	if c == nil {
		return nil, fmt.Errorf("geo: nil Curve")
	}
	return SegmentCoords(c.Segments, msg)
}

// ElevatedCurveCoords flattens an aixm:ElevatedCurve into a polyline.
func ElevatedCurveCoords(c *features.ElevatedCurveType, msg *parse.Message) ([][2]float64, error) {
	if c == nil {
		return nil, fmt.Errorf("geo: nil ElevatedCurve")
	}
	return SegmentCoords(c.Segments, msg)
}

// SurfaceRings returns the closed, RFC 7946 oriented rings (exterior first,
// then holes) of the first polygon patch of a surface.
func SurfaceRings(patches *gml.SurfacePatchesType, msg *parse.Message) ([][][2]float64, error) {
	return extractAllRings(patches, centreResolverFor(msg), msg)
}

// SurfacePolygons returns one polygon (list of rings) per polygon patch.
func SurfacePolygons(patches *gml.SurfacePatchesType, msg *parse.Message) ([][][][2]float64, error) {
	if patches == nil || len(patches.PolygonPatch) == 0 {
		return nil, fmt.Errorf("geo: surface has no patches")
	}
	res := centreResolverFor(msg)
	var out [][][][2]float64
	for i := range patches.PolygonPatch {
		single := &gml.SurfacePatchesType{PolygonPatch: patches.PolygonPatch[i : i+1]}
		rings, err := extractAllRings(single, res, msg)
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
	// Volume is the inline volume, nil when the component is a reference.
	Volume *features.AirspaceVolumeType
	// Geometry is GeoJSONPolygon, GeoJSONMultiPolygon, GeoJSONLineString
	// (corridor centreline) or nil.
	Geometry any
}

// AirspaceComponents returns the components of the airspace's first
// timeslice in operationSequence order, each converted to geometry, without
// combining them.  Unlike AirspaceToGeoJSONWithRefs it accepts INTERS and
// SUBTR components; callers evaluate the operations themselves, e.g. with
// PostGIS ST_Intersection / ST_Difference.
func AirspaceComponents(a *features.AirspaceType, msg *parse.Message) ([]AirspaceComponent, error) {
	if a == nil || len(a.TimeSlice) == 0 {
		return nil, fmt.Errorf("geo: airspace has no timeslices")
	}
	ts := a.TimeSlice[0].AirspaceTimeSlice
	res := centreResolverFor(msg)

	var out []AirspaceComponent
	for _, gc := range orderedComponents(ts.GeometryComponent) {
		c := AirspaceComponent{Sequence: componentSequence(gc)}
		agc := gc.AirspaceGeometryComponent
		if agc == nil {
			if gc.HrefAttr == "" || msg == nil {
				continue
			}
			sub, ok := msg.Resolve(gc.HrefAttr)
			if !ok {
				continue
			}
			subA, ok := sub.(*features.AirspaceType)
			if !ok {
				continue
			}
			fc, err := AirspaceToGeoJSONWithRefs(subA, msg)
			if err != nil {
				return nil, err
			}
			c.Geometry = multiOf(polygonsOf(fc))
			out = append(out, c)
			continue
		}
		c.Operation = codeValue(agc.Operation)
		c.Volume = agc.TheAirspaceVolume.AirspaceVolume
		if c.Volume != nil {
			g, err := volumeGeometry(c.Volume, res, msg, 0)
			if err != nil {
				return nil, err
			}
			c.Geometry = g
		}
		out = append(out, c)
	}
	return out, nil
}

func multiOf(polys [][][][2]float64) any {
	switch len(polys) {
	case 0:
		return nil
	case 1:
		return GeoJSONPolygon{Type: "Polygon", Coordinates: polys[0]}
	default:
		return GeoJSONMultiPolygon{Type: "MultiPolygon", Coordinates: polys}
	}
}
