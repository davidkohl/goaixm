package geo

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
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

// Feature is a GeoJSON Feature object.
type Feature struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Geometry   any            `json:"geometry"`
}

// Polygon is a GeoJSON Polygon geometry (exterior + optional holes).
type Polygon struct {
	Type        string         `json:"type"`
	Coordinates [][][2]float64 `json:"coordinates"`
}

// MultiPolygon is a GeoJSON MultiPolygon geometry.
type MultiPolygon struct {
	Type        string           `json:"type"`
	Coordinates [][][][2]float64 `json:"coordinates"`
}

// LineString is a GeoJSON LineString geometry.
type LineString struct {
	Type        string       `json:"type"`
	Coordinates [][2]float64 `json:"coordinates"`
}

// FeatureCollection is a GeoJSON FeatureCollection.
type FeatureCollection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
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
func AirspaceToGeoJSON(a *aixm.Airspace) (*FeatureCollection, error) {
	return airspaceToGeoJSON(a, nil, false)
}

// AirspaceToGeoJSONWithRefs converts an airspace to GeoJSON, resolving
// references against msg: sub-airspaces contributed via contributorAirspace,
// GeoBorder curves referenced from a curveMember, and arc centres given by
// reference.  When more than one polygon results the geometry is a
// MultiPolygon.
func AirspaceToGeoJSONWithRefs(a *aixm.Airspace, msg *goaixm.Message) (*FeatureCollection, error) {
	return airspaceToGeoJSON(a, msg, true)
}

// airspaceToGeoJSON converts an airspace.  When merge is set, every polygon is
// gathered into a single feature — a MultiPolygon if there is more than one —
// which is how the reference-resolving form assembles an airspace from its
// contributing parts.  Otherwise each volume yields its own feature, carrying
// that volume's vertical limits.
func airspaceToGeoJSON(a *aixm.Airspace, msg *goaixm.Message, merge bool) (*FeatureCollection, error) {
	if a == nil {
		return nil, fmt.Errorf("geojson: nil airspace")
	}
	if len(a.TimeSlice) == 0 {
		return nil, fmt.Errorf("geojson: airspace has no timeslices")
	}
	ts := a.TimeSlice[0]
	props := airspaceProps(a, ts)
	res := centreResolverFor(msg)

	components := orderedComponents(ts.GeometryComponent)

	var (
		polygons [][][][2]float64
		lines    []Feature
		firstErr error
		// volumes records the volume each polygon came from, so that a
		// single-volume airspace can report that volume's vertical limits.
		volumes []*aixm.AirspaceVolume
	)

	for i := range components {
		agc := &components[i]
		if op := codeValue(agc.Operation); op == aixm.AggregationIntersection ||
			op == aixm.AggregationSubtraction {
			return nil, fmt.Errorf(
				"geojson: airspace %s uses unsupported aggregation operation %s",
				a.UUID(), op)
		}

		vol := agc.TheAirspaceVolume
		if vol == nil {
			continue
		}

		geom, err := volumeGeometry(vol, res, msg, 0, nil)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		switch g := geom.(type) {
		case nil:
			// No geometry on this component.
		case Polygon:
			polygons = append(polygons, g.Coordinates)
			volumes = append(volumes, vol)
		case MultiPolygon:
			for range g.Coordinates {
				volumes = append(volumes, vol)
			}
			polygons = append(polygons, g.Coordinates...)
		case LineString:
			lines = append(lines, Feature{
				Type:       "Feature",
				Properties: enrichWithLimits(props, vol),
				Geometry:   g,
			})
		}
	}

	fc := &FeatureCollection{Type: "FeatureCollection"}

	switch {
	case len(polygons) == 0:
		// nothing
	case len(polygons) == 1:
		fc.Features = append(fc.Features, Feature{
			Type:       "Feature",
			Properties: volumeProps(props, volumes[0]),
			Geometry:   Polygon{Type: "Polygon", Coordinates: polygons[0]},
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
		fc.Features = append(fc.Features, Feature{
			Type:       "Feature",
			Properties: mergedProps,
			Geometry:   MultiPolygon{Type: "MultiPolygon", Coordinates: polygons},
		})
	default:
		// Distinct volumes, each with its own vertical limits.
		for i, poly := range polygons {
			fc.Features = append(fc.Features, Feature{
				Type:       "Feature",
				Properties: volumeProps(props, volumes[i]),
				Geometry:   Polygon{Type: "Polygon", Coordinates: poly},
			})
		}
	}
	fc.Features = append(fc.Features, lines...)

	if len(fc.Features) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("geojson: no geometry found for airspace %s", a.UUID())
	}
	return fc, nil
}

// MarshalGeoJSON marshals a FeatureCollection to indented JSON.
func MarshalGeoJSON(fc *FeatureCollection) ([]byte, error) {
	return json.MarshalIndent(fc, "", "  ")
}

// ---- Volume geometry --------------------------------------------------------

// volumeGeometry converts one airspace volume to a GeoJSON geometry.  It
// returns a nil geometry, and no error, when the volume carries no geometry of
// its own and contributes none by reference.
func volumeGeometry(vol *aixm.AirspaceVolume, res centreResolver, msg *goaixm.Message, depth int, warn func(string)) (any, error) {
	if patches := vol.HorizontalProjection.Patches(); patches != nil {
		rings, err := extractAllRings(patches, res, msg, warn)
		if err != nil {
			return nil, err
		}
		return Polygon{Type: "Polygon", Coordinates: rings}, nil
	}

	// A corridor: a centreline with a width.  The centreline is emitted as a
	// LineString; see the package documentation on why it is not buffered.
	if segs := vol.Centreline.Segments(); segs != nil {
		coords, err := segmentsToCoords(segs, res, msg)
		if err != nil {
			return nil, err
		}
		return LineString{Type: "LineString", Coordinates: coords}, nil
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
			return Polygon{Type: "Polygon", Coordinates: polys[0]}, nil
		default:
			return MultiPolygon{Type: "MultiPolygon", Coordinates: polys}, nil
		}
	}

	return nil, nil
}

