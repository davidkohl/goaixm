package features_test

import (
	"os"
	"testing"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/gml"
	"gitlab.com/davidkohl/goaixm/parse"
	"gitlab.com/davidkohl/goaixm/types"
)

func TestValidateAirspaceMissingDesignator(t *testing.T) {
	a := &features.AirspaceType{}
	// No identifier, no designator in timeslice.
	ts := features.AirspaceTimeSliceType{Interpretation: "BASELINE"}
	a.TimeSlice = []features.AirspaceTimeSlicePropertyType{
		{AirspaceTimeSlice: ts},
	}
	errs := features.Validate(a)
	if len(errs) == 0 {
		t.Fatal("expected validation errors, got none")
	}
	found := false
	for _, e := range errs {
		if e.Field == "designator" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected designator error; got: %v", errs)
	}
}

func TestValidateAirspaceNoTimeslice(t *testing.T) {
	a := &features.AirspaceType{}
	errs := features.Validate(a)
	hasTimeSliceErr := false
	for _, e := range errs {
		if e.Field == "timeSlice" {
			hasTimeSliceErr = true
		}
	}
	if !hasTimeSliceErr {
		t.Errorf("expected timeSlice error; got: %v", errs)
	}
}

func TestValidateAirspaceNoBaseline(t *testing.T) {
	a := &features.AirspaceType{}
	a.Identifier = &gml.CodeType{Value: "test-uuid"}
	ts := features.AirspaceTimeSliceType{
		Interpretation: "SNAPSHOT",
		Designator:     &types.CodeAirspaceDesignatorType{Value: "EGTT"},
		Type:           &types.CodeAirspaceType{Value: "CTA"},
	}
	a.TimeSlice = []features.AirspaceTimeSlicePropertyType{
		{AirspaceTimeSlice: ts},
	}
	errs := features.Validate(a)
	found := false
	for _, e := range errs {
		if e.Field == "interpretation" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'no BASELINE' error; got: %v", errs)
	}
}

func TestValidateDesignatedPointValid(t *testing.T) {
	dp := &features.DesignatedPointType{}
	dp.Identifier = &gml.CodeType{Value: "test-uuid"}
	ts := features.DesignatedPointTimeSliceType{
		Interpretation: "BASELINE",
		Designator:     &types.CodeDesignatedPointDesignatorType{Value: "ALPHA"},
	}
	dp.TimeSlice = []features.DesignatedPointTimeSlicePropertyType{
		{DesignatedPointTimeSlice: ts},
	}
	errs := features.Validate(dp)
	for _, e := range errs {
		if e.Field == "designator" || e.Field == "timeSlice" {
			t.Errorf("unexpected error: %v", e)
		}
	}
}

func TestValidateNavaidMissingType(t *testing.T) {
	n := &features.NavaidType{}
	n.Identifier = &gml.CodeType{Value: "nav-uuid"}
	ts := features.NavaidTimeSliceType{
		Interpretation: "BASELINE",
		Designator:     &types.CodeNavaidDesignatorType{Value: "BOR"},
		// Type intentionally missing.
	}
	n.TimeSlice = []features.NavaidTimeSlicePropertyType{
		{NavaidTimeSlice: ts},
	}
	errs := features.Validate(n)
	found := false
	for _, e := range errs {
		if e.Field == "type" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'type' error; got: %v", errs)
	}
}

func TestValidateAllDonlon(t *testing.T) {
	f, err := os.Open("../testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	msg, err := parse.ParseMessage(f)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	var feats []features.Feature
	for _, v := range msg.Features() {
		if feat, ok := v.(features.Feature); ok {
			feats = append(feats, feat)
		}
	}

	errs := features.ValidateAll(feats)
	// Log violations — DONLON is official data but may have airspaces without
	// inline geometry (the validator checks for it).
	geometryErrCount := 0
	for _, e := range errs {
		if e.Field == "geometryComponent" {
			geometryErrCount++
		} else {
			t.Logf("unexpected validation error: %v", e)
		}
	}
	t.Logf("DONLON: %d features, %d geometry-missing warnings", len(feats), geometryErrCount)
}
