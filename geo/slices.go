package geo

import (
	"strconv"
	"strings"
	"time"

	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/xmlnode"
)

// sliceHeader is what the choice of a time slice at an instant needs
// (AIXM 5.1 temporality concept).
type sliceHeader struct {
	interpretation string
	begin, end     *time.Time
	seq, corr      uint64
}

// validAt reports whether the slice describes the feature at instant at:
// a SNAPSHOT from its instant on, a BASELINE inside [begin, end).
func (h sliceHeader) validAt(at time.Time) bool {
	if h.begin == nil || h.begin.After(at) {
		return false
	}
	switch h.interpretation {
	case "SNAPSHOT":
		return true
	case "BASELINE":
		return h.end == nil || h.end.After(at)
	}
	return false
}

// sliceAt chooses among the SNAPSHOT and BASELINE slices valid at at the
// one with the latest start, then the highest sequenceNumber, then the
// highest correctionNumber (the first of equals); -1 when none is valid.
func sliceAt(hs []sliceHeader, at time.Time) int {
	best := -1
	for i, s := range hs {
		if !s.validAt(at) {
			continue
		}
		if best < 0 {
			best = i
			continue
		}
		b := hs[best]
		switch {
		case !s.begin.Equal(*b.begin):
			if s.begin.After(*b.begin) {
				best = i
			}
		case s.seq != b.seq:
			if s.seq > b.seq {
				best = i
			}
		case s.corr > b.corr:
			best = i
		}
	}
	return best
}

// parseTimePosition reads an ISO 8601 time position as written in AIXM
// (with or without zone, or a date); nil for empty, indeterminate or
// unparsable values.
func parseTimePosition(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func typedHeader(c *aixm.TimeSliceCommon) sliceHeader {
	h := sliceHeader{interpretation: strings.TrimSpace(string(c.Interpretation))}
	seq, corr := c.Sequence()
	h.seq, h.corr = uint64(seq), uint64(corr)
	switch vt := c.ValidTime; {
	case vt.TimeInstant != nil:
		h.begin = parseTimePosition(vt.TimeInstant.TimePosition.Value)
	case vt.TimePeriod != nil:
		if vt.TimePeriod.BeginPosition != nil {
			h.begin = parseTimePosition(vt.TimePeriod.BeginPosition.Value)
		}
		if vt.TimePeriod.EndPosition != nil {
			h.end = parseTimePosition(vt.TimePeriod.EndPosition.Value)
		}
	}
	return h
}

func nodeHeader(ts *xmlnode.Node) sliceHeader {
	h := sliceHeader{interpretation: strings.TrimSpace(ts.TextAt("interpretation"))}
	h.seq, _ = strconv.ParseUint(strings.TrimSpace(ts.TextAt("sequenceNumber")), 10, 64)
	h.corr, _ = strconv.ParseUint(strings.TrimSpace(ts.TextAt("correctionNumber")), 10, 64)
	if t := ts.TextAt("validTime/TimeInstant/timePosition"); t != "" {
		h.begin = parseTimePosition(t)
	} else {
		h.begin = parseTimePosition(ts.TextAt("validTime/TimePeriod/beginPosition"))
		h.end = parseTimePosition(ts.TextAt("validTime/TimePeriod/endPosition"))
	}
	return h
}

// borderSlice returns the index of the GeoBorder time slice the border
// curve is taken from: with Options.At the slice valid then (when it has
// a curve), else the first slice with a curve; -1 if none has one.
func (cv *conv) borderSlice(b *aixm.GeoBorder) int {
	if !cv.opt.At.IsZero() {
		hs := make([]sliceHeader, len(b.TimeSlice))
		for i := range b.TimeSlice {
			hs[i] = typedHeader(&b.TimeSlice[i].TimeSliceCommon)
		}
		if i := sliceAt(hs, cv.opt.At); i >= 0 && b.TimeSlice[i].Border.Segments() != nil {
			return i
		}
	}
	for i := range b.TimeSlice {
		if b.TimeSlice[i].Border.Segments() != nil {
			return i
		}
	}
	return -1
}

// refAirspace returns a referenced (contributor) airspace as conversion
// reads it, from its first time slice: with Options.At a shallow copy with
// the slice valid then moved to the front.
func (cv *conv) refAirspace(a *aixm.Airspace) *aixm.Airspace {
	if cv.opt.At.IsZero() || a == nil {
		return a
	}
	if i := airspaceSlice(a, cv.opt.At); i > 0 {
		cp := *a
		cp.TimeSlice = append([]aixm.AirspaceTimeSlice{a.TimeSlice[i]}, a.TimeSlice[:i]...)
		cp.TimeSlice = append(cp.TimeSlice, a.TimeSlice[i+1:]...)
		return &cp
	}
	return a
}

// airspaceSlice is the airspace slice valid at at (0 when none is).
func airspaceSlice(a *aixm.Airspace, at time.Time) int {
	hs := make([]sliceHeader, len(a.TimeSlice))
	for i := range a.TimeSlice {
		hs[i] = typedHeader(&a.TimeSlice[i].TimeSliceCommon)
	}
	return max(sliceAt(hs, at), 0)
}
