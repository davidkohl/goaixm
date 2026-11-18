package goaixm_test

import (
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
)

func TestDonlonFeatureCount(t *testing.T) {
	msg := parseDonlon(t, goaixm.Options{})
	if got := len(msg.Features()); got != 147 {
		t.Errorf("expected 147 features, got %d", got)
	}
	if w := msg.Warnings(); len(w) != 0 {
		t.Errorf("expected no warnings, got %d (first: %v)", len(w), w[0])
	}
}

func designators[T any](items []T, get func(T) *aixm.Value) map[string]T {
	out := map[string]T{}
	for _, it := range items {
		if d := get(it).String(); d != "" {
			out[d] = it
		}
	}
	return out
}

func TestDonlonAirports(t *testing.T) {
	msg := parseDonlon(t, goaixm.Options{})
	ads := goaixm.FeaturesOf[*aixm.AirportHeliport](msg)
	if len(ads) != 2 {
		t.Fatalf("expected 2 airports, got %d", len(ads))
	}
	by := designators(ads, func(a *aixm.AirportHeliport) *aixm.Value { return a.TimeSlice[0].Designator })
	for _, want := range []string{"EADD", "EADH"} {
		if _, ok := by[want]; !ok {
			t.Errorf("airport %s not found", want)
		}
	}
	if got := by["EADD"].UUID(); got != "1b54b2d6-a5ff-4e57-94c2-f4047a381c64" {
		t.Errorf("EADD identifier = %q", got)
	}
	if arp := by["EADD"].TimeSlice[0].ARP; arp.Pos() == nil {
		t.Error("EADD ARP has no position")
	}
}

func TestDonlonAirspaces(t *testing.T) {
	msg := parseDonlon(t, goaixm.Options{})
	as := goaixm.FeaturesOf[*aixm.Airspace](msg)
	if len(as) != 20 {
		t.Fatalf("expected 20 airspaces, got %d", len(as))
	}
	var withClass, withVolume int
	for _, a := range as {
		ts := a.TimeSlice[0]
		if ts.Interpretation != aixm.Baseline {
			t.Errorf("airspace %s: interpretation = %q", a.UUID(), ts.Interpretation)
		}
		if len(ts.Class) > 0 {
			withClass++
		}
		for _, gc := range ts.GeometryComponent {
			if gc.TheAirspaceVolume != nil {
				withVolume++
				break
			}
		}
	}
	if withVolume == 0 {
		t.Error("no airspace volume decoded")
	}
	t.Logf("airspaces with class: %d, with volume: %d", withClass, withVolume)
}

func TestDonlonNavaids(t *testing.T) {
	msg := parseDonlon(t, goaixm.Options{})
	navs := goaixm.FeaturesOf[*aixm.Navaid](msg)
	if len(navs) != 14 {
		t.Fatalf("expected 14 navaids, got %d", len(navs))
	}
	by := designators(navs, func(n *aixm.Navaid) *aixm.Value { return n.TimeSlice[0].Designator })
	if bor, ok := by["BOR"]; !ok || bor.UUID() != "08a1bbd5-ea70-4fe3-836a-ea9686349495" {
		t.Errorf("navaid BOR = %+v", bor)
	}
	if _, ok := by["DON"]; !ok {
		t.Error("navaid DON not found")
	}
	var withEquipment int
	for _, n := range navs {
		if len(n.TimeSlice[0].NavaidEquipment) > 0 {
			withEquipment++
		}
	}
	if withEquipment == 0 {
		t.Error("no navaid equipment decoded (inline NavaidComponent objects)")
	}
}

func TestDonlonRunwaysPointsRoutes(t *testing.T) {
	msg := parseDonlon(t, goaixm.Options{})
	rwys := designators(goaixm.FeaturesOf[*aixm.Runway](msg),
		func(r *aixm.Runway) *aixm.Value { return r.TimeSlice[0].Designator })
	for _, want := range []string{"09L/27R", "03/21", "09R/27L"} {
		if _, ok := rwys[want]; !ok {
			t.Errorf("runway %q not found", want)
		}
	}
	dps := goaixm.FeaturesOf[*aixm.DesignatedPoint](msg)
	if len(dps) != 14 {
		t.Errorf("expected 14 designated points, got %d", len(dps))
	}
	by := designators(dps, func(d *aixm.DesignatedPoint) *aixm.Value { return d.TimeSlice[0].Designator })
	for _, want := range []string{"TEMPO", "ABOLA", "ATLIM", "TORSA"} {
		if _, ok := by[want]; !ok {
			t.Errorf("designated point %q not found", want)
		}
	}
	if n := len(goaixm.FeaturesOf[*aixm.Route](msg)); n != 4 {
		t.Errorf("expected 4 routes, got %d", n)
	}
}

func TestDonlonFeatureLookup(t *testing.T) {
	msg := parseDonlon(t, goaixm.Options{})
	for id, kind := range map[string]string{
		"1b54b2d6-a5ff-4e57-94c2-f4047a381c64": "AirportHeliport",
		"08a1bbd5-ea70-4fe3-836a-ea9686349495": "Navaid",
		"9e51668f-bf8a-4f5b-ba6e-27087972b9b8": "Runway",
		"81e47548-9f00-4970-b641-8ff8f99098a5": "DesignatedPoint",
	} {
		f, ok := msg.Feature(id)
		if !ok {
			t.Errorf("feature %s not found", id)
			continue
		}
		if f.Kind() != kind {
			t.Errorf("feature %s: kind %s, want %s", id, f.Kind(), kind)
		}
	}
}

// Every reference in DONLON route segments resolves within the message.
func TestDonlonRouteSegmentReferencesResolve(t *testing.T) {
	msg := parseDonlon(t, goaixm.Options{})
	segs := goaixm.FeaturesOf[*aixm.RouteSegment](msg)
	if len(segs) == 0 {
		t.Fatal("no route segments")
	}
	for _, s := range segs {
		ts := s.TimeSlice[0]
		for _, p := range []*aixm.EnRouteSegmentPoint{ts.Start, ts.End} {
			if p == nil {
				t.Errorf("segment %s: missing start or end", s.UUID())
				continue
			}
			ref := p.PointChoiceFixDesignatedPoint
			if ref == nil {
				ref = p.PointChoiceNavaidSystem
			}
			if ref == nil {
				continue
			}
			if _, ok := msg.Resolve(ref.Href); !ok {
				t.Errorf("segment %s: %s not resolved", s.UUID(), ref.Href)
			}
		}
	}
}

func TestDonlonValidate(t *testing.T) {
	msg := parseDonlon(t, goaixm.Options{})
	errs := aixm.ValidateAll(msg.Features())
	for _, e := range errs {
		if e.Field != "geometryComponent" {
			t.Logf("validation: %v", e)
		}
	}
}
