package geo

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/gml"
	"gitlab.com/davidkohl/goaixm/parse"
)

// DefaultArcPoints is the number of interpolated points used to approximate a
// circular arc or full circle when converting to a polygon.  Callers that need
// finer or coarser resolution should call SetArcPoints.
var DefaultArcPoints = 64

// GeoJSONFeature is a GeoJSON Feature object.
type GeoJSONFeature struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Geometry   any            `json:"geometry"`
}

// GeoJSONPolygon is a GeoJSON Polygon geometry (exterior + optional holes).
type GeoJSONPolygon struct {
	Type        string          `json:"type"`
	Coordinates [][][2]float64  `json:"coordinates"`
}

// GeoJSONMultiPolygon is a GeoJSON MultiPolygon geometry.
type GeoJSONMultiPolygon struct {
	Type        string            `json:"type"`
	Coordinates [][][][2]float64  `json:"coordinates"`
}

// GeoJSONFeatureCollection is a GeoJSON FeatureCollection.
type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

// AirspaceToGeoJSON converts an AirspaceType to a GeoJSON FeatureCollection,
// one Feature per geometry component.  Each polygon includes interior rings
// (holes) when present.
func AirspaceToGeoJSON(a *features.AirspaceType) (*GeoJSONFeatureCollection, error) {
	if len(a.TimeSlice) == 0 {
		return nil, fmt.Errorf("geojson: airspace has no timeslices")
	}
	ts := a.TimeSlice[0].AirspaceTimeSlice

	props := airspaceProps(a, ts)
	fc := &GeoJSONFeatureCollection{Type: "FeatureCollection"}

	for _, gc := range ts.GeometryComponent {
		agc := gc.AirspaceGeometryComponent
		if agc == nil {
			continue
		}
		vol := agc.TheAirspaceVolume.AirspaceVolume
		if vol == nil {
			continue
		}
		surf := vol.HorizontalProjection.Surface
		if surf == nil {
			continue
		}

		rings, err := extractAllRings(surf)
		if err != nil {
			return nil, err
		}

		feat := GeoJSONFeature{
			Type:       "Feature",
			Properties: enrichWithLimits(props, vol),
			Geometry: GeoJSONPolygon{
				Type:        "Polygon",
				Coordinates: rings,
			},
		}
		fc.Features = append(fc.Features, feat)
	}

	if len(fc.Features) == 0 {
		return nil, fmt.Errorf("geojson: no geometry found for airspace %s", a.GetIdentifier())
	}
	return fc, nil
}

// AirspaceToGeoJSONWithRefs converts an airspace to GeoJSON, resolving any
// FULL_GEOMETRY xlink:href references to sub-airspaces from msg.
// When referenced geometry is found the output geometry is a MultiPolygon.
func AirspaceToGeoJSONWithRefs(a *features.AirspaceType, msg *parse.Message) (*GeoJSONFeatureCollection, error) {
	if len(a.TimeSlice) == 0 {
		return nil, fmt.Errorf("geojson: airspace has no timeslices")
	}
	ts := a.TimeSlice[0].AirspaceTimeSlice
	props := airspaceProps(a, ts)

	// Collect all rings from all geometry components (inline + resolved xlinks).
	// Each entry is one polygon: [exterior, hole0, hole1, ...]
	var allPolygons [][][][2]float64

	for _, gc := range ts.GeometryComponent {
		agc := gc.AirspaceGeometryComponent
		if agc == nil {
			// Try to resolve via href.
			if gc.HrefAttr != "" && msg != nil {
				resolved, ok := msg.Resolve(gc.HrefAttr)
				if ok {
					if sub, ok := resolved.(*features.AirspaceType); ok {
						subFC, err := AirspaceToGeoJSON(sub)
						if err == nil {
							for _, f := range subFC.Features {
								if poly, ok := f.Geometry.(GeoJSONPolygon); ok {
									allPolygons = append(allPolygons, poly.Coordinates)
								}
							}
						}
					}
				}
			}
			continue
		}
		vol := agc.TheAirspaceVolume.AirspaceVolume
		if vol == nil {
			continue
		}
		surf := vol.HorizontalProjection.Surface
		if surf == nil {
			continue
		}
		rings, err := extractAllRings(surf)
		if err != nil {
			return nil, err
		}
		allPolygons = append(allPolygons, rings)
	}

	if len(allPolygons) == 0 {
		return nil, fmt.Errorf("geojson: no geometry found for airspace %s", a.GetIdentifier())
	}

	fc := &GeoJSONFeatureCollection{Type: "FeatureCollection"}
	if len(allPolygons) == 1 {
		fc.Features = append(fc.Features, GeoJSONFeature{
			Type:       "Feature",
			Properties: props,
			Geometry:   GeoJSONPolygon{Type: "Polygon", Coordinates: allPolygons[0]},
		})
	} else {
		fc.Features = append(fc.Features, GeoJSONFeature{
			Type:       "Feature",
			Properties: props,
			Geometry:   GeoJSONMultiPolygon{Type: "MultiPolygon", Coordinates: allPolygons},
		})
	}
	return fc, nil
}

