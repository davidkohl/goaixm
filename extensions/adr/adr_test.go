package adr_test

import (
	"os"
	"testing"

	_ "gitlab.com/davidkohl/goaixm/extensions/adr"
	"gitlab.com/davidkohl/goaixm/parse"
)

func TestParseADRFeature(t *testing.T) {
	f, err := os.Open("../../testdata/adr_message.xml")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	msg, err := parse.ParseMessage(f)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	all := msg.Features()
	if len(all) != 2 {
		t.Errorf("expected 2 features, got %d", len(all))
	}

	airports := msg.Airports()
	if len(airports) != 1 {
		t.Fatalf("expected 1 airport, got %d", len(airports))
	}
	if got := airports[0].GetIdentifier(); got != "a1b2c3d4-0002-0000-0000-000000000000" {
		t.Errorf("airport identifier = %q", got)
	}

	sets := msg.ByType("AirportHeliportSet")
	if len(sets) != 1 {
		t.Fatalf("expected 1 AirportHeliportSet, got %d", len(sets))
	}
}
