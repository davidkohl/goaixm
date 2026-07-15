package filter

import (
	"fmt"
	"reflect"
	"sort"
	"time"

	"gitlab.com/davidkohl/goaixm/features"
)

// EffectiveState holds the merged result of BASELINE + applicable deltas at a
// point in time.  The concrete timeslice value is stored in Slice and can be
// type-asserted to the feature-specific *XxxTimeSliceType.
type EffectiveState struct {
	// Slice is the merged timeslice.  It is a pointer to a concrete
	// *XxxTimeSliceType value (e.g. *features.AirspaceTimeSliceType).
	Slice features.TimeSlice
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
// Returns an error if no BASELINE valid at t exists.
func EffectiveAt(slices []features.TimeSlice, t time.Time) (*EffectiveState, error) {
	validAt := ByValidTime(t)

	// Step 1 — find best baseline.
	baselines := Chain(ByInterpretation(features.InterpretationBaseline), validAt).Apply(slices)
	if len(baselines) == 0 {
		return nil, fmt.Errorf("filter: no BASELINE timeslice valid at %v", t)
	}
	baseline := ApplyLatest(baselines)[0]

	// Step 2 — collect PERMDELTA.
	permdeltas := Chain(ByInterpretation(features.InterpretationPermDelta), validAt).Apply(slices)
	sortByVersion(permdeltas)

	// Step 3 — collect TEMPDELTA.
	tempdeltas := Chain(ByInterpretation(features.InterpretationTempDelta), validAt).Apply(slices)
	sortByVersion(tempdeltas)

	// Step 4 — deep-copy baseline, then apply deltas.
	merged := deepCopyTimeSlice(baseline)
	for _, delta := range permdeltas {
		applyDelta(merged, delta)
	}
	for _, delta := range tempdeltas {
		applyDelta(merged, delta)
	}

	return &EffectiveState{Slice: merged}, nil
}

// sortByVersion sorts timeslices ascending by (sequenceNumber, correctionNumber).
func sortByVersion(slices []features.TimeSlice) {
	sort.SliceStable(slices, func(i, j int) bool {
		si, ci := slices[i].GetSequenceNumber(), slices[i].GetCorrectionNumber()
		sj, cj := slices[j].GetSequenceNumber(), slices[j].GetCorrectionNumber()
		if si != sj {
			return si < sj
		}
		return ci < cj
	})
}

// deepCopyTimeSlice returns a deep copy of ts as a features.TimeSlice.
// ts must be a pointer to a struct.
func deepCopyTimeSlice(ts features.TimeSlice) features.TimeSlice {
	v := reflect.ValueOf(ts)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return ts
	}
	copy := reflect.New(v.Elem().Type())
	copyStructDeep(copy.Elem(), v.Elem())
	return copy.Interface().(features.TimeSlice)
}

// applyDelta overlays non-nil pointer fields from delta onto dst.
// Both dst and delta must be the same concrete *XxxTimeSliceType.
func applyDelta(dst features.TimeSlice, delta features.TimeSlice) {
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

// overlayStruct copies non-nil pointer fields and non-zero slice fields from
// src onto dst recursively.
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
