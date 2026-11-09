package parse_test

import (
	"context"
	"encoding/xml"
	"os"
	"testing"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/parse"
)

// Extension API tests, run against the EUROCONTROL DONLON reference data set.

// airspaceOverlay is a caller-defined partial schema: it declares only the
// elements it needs.
type airspaceOverlay struct {
	XMLName   xml.Name `xml:"http://www.aixm.aero/schema/5.1.1 Airspace"`
	TimeSlice []struct {
		Slice struct {
			Designator string `xml:"http://www.aixm.aero/schema/5.1.1 designator"`
			Type       string `xml:"http://www.aixm.aero/schema/5.1.1 type"`
		} `xml:"http://www.aixm.aero/schema/5.1.1 AirspaceTimeSlice"`
	} `xml:"http://www.aixm.aero/schema/5.1.1 timeSlice"`
}

// myAirport replaces the generated AirportHeliport type.
type myAirport struct {
	XMLName    xml.Name `xml:"http://www.aixm.aero/schema/5.1.1 AirportHeliport"`
	Identifier string   `xml:"http://www.opengis.net/gml/3.2 identifier"`
	TimeSlice  []struct {
		Designator string `xml:"http://www.aixm.aero/schema/5.1.1 AirportHeliportTimeSlice>designator"`
	} `xml:"http://www.aixm.aero/schema/5.1.1 timeSlice"`
}

func (a *myAirport) GetGMLID() string      { return "" }
func (a *myAirport) GetIdentifier() string { return a.Identifier }
func (a *myAirport) GetNames() []string    { return nil }

func parseDonlon(t *testing.T, opts parse.ParseOptions) *parse.Message {
	t.Helper()
	f, err := os.Open(donlonFile)
	if err != nil {
		t.Fatalf("open DONLON fixture: %v", err)
	}
	defer f.Close()
	msg, err := parse.ParseMessageWithOptions(context.Background(), f, opts)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func TestRegistryExtendAndOverride(t *testing.T) {
	reg := parse.DefaultRegistry.Clone()
	reg.Extend("Airspace", func() any { return new(airspaceOverlay) })
	reg.Register("AirportHeliport", func() any { return new(myAirport) })

	msg := parseDonlon(t, parse.ParseOptions{Registry: reg, KeepRaw: true})

	// Overlay: every airspace carries an overlay agreeing with the typed model.
	as := msg.Airspaces()
	if len(as) == 0 {
		t.Fatal("no airspaces")
	}
	for _, a := range as {
		mem, ok := msg.Member(a)
		if !ok {
			t.Fatalf("no member for airspace %s", a.GetIdentifier())
		}
		ov, ok := parse.Extension[*airspaceOverlay](mem)
		if !ok {
			t.Fatalf("overlay missing for airspace %s", a.GetIdentifier())
		}
		if len(ov.TimeSlice) != len(a.TimeSlice) {
			t.Fatalf("overlay timeslices %d, typed %d", len(ov.TimeSlice), len(a.TimeSlice))
		}
		for i, ts := range a.TimeSlice {
			want := ""
			if d := ts.AirspaceTimeSlice.Designator; d != nil {
				want = d.Value
			}
			if got := ov.TimeSlice[i].Slice.Designator; got != want {
				t.Errorf("airspace %s slice %d: overlay designator %q, typed %q", a.GetIdentifier(), i, got, want)
			}
		}
	}

	// KeepRaw gives a lossless tree of the same feature.
	mem, _ := msg.Member(as[0])
	n, err := mem.Node()
	if err != nil {
		t.Fatal(err)
	}
	if n.Local() != "Airspace" || n.TextAt("identifier") != as[0].GetIdentifier() {
		t.Errorf("node %s / %q does not match airspace %s", n.Local(), n.TextAt("identifier"), as[0].GetIdentifier())
	}

	// Override: AirportHeliport now decodes into myAirport and is indexed.
	ad := msg.ByType("AirportHeliport")
	if len(ad) != 2 {
		t.Fatalf("airports = %d, want 2", len(ad))
	}
	for _, v := range ad {
		my, ok := v.(*myAirport)
		if !ok {
			t.Fatalf("override not applied: %T", v)
		}
		if _, ok := msg.Feature(my.Identifier); !ok {
			t.Errorf("overridden airport %s not indexed", my.Identifier)
		}
		if len(my.TimeSlice) == 0 || my.TimeSlice[0].Designator == "" {
			t.Errorf("overridden airport %s has no designator", my.Identifier)
		}
	}

	// The default registry is untouched.
	plain := parseDonlon(t, parse.ParseOptions{})
	if _, ok := plain.ByType("AirportHeliport")[0].(*features.AirportHeliportType); !ok {
		t.Error("override leaked into DefaultRegistry")
	}
	if len(plain.Members()) == 0 || len(plain.Extensions(plain.Airspaces()[0])) != 0 {
		t.Error("overlay leaked into DefaultRegistry")
	}
}

func TestGenericFallback(t *testing.T) {
	// An empty registry: everything falls back to the generic decoder.
	msg := parseDonlon(t, parse.ParseOptions{Registry: parse.NewRegistry(), Fallback: parse.FallbackGeneric})
	if len(msg.Warnings()) != 0 {
		t.Fatalf("warnings: %v", msg.Warnings()[0].Err)
	}
	if got := len(msg.Features()); got != 147 {
		t.Fatalf("features = %d, want 147", got)
	}
	for _, v := range msg.Features() {
		g, ok := v.(*parse.GenericFeature)
		if !ok {
			t.Fatalf("got %T", v)
		}
		if _, ok := msg.Feature(g.GetIdentifier()); !ok {
			t.Fatalf("generic %s %s not indexed", g.LocalName, g.GetIdentifier())
		}
	}

	// Without the fallback, unknown members are dropped with a warning each.
	msg = parseDonlon(t, parse.ParseOptions{Registry: parse.NewRegistry()})
	if len(msg.Features()) != 0 || len(msg.Warnings()) != 147 {
		t.Errorf("skip fallback: features=%d warnings=%d", len(msg.Features()), len(msg.Warnings()))
	}
}

func TestStreamWithOptions(t *testing.T) {
	f, err := os.Open(donlonFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	reg := parse.DefaultRegistry.Clone()
	reg.Extend("Airspace", func() any { return new(airspaceOverlay) })

	var members []parse.Member
	err = parse.StreamMessageWithOptions(context.Background(), f,
		parse.StreamOptions{Registry: reg, KeepRaw: true},
		func(m parse.Member) error { members = append(members, m); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 147 {
		t.Fatalf("members = %d, want 147", len(members))
	}
	var airspaces int
	for _, m := range members {
		if m.LocalName != "Airspace" {
			continue
		}
		airspaces++
		if _, ok := parse.Extension[*airspaceOverlay](m); !ok {
			t.Fatalf("stream overlay missing on member %d", m.Index)
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
