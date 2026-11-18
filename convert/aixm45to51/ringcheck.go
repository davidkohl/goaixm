package aixm45to51

import (
	"encoding/xml"
	"fmt"
	"math"
	"sort"

	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/geo"
)

// The ring check (Config.RingCheck): every horizontal projection written
// for a boundary is turned into positions the way a consumer does it (geo
// with the configured options, border stretches inline) and tested as a
// simple polygon ring in the lon/lat plane, as GeoJSON and OGC validity
// see it.  A ring that crosses or touches itself, or doubles back on
// itself (a spike), is reported as ring_invalid with where it happens.
// The geometry is written as the source gives it: the check reports, it
// does not repair.

// checkRing checks the planned ring of boundary b.
func (r *run) checkRing(b *boundary, issue geomIssue) {
	rg := b.plan
	if rg == nil || rg.degenerate != "" || len(rg.segs) == 0 || rg.segs[0].kind == segCircle {
		return
	}
	if hasBorderRef(rg.segs) {
		// The positions a consumer gets by resolving the reference: the
		// same cut, inline (equal vertex by vertex, see Reference).
		rg = r.buildRing(b, Inline, func(string, convert.Severity, string) {})
	}
	ring, err := r.ringPositions(rg)
	if err != nil {
		issue("ring_invalid", convert.Warning, "horizontal projection not readable for the ring check: "+err.Error())
		return
	}
	probs := ringProblems(ring)
	if len(probs) == 0 {
		return
	}
	first := probs[0]
	near := ""
	if k, d := nearestVertex(b, first.at); k >= 0 {
		near = fmt.Sprintf(", %.0f m from boundary vertex %d", d, k+1)
	}
	more := ""
	if len(probs) > 1 {
		more = fmt.Sprintf(" (%d places in all)", len(probs))
	}
	issue("ring_invalid", convert.Warning, fmt.Sprintf("horizontal projection is not a valid polygon: %s at %.6f %.6f (lat long)%s%s; written as the source gives it",
		first.what, first.at[1], first.at[0], near, more))
}

// ringPositions renders the ring as the converter writes it and turns it
// into [lon, lat] positions with geo (Config.RingCheck options).
func (r *run) ringPositions(rg *ring) ([][2]float64, error) {
	f := newFeature("Airspace", "00000000-0000-0000-0000-000000000000")
	doc := `<x xmlns:aixm="` + nsAIXM + `" xmlns:gml="` + nsGML + `" xmlns:xlink="` + nsXLink + `">` + r.surface(f, rg) + `</x>`
	var v struct {
		S aixm.Surface `xml:"http://www.aixm.aero/schema/5.1.1 Surface"`
	}
	if err := xml.Unmarshal([]byte(doc), &v); err != nil {
		return nil, err
	}
	rings, err := r.cfg.RingCheck.SurfaceRings(v.S.Patches, nil)
	if err != nil {
		return nil, err
	}
	if len(rings) == 0 || len(rings[0]) == 0 {
		return nil, fmt.Errorf("no positions")
	}
	return rings[0], nil
}

// nearestVertex returns the index of the boundary vertex nearest to p and
// its distance in metres (-1 when there is none).
func nearestVertex(b *boundary, p [2]float64) (int, float64) {
	best, bi := math.Inf(1), -1
	for i, v := range b.vx {
		if !v.ok {
			continue
		}
		if d := geoDistance(p, v.p.ll); d < best {
			best, bi = d, i
		}
	}
	return bi, best
}

func geoDistance(a, b [2]float64) float64 { return geo.GeodesicDistance(a[1], a[0], b[1], b[0]) }

type ringProblem struct {
	what string
	at   [2]float64
}

