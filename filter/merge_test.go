package filter_test

import (
	"errors"
	"testing"

	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/filter"
)

func TestEffectiveAtBaselineOnly(t *testing.T) {
	ts, err := filter.EffectiveAt([]aixm.TimeSlice{slice(aixm.Baseline, 1, "EGTT", "", "", "")}, at2025)
	if err != nil {
		t.Fatal(err)
	}
	if designator(t, ts) != "EGTT" {
		t.Errorf("designator = %s", designator(t, ts))
	}
}

func TestEffectiveAtNoBaseline(t *testing.T) {
	_, err := filter.EffectiveAt([]aixm.TimeSlice{slice(aixm.Snapshot, 1, "EGTT", "", "", "")}, at2025)
	if !errors.Is(err, filter.ErrNoBaseline) {
		t.Errorf("err = %v, want ErrNoBaseline", err)
	}
}

func TestEffectiveAtPermDelta(t *testing.T) {
	ts, err := filter.EffectiveAt([]aixm.TimeSlice{
		slice(aixm.Baseline, 1, "EGTT", "", "", ""),
		slice(aixm.PermDelta, 2, "EGTT-NEW", "", "", ""),
	}, at2025)
	if err != nil {
		t.Fatal(err)
	}
	if designator(t, ts) != "EGTT-NEW" {
		t.Errorf("designator = %s, want EGTT-NEW", designator(t, ts))
	}
	if ts.Common().Interpretation != aixm.Baseline {
		t.Errorf("merged interpretation = %s, want the baseline's", ts.Common().Interpretation)
	}
}

func TestEffectiveAtTempDeltaOverridesPermDelta(t *testing.T) {
	base := slice(aixm.Baseline, 1, "EGTT", "CTA", "", "")
	ts, err := filter.EffectiveAt([]aixm.TimeSlice{
		slice(aixm.TempDelta, 3, "EGTT-TEMP", "", "", ""),
		base,
		slice(aixm.PermDelta, 2, "EGTT-PERM", "", "", ""),
	}, at2025)
	if err != nil {
		t.Fatal(err)
	}
	a := ts.(*aixm.AirspaceTimeSlice)
	if a.Designator.String() != "EGTT-TEMP" {
		t.Errorf("designator = %s, want EGTT-TEMP", a.Designator.String())
	}
	if a.Type.String() != "CTA" {
		t.Errorf("type = %s, want CTA (inherited from baseline)", a.Type.String())
	}
	if base.Designator.String() != "EGTT" {
		t.Error("EffectiveAt modified its input")
	}
}

func TestEffectiveAtOutsideValidTime(t *testing.T) {
	ts, err := filter.EffectiveAt([]aixm.TimeSlice{
		slice(aixm.Baseline, 1, "EGTT", "", "", ""),
		slice(aixm.PermDelta, 2, "EGTT-2022", "", "2022-01-01", "2023-01-01"),
	}, at2025)
	if err != nil {
		t.Fatal(err)
	}
	if designator(t, ts) != "EGTT" {
		t.Errorf("delta outside its validity was applied: %s", designator(t, ts))
	}
}
