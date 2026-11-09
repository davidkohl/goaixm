package geo

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/gml"
	"gitlab.com/davidkohl/goaixm/parse"
	"gitlab.com/davidkohl/goaixm/types"
)

// DefaultArcPoints is the number of interpolated points used to approximate a
// full 360-degree circle.  Arcs spanning less than a full turn use
// proportionally fewer points, down to a floor of arcMinPoints.
var DefaultArcPoints = 64

// arcMinPoints is the fewest points used for an arc of any sweep, so that very
// short arcs still read as curves rather than as single chords.
const arcMinPoints = 8

// joinTolerance is the angular distance, in degrees, within which two
// positions are treated as the same point when joining curve segments.  It
// corresponds to roughly a tenth of a millimetre on the ground.
//
// Segment endpoints computed geodesically will not be bit-identical to the
// hand-written coordinates that follow them in the source data, so an exact
// comparison would leave a duplicate vertex at every join.
const joinTolerance = 1e-9

// maxContributorDepth bounds the recursion when an airspace aggregates the
// geometry of other airspaces, which may themselves aggregate further.
const maxContributorDepth = 8

// GeoJSONFeature is a GeoJSON Feature object.
type GeoJSONFeature struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Geometry   any            `json:"geometry"`
}

// GeoJSONPolygon is a GeoJSON Polygon geometry (exterior + optional holes).
type GeoJSONPolygon struct {
	Type        string         `json:"type"`
	Coordinates [][][2]float64 `json:"coordinates"`
}

// GeoJSONMultiPolygon is a GeoJSON MultiPolygon geometry.
type GeoJSONMultiPolygon struct {
	Type        string           `json:"type"`
	Coordinates [][][][2]float64 `json:"coordinates"`
}

// GeoJSONLineString is a GeoJSON LineString geometry.
type GeoJSONLineString struct {
	Type        string       `json:"type"`
	Coordinates [][2]float64 `json:"coordinates"`
}

// GeoJSONFeatureCollection is a GeoJSON FeatureCollection.
type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

// centreResolver resolves an xlink:href to a latitude and longitude.  It is
// used for arc centres given as a gml:pointProperty reference rather than
// inline coordinates.
type centreResolver func(href string) (lat, lon float64, ok bool)

// noResolver resolves nothing; it is used when no message is available.
func noResolver(string) (float64, float64, bool) { return 0, 0, false }

// AirspaceToGeoJSON converts an AirspaceType to a GeoJSON FeatureCollection,
// one Feature per geometry component.  Each polygon includes interior rings
// (holes) when present.
//
// Geometry that refers to other features — an aggregated sub-airspace, a
// referenced GeoBorder, or an arc centre held in a separate Point — cannot be
// resolved without the surrounding message; use AirspaceToGeoJSONWithRefs for
// those.
func AirspaceToGeoJSON(a *features.AirspaceType) (*GeoJSONFeatureCollection, error) {
	return airspaceToGeoJSON(a, nil, false)
}

// AirspaceToGeoJSONWithRefs converts an airspace to GeoJSON, resolving
// references against msg: sub-airspaces contributed via contributorAirspace,
// GeoBorder curves referenced from a curveMember, and arc centres given by
// reference.  When more than one polygon results the geometry is a
// MultiPolygon.
func AirspaceToGeoJSONWithRefs(a *features.AirspaceType, msg *parse.Message) (*GeoJSONFeatureCollection, error) {
	return airspaceToGeoJSON(a, msg, true)
}

