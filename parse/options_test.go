package parse_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"gitlab.com/davidkohl/goaixm/parse"
)

func TestParseMessageWithOptionsMaxBytes(t *testing.T) {
	data, err := os.ReadFile("../testdata/basic_message.xml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	// Should succeed with limit larger than file.
	msg, err := parse.ParseMessageWithOptions(context.Background(), bytes.NewReader(data), parse.ParseOptions{
		MaxBytes: int64(len(data)) + 1024,
	})
	if err != nil {
		t.Fatalf("ParseMessageWithOptions: %v", err)
	}
	if len(msg.Features()) != 3 {
		t.Errorf("expected 3 features, got %d", len(msg.Features()))
	}

	// Should fail with limit smaller than file.
	_, err = parse.ParseMessageWithOptions(context.Background(), bytes.NewReader(data), parse.ParseOptions{
		MaxBytes: 10,
	})
	if err == nil {
		t.Fatal("expected error for MaxBytes exceeded, got nil")
	}
	if !strings.Contains(err.Error(), "maximum size") {
		t.Errorf("error should mention maximum size, got: %v", err)
	}
}

func TestParseMessageWithOptionsStrictMode(t *testing.T) {
	// StrictMode=false: unknown types become warnings, not errors.
	f, err := os.Open("../testdata/basic_message.xml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	msg, err := parse.ParseMessageWithOptions(context.Background(), f, parse.ParseOptions{
		StrictMode: false,
	})
	if err != nil {
		t.Fatalf("StrictMode=false: %v", err)
	}
	if len(msg.Features()) == 0 {
		t.Error("expected features")
	}
}

func TestParseMessageWithOptionsContextCancelled(t *testing.T) {
	data, err := os.ReadFile("../testdata/basic_message.xml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = parse.ParseMessageWithOptions(ctx, bytes.NewReader(data), parse.ParseOptions{})
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}

func TestParseMessageWithOptionsNilRegistry(t *testing.T) {
	// Nil registry should fall back to DefaultRegistry.
	f, err := os.Open("../testdata/basic_message.xml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	msg, err := parse.ParseMessageWithOptions(context.Background(), f, parse.ParseOptions{
		Registry: nil,
	})
	if err != nil {
		t.Fatalf("nil registry: %v", err)
	}
	if len(msg.Features()) != 3 {
		t.Errorf("expected 3 features, got %d", len(msg.Features()))
	}
}

func TestParseADRMessage(t *testing.T) {
	f, err := os.Open("../testdata/adr_message.xml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	msg, err := parse.ParseMessage(f)
	if err != nil {
		t.Fatalf("ParseMessage on ADR fixture: %v", err)
	}
	if len(msg.Features()) == 0 {
		t.Error("expected at least one feature")
	}
	t.Logf("ADR fixture warnings: %d", len(msg.Warnings()))
}

func TestMessageTypedAccessorsDonlon(t *testing.T) {
	f, err := os.Open("../testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	msg, err := parse.ParseMessage(f)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	// These typed accessors were added in Phase 6 and were at 0% coverage.
	// DONLON contains runways and designated points but not SIDs/STARs/procedures.
	tests := []struct {
		name  string
		count int
		want  string
	}{
		{"Runways", len(msg.Runways()), ">=1"},
		{"DesignatedPoints", len(msg.DesignatedPoints()), ">=1"},
		{"Routes", len(msg.Routes()), ">=1"},
	}
	for _, tc := range tests {
		if tc.count == 0 {
			t.Errorf("%s: got 0, want %s", tc.name, tc.want)
		}
	}

	// Accessors for types not present in DONLON should return empty, not panic.
	_ = msg.RunwayDirections()
	_ = msg.Procedures()
	_ = msg.Departures()
	_ = msg.Arrivals()
	_ = msg.Taxiways()
	_ = msg.Units()
	_ = msg.Aprons()
}