// resolveContributors returns the polygons contributed to vol by the airspace
// its contributorAirspace dependency names.
func resolveContributors(vol *aixm.AirspaceVolume, msg *goaixm.Message, depth int) ([][][][2]float64, error) {
	if depth >= maxContributorDepth {
		return nil, fmt.Errorf("geojson: contributor airspace nesting exceeds %d levels", maxContributorDepth)
	}
	dep := vol.ContributorAirspace
	if dep == nil || dep.TheAirspace == nil {
		return nil, nil
	}

	// FULL_GEOMETRY and HORZ_PROJECTION both contribute the horizontal
	// geometry; they differ only in vertical treatment, which a 2-D GeoJSON
	// projection does not represent.
	if d := codeValue(dep.Dependency); d != "" &&
		d != aixm.DependencyFullGeometry && d != aixm.DependencyHorzProjection {
		return nil, nil
	}

	if msg == nil || dep.TheAirspace.Href == "" {
		return nil, nil
	}
	resolved, ok := msg.Resolve(dep.TheAirspace.Href)
	if !ok {
		return nil, nil
	}
	sub, ok := resolved.(*aixm.Airspace)
	if !ok {
		return nil, nil
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
func airspaceToGeoJSONDepth(a *aixm.Airspace, msg *goaixm.Message, depth int) (*FeatureCollection, error) {
	if len(a.TimeSlice) == 0 {
		return nil, fmt.Errorf("geojson: airspace has no timeslices")
	}
	ts := a.TimeSlice[0]
	res := centreResolverFor(msg)

	var polygons [][][][2]float64
	for _, agc := range orderedComponents(ts.GeometryComponent) {
		if agc.TheAirspaceVolume == nil {
			continue
		}
		geom, err := volumeGeometry(agc.TheAirspaceVolume, res, msg, depth, nil)
		if err != nil {
			continue
		}
		switch g := geom.(type) {
		case Polygon:
			polygons = append(polygons, g.Coordinates)
		case MultiPolygon:
			polygons = append(polygons, g.Coordinates...)
		}
	}
	if len(polygons) == 0 {
		return nil, fmt.Errorf("geojson: no geometry found for airspace %s", a.UUID())
	}

	fc := &FeatureCollection{Type: "FeatureCollection"}
	props := airspaceProps(a, ts)
	if len(polygons) == 1 {
		fc.Features = append(fc.Features, Feature{
			Type: "Feature", Properties: props,
			Geometry: Polygon{Type: "Polygon", Coordinates: polygons[0]},
		})
	} else {
		fc.Features = append(fc.Features, Feature{
			Type: "Feature", Properties: props,
			Geometry: MultiPolygon{Type: "MultiPolygon", Coordinates: polygons},
		})
	}
	return fc, nil
}

// orderedComponents returns the geometry components sorted by their
// operationSequence, so that aggregation is applied in the intended order.
// Components without a sequence keep their document order.
func orderedComponents(in []aixm.AirspaceGeometryComponent) []aixm.AirspaceGeometryComponent {
	out := make([]aixm.AirspaceGeometryComponent, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		return componentSequence(out[i]) < componentSequence(out[j])
	})
	return out
}

func componentSequence(c aixm.AirspaceGeometryComponent) int {
	if c.OperationSequence == nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(c.OperationSequence.Value))
	if err != nil {
		return 0
	}
	return n
}

// polygonsOf collects every polygon in a feature collection, flattening any
// MultiPolygon.
func polygonsOf(fc *FeatureCollection) [][][][2]float64 {
	var out [][][][2]float64
	if fc == nil {
		return nil
	}
	for _, f := range fc.Features {
		switch g := f.Geometry.(type) {
		case Polygon:
			out = append(out, g.Coordinates)
		case MultiPolygon:
			out = append(out, g.Coordinates...)
		}
	}
	return out
}

// ---- Properties -------------------------------------------------------------

func airspaceProps(a *aixm.Airspace, ts aixm.AirspaceTimeSlice) map[string]any {
	props := map[string]any{"identifier": a.UUID()}
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
func volumeProps(base map[string]any, vol *aixm.AirspaceVolume) map[string]any {
	if vol == nil {
		return base
	}
	return enrichWithLimits(base, vol)
}

// sameVolume reports whether every polygon came from one and the same volume.
func sameVolume(volumes []*aixm.AirspaceVolume) bool {
	for _, v := range volumes[1:] {
		if v != volumes[0] {
			return false
		}
	}
	return true
}

func enrichWithLimits(base map[string]any, vol *aixm.AirspaceVolume) map[string]any {
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

// widthUom reports the unit of measure of a volume's width, assuming metres
// when the data gives none.
func widthUom(vol *aixm.AirspaceVolume) string {
	if vol.Width != nil && vol.Width.UOM != "" {
		return vol.Width.UOM
	}
	return "M"
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

// codeValue returns the trimmed text of an AIXM code value, or "" when absent.
func codeValue(v *aixm.Value) string { return v.String() }
