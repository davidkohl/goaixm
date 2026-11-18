package geo

import (
	"strings"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/gml"
)

// ---- Reference resolution ---------------------------------------------------

// centreResolverFor returns a resolver for same-document Point references.
//
// goaixm.Message indexes features by their UUID identifier, so a fragment
// reference such as "#p9898" naming a gml:id on a Point nested inside another
// feature cannot be resolved through it.  This walks the message's features
// for such points instead.
func centreResolverFor(msg *goaixm.Message) centreResolver {
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
func gmlIDIndex(msg *goaixm.Message) map[string][2]float64 {
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

	add := func(id string, pos *gml.DirectPosition) {
		if pos == nil {
			return
		}
		if lat, lon, err := firstLatLon(pos.Value); err == nil {
			out = append(out, identifiedPoint{id, lat, lon})
		}
	}
	addPlain := func(p *aixm.PointProperty) {
		switch {
		case p == nil:
		case p.Point != nil:
			add(p.Point.GMLID, p.Point.Pos)
		case p.ElevatedPoint != nil:
			add(p.ElevatedPoint.GMLID, p.ElevatedPoint.Pos)
		}
	}
	addElevated := func(p *aixm.ElevatedPointProperty) {
		if p != nil && p.ElevatedPoint != nil {
			add(p.ElevatedPoint.GMLID, p.ElevatedPoint.Pos)
		}
	}

	switch v := feat.(type) {
	case *aixm.DesignatedPoint:
		for _, ts := range v.TimeSlice {
			addPlain(ts.Location)
		}
	case *aixm.Navaid:
		for _, ts := range v.TimeSlice {
			addElevated(ts.Location)
		}
	case *aixm.AirportHeliport:
		for _, ts := range v.TimeSlice {
			addElevated(ts.ARP)
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
