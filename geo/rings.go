package geo

import (
	"fmt"
	"math"
	"strings"

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
func extractAllRings(patches *gml.SurfacePatches, cv *conv, msg *goaixm.Message, warn func(string)) ([][][2]float64, error) {
	if patches == nil || len(patches.PolygonPatch) == 0 {
		return nil, fmt.Errorf("geojson: surface has no patches")
	}
	patch := patches.PolygonPatch[0]
	if patch.Exterior == nil {
		return nil, fmt.Errorf("geojson: patch has no exterior")
	}

	exterior, err := extractAbstractRingCoords(patch.Exterior, cv, msg, warn)
	if err != nil {
		return nil, err
	}
	exterior, err = finishRing(exterior, true, cv)
	if err != nil {
		return nil, fmt.Errorf("geojson: exterior ring: %w", err)
	}

	rings := [][][2]float64{exterior}
	for i := range patch.Interior {
		hole, err := extractAbstractRingCoords(&patch.Interior[i], cv, msg, prefixWarn(warn, fmt.Sprintf("interior ring %d: ", i)))
		if err != nil {
			return nil, fmt.Errorf("geojson: interior ring %d: %w", i, err)
		}
		hole, err = finishRing(hole, false, cv)
		if err != nil {
			return nil, fmt.Errorf("geojson: interior ring %d: %w", i, err)
		}
		rings = append(rings, hole)
	}
	return rings, nil
}

// finishRing closes a ring and orients it, rejecting rings too small to
// enclose an area.
func finishRing(ring [][2]float64, ccw bool, cv *conv) ([][2]float64, error) {
	if n := len(ring); cv != nil && n > 1 && samePoint(ring[0], ring[n-1]) &&
		cv.computed[ring[0]] && !cv.computed[ring[n-1]] {
		ring[0] = ring[n-1] // the source position closes the ring
	}
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

func extractAbstractRingCoords(r *gml.AbstractRingProperty, cv *conv, msg *goaixm.Message, warn func(string)) ([][2]float64, error) {
	if r.LinearRing != nil {
		return extractLinearRingCoords(r.LinearRing)
	}
	if r.Ring != nil {
		return extractRingTypeCoords(r.Ring, cv, msg, warn)
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
// resolving referenced border curves against msg where necessary, and joins
// them end to end.  Problems that leave the ring open or distorted are
// reported through warn (which may be nil); the ring is still returned.
func extractRingTypeCoords(ring *gml.Ring, cv *conv, msg *goaixm.Message, warn func(string)) ([][2]float64, error) {
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
			borderCoords, err := resolveBorderCurve(cm.Href, cv, msg)
			if err != nil {
				return nil, err
			}
			// Consecutive references to the same border describe one
			// continuous stretch of it (sources may split it into several
			// sections); it is trimmed once, as a whole.
			if k := len(members) - 1; k >= 0 && members[k].isBorder && members[k].href == cm.Href {
				continue
			}
			members = append(members, ringMember{borderCoords, true, cm.Href})
			continue
		}

		c, err := segmentsToCoords(curve.Segments, cv, msg)
		if err != nil {
			return nil, err
		}
		if len(c) > 0 {
			members = append(members, ringMember{coords: c})
		}
	}

	trimBorders(members)
	coords := joinMembers(members, warn, cv)
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
	href     string
}

// trimBorders reduces each referenced border to the stretch the ring
// borrows, oriented in the ring's direction of travel.
//
// A GeoBorder is published as a complete national boundary, while a ring may
// use only part of it.  The ring itself says which part: the border runs from
// where the previous member ends to where the next member begins.  When a
// neighbour is itself a referenced border, the two borders meet at a common
// point (typically a tripoint shared by their endpoints), which is where one
// hands over to the other.  A border whose neighbours sit on its endpoints is
// therefore used whole, reversed if the ring travels against it.
//
// The borders are trimmed against their untrimmed neighbours, so the result
// does not depend on the order in which members are processed.
func trimBorders(members []ringMember) {
	n := len(members)
	if n < 2 {
		return
	}
	trimmed := make([][][2]float64, n)
	for i := range members {
		m := members[i]
		if !m.isBorder || len(m.coords) < 2 {
			trimmed[i] = m.coords
			continue
		}
		prev, next := members[(i-1+n)%n], members[(i+1)%n]
		from := attach(m.coords, prev, false)
		to := attach(m.coords, next, true)
		trimmed[i] = cutBorder(m.coords, from, to)
	}
	for i := range members {
		members[i].coords = trimmed[i]
	}
}

// borderCut is a position on a border polyline: the parameter s = i + t lies
// on the segment from vertex i to vertex i+1, and p is the position itself.
type borderCut struct {
	s float64
	p [2]float64
}

// attach returns where the neighbouring member nb attaches to border: the
// end of a preceding curve or the start of a following one, projected onto
// the border, or, for a neighbouring border, the point where the two meet.
func attach(border [][2]float64, nb ringMember, following bool) borderCut {
	if len(nb.coords) == 0 {
		if following {
			return borderCut{float64(len(border) - 1), border[len(border)-1]}
		}
		return borderCut{0, border[0]}
	}
	if !nb.isBorder {
		p := nb.coords[len(nb.coords)-1]
		if following {
			p = nb.coords[0]
		}
		// The neighbour's own position is kept, so that the join is exact;
		// only its place along the border is needed.
		return borderCut{project(border, p), p}
	}
	j := junctionIndex(border, nb.coords)
	return borderCut{float64(j), border[j]}
}

// junctionIndex returns the index of the position of border b at which it
// meets border other.  Borders meet where an endpoint of one lies on the
// other; endpoint-to-endpoint junctions are preferred on ties.
func junctionIndex(b, other [][2]float64) int {
	best, bestD := 0, math.Inf(1)
	for _, e := range []int{0, len(b) - 1} {
		j := nearestIndex(other, b[e])
		if d := dist2(b[e], other[j]); d < bestD {
			best, bestD = e, d
		}
	}
	for _, p := range [][2]float64{other[0], other[len(other)-1]} {
		j := nearestIndex(b, p)
		if d := dist2(b[j], p); d < bestD {
			best, bestD = j, d
		}
	}
	return best
}

// project returns the parameter s = i + t of the point of the border
// polyline nearest to p.
func project(border [][2]float64, p [2]float64) float64 {
	best, bestD := 0.0, math.Inf(1)
	// Scale longitude so that distances are roughly isotropic.
	k := math.Cos(p[1] * math.Pi / 180)
	for i := 0; i+1 < len(border); i++ {
		a, b := border[i], border[i+1]
		ax, ay := (a[0]-p[0])*k, a[1]-p[1]
		bx, by := (b[0]-p[0])*k, b[1]-p[1]
		dx, dy := bx-ax, by-ay
		t := 0.0
		if l := dx*dx + dy*dy; l > 0 {
			t = math.Max(0, math.Min(1, -(ax*dx+ay*dy)/l))
		}
		x, y := ax+t*dx, ay+t*dy
		if d := x*x + y*y; d < bestD {
			best, bestD = float64(i)+t, d
		}
	}
	return best
}

// cutBorder returns the stretch of border from one cut to the other: the
// cut positions and the border vertices strictly between them, reversed when
// the ring travels against the border's own direction.
func cutBorder(border [][2]float64, from, to borderCut) [][2]float64 {
	out := [][2]float64{from.p}
	if from.s <= to.s {
		for k := int(math.Floor(from.s)) + 1; float64(k) < to.s; k++ {
			out = appendDedup(out, border[k:k+1])
		}
	} else {
		for k := int(math.Ceil(from.s)) - 1; float64(k) > to.s; k-- {
			out = appendDedup(out, border[k:k+1])
		}
	}
	return appendDedup(out, [][2]float64{to.p})
}

// joinMembers concatenates the members end to end.  A member that, after the
// previous one, connects by its end rather than its start is reversed.  Joins
// wider than maxJoinGap, and a ring that does not close, are reported through
// warn: the gap is bridged by a straight line, which may cut across the area
// the source describes.
func joinMembers(members []ringMember, warn func(string), cv *conv) [][2]float64 {
	var coords [][2]float64
	for k, m := range members {
		c := m.coords
		if len(c) == 0 {
			continue
		}
		if len(coords) > 0 {
			last := coords[len(coords)-1]
			dStart, dEnd := distMetres(last, c[0]), distMetres(last, c[len(c)-1])
			if len(c) > 1 && dStart > maxJoinGap && dEnd <= maxJoinGap {
				c = reversed(c)
				dStart = dEnd
			}
			if dStart > maxJoinGap && warn != nil {
				warn(fmt.Sprintf("ring member %d%s does not connect to the previous member: gap of %.0f m between (%.6f, %.6f) and (%.6f, %.6f)",
					k+1, hrefNote(m), dStart, last[1], last[0], c[0][1], c[0][0]))
			}
		}
		coords = cv.appendDedup(coords, c)
	}
	if len(coords) > 1 && warn != nil {
		if d := distMetres(coords[len(coords)-1], coords[0]); d > maxJoinGap {
			warn(fmt.Sprintf("ring does not close: gap of %.0f m between its last (%.6f, %.6f) and first (%.6f, %.6f) position",
				d, coords[len(coords)-1][1], coords[len(coords)-1][0], coords[0][1], coords[0][0]))
		}
	}
	return coords
}

func hrefNote(m ringMember) string {
	if m.href == "" {
		return ""
	}
	return " (border " + m.href + ")"
}

func reversed(c [][2]float64) [][2]float64 {
	out := make([][2]float64, len(c))
	for i := range c {
		out[len(c)-1-i] = c[i]
	}
	return out
}

// maxJoinGap is the distance, in metres, beyond which two ring members that
// should meet are reported as not connecting.  Borders are trimmed at their
// position nearest to the neighbouring curve's endpoint, which need not be a
// border vertex, so small gaps are normal.
const maxJoinGap = 1000.0

// distMetres is the approximate ground distance between two lon/lat positions
// (equirectangular; adequate for distances of a few kilometres).
func distMetres(a, b [2]float64) float64 {
	const r = 6371008.8
	lat := (a[1] + b[1]) / 2 * math.Pi / 180
	dx := (b[0] - a[0]) * math.Pi / 180 * math.Cos(lat)
	dy := (b[1] - a[1]) * math.Pi / 180
	return r * math.Hypot(dx, dy)
}

func dist2(a, b [2]float64) float64 {
	dx, dy := a[0]-b[0], a[1]-b[1]
	return dx*dx + dy*dy
}

// nearestIndex returns the index of the border position closest to p.
func nearestIndex(border [][2]float64, p [2]float64) int {
	best, bestD := 0, math.Inf(1)
	for i, c := range border {
		// Planar in lon/lat: adequate for picking the nearest of a dense set
		// of boundary positions.
		if d := dist2(c, p); d < bestD {
			best, bestD = i, d
		}
	}
	return best
}

// resolveBorderCurve returns the coordinates of a GeoBorder referenced from a
// curveMember.
func resolveBorderCurve(href string, cv *conv, msg *goaixm.Message) ([][2]float64, error) {
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
			return segmentsToCoords(segs, cv, msg)
		}
	}
	return nil, fmt.Errorf("geojson: referenced border %s has no curve", href)
}