// airspaceToGeoJSON converts an airspace.  When merge is set, every polygon is
// gathered into a single feature — a MultiPolygon if there is more than one —
// which is how the reference-resolving form assembles an airspace from its
// contributing parts.  Otherwise each volume yields its own feature, carrying
// that volume's vertical limits.
func airspaceToGeoJSON(a *features.AirspaceType, msg *parse.Message, merge bool) (*GeoJSONFeatureCollection, error) {
	if a == nil {
		return nil, fmt.Errorf("geojson: nil airspace")
	}
	if len(a.TimeSlice) == 0 {
		return nil, fmt.Errorf("geojson: airspace has no timeslices")
	}
	ts := a.TimeSlice[0].AirspaceTimeSlice
	props := airspaceProps(a, ts)
	res := centreResolverFor(msg)

	components := orderedComponents(ts.GeometryComponent)

	var (
		polygons [][][][2]float64
		lines    []GeoJSONFeature
		firstErr error
		// volumes records the volume each polygon came from, so that a
		// single-volume airspace can report that volume's vertical limits.
		volumes []*features.AirspaceVolumeType
	)

	for _, gc := range components {
		agc := gc.AirspaceGeometryComponent
		if agc == nil {
			// A geometry component held entirely by reference.
			if gc.HrefAttr == "" || msg == nil {
				continue
			}
			sub, ok := msg.Resolve(gc.HrefAttr)
			if !ok {
				continue
			}
			if subAirspace, ok := sub.(*features.AirspaceType); ok {
				subFC, err := AirspaceToGeoJSONWithRefs(subAirspace, msg)
				if err != nil {
					continue
				}
				polygons = append(polygons, polygonsOf(subFC)...)
			}
			continue
		}

		if op := codeValue(agc.Operation); op == features.AggregationIntersection ||
			op == features.AggregationSubtraction {
			return nil, fmt.Errorf(
				"geojson: airspace %s uses unsupported aggregation operation %s",
				a.GetIdentifier(), op)
		}

		vol := agc.TheAirspaceVolume.AirspaceVolume
		if vol == nil {
			continue
		}

		geom, err := volumeGeometry(vol, res, msg, 0)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		switch g := geom.(type) {
		case nil:
			// No geometry on this component.
		case GeoJSONPolygon:
			polygons = append(polygons, g.Coordinates)
			volumes = append(volumes, vol)
		case GeoJSONMultiPolygon:
			for range g.Coordinates {
				volumes = append(volumes, vol)
			}
			polygons = append(polygons, g.Coordinates...)
		case GeoJSONLineString:
			lines = append(lines, GeoJSONFeature{
				Type:       "Feature",
				Properties: enrichWithLimits(props, vol),
				Geometry:   g,
			})
		}
	}

	fc := &GeoJSONFeatureCollection{Type: "FeatureCollection"}

	switch {
	case len(polygons) == 0:
		// nothing
	case len(polygons) == 1:
		fc.Features = append(fc.Features, GeoJSONFeature{
			Type:       "Feature",
			Properties: volumeProps(props, volumes[0]),
			Geometry:   GeoJSONPolygon{Type: "Polygon", Coordinates: polygons[0]},
		})
	case merge || sameVolume(volumes):
		// Either the caller asked for the airspace to be assembled as a whole,
		// or one volume contributed several polygons.  Both give a single
		// feature whose geometry is a MultiPolygon.
		// Vertical limits are only meaningful when every polygon shares a
		// volume; a merged geometry spanning several volumes has none.
		mergedProps := props
		if sameVolume(volumes) {
			mergedProps = volumeProps(props, volumes[0])
		}
		fc.Features = append(fc.Features, GeoJSONFeature{
			Type:       "Feature",
			Properties: mergedProps,
			Geometry:   GeoJSONMultiPolygon{Type: "MultiPolygon", Coordinates: polygons},
		})
	default:
		// Distinct volumes, each with its own vertical limits.
		for i, poly := range polygons {
			fc.Features = append(fc.Features, GeoJSONFeature{
				Type:       "Feature",
				Properties: volumeProps(props, volumes[i]),
				Geometry:   GeoJSONPolygon{Type: "Polygon", Coordinates: poly},
			})
		}
	}
	fc.Features = append(fc.Features, lines...)

	if len(fc.Features) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("geojson: no geometry found for airspace %s", a.GetIdentifier())
	}
	return fc, nil
}

// MarshalGeoJSON marshals a GeoJSONFeatureCollection to indented JSON.
func MarshalGeoJSON(fc *GeoJSONFeatureCollection) ([]byte, error) {
	return json.MarshalIndent(fc, "", "  ")
}

// ---- Volume geometry --------------------------------------------------------

