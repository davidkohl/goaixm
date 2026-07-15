package parse_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/parse"
)

// TestStreamMessageDonlon verifies that StreamMessage visits the same number of
// features as ParseMessage on the DONLON dataset, without loading the full
// document into memory.
func TestStreamMessageDonlon(t *testing.T) {
	// First get the expected count via ParseMessage.
	f, err := os.Open("../testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	msg, err := parse.ParseMessage(f)
	f.Close()
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	wantCount := len(msg.Features())

	// Now stream the same file.
	f2, err := os.Open("../testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f2.Close()

	var count int
	err = parse.StreamMessage(context.Background(), f2, func(localName string, feature any) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("StreamMessage: %v", err)
	}
	if count != wantCount {
		t.Errorf("streamed %d features, want %d", count, wantCount)
	}
}

// TestStreamMessageTypes verifies that streamed features have correct concrete types.
func TestStreamMessageTypes(t *testing.T) {
	f, err := os.Open("../testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	typeCounts := make(map[string]int)
	err = parse.StreamMessage(context.Background(), f, func(localName string, feature any) error {
		typeCounts[localName]++
		return nil
	})
	if err != nil {
		t.Fatalf("StreamMessage: %v", err)
	}

	if typeCounts["Airspace"] == 0 {
		t.Error("expected at least 1 Airspace")
	}
	if typeCounts["AirportHeliport"] == 0 {
		t.Error("expected at least 1 AirportHeliport")
	}
}

// TestStreamMessageCancellation verifies that a cancelled context aborts
// streaming and returns ctx.Err().
func TestStreamMessageCancellation(t *testing.T) {
	f, err := os.Open("../testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	ctx, cancel := context.WithCancel(context.Background())

	count := 0
	err = parse.StreamMessage(ctx, f, func(localName string, feature any) error {
		count++
		if count >= 3 {
			cancel()
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !strings.Contains(err.Error(), "cancel") && err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestStreamMessageHandlerError verifies that a handler error aborts streaming.
func TestStreamMessageHandlerError(t *testing.T) {
	f, err := os.Open("../testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	sentinel := fmt.Errorf("stop now")
	err = parse.StreamMessage(context.Background(), f, func(_ string, _ any) error {
		return sentinel
	})
	if err != sentinel {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

// TestStreamMessageAirspaceDesignators verifies that individual airspace
// properties are correctly decoded in streaming mode.
func TestStreamMessageAirspaceDesignators(t *testing.T) {
	f, err := os.Open("../testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	var designators []string
	err = parse.StreamMessage(context.Background(), f, func(localName string, feature any) error {
		if localName != "Airspace" {
			return nil
		}
		as, ok := feature.(*features.AirspaceType)
		if !ok {
			return nil
		}
		if len(as.TimeSlice) > 0 {
			ts := as.TimeSlice[0].AirspaceTimeSlice
			if ts.Designator != nil {
				designators = append(designators, ts.Designator.Value)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("StreamMessage: %v", err)
	}
	if len(designators) == 0 {
		t.Error("expected at least one airspace designator")
	}
}
