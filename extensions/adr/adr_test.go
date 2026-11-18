package adr_test

import (
	"context"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/extensions/adr"
)

const fixture = "../../testdata/adr_message.xml"

func parse(t *testing.T, reg *goaixm.Registry) *goaixm.Message {
	t.Helper()
	msg, err := goaixm.ParseFile(context.Background(), fixture, goaixm.Options{Registry: reg})
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

func TestRegister(t *testing.T) {
	reg := goaixm.NewDefaultRegistry()
	adr.Register(reg)
	msg := parse(t, reg)

	if n := len(msg.Features()); n != 2 {
		t.Errorf("expected 2 features, got %d", n)
	}
	sets := goaixm.FeaturesOf[*adr.AirportHeliportSet](msg)
	if len(sets) != 1 {
		t.Fatalf("expected 1 AirportHeliportSet, got %d", len(sets))
	}
	ts := sets[0].TimeSlices()
	if len(ts) != 1 || sets[0].TimeSlice[0].Name.String() != "ECAC Airports Set" {
		t.Errorf("set time slice = %+v", sets[0].TimeSlice)
	}
	if ts[0].Common().Interpretation == "" {
		t.Error("ADR time slice lost its interpretation")
	}

	// Without registration the ADR feature is reported, not decoded.
	plain := parse(t, nil)
	if len(plain.Features()) != 1 || len(plain.Warnings()) != 1 {
		t.Errorf("unregistered: features=%d warnings=%d", len(plain.Features()), len(plain.Warnings()))
	}
}

func TestDecodeExtension(t *testing.T) {
	msg := parse(t, nil)
	ads := goaixm.FeaturesOf[*aixm.AirportHeliport](msg)
	if len(ads) != 1 {
		t.Fatalf("expected 1 airport, got %d", len(ads))
	}
	if got := ads[0].UUID(); got != "a1b2c3d4-0002-0000-0000-000000000000" {
		t.Errorf("airport identifier = %q", got)
	}
	var found bool
	for _, e := range ads[0].TimeSlice[0].Extension {
		if ext, ok := adr.DecodeExtension[adr.AirportHeliportExtension](e); ok {
			found = true
			if ext.NMDesignator.String() != "LFPG" {
				t.Errorf("nmDesignator = %q", ext.NMDesignator.String())
			}
		}
		if _, ok := adr.DecodeExtension[adr.AirspaceExtension](e); ok {
			t.Error("airport extension decoded as airspace extension")
		}
	}
	if !found {
		t.Error("no ADR AirportHeliportExtension found")
	}
}