// volumeGeometry converts one airspace volume to a GeoJSON geometry.  It
// returns a nil geometry, and no error, when the volume carries no geometry of
// its own and contributes none by reference.
func volumeGeometry(vol *features.AirspaceVolumeType, res centreResolver, msg *parse.Message, depth int) (any, error) {
	if patches := vol.HorizontalProjection.Patches(); patches != nil {
		rings, err := extractAllRings(patches, res, msg)
		if err != nil {
			return nil, err
		}
		return GeoJSONPolygon{Type: "Polygon", Coordinates: rings}, nil
	}

	// A corridor: a centreline with a width.  The centreline is emitted as a
	// LineString; see the package documentation on why it is not buffered.
	if vol.Centreline != nil && vol.Centreline.Curve != nil {
		coords, err := extractCurveCoords(vol.Centreline.Curve, res, msg)
		if err != nil {
			return nil, err
		}
		return GeoJSONLineString{Type: "LineString", Coordinates: coords}, nil
	}

	if vol.ContributorAirspace != nil {
		polys, err := resolveContributors(vol, msg, depth)
		if err != nil {
			return nil, err
		}
		switch len(polys) {
		case 0:
			return nil, nil
		case 1:
			return GeoJSONPolygon{Type: "Polygon", Coordinates: polys[0]}, nil
		default:
			return GeoJSONMultiPolygon{Type: "MultiPolygon", Coordinates: polys}, nil
		}
	}

	return nil, nil
}

// resolveContributors returns the polygons contributed to vol by the airspace
// its contributorAirspace dependency names.
func resolveContributors(vol *features.AirspaceVolumeType, msg *parse.Message, depth int) ([][][][2]float64, error) {
	if depth >= maxContributorDepth {
		return nil, fmt.Errorf("geojson: contributor airspace nesting exceeds %d levels", maxContributorDepth)
	}
	dep := vol.ContributorAirspace.AirspaceVolumeDependency
	if dep == nil || dep.TheAirspace == nil {
		return nil, nil
	}

	// FULL_GEOMETRY and HORZ_PROJECTION both contribute the horizontal
	// geometry; they differ only in vertical treatment, which a 2-D GeoJSON
	// projection does not represent.
	if d := codeValue(dep.Dependency); d != "" &&
		d != features.DependencyFullGeometry && d != features.DependencyHorzProjection {
		return nil, nil
	}

	sub := dep.TheAirspace.Airspace
	if sub == nil {
		if msg == nil || dep.TheAirspace.HrefAttr == "" {
			return nil, nil
		}
		resolved, ok := msg.Resolve(dep.TheAirspace.HrefAttr)
		if !ok {
			return nil, nil
		}
		sub, _ = resolved.(*features.AirspaceType)
		if sub == nil {
			return nil, nil
		}
	}

	fc, err := airspaceToGeoJSONDepth(sub, msg, depth+1)
	if err != nil {
		// A contributor we cannot convert simply contributes nothing; the
		// airspace may still have geometry from its other components.
		return nil, nil
	}
	return polygonsOf(fc), nil
}

// airspaceToGeoJSONDepth converts a contributed airspace while tracking the
// aggregation depth already reached.
func airspaceToGeoJSONDepth(a *features.AirspaceType, msg *parse.Message, depth int) (*GeoJSONFeatureCollection, error) {
	if len(a.TimeSlice) == 0 {
		return nil, fmt.Errorf("geojson: airspace has no timeslices")
	}
	ts := a.TimeSlice[0].AirspaceTimeSlice
	res := centreResolverFor(msg)

	var polygons [][][][2]float64
	for _, gc := range orderedComponents(ts.GeometryComponent) {
		agc := gc.AirspaceGeometryComponent
		if agc == nil || agc.TheAirspaceVolume.AirspaceVolume == nil {
			continue
		}
		geom, err := volumeGeometry(agc.TheAirspaceVolume.AirspaceVolume, res, msg, depth)
		if err != nil {
			continue
		}
		switch g := geom.(type) {
		case GeoJSONPolygon:
			polygons = append(polygons, g.Coordinates)
		case GeoJSONMultiPolygon:
			polygons = append(polygons, g.Coordinates...)
		}
	}
	if len(polygons) == 0 {
		return nil, fmt.Errorf("geojson: no geometry found for airspace %s", a.GetIdentifier())
	}

	fc := &GeoJSONFeatureCollection{Type: "FeatureCollection"}
	props := airspaceProps(a, ts)
	if len(polygons) == 1 {
		fc.Features = append(fc.Features, GeoJSONFeature{
			Type: "Feature", Properties: props,
			Geometry: GeoJSONPolygon{Type: "Polygon", Coordinates: polygons[0]},
		})
	} else {
		fc.Features = append(fc.Features, GeoJSONFeature{
			Type: "Feature", Properties: props,
			Geometry: GeoJSONMultiPolygon{Type: "MultiPolygon", Coordinates: polygons},
		})
	}
	return fc, nil
}

