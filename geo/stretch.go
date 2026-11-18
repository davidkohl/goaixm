package geo

import (
	"errors"
	"math"
)

// Stretch is the part of a border polyline that a boundary follows between
// two of its own positions (see BorderStretch).
type Stretch struct {
	// Coords are the positions of the stretch ([lon, lat]): from, the
	// border vertices strictly between the two cuts (with
	// StretchOptions.CutPoints also the cut points), and to.
	Coords [][2]float64
	// FromOffset and ToOffset are the distances in metres from from and to
	// to their nearest points on the border.
	FromOffset, ToOffset float64
	// Reversed reports that the stretch runs against the border's own
	// direction.
	Reversed bool
	// Closed reports that the border is a closed ring (its first and last
	// positions coincide); Wrapped that the stretch taken runs through
	// that closing position (StretchOptions.Wrap only).
	Closed, Wrapped bool
}

// StretchOptions control BorderStretch.
type StretchOptions struct {
	// CutPoints inserts the cut point on the border after from (before
	// to) when from (to) lies off the border, as Options.BorderCutPoints
	// does for referenced borders.
	CutPoints bool
	// Wrap, for a closed border, takes the shorter of the two ways round
	// between the cuts, which may run through the closing position.
	// Without it, and always for an open border, the stretch is the one
	// that does not pass the border's ends: the rule geo applies to a
	// referenced GeoBorder in a ring.
	Wrap bool
}

// ErrShortBorder is returned by BorderStretch for a border of fewer than
// two positions.
var ErrShortBorder = errors.New("geo: border has fewer than two positions")

// BorderStretch returns the stretch of border ([lon, lat] positions) that a
// boundary follows from its position from to its position to: from and to
// are projected onto the border (nearest point, locally isotropic metric),
// and the stretch consists of from, the border vertices strictly between
// the two projections in the direction of travel, and to.  from and to are
// kept as given, so the stretch joins the boundary's own positions exactly.
//
// This is the cut geo applies to a GeoBorder referenced from a ring
// (gml:curveMember xlink:href) whose neighbouring curves end at from and
// start at to, so that writing the stretch inline and referencing the
// border give the same positions.
func BorderStretch(border [][2]float64, from, to [2]float64, opt StretchOptions) (Stretch, error) {
	n := len(border)
	if n < 2 {
		return Stretch{}, ErrShortBorder
	}
	s1, s2 := project(border, from), project(border, to)
	c1, c2 := borderCut{s1, from, true}, borderCut{s2, to, true}
	st := Stretch{Coords: cutBorder(border, c1, c2, opt.CutPoints), Reversed: s1 > s2}
	p1, p2 := borderPoint(border, s1), borderPoint(border, s2)
	st.FromOffset = GeodesicDistance(from[1], from[0], p1[1], p1[0])
	st.ToOffset = GeodesicDistance(to[1], to[0], p2[1], p2[0])
	st.Closed = n >= 4 && samePoint(border[0], border[n-1])
	if !st.Closed || !opt.Wrap || s1 == s2 {
		return st, nil
	}
	// The other way round: from the first cut to the border end it faces
	// when travelling away from the second cut, then from the other end
	// (the same position) to the second cut.
	first, last := borderCut{s: 0, p: border[0]}, borderCut{s: float64(n - 1), p: border[n-1]}
	var other [][2]float64
	if s1 < s2 {
		other = appendDedup(cutBorder(border, c1, first, opt.CutPoints), cutBorder(border, last, c2, opt.CutPoints))
	} else {
		other = appendDedup(cutBorder(border, c1, last, opt.CutPoints), cutBorder(border, first, c2, opt.CutPoints))
	}
	if pathLength(other) < pathLength(st.Coords) {
		st.Coords, st.Wrapped, st.Reversed = other, true, !st.Reversed
	}
	return st, nil
}

// pathLength is the geodesic length of a polyline in metres.
func pathLength(c [][2]float64) float64 {
	l := 0.0
	for i := 1; i < len(c); i++ {
		l += GeodesicDistance(c[i-1][1], c[i-1][0], c[i][1], c[i][0])
	}
	return l
}

// PathLength returns the geodesic length in metres of a polyline of
// [lon, lat] positions, each piece a geodesic (Vincenty inverse on WGS-84).
func PathLength(c [][2]float64) float64 { return pathLength(c) }

// GeodesicAzimuth returns the initial true bearing in degrees (0 to 360) of
// the geodesic from (lat1, lon1) to (lat2, lon2) on WGS-84, and false when
// the points coincide or the inverse problem does not converge
// (near-antipodal points).
func GeodesicAzimuth(lat1, lon1, lat2, lon2 float64) (float64, bool) {
	d, az, ok := geodesicInverse(lat1, lon1, lat2, lon2)
	if !ok || d == 0 {
		return 0, false
	}
	az = math.Mod(az, 360)
	if az < 0 {
		az += 360
	}
	return az, true
}
