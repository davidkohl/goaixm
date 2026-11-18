package filter_test

import (
	"errors"
	"testing"
	"time"

	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/filter"
	"github.com/davidkohl/goaixm/gml"
)

var at2025 = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

// slice builds an airspace time slice.  begin/end may be "" for open ends;
// typ may be "" for no type.
func slice(interp aixm.Interpretation, seq uint32, designator, typ, begin, end string) *aixm.AirspaceTimeSlice {
	ts := &aixm.AirspaceTimeSlice{TimeSliceCommon: aixm.TimeSliceCommon{
		Interpretation: interp,
		SequenceNumber: &seq,
		ValidTime: gml.TimePrimitiveProperty{TimePeriod: &gml.TimePeriod{
			BeginPosition: &gml.TimePosition{Value: begin},
			EndPosition:   &gml.TimePosition{Value: end},
		}},
	}}
	if designator != "" {
		ts.Designator = &aixm.CodeAirspaceDesignator{Value: designator}
	}
	if typ != "" {
		ts.Type = &aixm.CodeAirspace{Value: typ}
	}
	return ts
}

func designator(t *testing.T, ts aixm.TimeSlice) string {
	t.Helper()
	a, ok := ts.(*aixm.AirspaceTimeSlice)
	if !ok {
		t.Fatalf("got %T", ts)
	}
	return a.Designator.String()
}

func TestByInterpretation(t *testing.T) {
	slices := []aixm.TimeSlice{
		slice(aixm.Baseline, 1, "A", "", "", ""),
		slice(aixm.Snapshot, 1, "B", "", "", ""),
		slice(aixm.Baseline, 2, "C", "", "", ""),
	}
	if got := filter.ByInterpretation(aixm.Baseline).Apply(slices); len(got) != 2 {
		t.Errorf("got %d baselines, want 2", len(got))
	}
}

func TestByValidTime(t *testing.T) {
	slices := []aixm.TimeSlice{
		slice(aixm.Baseline, 1, "OLD", "", "2020-01-01", "2021-01-01"),
		slice(aixm.Baseline, 1, "NOW", "", "2024-01-01T00:00:00Z", ""),
	}
	got := filter.ByValidTime(at2025).Apply(slices)
	if len(got) != 1 || designator(t, got[0]) != "NOW" {
		t.Errorf("got %v", got)
	}
}

func TestLatest(t *testing.T) {
	if filter.Latest(nil) != nil {
		t.Error("Latest(nil) != nil")
	}
	slices := []aixm.TimeSlice{
		slice(aixm.Baseline, 1, "ONE", "", "", ""),
		slice(aixm.Baseline, 3, "THREE", "", "", ""),
		slice(aixm.Baseline, 2, "TWO", "", "", ""),
	}
	if got := designator(t, filter.Latest(slices)); got != "THREE" {
		t.Errorf("Latest = %s", got)
	}
}

func TestChain(t *testing.T) {
	slices := []aixm.TimeSlice{
		slice(aixm.Baseline, 1, "A", "", "2024-01-01", ""),
		slice(aixm.Snapshot, 1, "B", "", "2024-01-01", ""),
		slice(aixm.Baseline, 1, "C", "", "2026-01-01", ""),
	}
	got := filter.Chain(filter.ByInterpretation(aixm.Baseline), filter.ByValidTime(at2025)).Apply(slices)
	if len(got) != 1 || designator(t, got[0]) != "A" {
		t.Errorf("got %v", got)
	}
}

func TestNewPredicate(t *testing.T) {
	f := filter.New(func(ts aixm.TimeSlice) bool { return ts.Common().SequenceNumber != nil })
	if got := f.Apply([]aixm.TimeSlice{slice(aixm.Baseline, 1, "A", "", "", "")}); len(got) != 1 {
		t.Errorf("got %d", len(got))
	}
}

func TestCurrentPrefersSnapshot(t *testing.T) {
	slices := []aixm.TimeSlice{
		slice(aixm.Baseline, 1, "BASE", "", "", ""),
		slice(aixm.Snapshot, 1, "SNAP", "", "", ""),
	}
	ts, err := filter.Current(slices, at2025)
	if err != nil || designator(t, ts) != "SNAP" {
		t.Errorf("Current = %v, %v", ts, err)
	}
	ts, err = filter.Current(slices[:1], at2025)
	if err != nil || designator(t, ts) != "BASE" {
		t.Errorf("Current without snapshot = %v, %v", ts, err)
	}
	if _, err := filter.Current(nil, at2025); !errors.Is(err, filter.ErrNoBaseline) {
		t.Errorf("Current(nil) err = %v", err)
	}
}