// segmentsToCoords converts a curve's segments, in document order, to a
// polyline.  Order matters: a boundary that runs out along a radial, around an
// arc and back would self-intersect if its segments were reordered.
func segmentsToCoords(segs *gml.CurveSegments, cv *conv, msg *goaixm.Message) ([][2]float64, error) {
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
			c = densifyGeodesic(c, cv.opt.GeodesicTolerance)
		case seg.LineStringSegment != nil:
			if seg.LineStringSegment.PosList != nil {
				c, err = posListToCoords(seg.LineStringSegment.PosList.Value)
			} else if len(seg.LineStringSegment.Pos) > 0 {
				c, err = posSliceToCoords(seg.LineStringSegment.Pos)
			}
			if strings.EqualFold(strings.TrimSpace(seg.LineStringSegment.Interpolation), "geodesic") {
				c = densifyGeodesic(c, cv.opt.GeodesicTolerance)
			}
		case seg.Arc != nil:
			c, err = discretizeArc(seg.Arc.Pos, seg.Arc.PosList, cv.opt.ArcTolerance)
		case seg.Circle != nil:
			c, err = discretizeCircle(seg.Circle.Pos, seg.Circle.PosList, cv.opt.ArcTolerance)
		case seg.ArcByCenterPoint != nil:
			c, err = discretizeArcByCenter(seg.ArcByCenterPoint, cv)
		case seg.CircleByCenterPoint != nil:
			c, err = discretizeCircleByCenter(seg.CircleByCenterPoint, cv)
		}
		if err != nil {
			return nil, err
		}
		if seg.Arc != nil || seg.Circle != nil || seg.ArcByCenterPoint != nil || seg.CircleByCenterPoint != nil {
			cv.markComputed(c)
		}
		coords = cv.appendDedup(coords, c)
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

// prefixWarn returns warn with every message prefixed, or nil for nil.
func prefixWarn(warn func(string), prefix string) func(string) {
	if warn == nil {
		return nil
	}
	return func(s string) { warn(prefix + s) }
}