// orderedComponents returns the geometry components sorted by their
// operationSequence, so that aggregation is applied in the intended order.
// Components without a sequence keep their document order.
func orderedComponents(in []features.AirspaceGeometryComponentPropertyType) []features.AirspaceGeometryComponentPropertyType {
	out := make([]features.AirspaceGeometryComponentPropertyType, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		return componentSequence(out[i]) < componentSequence(out[j])
	})
	return out
}

func componentSequence(c features.AirspaceGeometryComponentPropertyType) int {
	if c.AirspaceGeometryComponent == nil || c.AirspaceGeometryComponent.OperationSequence == nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(c.AirspaceGeometryComponent.OperationSequence.Value))
	if err != nil {
		return 0
	}
	return n
}

// polygonsOf collects every polygon in a feature collection, flattening any
// MultiPolygon.
func polygonsOf(fc *GeoJSONFeatureCollection) [][][][2]float64 {
	var out [][][][2]float64
	if fc == nil {
		return nil
	}
	for _, f := range fc.Features {
		switch g := f.Geometry.(type) {
		case GeoJSONPolygon:
			out = append(out, g.Coordinates)
		case GeoJSONMultiPolygon:
			out = append(out, g.Coordinates...)
		}
	}
	return out
}

// ---- Properties -------------------------------------------------------------

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

// volumeProps returns the airspace properties enriched with the vertical
// limits of vol, or the properties unchanged when there is no volume.
func volumeProps(base map[string]any, vol *features.AirspaceVolumeType) map[string]any {
	if vol == nil {
		return base
	}
	return enrichWithLimits(base, vol)
}

// sameVolume reports whether every polygon came from one and the same volume.
func sameVolume(volumes []*features.AirspaceVolumeType) bool {
	for _, v := range volumes[1:] {
		if v != volumes[0] {
			return false
		}
	}
	return true
}

func enrichWithLimits(base map[string]any, vol *features.AirspaceVolumeType) map[string]any {
	out := make(map[string]any, len(base)+6)
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
	// A corridor is reported with its width so that a consumer can buffer the
	// centreline itself; see the note on volumeGeometry.
	if vol.Centreline != nil && vol.Width != nil {
		out["geometryKind"] = "centreline"
		if m, err := LengthToMetres(parseFloat(vol.Width.Value), widthUom(vol)); err == nil {
			out["width_m"] = m
		}
	}
	return out
}

// widthUom reports the unit of measure recorded for a volume's width.  The
// generated AIXM value types keep only the character data, so the unit is not
// available and metres — the AIXM default for a corridor width — is assumed.
func widthUom(*features.AirspaceVolumeType) string { return "M" }

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

// codeValue returns the trimmed text of an AIXM code value, or "" when absent.
func codeValue(v *types.NilableString) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(v.Value)
}

// ---- Ring extraction --------------------------------------------------------

