package geo

import (
	"fmt"
	"math"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/gml"
)

// ---- Ring extraction --------------------------------------------------------

// extractAllRings returns the exterior ring at index 0 followed by any
// interior rings (holes) from the first patch of the surface.
//
// Rings are closed and oriented per RFC 7946: exteriors counter-clockwise,
// holes clockwise.
func extractAllRings(patches *gml.SurfacePatches, res centreResolver, msg *goaixm.Message) ([][][2]float64, error) {
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

func extractAbstractRingCoords(r *gml.AbstractRingProperty, res centreResolver, msg *goaixm.Message) ([][2]float64, error) {
	if r.LinearRing != nil {
		return extractLinearRingCoords(r.LinearRing)
	}
	if r.Ring != nil {
		return extractRingTypeCoords(r.Ring, res, msg)
	}
	return nil, fmt.Errorf("geojson: exterior has neither LinearRing nor Ring")
}

func extractLinearRingCoords(lr *gml.LinearRing) ([][2]float64, error) {
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
func extractRingTypeCoords(ring *gml.Ring, res centreResolver, msg *goaixm.Message) ([][2]float64, error) {
	// Resolve each member first.  A referenced border contributes only the
	// stretch lying between its neighbours, so the neighbouring members have
	// to be known before it can be trimmed.
	members := make([]ringMember, 0, len(ring.CurveMember))

	for _, cm := range ring.CurveMember {
		curve := cm.Curve

		if curve == nil {
			// A border held by reference, e.g. a shared national boundary.
			if cm.Href == "" {
				continue
			}
			if msg == nil {
				return nil, fmt.Errorf(
					"geojson: ring references border %s, which needs the surrounding message to resolve",
					cm.Href)
			}
			borderCoords, err := resolveBorderCurve(cm.Href, res, msg)
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
func resolveBorderCurve(href string, res centreResolver, msg *goaixm.Message) ([][2]float64, error) {
	target, ok := msg.Resolve(href)
	if !ok {
		return nil, fmt.Errorf("geojson: cannot resolve referenced border %s", href)
	}

	border, ok := target.(*aixm.GeoBorder)
	if !ok {
		return nil, fmt.Errorf("geojson: reference %s is not a GeoBorder", href)
	}
	for _, ts := range border.TimeSlice {
		if segs := ts.Border.Segments(); segs != nil {
			return segmentsToCoords(segs, res, msg)
		}
	}
	return nil, fmt.Errorf("geojson: referenced border %s has no curve", href)
}

// segmentsToCoords converts a curve's segments, in document order, to a
// polyline.  Order matters: a boundary that runs out along a radial, around an
// arc and back would self-intersect if its segments were reordered.
func segmentsToCoords(segs *gml.CurveSegments, res centreResolver, msg *goaixm.Message) ([][2]float64, error) {
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
