package features_test

import (
	"testing"
	"time"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/gml"
	"gitlab.com/davidkohl/goaixm/types"
)

// ---- Feature interface methods ---------------------------------------------

func TestGetGMLID(t *testing.T) {
	a := &features.AirspaceType{GMLID: "AS-1"}
	if got := a.GetGMLID(); got != "AS-1" {
		t.Errorf("GetGMLID = %q, want AS-1", got)
	}

	ad := &features.AirportHeliportType{GMLID: "AD-1"}
	if got := ad.GetGMLID(); got != "AD-1" {
		t.Errorf("GetGMLID = %q, want AD-1", got)
	}

	rwy := &features.RunwayType{GMLID: "RWY-1"}
	if got := rwy.GetGMLID(); got != "RWY-1" {
		t.Errorf("GetGMLID = %q, want RWY-1", got)
	}

	nav := &features.NavaidType{GMLID: "NAV-1"}
	if got := nav.GetGMLID(); got != "NAV-1" {
		t.Errorf("GetGMLID = %q, want NAV-1", got)
	}

	dp := &features.DesignatedPointType{GMLID: "DP-1"}
	if got := dp.GetGMLID(); got != "DP-1" {
		t.Errorf("GetGMLID = %q, want DP-1", got)
	}
}

func TestGetNames(t *testing.T) {
	a := &features.AirspaceType{
		Name: []gml.CodeType{{Value: "LONDON UIR"}, {Value: "EGTT"}},
	}
	names := a.GetNames()
	if len(names) != 2 || names[0] != "LONDON UIR" || names[1] != "EGTT" {
		t.Errorf("GetNames = %v, want [LONDON UIR EGTT]", names)
	}

	// Empty names slice.
	a2 := &features.AirspaceType{}
	if got := a2.GetNames(); len(got) != 0 {
		t.Errorf("GetNames on empty = %v, want []", got)
	}
}

func TestGetIdentifierNil(t *testing.T) {
	a := &features.AirspaceType{} // nil Identifier
	if got := a.GetIdentifier(); got != "" {
		t.Errorf("GetIdentifier nil = %q, want empty", got)
	}
}

// ---- TimeSlices() ----------------------------------------------------------

func makeSeqZero() (*uint32, *uint32) {
	seq, cor := uint32(1), uint32(0)
	return &seq, &cor
}

func TestAirspaceTimeSlices(t *testing.T) {
	seq, cor := makeSeqZero()
	a := &features.AirspaceType{
		TimeSlice: []features.AirspaceTimeSlicePropertyType{
			{AirspaceTimeSlice: features.AirspaceTimeSliceType{
				Interpretation:   "BASELINE",
				SequenceNumber:   seq,
				CorrectionNumber: cor,
			}},
		},
	}
	slices := a.TimeSlices()
	if len(slices) != 1 {
		t.Fatalf("TimeSlices len = %d, want 1", len(slices))
	}
	if slices[0].GetInterpretation() != features.InterpretationBaseline {
		t.Errorf("interpretation = %v", slices[0].GetInterpretation())
	}
	if slices[0].GetSequenceNumber() != 1 {
		t.Errorf("sequenceNumber = %d", slices[0].GetSequenceNumber())
	}
}

func TestAirportHeliportTimeSlices(t *testing.T) {
	seq, cor := makeSeqZero()
	ad := &features.AirportHeliportType{
		TimeSlice: []features.AirportHeliportTimeSlicePropertyType{
			{AirportHeliportTimeSlice: features.AirportHeliportTimeSliceType{
				Interpretation:   "BASELINE",
				SequenceNumber:   seq,
				CorrectionNumber: cor,
			}},
		},
	}
	slices := ad.TimeSlices()
	if len(slices) != 1 {
		t.Fatalf("TimeSlices len = %d, want 1", len(slices))
	}
	if slices[0].GetInterpretation() != features.InterpretationBaseline {
		t.Errorf("interpretation = %v", slices[0].GetInterpretation())
	}
}

func TestRunwayTimeSlices(t *testing.T) {
	seq, cor := makeSeqZero()
	r := &features.RunwayType{
		TimeSlice: []features.RunwayTimeSlicePropertyType{
			{RunwayTimeSlice: features.RunwayTimeSliceType{
				Interpretation:   "BASELINE",
				SequenceNumber:   seq,
				CorrectionNumber: cor,
			}},
		},
	}
	slices := r.TimeSlices()
	if len(slices) != 1 {
		t.Fatalf("TimeSlices len = %d, want 1", len(slices))
	}
}

func TestNavaidTimeSlices(t *testing.T) {
	seq, cor := makeSeqZero()
	n := &features.NavaidType{
		TimeSlice: []features.NavaidTimeSlicePropertyType{
			{NavaidTimeSlice: features.NavaidTimeSliceType{
				Interpretation:   "BASELINE",
				SequenceNumber:   seq,
				CorrectionNumber: cor,
			}},
		},
	}
	slices := n.TimeSlices()
	if len(slices) != 1 {
		t.Fatalf("TimeSlices len = %d, want 1", len(slices))
	}
	if slices[0].GetSequenceNumber() != 1 {
		t.Errorf("sequenceNumber = %d", slices[0].GetSequenceNumber())
	}
}