// MarshalGeoJSON marshals a GeoJSONFeatureCollection to indented JSON.
func MarshalGeoJSON(fc *GeoJSONFeatureCollection) ([]byte, error) {
	return json.MarshalIndent(fc, "", "  ")
}

// ---- Internal helpers -------------------------------------------------------

func airspaceProps(a *features.AirspaceType, ts features.AirspaceTimeSliceType) map[string]any {
	props := map[string]any{"identifier": a.GetIdentifier()}
	if ts.Designator != nil {
		props["designator"] = ts.Designator.Value
	}
	if ts.Name != nil {
		props["name"] = strings.TrimSpace(ts.Name.Value)
	}
	if ts.Type != nil {
		props["type"] = strings.TrimSpace(ts.Type.Value)
	}
	props["interpretation"] = ts.Interpretation
	return props
}

func enrichWithLimits(base map[string]any, vol *features.AirspaceVolumeType) map[string]any {
	out := make(map[string]any, len(base)+4)
	for k, v := range base {
		out[k] = v
	}
	if vol.UpperLimit != nil {
		out["upperLimit"] = vol.UpperLimit.Value
		if vol.UpperLimitReference != nil {
			out["upperLimitReference"] = vol.UpperLimitReference.Value
		}
	}
	if vol.LowerLimit != nil {
		out["lowerLimit"] = vol.LowerLimit.Value
		if vol.LowerLimitReference != nil {
			out["lowerLimitReference"] = vol.LowerLimitReference.Value
		}
	}
	return out
}

// extractAllRings returns the exterior ring at index 0 followed by any
// interior rings (holes) from the first patch of surf.
func extractAllRings(surf *features.SurfaceType) ([][][2]float64, error) {
	if surf.Patches == nil || len(surf.Patches.PolygonPatch) == 0 {
		return nil, fmt.Errorf("geojson: surface has no patches")
	}
	patch := surf.Patches.PolygonPatch[0]
	if patch.Exterior == nil {
		return nil, fmt.Errorf("geojson: patch has no exterior")
	}
	exterior, err := extractAbstractRingCoords(patch.Exterior)
	if err != nil {
		return nil, err
	}
	rings := [][][2]float64{exterior}
	for i := range patch.Interior {
		hole, err := extractAbstractRingCoords(&patch.Interior[i])
		if err != nil {
			return nil, fmt.Errorf("geojson: interior ring %d: %w", i, err)
		}
		rings = append(rings, hole)
	}
	return rings, nil
}

func extractAbstractRingCoords(r *gml.AbstractRingPropertyType) ([][2]float64, error) {
	if r.LinearRing != nil {
		return extractLinearRingCoords(r.LinearRing)
	}
	if r.Ring != nil {
		return extractRingTypeCoords(r.Ring)
	}
	return nil, fmt.Errorf("geojson: exterior has neither LinearRing nor Ring")
}

func extractLinearRingCoords(lr *gml.LinearRingType) ([][2]float64, error) {
	if lr.PosList != nil {
		return posListToCoords(lr.PosList.Value)
	}
	if len(lr.Pos) > 0 {
		return posSliceToCoords(lr.Pos)
	}
	return nil, fmt.Errorf("geojson: LinearRing has no positions")
}

func extractRingTypeCoords(ring *gml.RingType) ([][2]float64, error) {
	var coords [][2]float64
	for _, cm := range ring.CurveMember {
		if cm.Curve == nil || cm.Curve.Segments == nil {
			continue
		}
		segs := cm.Curve.Segments

		for _, gs := range segs.GeodesicString {
			var c [][2]float64
			var err error
			if gs.PosList != nil {
				c, err = posListToCoords(gs.PosList.Value)
			} else if len(gs.Pos) > 0 {
				c, err = posSliceToCoords(gs.Pos)
			}
			if err != nil {
				return nil, err
			}
			coords = appendDedup(coords, c)
		}

		for _, ls := range segs.LineStringSegment {
			if ls.PosList == nil {
				continue
			}
			c, err := posListToCoords(ls.PosList.Value)
			if err != nil {
				return nil, err
			}
			coords = appendDedup(coords, c)
		}

		for _, arc := range segs.Arc {
			c, err := discretizeArc(arc.Pos, arc.PosList, DefaultArcPoints)
			if err != nil {
				return nil, err
			}
			coords = appendDedup(coords, c)
		}

		for _, circle := range segs.Circle {
			c, err := discretizeCircle(circle.Pos, circle.PosList, DefaultArcPoints)
			if err != nil {
				return nil, err
			}
			coords = appendDedup(coords, c)
		}
	}
	if len(coords) == 0 {
		return nil, fmt.Errorf("geojson: Ring has no coordinates")
	}
	return coords, nil
}

// appendDedup appends src to dst, skipping the first point of src if it
// duplicates the last point of dst (avoids repeated join points between segments).
func appendDedup(dst, src [][2]float64) [][2]float64 {
	if len(src) == 0 {
		return dst
	}
	if len(dst) > 0 && dst[len(dst)-1] == src[0] {
		src = src[1:]
	}
	return append(dst, src...)
}