// extractAllRings returns the exterior ring at index 0 followed by any
// interior rings (holes) from the first patch of the surface.
//
// Rings are closed and oriented per RFC 7946: exteriors counter-clockwise,
// holes clockwise.
func extractAllRings(patches *gml.SurfacePatchesType, res centreResolver, msg *parse.Message) ([][][2]float64, error) {
	if patches == nil || len(patches.PolygonPatch) == 0 {
		return nil, fmt.Errorf("geojson: surface has no patches")
	}
	patch := patches.PolygonPatch[0]
	if patch.Exterior == nil {
		return nil, fmt.Errorf("geojson: patch has no exterior")
	}

	exterior, err := extractAbstractRingCoords(patch.Exterior, res, msg)
	if err != nil {
		return nil, err
	}
	exterior, err = finishRing(exterior, true)
	if err != nil {
		return nil, fmt.Errorf("geojson: exterior ring: %w", err)
	}

	rings := [][][2]float64{exterior}
	for i := range patch.Interior {
		hole, err := extractAbstractRingCoords(&patch.Interior[i], res, msg)
		if err != nil {
			return nil, fmt.Errorf("geojson: interior ring %d: %w", i, err)
		}
		hole, err = finishRing(hole, false)
		if err != nil {
			return nil, fmt.Errorf("geojson: interior ring %d: %w", i, err)
		}
		rings = append(rings, hole)
	}
	return rings, nil
}

// finishRing closes a ring and orients it, rejecting rings too small to
// enclose an area.
func finishRing(ring [][2]float64, ccw bool) ([][2]float64, error) {
	ring = closeRing(ring)
	if len(ring) < 4 {
		return nil, fmt.Errorf("geojson: ring has %d positions, need at least 4", len(ring))
	}
	return orientRing(ring, ccw), nil
}

// closeRing appends the first position to the end of the ring when it is not
// already closed, as RFC 7946 requires.
func closeRing(ring [][2]float64) [][2]float64 {
	if len(ring) == 0 {
		return ring
	}
	if samePoint(ring[0], ring[len(ring)-1]) {
		// Already closed; make the coincidence exact.
		ring[len(ring)-1] = ring[0]
		return ring
	}
	return append(ring, ring[0])
}

// signedArea returns twice the signed planar area of a ring.  A positive value
// means the ring winds counter-clockwise.
//
// The planar shoelace formula is adequate for deciding orientation at the
// scale of an airspace boundary.
func signedArea(ring [][2]float64) float64 {
	var sum float64
	for i := 0; i < len(ring)-1; i++ {
		sum += ring[i][0]*ring[i+1][1] - ring[i+1][0]*ring[i][1]
	}
	return sum
}

// orientRing reverses the ring when its winding does not match ccw.
func orientRing(ring [][2]float64, ccw bool) [][2]float64 {
	if (signedArea(ring) >= 0) == ccw {
		return ring
	}
	for i, j := 0, len(ring)-1; i < j; i, j = i+1, j-1 {
		ring[i], ring[j] = ring[j], ring[i]
	}
	return ring
}

