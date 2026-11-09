package aixm_test

import (
	"testing"

	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/gml"
)

func hasField(errs []aixm.ValidationError, field string) bool {
	for _, e := range errs {
		if e.Field == field {
			return true
		}
	}
	return false
}

func TestValidateAirspaceMissingDesignator(t *testing.T) {
	a := &aixm.Airspace{TimeSlice: []aixm.AirspaceTimeSlice{
		{TimeSliceCommon: aixm.TimeSliceCommon{Interpretation: aixm.Baseline}},
	}}
	errs := aixm.Validate(a)
	if !hasField(errs, "designator") {
		t.Errorf("expected designator error; got: %v", errs)
	}
	if !hasField(errs, "identifier") {
		t.Errorf("expected identifier error; got: %v", errs)
	}
}

func TestValidateAirspaceNoTimeslice(t *testing.T) {
	if errs := aixm.Validate(&aixm.Airspace{}); !hasField(errs, "timeSlice") {
		t.Errorf("expected timeSlice error; got: %v", errs)
	}
}

func TestValidateAirspaceNoBaseline(t *testing.T) {
	a := &aixm.Airspace{}
	a.Identifier = &gml.Code{Value: "test-uuid"}
	a.TimeSlice = []aixm.AirspaceTimeSlice{{
		TimeSliceCommon: aixm.TimeSliceCommon{Interpretation: aixm.Snapshot},
		Designator:      &aixm.CodeAirspaceDesignator{Value: "EGTT"},
		Type:            &aixm.CodeAirspace{Value: "CTA"},
	}}
	errs := aixm.Validate(a)
	if !hasField(errs, "interpretation") {
		t.Errorf("expected 'no BASELINE' error; got: %v", errs)
	}
	for _, e := range errs {
		if e.FeatureID != "test-uuid" {
			t.Errorf("error not stamped with feature id: %v", e)
		}
	}
}

func TestValidateDesignatedPointValid(t *testing.T) {
	dp := &aixm.DesignatedPoint{}
	dp.Identifier = &gml.Code{Value: "test-uuid"}
	dp.TimeSlice = []aixm.DesignatedPointTimeSlice{{
		TimeSliceCommon: aixm.TimeSliceCommon{Interpretation: aixm.Baseline},
		Designator:      &aixm.CodeDesignatedPointDesignator{Value: "ALPHA"},
	}}
	if errs := aixm.Validate(dp); len(errs) != 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestValidateNavaidMissingType(t *testing.T) {
	n := &aixm.Navaid{}
	n.Identifier = &gml.Code{Value: "nav-uuid"}
	n.TimeSlice = []aixm.NavaidTimeSlice{{
		TimeSliceCommon: aixm.TimeSliceCommon{Interpretation: aixm.Baseline},
		Designator:      &aixm.CodeNavaidDesignator{Value: "BOR"},
	}}
	if errs := aixm.Validate(n); !hasField(errs, "type") {
		t.Errorf("expected 'type' error; got: %v", errs)
	}
}

func TestValidateRouteSegmentMissingEnds(t *testing.T) {
	rs := &aixm.RouteSegment{TimeSlice: []aixm.RouteSegmentTimeSlice{{}}}
	errs := aixm.Validate(rs)
	if !hasField(errs, "start") || !hasField(errs, "end") {
		t.Errorf("expected start and end errors; got: %v", errs)
	}
}