// ringProblems tests a closed ring of [lon, lat] positions as a simple
// polygon ring in the plane: no spike (an edge doubling back on the one
// before), no position used twice, and no two edges that cross or touch
// other than neighbours at their shared position.  Problems come in ring
// order.
func ringProblems(ring [][2]float64) []ringProblem {
	// Drop repeated consecutive positions and the closing position.
	var p [][2]float64
	for _, q := range ring {
		if len(p) == 0 || p[len(p)-1] != q {
			p = append(p, q)
		}
	}
	if len(p) > 1 && p[0] == p[len(p)-1] {
		p = p[:len(p)-1]
	}
	n := len(p)
	if n < 3 {
		return []ringProblem{{what: fmt.Sprintf("only %d distinct positions", n), at: ring[0]}}
	}
	var out []ringProblem
	// Spikes: the edge from i back over the edge into i.
	for i := 0; i < n; i++ {
		a, b, c := p[(i+n-1)%n], p[i], p[(i+1)%n]
		u := [2]float64{a[0] - b[0], a[1] - b[1]}
		w := [2]float64{c[0] - b[0], c[1] - b[1]}
		cr := u[0]*w[1] - u[1]*w[0]
		dot := u[0]*w[0] + u[1]*w[1]
		if dot > 0 && math.Abs(cr) <= 1e-12*math.Hypot(u[0], u[1])*math.Hypot(w[0], w[1]) {
			out = append(out, ringProblem{"spike (the ring doubles back on itself)", b})
		}
	}
	// A position used twice: the ring touches itself there.
	seen := make(map[[2]float64]int, n)
	for i, q := range p {
		if _, dup := seen[q]; dup {
			out = append(out, ringProblem{"ring touches itself at a repeated position", q})
		}
		seen[q] = i
	}
	// Edges that cross or touch: sweep over the edges sorted by their
	// smallest longitude.
	type edge struct {
		i          int
		minX, maxX float64
	}
	edges := make([]edge, n)
	for i := range p {
		a, b := p[i], p[(i+1)%n]
		edges[i] = edge{i, math.Min(a[0], b[0]), math.Max(a[0], b[0])}
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].minX < edges[j].minX })
	var active []edge
	for _, e := range edges {
		keep := active[:0]
		for _, o := range active {
			if o.maxX >= e.minX {
				keep = append(keep, o)
			}
		}
		active = keep
		for _, o := range active {
			i, j := e.i, o.i
			if j == (i+1)%n || i == (j+1)%n {
				continue // neighbours share a position (spikes above)
			}
			if at, ok := segmentsMeet(p[i], p[(i+1)%n], p[j], p[(j+1)%n]); ok {
				if _, dup := seen[at]; dup && (at == p[i] || at == p[(i+1)%n]) && (at == p[j] || at == p[(j+1)%n]) {
					continue // a repeated position, reported above
				}
				out = append(out, ringProblem{"edges cross or touch", at})
			}
		}
		active = append(active, e)
	}
	return out
}

// segmentsMeet reports whether the closed segments ab and cd have a point
// in common, and one such point.
func segmentsMeet(a, b, c, d [2]float64) ([2]float64, bool) {
	orient := func(p, q, r [2]float64) float64 {
		return (q[0]-p[0])*(r[1]-p[1]) - (q[1]-p[1])*(r[0]-p[0])
	}
	on := func(p, q, r [2]float64) bool { // r on segment pq, given collinear
		return math.Min(p[0], q[0]) <= r[0] && r[0] <= math.Max(p[0], q[0]) &&
			math.Min(p[1], q[1]) <= r[1] && r[1] <= math.Max(p[1], q[1])
	}
	d1, d2 := orient(c, d, a), orient(c, d, b)
	d3, d4 := orient(a, b, c), orient(a, b, d)
	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) && ((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		t := d1 / (d1 - d2)
		return [2]float64{a[0] + t*(b[0]-a[0]), a[1] + t*(b[1]-a[1])}, true
	}
	switch {
	case d1 == 0 && on(c, d, a):
		return a, true
	case d2 == 0 && on(c, d, b):
		return b, true
	case d3 == 0 && on(a, b, c):
		return c, true
	case d4 == 0 && on(a, b, d):
		return d, true
	}
	return [2]float64{}, false
}
