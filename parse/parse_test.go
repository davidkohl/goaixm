package parse_test

import (
	"os"
	"testing"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/parse"
)

func TestParseBasicMessage(t *testing.T) {
	f, err := os.Open("../testdata/basic_message.xml")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	msg, err := parse.ParseMessage(f)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	all := msg.Features()
	if len(all) != 3 {
		t.Errorf("expected 3 features, got %d", len(all))
	}

	airspaces := msg.Airspaces()
	if len(airspaces) != 1 {
		t.Fatalf("expected 1 airspace, got %d", len(airspaces))
	}
	as := airspaces[0]
	if got := as.GetIdentifier(); got != "e34e0e14-0001-0000-0000-000000000000" {
		t.Errorf("airspace identifier = %q", got)
	}
	if len(as.TimeSlice) != 1 {
		t.Fatalf("expected 1 timeslice, got %d", len(as.TimeSlice))
	}
	ts := as.TimeSlice[0].AirspaceTimeSlice
	if ts.Interpretation != "BASELINE" {
		t.Errorf("interpretation = %q, want BASELINE", ts.Interpretation)
	}
	if ts.Designator == nil || ts.Designator.Value != "EGTT" {
		t.Errorf("airspace designator: got %+v", ts.Designator)
	}
}

func TestParseAirportHeliport(t *testing.T) {
	f, err := os.Open("../testdata/basic_message.xml")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	msg, err := parse.ParseMessage(f)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	airports := msg.Airports()
	if len(airports) != 1 {
		t.Fatalf("expected 1 airport, got %d", len(airports))
	}
	ad := airports[0]
	if got := ad.GetIdentifier(); got != "e34e0e14-0002-0000-0000-000000000000" {
		t.Errorf("airport identifier = %q", got)
	}
	ts := ad.TimeSlice[0].AirportHeliportTimeSlice
	if ts.Designator == nil || ts.Designator.Value != "EGLL" {
		t.Errorf("airport designator: got %+v", ts.Designator)
	}
}

func TestFeatureLookup(t *testing.T) {
	f, err := os.Open("../testdata/basic_message.xml")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	msg, err := parse.ParseMessage(f)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	v, ok := msg.Feature("e34e0e14-0001-0000-0000-000000000000")
	if !ok {
		t.Fatal("feature not found by identifier")
	}
	_, isAirspace := v.(*features.AirspaceType)
	if !isAirspace {
		t.Errorf("expected *AirspaceType, got %T", v)
	}
}

func TestByType(t *testing.T) {
	f, err := os.Open("../testdata/basic_message.xml")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	msg, err := parse.ParseMessage(f)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	navaids := msg.Navaids()
	if len(navaids) != 1 {
		t.Fatalf("expected 1 navaid, got %d", len(navaids))
	}
	nav := navaids[0]
	ts := nav.TimeSlice[0].NavaidTimeSlice
	if ts.Type == nil || ts.Type.Value != "NDB" {
		t.Errorf("navaid type: got %+v", ts.Type)
	}
}
