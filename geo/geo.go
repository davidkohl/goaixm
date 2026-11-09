// Package geo converts AIXM geometry to coordinates and RFC 7946 GeoJSON.
//
// Airspaces are assembled from their volumes, including arcs and circles
// (discretised geodesically on the WGS-84 ellipsoid), referenced GeoBorders
// and contributor airspaces:
//
//	fc, err := geo.AirspaceToGeoJSONWithRefs(airspace, msg)
//
// The lower-level helpers (FromPoint, SegmentCoords, SurfacePolygons, …)
// work on any aixm geometry, including that of custom extension types.
// All coordinate pairs are [lon, lat].
package geo

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/gml"
)

// Coordinate represents a WGS-84 longitude/latitude position with optional elevation.
type Coordinate struct {
	Lon, Lat float64
	Elev     float64 // metres above MSL; 0 if unknown
}

// FromElevatedPoint extracts the coordinate and elevation of an
// aixm:ElevatedPoint.
func FromElevatedPoint(p *aixm.ElevatedPoint) (Coordinate, error) {
	if p == nil {
		return Coordinate{}, fmt.Errorf("geo: nil ElevatedPoint")
	}
	if p.Pos == nil {
		return Coordinate{}, fmt.Errorf("geo: ElevatedPoint has no pos")
	}
	coords, err := parsePosList(p.Pos.Value, p.SrsDimension)
	if err != nil || len(coords) < 2 {
		return Coordinate{}, fmt.Errorf("geo: parse pos: %w", err)
	}
	c := Coordinate{Lat: coords[0], Lon: coords[1]}
	if len(coords) >= 3 {
		c.Elev = coords[2]
	}
	return c, nil
}

// FromElevatedSurface extracts the exterior ring of an aixm:ElevatedSurface.
func FromElevatedSurface(s *aixm.ElevatedSurface) ([]Coordinate, error) {
	if s == nil {
		return nil, fmt.Errorf("geo: nil ElevatedSurface")
	}
	return extractSurface(s.Patches, s.SrsDimension)
}

// FromSurface extracts the exterior ring of an aixm:Surface.
func FromSurface(s *aixm.Surface) ([]Coordinate, error) {
	if s == nil {
		return nil, fmt.Errorf("geo: nil Surface")
	}
	return extractSurface(s.Patches, s.SrsDimension)
}

func extractSurface(patches *gml.SurfacePatches, dim uint) ([]Coordinate, error) {
	if patches == nil || len(patches.PolygonPatch) == 0 {
		return nil, fmt.Errorf("geo: surface has no patches")
	}
	patch := patches.PolygonPatch[0]
	if patch.Exterior == nil || patch.Exterior.LinearRing == nil {
		return nil, fmt.Errorf("geo: patch has no exterior ring")
	}
	ring := patch.Exterior.LinearRing
	if ring.PosList == nil {
		return nil, fmt.Errorf("geo: linear ring has no posList")
	}
	vals, err := parsePosList(ring.PosList.Value, dim)
	if err != nil {
		return nil, err
	}
	stride := 2
	if dim == 3 {
		stride = 3
	}
	if len(vals)%stride != 0 {
		return nil, fmt.Errorf("geo: posList length %d not divisible by stride %d", len(vals), stride)
	}
	coords := make([]Coordinate, 0, len(vals)/stride)
	for i := 0; i+stride <= len(vals); i += stride {
		c := Coordinate{Lat: vals[i], Lon: vals[i+1]}
		if stride == 3 {
			c.Elev = vals[i+2]
		}
		coords = append(coords, c)
	}
	return coords, nil
}

// WKTPoint renders a Coordinate as a WKT POINT string.
func WKTPoint(c Coordinate) string {
	return fmt.Sprintf("POINT(%g %g)", c.Lon, c.Lat)
}

// WKTPolygon renders a coordinate ring as a WKT POLYGON string.
func WKTPolygon(coords []Coordinate) string {
	if len(coords) == 0 {
		return "POLYGON EMPTY"
	}
	parts := make([]string, len(coords))
	for i, c := range coords {
		parts[i] = fmt.Sprintf("%g %g", c.Lon, c.Lat)
	}
	return fmt.Sprintf("POLYGON((%s))", strings.Join(parts, ","))
}

// BoundingBox returns the axis-aligned bounding box of a set of coordinates.
func BoundingBox(coords []Coordinate) (minLon, minLat, maxLon, maxLat float64) {
	if len(coords) == 0 {
		return
	}
	minLon, minLat = coords[0].Lon, coords[0].Lat
	maxLon, maxLat = coords[0].Lon, coords[0].Lat
	for _, c := range coords[1:] {
		if c.Lon < minLon {
			minLon = c.Lon
		}
		if c.Lat < minLat {
			minLat = c.Lat
		}
		if c.Lon > maxLon {
			maxLon = c.Lon
		}
		if c.Lat > maxLat {
			maxLat = c.Lat
		}
	}
	return
}

// parsePosList parses a space-separated list of doubles.
// srsDimension hint is informational; we always parse what's there.
func parsePosList(s string, _ uint) ([]float64, error) {
	fields := strings.Fields(s)
	vals := make([]float64, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return nil, fmt.Errorf("geo: invalid coordinate %q: %w", f, err)
		}
		vals = append(vals, v)
	}
	return vals, nil
}
