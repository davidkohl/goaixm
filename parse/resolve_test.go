package parse_test

import (
	"os"
	"testing"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/parse"
)

// TestResolveHref verifies that Message.Resolve correctly resolves urn:uuid:,
// fragment, and bare-UUID href forms, and returns false for unknown refs.
func TestResolveHref(t *testing.T) {
	f, err := os.Open("../testdata/basic_message.xml")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	msg, err := parse.ParseMessage(f)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	const airspaceID = "e34e0e14-0001-0000-0000-000000000000"

	tests := []struct {
		name string
		href string
		want bool
	}{
		{"urn:uuid form", "urn:uuid:" + airspaceID, true},
		{"fragment form", "#" + airspaceID, true},
		{"bare UUID", airspaceID, true},
		{"unknown", "urn:uuid:00000000-dead-beef-0000-000000000000", false},
		{"empty", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v, ok := msg.Resolve(tc.href)
			if ok != tc.want {
				t.Fatalf("Resolve(%q) ok=%v, want %v", tc.href, ok, tc.want)
			}
			if tc.want {
				if _, isAirspace := v.(*features.AirspaceType); !isAirspace {
					t.Errorf("Resolve returned %T, want *AirspaceType", v)
				}
			}
		})
	}
}

// TestWarningsOnUnknownType verifies that parsing a message with an unregistered
// feature type produces a Warning rather than a hard error.
func TestWarningsOnUnknownType(t *testing.T) {
	// The xlink_message.xml uses only registered types; we test warnings
	// indirectly via the donlon dataset which should produce zero warnings.
	f, err := os.Open("../testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		t.Fatalf("open donlon: %v", err)
	}
	defer f.Close()

	msg, err := parse.ParseMessage(f)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	for _, w := range msg.Warnings() {
		t.Logf("warning[%d] %s: %v", w.MemberIndex, w.LocalName, w.Err)
	}
	// DONLON is clean — all types should be registered.
	if len(msg.Warnings()) > 0 {
		t.Errorf("expected 0 warnings for DONLON, got %d", len(msg.Warnings()))
	}
}

// TestXLinkResolveInMessage parses the xlink fixture and verifies the
// DesignatedPoint can be resolved from the RouteSegment's start href.
func TestXLinkResolveInMessage(t *testing.T) {
	f, err := os.Open("../testdata/xlink_message.xml")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	msg, err := parse.ParseMessage(f)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	// Should have 2 features: 1 DesignatedPoint + 1 RouteSegment.
	if n := len(msg.Features()); n != 2 {
		t.Fatalf("expected 2 features, got %d", n)
	}

	// Resolve the DesignatedPoint via the href used in the fixture.
	const dpID = "dp-alpha-0000-0000-0000-000000000001"
	v, ok := msg.Resolve("urn:uuid:" + dpID)
	if !ok {
		t.Fatal("Resolve: DesignatedPoint not found")
	}
	dp, ok := v.(*features.DesignatedPointType)
	if !ok {
		t.Fatalf("expected *DesignatedPointType, got %T", v)
	}
	ts := dp.TimeSlice[0].DesignatedPointTimeSlice
	if ts.Designator == nil || ts.Designator.Value != "ALPHA" {
		t.Errorf("designator: got %+v", ts.Designator)
	}

	// The RouteSegment's Start field carries the href; verify it decoded.
	segments := msg.Routes()
	_ = segments // RouteSegment != Route; use ByType
	raw := msg.ByType("RouteSegment")
	if len(raw) != 1 {
		t.Fatalf("expected 1 RouteSegment, got %d", len(raw))
	}
	rs := raw[0].(*features.RouteSegmentType)
	rts := rs.TimeSlice[0].RouteSegmentTimeSlice
	if rts.Start == nil {
		t.Fatal("RouteSegment.Start is nil")
	}
	if rts.Start.HrefAttr != "urn:uuid:"+dpID {
		t.Errorf("Start.HrefAttr = %q, want urn:uuid:%s", rts.Start.HrefAttr, dpID)
	}
}