func TestDesignatedPointTimeSlices(t *testing.T) {
	seq, cor := makeSeqZero()
	dp := &features.DesignatedPointType{
		TimeSlice: []features.DesignatedPointTimeSlicePropertyType{
			{DesignatedPointTimeSlice: features.DesignatedPointTimeSliceType{
				Interpretation:   "BASELINE",
				SequenceNumber:   seq,
				CorrectionNumber: cor,
				Designator:       &types.CodeDesignatedPointDesignatorType{Value: "ALPHA"},
			}},
		},
	}
	slices := dp.TimeSlices()
	if len(slices) != 1 {
		t.Fatalf("TimeSlices len = %d, want 1", len(slices))
	}
	if slices[0].GetCorrectionNumber() != 0 {
		t.Errorf("correctionNumber = %d", slices[0].GetCorrectionNumber())
	}
}

// ---- TimeSlice interface nil-guard -----------------------------------------

func TestTimeSliceNilSequence(t *testing.T) {
	// SequenceNumber and CorrectionNumber nil → should return 0 not panic.
	ts := &features.AirspaceTimeSliceType{Interpretation: "BASELINE"}
	if ts.GetSequenceNumber() != 0 {
		t.Errorf("nil SequenceNumber should return 0")
	}
	if ts.GetCorrectionNumber() != 0 {
		t.Errorf("nil CorrectionNumber should return 0")
	}
}

// ---- ValidTime on NavaidTimeSlice / DesignatedPointTimeSlice ---------------

func TestNavaidTimeSliceValidTime(t *testing.T) {
	ts := &features.NavaidTimeSliceType{
		Interpretation: "BASELINE",
		ValidTime: gml.TimePrimitivePropertyType{
			TimePeriod: &gml.TimePeriodType{
				BeginPosition: &gml.TimePositionType{Value: "2020-01-01"},
				EndPosition:   &gml.TimePositionType{Value: "2030-01-01"},
			},
		},
	}
	vt := ts.GetValidTime()
	if vt.TimePeriod == nil {
		t.Fatal("GetValidTime returned nil TimePeriod")
	}
	if vt.TimePeriod.BeginPosition.Value != "2020-01-01" {
		t.Errorf("BeginPosition = %q", vt.TimePeriod.BeginPosition.Value)
	}
}

func TestDesignatedPointTimeSliceValidTime(t *testing.T) {
	ts := &features.DesignatedPointTimeSliceType{
		Interpretation: "BASELINE",
		ValidTime: gml.TimePrimitivePropertyType{
			TimePeriod: &gml.TimePeriodType{
				BeginPosition: &gml.TimePositionType{Value: "2021-06-01"},
			},
		},
	}
	vt := ts.GetValidTime()
	if vt.TimePeriod == nil || vt.TimePeriod.BeginPosition.Value != "2021-06-01" {
		t.Errorf("GetValidTime: %+v", vt)
	}
}

// ---- EffectiveAt integration via TimeSlices() ------------------------------

func TestEffectiveAtViaTimeSlices(t *testing.T) {
	// Verify that the TimeSlices() method produces slices usable by filter.EffectiveAt.
	// Import filter here would create a cycle (features → filter → features),
	// so we test the TimeSlice values directly instead.
	seq1, cor0 := uint32(1), uint32(0)
	a := &features.AirspaceType{
		TimeSlice: []features.AirspaceTimeSlicePropertyType{
			{AirspaceTimeSlice: features.AirspaceTimeSliceType{
				Interpretation:   "BASELINE",
				SequenceNumber:   &seq1,
				CorrectionNumber: &cor0,
				ValidTime: gml.TimePrimitivePropertyType{
					TimePeriod: &gml.TimePeriodType{
						BeginPosition: &gml.TimePositionType{Value: "2020-01-01"},
						EndPosition:   &gml.TimePositionType{Value: "2030-01-01"},
					},
				},
			}},
		},
	}
	slices := a.TimeSlices()
	if len(slices) != 1 {
		t.Fatalf("len = %d", len(slices))
	}
	ts := slices[0]
	if ts.GetInterpretation() != features.InterpretationBaseline {
		t.Errorf("interpretation = %v", ts.GetInterpretation())
	}
	vt := ts.GetValidTime()
	if vt.TimePeriod == nil {
		t.Fatal("no valid time")
	}
	// Confirm the period covers a known date.
	begin := vt.TimePeriod.BeginPosition.Value
	if begin != "2020-01-01" {
		t.Errorf("begin = %q", begin)
	}
	_ = time.Now() // ensures time import used
}
