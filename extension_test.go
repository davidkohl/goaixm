package goaixm_test

import (
	"context"
	"encoding/xml"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
)

// Extension API tests, run against the EUROCONTROL DONLON reference data set.

// airspaceOverlay is a caller-defined partial schema: it declares only the
// elements it needs.
type airspaceOverlay struct {
	TimeSlice []struct {
		Designator string `xml:"http://www.aixm.aero/schema/5.1.1 designator"`
		Type       string `xml:"http://www.aixm.aero/schema/5.1.1 type"`
	} `xml:"http://www.aixm.aero/schema/5.1.1 timeSlice>AirspaceTimeSlice"`
}

// myAirport replaces the generated AirportHeliport type.
type myAirport struct {
	XMLName xml.Name `xml:"http://www.aixm.aero/schema/5.1.1 AirportHeliport"`
	aixm.FeatureCommon
	Designators []string `xml:"http://www.aixm.aero/schema/5.1.1 timeSlice>AirportHeliportTimeSlice>designator"`
}

func (*myAirport) Kind() string                 { return "AirportHeliport" }
func (*myAirport) TimeSlices() []aixm.TimeSlice { return nil }

func TestRegistryExtendAndRegister(t *testing.T) {
	reg := goaixm.NewDefaultRegistry()
	reg.Extend("Airspace", func() any { return new(airspaceOverlay) })
	reg.Register("AirportHeliport", func() aixm.Feature { return new(myAirport) })

	msg := parseDonlon(t, goaixm.Options{Registry: reg, KeepRaw: true})

	// Overlay: every airspace carries an overlay agreeing with the model.
	as := goaixm.FeaturesOf[*aixm.Airspace](msg)
	if len(as) == 0 {
		t.Fatal("no airspaces")
	}
	for _, a := range as {
		mem, ok := msg.Member(a)
		if !ok {
			t.Fatalf("no member for airspace %s", a.UUID())
		}
		ov, ok := goaixm.Extension[*airspaceOverlay](mem)
		if !ok {
			t.Fatalf("overlay missing for airspace %s", a.UUID())
		}
		if len(ov.TimeSlice) != len(a.TimeSlice) {
			t.Fatalf("overlay slices %d, model %d", len(ov.TimeSlice), len(a.TimeSlice))
		}
		for i, ts := range a.TimeSlice {
			if got, want := ov.TimeSlice[i].Designator, ts.Designator.String(); got != want {
				t.Errorf("airspace %s slice %d: overlay %q, model %q", a.UUID(), i, got, want)
			}
		}
		if len(msg.Extensions(a)) != 1 {
			t.Errorf("Extensions(%s) = %d", a.UUID(), len(msg.Extensions(a)))
		}
	}

	// KeepRaw gives a lossless tree of the same feature.
	mem, _ := msg.Member(as[0])
	n, err := mem.Node()
	if err != nil {
		t.Fatal(err)
	}
	if n.Local() != "Airspace" || n.TextAt("identifier") != as[0].UUID() {
		t.Errorf("node %s / %q does not match airspace %s", n.Local(), n.TextAt("identifier"), as[0].UUID())
	}

	// Register: AirportHeliport now decodes into myAirport and is indexed.
	ads := msg.ByKind("AirportHeliport")
	if len(ads) != 2 {
		t.Fatalf("airports = %d, want 2", len(ads))
	}
	for _, f := range ads {
		my, ok := f.(*myAirport)
		if !ok {
			t.Fatalf("Register not applied: %T", f)
		}
		if _, ok := msg.Feature(my.UUID()); !ok {
			t.Errorf("airport %s not indexed", my.UUID())
		}
		if len(my.Designators) == 0 || my.Designators[0] == "" {
			t.Errorf("airport %s has no designator", my.UUID())
		}
	}

	// The default registry is untouched.
	plain := parseDonlon(t, goaixm.Options{})
	if _, ok := plain.ByKind("AirportHeliport")[0].(*aixm.AirportHeliport); !ok {
		t.Error("Register leaked into the default registry")
	}
	if len(plain.Extensions(plain.ByKind("Airspace")[0])) != 0 {
		t.Error("Extend leaked into the default registry")
	}
}

func TestRegistryClone(t *testing.T) {
	a := goaixm.NewDefaultRegistry()
	b := a.Clone()
	b.Register("Custom", func() aixm.Feature { return new(myAirport) })
	if a.Has("Custom") || !b.Has("Custom") {
		t.Error("Clone is not independent")
	}
	if len(a.Kinds()) < 100 || len(b.Kinds()) != len(a.Kinds())+1 {
		t.Errorf("kinds: %d / %d", len(a.Kinds()), len(b.Kinds()))
	}
}

func TestGenericFallback(t *testing.T) {
	// An empty registry: everything falls back to the generic decoder.
	msg := parseDonlon(t, goaixm.Options{Registry: goaixm.NewRegistry(), Fallback: goaixm.FallbackGeneric})
	if len(msg.Warnings()) != 0 {
		t.Fatalf("warnings: %v", msg.Warnings()[0])
	}
	if got := len(msg.Features()); got != 147 {
		t.Fatalf("features = %d, want 147", got)
	}
	for _, f := range msg.Features() {
		g, ok := f.(*goaixm.GenericFeature)
		if !ok {
			t.Fatalf("got %T", f)
		}
		if _, ok := msg.Feature(g.UUID()); !ok {
			t.Fatalf("generic %s %s not indexed", g.Kind(), g.UUID())
		}
	}

	// Without the fallback, unknown members are dropped with a warning each.
	msg = parseDonlon(t, goaixm.Options{Registry: goaixm.NewRegistry()})
	if len(msg.Features()) != 0 || len(msg.Warnings()) != 147 {
		t.Errorf("skip fallback: features=%d warnings=%d", len(msg.Features()), len(msg.Warnings()))
	}
}

func TestStreamWithOverlay(t *testing.T) {
	requireDonlon(t)
	reg := goaixm.NewDefaultRegistry()
	reg.Extend("Airspace", func() any { return new(airspaceOverlay) })

	var members []goaixm.Member
	err := goaixm.StreamFile(context.Background(), donlonFile, goaixm.Options{Registry: reg, KeepRaw: true},
		func(m goaixm.Member) error { members = append(members, m); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 147 {
		t.Fatalf("members = %d, want 147", len(members))
	}
	var airspaces int
	for _, m := range members {
		if m.Kind != "Airspace" {
			continue
		}
		airspaces++
		if _, ok := goaixm.Extension[*airspaceOverlay](m); !ok {
			t.Fatalf("overlay missing on member %d", m.Index)
		}
		n, err := m.Node()
		if err != nil {
			t.Fatal(err)
		}
		if n.Local() != "Airspace" {
			t.Errorf("node root = %s", n.Local())
		}
	}
	if airspaces == 0 {
		t.Error("no airspaces streamed")
	}
}
