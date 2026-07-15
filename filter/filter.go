// Package filter provides predicates for selecting AIXM TimeSlices.
package filter

import (
	"time"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/gml"
)

// TimeSlicePredicate is a function that returns true if a TimeSlice should be included.
type TimeSlicePredicate func(ts features.TimeSlice) bool

// TimeSliceFilter applies a predicate to a slice of TimeSlice values.
type TimeSliceFilter struct {
	pred TimeSlicePredicate
}

// Apply returns the subset of slices for which the filter predicate is true.
func (f TimeSliceFilter) Apply(slices []features.TimeSlice) []features.TimeSlice {
	out := slices[:0:0]
	for _, ts := range slices {
		if f.pred(ts) {
			out = append(out, ts)
		}
	}
	return out
}

// ByInterpretation returns a filter that accepts only timeslices with the given interpretation.
func ByInterpretation(i features.Interpretation) TimeSliceFilter {
	return TimeSliceFilter{func(ts features.TimeSlice) bool {
		return ts.GetInterpretation() == i
	}}
}

// ByValidTime returns a filter that accepts timeslices whose validTime period contains t.
// A timeslice with no validTime is always included.
func ByValidTime(t time.Time) TimeSliceFilter {
	return TimeSliceFilter{func(ts features.TimeSlice) bool {
		vt := ts.GetValidTime()
		return timePeriodContains(vt, t)
	}}
}

// Latest returns a filter that — after applying — leaves only the single timeslice
// with the highest (sequenceNumber, correctionNumber) pair.
// If multiple slices tie on both numbers, the first one wins.
func Latest() TimeSliceFilter {
	return TimeSliceFilter{func(_ features.TimeSlice) bool { return true }}
	// NOTE: Latest is handled by ApplyLatest to allow stateful selection.
}

// ApplyLatest returns a slice containing only the most recent timeslice.
func ApplyLatest(slices []features.TimeSlice) []features.TimeSlice {
	if len(slices) == 0 {
		return nil
	}
	best := slices[0]
	for _, ts := range slices[1:] {
		bs := best.GetSequenceNumber()
		bc := best.GetCorrectionNumber()
		cs := ts.GetSequenceNumber()
		cc := ts.GetCorrectionNumber()
		if cs > bs || (cs == bs && cc > bc) {
			best = ts
		}
	}
	return []features.TimeSlice{best}
}

// Chain combines multiple filters with AND semantics.
func Chain(filters ...TimeSliceFilter) TimeSliceFilter {
	return TimeSliceFilter{func(ts features.TimeSlice) bool {
		for _, f := range filters {
			if !f.pred(ts) {
				return false
			}
		}
		return true
	}}
}

// ---- Helpers ---------------------------------------------------------------

func timePeriodContains(tp gml.TimePrimitivePropertyType, t time.Time) bool {
	period := tp.TimePeriod
	if period == nil {
		return true // no constraint
	}
	begin := parseTimePosition(period.BeginPosition)
	end := parseTimePosition(period.EndPosition)
	if begin.IsZero() && end.IsZero() {
		return true
	}
	if !begin.IsZero() && t.Before(begin) {
		return false
	}
	if !end.IsZero() && t.After(end) {
		return false
	}
	return true
}

func parseTimePosition(pos *gml.TimePositionType) time.Time {
	if pos == nil || pos.Value == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, pos.Value); err == nil {
			return t
		}
	}
	return time.Time{}
}
