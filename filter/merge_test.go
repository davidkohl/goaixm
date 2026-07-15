package filter_test

import (
	"testing"
	"time"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/filter"
	"gitlab.com/davidkohl/goaixm/gml"
	"gitlab.com/davidkohl/goaixm/types"
)

func makeAirspaceTS(interp features.Interpretation, seq uint32, designator string) features.TimeSlice {
	ts := &features.AirspaceTimeSliceType{
		Interpretation: string(interp),
		ValidTime: gml.TimePrimitivePropertyType{
			TimePeriod: &gml.TimePeriodType{
				BeginPosition: &gml.TimePositionType{Value: "2020-01-01"},
				EndPosition:   &gml.TimePositionType{Value: "2030-01-01"},
			},
		},
	}
	ts.SequenceNumber = &seq
	zero := uint32(0)
	ts.CorrectionNumber = &zero
	if designator != "" {
		ts.Designator = &types.CodeAirspaceDesignatorType{Value: designator}
	}
	return ts
}

func makeAirspaceTSWithType(interp features.Interpretation, seq uint32, designator, typ string) features.TimeSlice {
	ts := makeAirspaceTS(interp, seq, designator).(*features.AirspaceTimeSliceType)
	if typ != "" {
		ts.Type = &types.CodeAirspaceType{Value: typ}
	}
	return ts
}

// TestEffectiveAtBaselineOnly verifies that when only a BASELINE exists, the
// effective state returns exactly those field values.
func TestEffectiveAtBaselineOnly(t *testing.T) {
	slices := []features.TimeSlice{
		makeAirspaceTS(features.InterpretationBaseline, 1, "EGTT"),
	}
	state, err := filter.EffectiveAt(slices, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("EffectiveAt: %v", err)
	}
	ts := state.Slice.(*features.AirspaceTimeSliceType)
	if ts.Designator == nil || ts.Designator.Value != "EGTT" {
		t.Errorf("designator = %+v, want EGTT", ts.Designator)
	}
}

// TestEffectiveAtNoBaseline verifies that an error is returned when no BASELINE
// is valid at the requested time.
func TestEffectiveAtNoBaseline(t *testing.T) {
	slices := []features.TimeSlice{
		makeAirspaceTS(features.InterpretationSnapshot, 1, "EGTT"),
	}
	_, err := filter.EffectiveAt(slices, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected error for no BASELINE, got nil")
	}
}

// TestEffectiveAtPermDelta verifies that a PERMDELTA overrides a field from
// the BASELINE.
func TestEffectiveAtPermDelta(t *testing.T) {
	slices := []features.TimeSlice{
		makeAirspaceTS(features.InterpretationBaseline, 1, "EGTT"),
		makeAirspaceTS(features.InterpretationPermDelta, 2, "EGTT-NEW"),
	}
	state, err := filter.EffectiveAt(slices, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("EffectiveAt: %v", err)
	}
	ts := state.Slice.(*features.AirspaceTimeSliceType)
	if ts.Designator == nil || ts.Designator.Value != "EGTT-NEW" {
		t.Errorf("after PERMDELTA, designator = %+v, want EGTT-NEW", ts.Designator)
	}
}

// TestEffectiveAtTempDeltaOverridesPermDelta verifies TEMPDELTA applies on top
// of a PERMDELTA, and that non-overridden fields are preserved from the baseline.
func TestEffectiveAtTempDeltaOverridesPermDelta(t *testing.T) {
	slices := []features.TimeSlice{
		makeAirspaceTSWithType(features.InterpretationBaseline, 1, "EGTT", "CTA"),
		makeAirspaceTSWithType(features.InterpretationPermDelta, 2, "EGTT-PERM", ""),
		makeAirspaceTSWithType(features.InterpretationTempDelta, 3, "EGTT-TEMP", ""),
	}
	state, err := filter.EffectiveAt(slices, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("EffectiveAt: %v", err)
	}
	ts := state.Slice.(*features.AirspaceTimeSliceType)
	// TEMPDELTA should have applied last.
	if ts.Designator == nil || ts.Designator.Value != "EGTT-TEMP" {
		t.Errorf("designator = %+v, want EGTT-TEMP", ts.Designator)
	}
	// Type was not overridden by any delta — baseline value should be preserved.
	if ts.Type == nil || ts.Type.Value != "CTA" {
		t.Errorf("type = %+v, want CTA (inherited from baseline)", ts.Type)
	}
}

// TestEffectiveAtOutsideValidTime verifies that a delta outside the valid time
// window is not applied.
func TestEffectiveAtOutsideValidTime(t *testing.T) {
	baseline := makeAirspaceTS(features.InterpretationBaseline, 1, "EGTT")

	// PERMDELTA only valid in 2022.
	delta := &features.AirspaceTimeSliceType{
		Interpretation: string(features.InterpretationPermDelta),
		ValidTime: gml.TimePrimitivePropertyType{
			TimePeriod: &gml.TimePeriodType{
				BeginPosition: &gml.TimePositionType{Value: "2022-01-01"},
				EndPosition:   &gml.TimePositionType{Value: "2023-01-01"},
			},
		},
	}
	seq := uint32(2)
	zero := uint32(0)
	delta.SequenceNumber = &seq
	delta.CorrectionNumber = &zero
	delta.Designator = &types.CodeAirspaceDesignatorType{Value: "EGTT-DELTA"}

	slices := []features.TimeSlice{baseline, delta}

	// Query in 2025 — delta should NOT apply.
	state, err := filter.EffectiveAt(slices, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("EffectiveAt: %v", err)
	}
	ts := state.Slice.(*features.AirspaceTimeSliceType)
	if ts.Designator == nil || ts.Designator.Value != "EGTT" {
		t.Errorf("designator = %+v, want EGTT (delta should not apply in 2025)", ts.Designator)
	}
}
