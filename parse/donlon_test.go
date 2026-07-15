package parse_test

import (
	"os"
	"testing"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/parse"
)

const donlonFile = "../testdata/donlon/EA_AIP_DS_FULL_20170701.xml"

func openDonlon(t *testing.T) *parse.Message {
	t.Helper()
	f, err := os.Open(donlonFile)
	if err != nil {
		t.Fatalf("open DONLON fixture: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	msg, err := parse.ParseMessage(f)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	return msg
}

// TestDonlonFeatureCount verifies all 147 hasMember elements decode successfully.
func TestDonlonFeatureCount(t *testing.T) {
	msg := openDonlon(t)
	if got := len(msg.Features()); got != 147 {
		t.Errorf("expected 147 features, got %d", got)
	}
}

// TestDonlonAirports checks both DONLON airports decode with correct designators.
func TestDonlonAirports(t *testing.T) {
	msg := openDonlon(t)

	airports := msg.Airports()
	if len(airports) != 2 {
		t.Fatalf("expected 2 airports, got %d", len(airports))
	}

	byDesignator := make(map[string]*features.AirportHeliportType)
	for _, a := range airports {
		ts := a.TimeSlice[0].AirportHeliportTimeSlice
		if ts.Designator != nil {
			byDesignator[ts.Designator.Value] = a
		}
	}

	for _, want := range []string{"EADD", "EADH"} {
		if _, ok := byDesignator[want]; !ok {
			t.Errorf("airport %s not found", want)
		}
	}

	eadd := byDesignator["EADD"]
	if eadd.GetIdentifier() != "1b54b2d6-a5ff-4e57-94c2-f4047a381c64" {
		t.Errorf("EADD identifier = %q", eadd.GetIdentifier())
	}
}

// TestDonlonAirspaces checks all 20 airspaces decode with BASELINE interpretation.
func TestDonlonAirspaces(t *testing.T) {
	msg := openDonlon(t)

	airspaces := msg.Airspaces()
	if len(airspaces) != 20 {
		t.Fatalf("expected 20 airspaces, got %d", len(airspaces))
	}

	for _, a := range airspaces {
		ts := a.TimeSlice[0].AirspaceTimeSlice
		if ts.Interpretation != "BASELINE" {
			t.Errorf("airspace %s: interpretation = %q, want BASELINE", a.GetIdentifier(), ts.Interpretation)
		}
	}
}

// TestDonlonNavaids checks all 14 navaids decode; spot-checks BOR and DON.
func TestDonlonNavaids(t *testing.T) {
	msg := openDonlon(t)

	navaids := msg.Navaids()
	if len(navaids) != 14 {
		t.Fatalf("expected 14 navaids, got %d", len(navaids))
	}

	byDesignator := make(map[string]*features.NavaidType)
	for _, n := range navaids {
		ts := n.TimeSlice[0].NavaidTimeSlice
		if ts.Designator != nil {
			byDesignator[ts.Designator.Value] = n
		}
	}

	bor, ok := byDesignator["BOR"]
	if !ok {
		t.Fatal("navaid BOR not found")
	}
	if bor.GetIdentifier() != "08a1bbd5-ea70-4fe3-836a-ea9686349495" {
		t.Errorf("BOR identifier = %q", bor.GetIdentifier())
	}

	if _, ok := byDesignator["DON"]; !ok {
		t.Error("navaid DON not found")
	}
}

// TestDonlonRunways checks all 3 runways decode with correct designators.
func TestDonlonRunways(t *testing.T) {
	msg := openDonlon(t)

	runways := msg.Runways()
	if len(runways) != 3 {
		t.Fatalf("expected 3 runways, got %d", len(runways))
	}

	wantDesignators := map[string]bool{
		"09L/27R": false,
		"03/21":   false,
		"09R/27L": false,
	}
	for _, r := range runways {
		ts := r.TimeSlice[0].RunwayTimeSlice
		if ts.Designator != nil {
			wantDesignators[ts.Designator.Value] = true
		}
	}
	for des, found := range wantDesignators {
		if !found {
			t.Errorf("runway %q not found", des)
		}
	}
}

// TestDonlonDesignatedPoints checks all 14 designated points decode.
func TestDonlonDesignatedPoints(t *testing.T) {
	msg := openDonlon(t)

	dps := msg.DesignatedPoints()
	if len(dps) != 14 {
		t.Fatalf("expected 14 designated points, got %d", len(dps))
	}

	byDesignator := make(map[string]bool)
	for _, dp := range dps {
		ts := dp.TimeSlice[0].DesignatedPointTimeSlice
		if ts.Designator != nil {
			byDesignator[ts.Designator.Value] = true
		}
	}
	for _, want := range []string{"TEMPO", "ABOLA", "ATLIM", "TORSA"} {
		if !byDesignator[want] {
			t.Errorf("designated point %q not found", want)
		}
	}
}

// TestDonlonFeatureLookup verifies the identifier index built during decode.
func TestDonlonFeatureLookup(t *testing.T) {
	msg := openDonlon(t)

	tests := []struct {
		id      string
		wantType string
	}{
		{"1b54b2d6-a5ff-4e57-94c2-f4047a381c64", "*features.AirportHeliportType"},
		{"08a1bbd5-ea70-4fe3-836a-ea9686349495", "*features.NavaidType"},
		{"9e51668f-bf8a-4f5b-ba6e-27087972b9b8", "*features.RunwayType"},
		{"81e47548-9f00-4970-b641-8ff8f99098a5", "*features.DesignatedPointType"},
	}

	for _, tt := range tests {
		v, ok := msg.Feature(tt.id)
		if !ok {
			t.Errorf("feature %s not found", tt.id)
			continue
		}
		var gotType string
		switch v.(type) {
		case *features.AirportHeliportType:
			gotType = "*features.AirportHeliportType"
		case *features.NavaidType:
			gotType = "*features.NavaidType"
		case *features.RunwayType:
			gotType = "*features.RunwayType"
		case *features.DesignatedPointType:
			gotType = "*features.DesignatedPointType"
		default:
			gotType = "unknown"
		}
		if gotType != tt.wantType {
			t.Errorf("feature %s: type = %s, want %s", tt.id, gotType, tt.wantType)
		}
	}
}

// TestDonlonRoutes checks all 4 routes decode.
func TestDonlonRoutes(t *testing.T) {
	msg := openDonlon(t)

	routes := msg.Routes()
	if len(routes) != 4 {
		t.Fatalf("expected 4 routes, got %d", len(routes))
	}
}
