package filter_test

import (
	"testing"
	"time"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/filter"
	"gitlab.com/davidkohl/goaixm/gml"
)

// stubTS is a minimal TimeSlice for testing.
type stubTS struct {
	interp features.Interpretation
	begin  string
	end    string
	seq    uint32
	corr   uint32
}

func (s *stubTS) GetValidTime() gml.TimePrimitivePropertyType {
	begin := &gml.TimePositionType{Value: s.begin}
	end := &gml.TimePositionType{Value: s.end}
	return gml.TimePrimitivePropertyType{
		TimePeriod: &gml.TimePeriodType{
			BeginPosition: begin,
			EndPosition:   end,
		},
	}
}
func (s *stubTS) GetInterpretation() features.Interpretation { return s.interp }
func (s *stubTS) GetSequenceNumber() uint32                  { return s.seq }
func (s *stubTS) GetCorrectionNumber() uint32                { return s.corr }

func TestByInterpretation(t *testing.T) {
	slices := []features.TimeSlice{
		&stubTS{interp: features.InterpretationBaseline},
		&stubTS{interp: features.InterpretationSnapshot},
		&stubTS{interp: features.InterpretationBaseline},
	}
	got := filter.ByInterpretation(features.InterpretationBaseline).Apply(slices)
	if len(got) != 2 {
		t.Errorf("expected 2 BASELINE slices, got %d", len(got))
	}
}

func TestByValidTime(t *testing.T) {
	slices := []features.TimeSlice{
		&stubTS{begin: "2020-01-01T00:00:00Z", end: "2020-12-31T23:59:59Z"},
		&stubTS{begin: "2021-01-01T00:00:00Z", end: "2021-12-31T23:59:59Z"},
	}
	t2020 := time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC)
	got := filter.ByValidTime(t2020).Apply(slices)
	if len(got) != 1 {
		t.Errorf("expected 1 slice for 2020-06-01, got %d", len(got))
	}
}

func TestApplyLatest(t *testing.T) {
	slices := []features.TimeSlice{
		&stubTS{seq: 1, corr: 0},
		&stubTS{seq: 1, corr: 1},
		&stubTS{seq: 2, corr: 0},
	}
	got := filter.ApplyLatest(slices)
	if len(got) != 1 {
		t.Fatalf("expected 1 slice, got %d", len(got))
	}
	if got[0].GetSequenceNumber() != 2 {
		t.Errorf("expected seq=2, got %d", got[0].GetSequenceNumber())
	}
}

func TestChain(t *testing.T) {
	slices := []features.TimeSlice{
		&stubTS{interp: features.InterpretationBaseline, begin: "2020-01-01T00:00:00Z", end: "2020-12-31T23:59:59Z"},
		&stubTS{interp: features.InterpretationSnapshot, begin: "2020-01-01T00:00:00Z", end: "2020-12-31T23:59:59Z"},
		&stubTS{interp: features.InterpretationBaseline, begin: "2021-01-01T00:00:00Z", end: "2021-12-31T23:59:59Z"},
	}
	t2020 := time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC)
	got := filter.Chain(
		filter.ByInterpretation(features.InterpretationBaseline),
		filter.ByValidTime(t2020),
	).Apply(slices)
	if len(got) != 1 {
		t.Errorf("expected 1 slice, got %d", len(got))
	}
}
