package filter

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/davidkohl/goaixm/aixm"
)

// ErrNoBaseline is returned when no BASELINE time slice is valid at the
// requested time.
var ErrNoBaseline = errors.New("filter: no BASELINE time slice valid at the requested time")

// Current returns the state of a feature at t: the latest SNAPSHOT valid at
// t when there is one, otherwise the BASELINE merged with its deltas (see
// EffectiveAt).
func Current(slices []aixm.TimeSlice, t time.Time) (aixm.TimeSlice, error) {
	if snap := Latest(Chain(ByInterpretation(aixm.Snapshot), ByValidTime(t)).Apply(slices)); snap != nil {
		return snap, nil
	}
	return EffectiveAt(slices, t)
}

// EffectiveAt computes the effective state of a feature's timeslices at time t.
//
// Algorithm:
//  1. Find the BASELINE timeslice with the highest (sequenceNumber,
//     correctionNumber) that is valid at t.
//  2. Collect all PERMDELTA timeslices valid at t, ordered by sequence then
//     correction number.
//  3. Collect all TEMPDELTA timeslices valid at t, ordered by sequence then
//     correction number.
//  4. Apply each delta in order onto a deep copy of the baseline — a non-nil
//     pointer field in the delta overrides the corresponding field in the
//     running merged value; nil pointer fields are left unchanged (inherited).
//
// The result is a new value of the concrete time slice type (e.g.
// *aixm.AirspaceTimeSlice); the inputs are not modified.  The temporal
// properties (TimeSliceCommon) are those of the baseline.  ErrNoBaseline is
// returned when no BASELINE is valid at t.
func EffectiveAt(slices []aixm.TimeSlice, t time.Time) (aixm.TimeSlice, error) {
	validAt := ByValidTime(t)

	// Step 1 — find best baseline.
	baseline := Latest(Chain(ByInterpretation(aixm.Baseline), validAt).Apply(slices))
	if baseline == nil {
		return nil, fmt.Errorf("%w (%v)", ErrNoBaseline, t)
	}

	// Step 2 — collect PERMDELTA.
	permdeltas := Chain(ByInterpretation(aixm.PermDelta), validAt).Apply(slices)
	sortByVersion(permdeltas)

	// Step 3 — collect TEMPDELTA.
	tempdeltas := Chain(ByInterpretation(aixm.TempDelta), validAt).Apply(slices)
	sortByVersion(tempdeltas)

	// Step 4 — deep-copy baseline, then apply deltas.
	merged := deepCopyTimeSlice(baseline)
	for _, delta := range permdeltas {
		applyDelta(merged, delta)
	}
	for _, delta := range tempdeltas {
		applyDelta(merged, delta)
	}

	return merged, nil
}

// sortByVersion sorts timeslices ascending by (sequenceNumber, correctionNumber).
func sortByVersion(slices []aixm.TimeSlice) {
	sort.SliceStable(slices, func(i, j int) bool {
		si, ci := slices[i].Common().Sequence()
		sj, cj := slices[j].Common().Sequence()
		if si != sj {
			return si < sj
		}
		return ci < cj
	})
}

// deepCopyTimeSlice returns a deep copy of ts.
// ts must be a pointer to a struct.
func deepCopyTimeSlice(ts aixm.TimeSlice) aixm.TimeSlice {
	v := reflect.ValueOf(ts)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return ts
	}
	copy := reflect.New(v.Elem().Type())
	copyStructDeep(copy.Elem(), v.Elem())
	return copy.Interface().(aixm.TimeSlice)
}

// applyDelta overlays non-nil pointer fields from delta onto dst.
// Both must be the same concrete time slice type.
func applyDelta(dst aixm.TimeSlice, delta aixm.TimeSlice) {
	dstVal := reflect.ValueOf(dst)
	deltaVal := reflect.ValueOf(delta)
	if dstVal.Kind() != reflect.Pointer || deltaVal.Kind() != reflect.Pointer {
		return
	}
	dstElem := dstVal.Elem()
	deltaElem := deltaVal.Elem()
	if dstElem.Type() != deltaElem.Type() {
		return
	}
	overlayStruct(dstElem, deltaElem)
}

// overlayStruct copies non-nil pointer fields, non-empty slice fields and
// non-empty strings from src onto dst.  Embedded structs (the temporal
// TimeSliceCommon) are not overlaid: the result keeps the baseline's.
func overlayStruct(dst, src reflect.Value) {
	t := dst.Type()
	for i := 0; i < t.NumField(); i++ {
		df := dst.Field(i)
		sf := src.Field(i)
		ft := t.Field(i)
		if !ft.IsExported() {
			continue
		}
		switch sf.Kind() {
		case reflect.Pointer:
			if !sf.IsNil() {
				// Deep copy src pointer into dst.
				newPtr := reflect.New(sf.Elem().Type())
				copyStructDeep(newPtr.Elem(), sf.Elem())
				df.Set(newPtr)
			}
		case reflect.Slice:
			if sf.Len() > 0 {
				df.Set(reflect.AppendSlice(reflect.MakeSlice(sf.Type(), 0, sf.Len()), sf))
			}
		case reflect.String:
			// Non-empty string values override (covers Interpretation and similar).
			if sf.String() != "" {
				df.Set(sf)
			}
		}
	}
}

// copyStructDeep recursively deep-copies src into dst.
func copyStructDeep(dst, src reflect.Value) {
	if dst.Type() != src.Type() {
		return
	}
	switch src.Kind() {
	case reflect.Struct:
		for i := 0; i < src.NumField(); i++ {
			sf := src.Field(i)
			df := dst.Field(i)
			if !src.Type().Field(i).IsExported() {
				continue
			}
			copyStructDeep(df, sf)
		}
	case reflect.Pointer:
		if src.IsNil() {
			dst.Set(reflect.Zero(dst.Type()))
		} else {
			newPtr := reflect.New(src.Elem().Type())
			copyStructDeep(newPtr.Elem(), src.Elem())
			dst.Set(newPtr)
		}
	case reflect.Slice:
		if src.IsNil() {
			dst.Set(reflect.Zero(dst.Type()))
		} else {
			newSlice := reflect.MakeSlice(src.Type(), src.Len(), src.Cap())
			for j := 0; j < src.Len(); j++ {
				copyStructDeep(newSlice.Index(j), src.Index(j))
			}
			dst.Set(newSlice)
		}
	default:
		dst.Set(src)
	}
}
