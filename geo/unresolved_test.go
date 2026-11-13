package geo_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/geo"
)

// eammComponents parses the DONLON dataset (after applying edit to its
// text) and returns the components of EAMM, an aggregation of two
// contributor airspaces.
func eammComponents(t *testing.T, edit func(string) string) []geo.AirspaceComponent {
	t.Helper()
	raw, err := os.ReadFile(donlonPath)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := goaixm.Parse(context.Background(), strings.NewReader(edit(string(raw))), goaixm.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, ok := msg.Feature("fdaeffb4-6897-41fb-a33d-8861c2e91e69")
	if !ok {
		t.Fatal("EAMM not found")
	}
	comps, err := geo.AirspaceComponents(a.(*aixm.Airspace), msg)
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != 2 {
		t.Fatalf("%d components, want 2", len(comps))
	}
	return comps
}

// TestAirspaceComponentsUnresolved: a contributor airspace missing from the
// message contributes no geometry and is reported in Unresolved with its
// href and title; resolved contributors are not.
func TestAirspaceComponentsUnresolved(t *testing.T) {
	for i, c := range eammComponents(t, func(s string) string { return s }) {
		if len(c.Unresolved) != 0 || c.Geometry == nil {
			t.Errorf("complete dataset, component %d: unresolved %v, geometry %v", i, c.Unresolved, c.Geometry != nil)
		}
	}

	const missing = "urn:uuid:00000000-0000-0000-0000-00000000dead"
	comps := eammComponents(t, func(s string) string {
		return strings.Replace(s, `xlink:href="urn:uuid:0df377fe-dd53-4d60-b6c4-6546ef31d26b"`,
			`xlink:href="`+missing+`" xlink:title="EAMM PART 1"`, 1)
	})
	var got []aixm.Reference
	for _, c := range comps {
		got = append(got, c.Unresolved...)
	}
	if len(got) != 1 || got[0].Href != missing || got[0].Title != "EAMM PART 1" {
		t.Fatalf("unresolved = %+v, want one reference to %s titled EAMM PART 1", got, missing)
	}
	if comps[0].Geometry != nil || comps[1].Geometry == nil {
		t.Errorf("geometry: first %v (want none), second %v (want some)", comps[0].Geometry != nil, comps[1].Geometry != nil)
	}
	if len(comps[1].Unresolved) != 0 {
		t.Errorf("second component reported unresolved: %+v", comps[1].Unresolved)
	}
}

// TestAirspaceComponentsUnresolvedNoMessage: without a message nothing can
// be resolved, and nothing is reported.
func TestAirspaceComponentsUnresolvedNoMessage(t *testing.T) {
	raw, err := os.ReadFile(donlonPath)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := goaixm.Parse(context.Background(), strings.NewReader(string(raw)), goaixm.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := msg.Feature("fdaeffb4-6897-41fb-a33d-8861c2e91e69")
	comps, err := geo.AirspaceComponents(a.(*aixm.Airspace), nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range comps {
		if len(c.Unresolved) != 0 {
			t.Errorf("component %d: unresolved %v without a message", i, c.Unresolved)
		}
	}
}