// ---- Arc/Circle discretization ---------------------------------------------

// discretizeArc approximates a three-point circular arc as n line segments.
// Points are in GML lat-lon order; returned coords are [lon,lat].
func discretizeArc(pos []gml.DirectPositionType, posList *gml.DirectPositionListType, n int) ([][2]float64, error) {
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

	cx, cy, r := circumcenter(lon1, lat1, lon2, lat2, lon3, lat3)

	a1 := math.Atan2(lat1-cy, lon1-cx)
	a2 := math.Atan2(lat2-cy, lon2-cx)
	a3 := math.Atan2(lat3-cy, lon3-cx)

	// Determine direction (clockwise vs counter-clockwise).
	cross := (lon2-lon1)*(lat3-lat1) - (lat2-lat1)*(lon3-lon1)
	if cross < 0 {
		// Clockwise: normalise so a1 > a3.
		for a3 > a1 {
			a3 -= 2 * math.Pi
		}
		_ = a2
	} else {
		for a3 < a1 {
			a3 += 2 * math.Pi
		}
		_ = a2
	}

	coords := make([][2]float64, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		angle := a1 + t*(a3-a1)
		lon := cx + r*math.Cos(angle)
		lat := cy + r*math.Sin(angle)
		coords[i] = [2]float64{lon, lat}
	}
	return coords, nil
}

// discretizeCircle approximates a full circle (3 points on circumference) as
// n line segments.
func discretizeCircle(pos []gml.DirectPositionType, posList *gml.DirectPositionListType, n int) ([][2]float64, error) {
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

	cx, cy, r := circumcenter(lon1, lat1, lon2, lat2, lon3, lat3)

	coords := make([][2]float64, n+1)
	for i := 0; i <= n; i++ {
		angle := 2 * math.Pi * float64(i) / float64(n)
		coords[i] = [2]float64{cx + r*math.Cos(angle), cy + r*math.Sin(angle)}
	}
	return coords, nil
}

// arcPoints extracts lat-lon pairs from pos/posList, returning [][lat,lon].
func arcPoints(pos []gml.DirectPositionType, posList *gml.DirectPositionListType) ([][2]float64, error) {
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

// ---- Coordinate parsing (internal, lat-lon order) --------------------------

// posListToLatLon returns [][lat,lon] (NOT swapped — internal use only).
func posListToLatLon(s string) ([][2]float64, error) {
	fields := strings.Fields(s)
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("geojson: posList has odd number of values")
	}
	pts := make([][2]float64, 0, len(fields)/2)
	for i := 0; i+1 < len(fields); i += 2 {
		lat, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return nil, err
		}
		lon, err := strconv.ParseFloat(fields[i+1], 64)
		if err != nil {
			return nil, err
		}
		pts = append(pts, [2]float64{lat, lon})
	}
	return pts, nil
}

func posSliceToLatLon(pos []gml.DirectPositionType) ([][2]float64, error) {
	pts := make([][2]float64, 0, len(pos))
	for _, p := range pos {
		fields := strings.Fields(p.Value)
		if len(fields) < 2 {
			return nil, fmt.Errorf("geojson: pos %q has fewer than 2 values", p.Value)
		}
		lat, _ := strconv.ParseFloat(fields[0], 64)
		lon, _ := strconv.ParseFloat(fields[1], 64)
		pts = append(pts, [2]float64{lat, lon})
	}
	return pts, nil
}

// posListToCoords parses a GML posList (lat lon lat lon ...) into [lon,lat] pairs.
func posListToCoords(s string) ([][2]float64, error) {
	fields := strings.Fields(s)
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("geojson: posList has odd number of values")
	}
	coords := make([][2]float64, 0, len(fields)/2)
	for i := 0; i+1 < len(fields); i += 2 {
		lat, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return nil, fmt.Errorf("geojson: invalid lat %q: %w", fields[i], err)
		}
		lon, err := strconv.ParseFloat(fields[i+1], 64)
		if err != nil {
			return nil, fmt.Errorf("geojson: invalid lon %q: %w", fields[i+1], err)
		}
		coords = append(coords, [2]float64{lon, lat})
	}
	return coords, nil
}

// posSliceToCoords converts a slice of DirectPositionType (lat lon) to [lon,lat] pairs.
func posSliceToCoords(pos []gml.DirectPositionType) ([][2]float64, error) {
	coords := make([][2]float64, 0, len(pos))
	for _, p := range pos {
		fields := strings.Fields(p.Value)
		if len(fields) < 2 {
			return nil, fmt.Errorf("geojson: pos %q has fewer than 2 values", p.Value)
		}
		lat, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return nil, fmt.Errorf("geojson: invalid lat %q: %w", fields[0], err)
		}
		lon, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, fmt.Errorf("geojson: invalid lon %q: %w", fields[1], err)
		}
		coords = append(coords, [2]float64{lon, lat})
	}
	return coords, nil
}