func extractAbstractRingCoords(r *gml.AbstractRingPropertyType, res centreResolver, msg *parse.Message) ([][2]float64, error) {
	if r.LinearRing != nil {
		return extractLinearRingCoords(r.LinearRing)
	}
	if r.Ring != nil {
		return extractRingTypeCoords(r.Ring, res, msg)
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

// extractRingTypeCoords walks the curve members of a ring in document order,
// resolving referenced border curves against msg where necessary.
func extractRingTypeCoords(ring *gml.RingType, res centreResolver, msg *parse.Message) ([][2]float64, error) {
	// Resolve each member first.  A referenced border contributes only the
	// stretch lying between its neighbours, so the neighbouring members have
	// to be known before it can be trimmed.
	members := make([]ringMember, 0, len(ring.CurveMember))

	for _, cm := range ring.CurveMember {
		curve := cm.TheCurve()

		if curve == nil {
			// A border held by reference, e.g. a shared national boundary.
			if cm.HrefAttr == "" {
				continue
			}
			if msg == nil {
				return nil, fmt.Errorf(
					"geojson: ring references border %s, which needs the surrounding message to resolve",
					cm.HrefAttr)
			}
			borderCoords, err := resolveBorderCurve(cm.HrefAttr, res, msg)
			if err != nil {
				return nil, err
			}
			members = append(members, ringMember{borderCoords, true})
			continue
		}

		c, err := segmentsToCoords(curve.Segments, res, msg)
		if err != nil {
			return nil, err
		}
		members = append(members, ringMember{c, false})
	}

	// Trim each border to the span between the point its predecessor ended on
	// and the point its successor begins at.  A GeoBorder is a whole national
	// boundary; a ring borrows only the piece it shares.
	for i := range members {
		if !members[i].isBorder || len(members[i].coords) == 0 {
			continue
		}
		var from, to *[2]float64
		if p := previousEnd(members, i); p != nil {
			from = p
		}
		if n := nextStart(members, i); n != nil {
			to = n
		}
		members[i].coords = trimBorder(members[i].coords, from, to)
	}

	var coords [][2]float64
	for _, m := range members {
		coords = appendDedup(coords, m.coords)
	}

	if len(coords) == 0 {
		return nil, fmt.Errorf("geojson: Ring has no coordinates")
	}
	return coords, nil
}

// ringMember is one resolved curveMember of a ring: its positions, and
// whether they came from a referenced GeoBorder.
type ringMember struct {
	coords   [][2]float64
	isBorder bool
}

// previousEnd returns the position at which the nearest preceding non-border
// member ends, wrapping around the ring.  Borders are skipped because an
// untrimmed one has no meaningful endpoint yet.
func previousEnd(members []ringMember, i int) *[2]float64 {
	for k := 1; k < len(members); k++ {
		j := ((i-k)%len(members) + len(members)) % len(members)
		m := members[j]
		if m.isBorder || len(m.coords) == 0 {
			continue
		}
		p := m.coords[len(m.coords)-1]
		return &p
	}
	return nil
}

// nextStart returns the position at which the nearest following non-border
// member begins, wrapping around the ring.
func nextStart(members []ringMember, i int) *[2]float64 {
	for k := 1; k < len(members); k++ {
		j := (i + k) % len(members)
		m := members[j]
		if m.isBorder || len(m.coords) == 0 {
			continue
		}
		p := m.coords[0]
		return &p
	}
	return nil
}

// trimBorder reduces a whole GeoBorder to the stretch a ring actually borrows.
//
// A GeoBorder is published as a complete national boundary, often thousands of
// positions long, while the ring referencing it uses only the piece between the
// point its previous member ended on and the point its next member starts at.
// Splicing in the whole border would inflate the polygon to the size of the
// country.
//
// The nearest border position to each neighbour is found and the border is cut
// between them, in whichever direction matches the ring's travel.
func trimBorder(border [][2]float64, from, to *[2]float64) [][2]float64 {
	if len(border) < 2 || (from == nil && to == nil) {
		return border
	}

	start, end := 0, len(border)-1
	if from != nil {
		start = nearestIndex(border, *from)
	}
	if to != nil {
		end = nearestIndex(border, *to)
	}
	if start == end {
		return border[start : start+1]
	}

	if start < end {
		return border[start : end+1]
	}
	// The ring runs against the border's own direction.
	out := make([][2]float64, 0, start-end+1)
	for i := start; i >= end; i-- {
		out = append(out, border[i])
	}
	return out
}

// nearestIndex returns the index of the border position closest to p.
func nearestIndex(border [][2]float64, p [2]float64) int {
	best, bestD := 0, math.Inf(1)
	for i, c := range border {
		// Planar in lon/lat: adequate for picking the nearest of a dense set
		// of boundary positions.
		dx, dy := c[0]-p[0], c[1]-p[1]
		if d := dx*dx + dy*dy; d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

// resolveBorderCurve returns the coordinates of a GeoBorder referenced from a
// curveMember.
func resolveBorderCurve(href string, res centreResolver, msg *parse.Message) ([][2]float64, error) {
	target, ok := msg.Resolve(href)
	if !ok {
		return nil, fmt.Errorf("geojson: cannot resolve referenced border %s", href)
	}

	border, ok := target.(*features.GeoBorderType)
	if !ok {
		return nil, fmt.Errorf("geojson: reference %s is not a GeoBorder", href)
	}
	for _, ts := range border.TimeSlice {
		bts := ts.GeoBorderTimeSlice
		if bts.Border == nil || bts.Border.Curve == nil {
			continue
		}
		return extractCurveCoords(bts.Border.Curve, res, msg)
	}
	return nil, fmt.Errorf("geojson: referenced border %s has no curve", href)
}

// extractCurveCoords flattens an AIXM curve to a polyline.
func extractCurveCoords(c *features.CurveType, res centreResolver, msg *parse.Message) ([][2]float64, error) {
	return segmentsToCoords(c.Segments, res, msg)
}

// segmentsToCoords converts a curve's segments, in document order, to a
// polyline.  Order matters: a boundary that runs out along a radial, around an
// arc and back would self-intersect if its segments were reordered.
func segmentsToCoords(segs *gml.CurveSegmentsType, res centreResolver, msg *parse.Message) ([][2]float64, error) {
	if segs == nil {
		return nil, nil
	}
	var coords [][2]float64

	for _, seg := range segs.Ordered() {
		var (
			c   [][2]float64
			err error
		)
		switch {
		case seg.GeodesicString != nil:
			if seg.GeodesicString.PosList != nil {
				c, err = posListToCoords(seg.GeodesicString.PosList.Value)
			} else if len(seg.GeodesicString.Pos) > 0 {
				c, err = posSliceToCoords(seg.GeodesicString.Pos)
			}
		case seg.LineStringSegment != nil:
			if seg.LineStringSegment.PosList != nil {
				c, err = posListToCoords(seg.LineStringSegment.PosList.Value)
			}
		case seg.Arc != nil:
			c, err = discretizeArc(seg.Arc.Pos, seg.Arc.PosList, DefaultArcPoints)
		case seg.Circle != nil:
			c, err = discretizeCircle(seg.Circle.Pos, seg.Circle.PosList, DefaultArcPoints)
		case seg.ArcByCenterPoint != nil:
			c, err = discretizeArcByCenter(seg.ArcByCenterPoint, res)
		case seg.CircleByCenterPoint != nil:
			c, err = discretizeCircleByCenter(seg.CircleByCenterPoint, res)
		}
		if err != nil {
			return nil, err
		}
		coords = appendDedup(coords, c)
	}
	return coords, nil
}

// appendDedup appends src to dst, skipping the first point of src when it
// coincides with the last point of dst, so that segments joined end to end do
// not repeat their shared vertex.
func appendDedup(dst, src [][2]float64) [][2]float64 {
	if len(src) == 0 {
		return dst
	}
	if len(dst) > 0 && samePoint(dst[len(dst)-1], src[0]) {
		src = src[1:]
	}
	return append(dst, src...)
}

// samePoint reports whether two positions coincide to within joinTolerance.
func samePoint(a, b [2]float64) bool {
	return math.Abs(a[0]-b[0]) <= joinTolerance && math.Abs(a[1]-b[1]) <= joinTolerance
}

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
func discretizeArcByCenter(a *gml.ArcByCenterPointType, res centreResolver) ([][2]float64, error) {
	lat, lon, err := arcCentre(a.Pos, a.PosList, a.PointProperty, a.PointRep, res)
	if err != nil {
		return nil, err
	}
	radius, err := LengthToMetres(a.Radius.Value, a.Radius.Uom)
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
func discretizeCircleByCenter(c *gml.CircleByCenterPointType, res centreResolver) ([][2]float64, error) {
	lat, lon, err := arcCentre(c.Pos, c.PosList, c.PointProperty, c.PointRep, res)
	if err != nil {
		return nil, err
	}
	radius, err := LengthToMetres(c.Radius.Value, c.Radius.Uom)
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
func arcCentre(pos *gml.DirectPositionType, posList *gml.DirectPositionListType,
	pp, pr *gml.PointPropertyType, res centreResolver) (lat, lon float64, err error) {

	if pos != nil {
		return firstLatLon(pos.Value)
	}
	if posList != nil {
		return firstLatLon(posList.Value)
	}
	for _, p := range []*gml.PointPropertyType{pp, pr} {
		if p == nil {
			continue
		}
		if p.Point != nil && p.Point.Pos != nil {
			return firstLatLon(p.Point.Pos.Value)
		}
		if p.HrefAttr != "" {
			if res == nil {
				return 0, 0, fmt.Errorf(
					"geojson: arc centre references %s, which needs the surrounding message to resolve",
					p.HrefAttr)
			}
			if lat, lon, ok := res(p.HrefAttr); ok {
				return lat, lon, nil
			}
			return 0, 0, fmt.Errorf("geojson: cannot resolve arc centre %s", p.HrefAttr)
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

// ---- Reference resolution ---------------------------------------------------

// centreResolverFor returns a resolver for same-document Point references.
//
// parse.Message indexes features by their UUID identifier, so a fragment
// reference such as "#p9898" naming a gml:id on a Point nested inside another
// feature cannot be resolved through it.  This walks the message's features
// for such points instead.
func centreResolverFor(msg *parse.Message) centreResolver {
	if msg == nil {
		return noResolver
	}
	var index map[string][2]float64
	return func(href string) (float64, float64, bool) {
		if index == nil {
			index = gmlIDIndex(msg)
		}
		if p, ok := index[strings.TrimPrefix(href, "#")]; ok {
			return p[0], p[1], true
		}
		// A full feature reference may also name a point-bearing feature.
		if target, ok := msg.Resolve(href); ok {
			if lat, lon, ok := featurePoint(target); ok {
				return lat, lon, true
			}
		}
		return 0, 0, false
	}
}

// gmlIDIndex indexes the points of a message by their gml:id.
func gmlIDIndex(msg *parse.Message) map[string][2]float64 {
	index := make(map[string][2]float64)
	for _, f := range msg.Features() {
		indexPoints(f, index)
	}
	return index
}

// indexPoints records any Point reachable from a feature, keyed by gml:id.
func indexPoints(feat any, index map[string][2]float64) {
	for _, p := range featurePoints(feat) {
		if p.id != "" {
			index[p.id] = [2]float64{p.lat, p.lon}
		}
	}
}

// identifiedPoint is a position together with the gml:id of the element that
// carried it.
type identifiedPoint struct {
	id       string
	lat, lon float64
}

// featurePoints returns the positions a feature carries directly.
func featurePoints(feat any) []identifiedPoint {
	var out []identifiedPoint

	addPlain := func(p *features.PointPropertyType) {
		if p == nil || p.Point == nil || p.Point.Pos == nil {
			return
		}
		if lat, lon, err := firstLatLon(p.Point.Pos.Value); err == nil {
			out = append(out, identifiedPoint{p.Point.GMLID, lat, lon})
		}
	}
	addElevated := func(p *features.ElevatedPointPropertyType) {
		if p == nil || p.ElevatedPoint == nil || p.ElevatedPoint.Pos == nil {
			return
		}
		if lat, lon, err := firstLatLon(p.ElevatedPoint.Pos.Value); err == nil {
			out = append(out, identifiedPoint{p.ElevatedPoint.GMLID, lat, lon})
		}
	}

	switch v := feat.(type) {
	case *features.DesignatedPointType:
		for _, ts := range v.TimeSlice {
			addPlain(ts.DesignatedPointTimeSlice.Location)
		}
	case *features.NavaidType:
		for _, ts := range v.TimeSlice {
			addElevated(ts.NavaidTimeSlice.Location)
		}
	case *features.AirportHeliportType:
		for _, ts := range v.TimeSlice {
			addElevated(ts.AirportHeliportTimeSlice.ARP)
		}
	}
	return out
}

// featurePoint returns the representative point of a feature, when it has one.
func featurePoint(feat any) (lat, lon float64, ok bool) {
	pts := featurePoints(feat)
	if len(pts) == 0 {
		return 0, 0, false
	}
	return pts[0].lat, pts[0].lon, true
}

// ---- Coordinate parsing -----------------------------------------------------

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
		lat, lon, err := firstLatLon(p.Value)
		if err != nil {
			return nil, err
		}
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
		lat, lon, err := firstLatLon(p.Value)
		if err != nil {
			return nil, err
		}
		coords = append(coords, [2]float64{lon, lat})
	}
	return coords, nil
}
