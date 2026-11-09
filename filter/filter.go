// Package filter selects and merges AIXM time slices.
//
// Every time slice of package aixm implements aixm.TimeSlice, so the
// functions here work for all features:
//
//	slices := filter.Chain(
//		filter.ByInterpretation(aixm.Baseline),
//		filter.ByValidTime(now),
//	).Apply(airspace.TimeSlices())
//
//	ts, err := filter.Current(airspace.TimeSlices(), now) // SNAPSHOT or BASELINE+deltas
package filter

import (
	"time"

	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/gml"
)

// Predicate reports whether a time slice should be kept.
type Predicate func(ts aixm.TimeSlice) bool

// Filter selects time slices with a predicate.
type Filter struct {
	pred Predicate
}

// New returns a filter for pred.
func New(pred Predicate) Filter { return Filter{pred: pred} }

// Apply returns the slices for which the filter's predicate is true, in order.
func (f Filter) Apply(slices []aixm.TimeSlice) []aixm.TimeSlice {
	var out []aixm.TimeSlice
	for _, ts := range slices {
		if f.pred(ts) {
			out = append(out, ts)
		}
	}
	return out
}

// ByInterpretation keeps time slices with the given interpretation.
func ByInterpretation(i aixm.Interpretation) Filter {
	return Filter{func(ts aixm.TimeSlice) bool { return ts.Common().Interpretation == i }}
}

// ByValidTime keeps time slices whose validTime contains t.  A time slice
// without a validTime period is always kept.
func ByValidTime(t time.Time) Filter {
	return Filter{func(ts aixm.TimeSlice) bool { return periodContains(ts.Common().ValidTime, t) }}
}

// Chain combines filters with AND semantics.
func Chain(filters ...Filter) Filter {
	return Filter{func(ts aixm.TimeSlice) bool {
		for _, f := range filters {
			if !f.pred(ts) {
				return false
			}
		}
		return true
	}}
}

// Latest returns the time slice with the highest (sequenceNumber,
// correctionNumber), or nil for no slices.  Ties go to the first.
func Latest(slices []aixm.TimeSlice) aixm.TimeSlice {
	var best aixm.TimeSlice
	for _, ts := range slices {
		if best == nil || newer(ts, best) {
			best = ts
		}
	}
	return best
}

func newer(a, b aixm.TimeSlice) bool {
	as, ac := a.Common().Sequence()
	bs, bc := b.Common().Sequence()
	return as > bs || (as == bs && ac > bc)
}

func periodContains(tp gml.TimePrimitiveProperty, t time.Time) bool {
	period := tp.TimePeriod
	if period == nil {
		return true
	}
	begin := parseTimePosition(period.BeginPosition)
	end := parseTimePosition(period.EndPosition)
	if !begin.IsZero() && t.Before(begin) {
		return false
	}
	if !end.IsZero() && t.After(end) {
		return false
	}
	return true
}

func parseTimePosition(pos *gml.TimePosition) time.Time {
	if pos == nil || pos.Value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, pos.Value); err == nil {
			return t
		}
	}
	return time.Time{}
}
