package features

import "fmt"

// ValidationError describes a single data-quality problem found in a feature.
type ValidationError struct {
	FeatureID string
	Field     string
	Message   string
}

func (e ValidationError) Error() string {
	if e.FeatureID != "" {
		return fmt.Sprintf("validation: %s [%s]: %s", e.FeatureID, e.Field, e.Message)
	}
	return fmt.Sprintf("validation: [%s]: %s", e.Field, e.Message)
}

// Validate returns all data-quality violations found in f.
// An empty slice means the feature is valid.
func Validate(f Feature) []ValidationError {
	var errs []ValidationError

	if f.GetIdentifier() == "" {
		errs = append(errs, ValidationError{
			Field:   "identifier",
			Message: "missing GML identifier (UUID)",
		})
	}

	// Dispatch to per-type validators.
	switch v := f.(type) {
	case *AirspaceType:
		errs = append(errs, validateAirspace(v)...)
	case *AirportHeliportType:
		errs = append(errs, validateAirportHeliport(v)...)
	case *RunwayType:
		errs = append(errs, validateRunway(v)...)
	case *NavaidType:
		errs = append(errs, validateNavaid(v)...)
	case *DesignatedPointType:
		errs = append(errs, validateDesignatedPoint(v)...)
	case *RouteSegmentType:
		errs = append(errs, validateRouteSegment(v)...)
	}

	// Stamp each error with the feature ID.
	id := f.GetIdentifier()
	for i := range errs {
		if errs[i].FeatureID == "" {
			errs[i].FeatureID = id
		}
	}
	return errs
}

// ---- Per-type validators ---------------------------------------------------

func validateAirspace(a *AirspaceType) []ValidationError {
	var errs []ValidationError
	if len(a.TimeSlice) == 0 {
		errs = append(errs, ValidationError{Field: "timeSlice", Message: "no timeslices"})
		return errs
	}
	hasBaseline := false
	for _, tsp := range a.TimeSlice {
		ts := tsp.AirspaceTimeSlice
		if ts.Interpretation == string(InterpretationBaseline) {
			hasBaseline = true
		}
		if ts.Designator == nil || ts.Designator.Value == "" {
			errs = append(errs, ValidationError{Field: "designator", Message: "missing designator in timeslice"})
		}
		if ts.Type == nil || ts.Type.Value == "" {
			errs = append(errs, ValidationError{Field: "type", Message: "missing airspace type in timeslice"})
		}
		if len(ts.GeometryComponent) == 0 {
			errs = append(errs, ValidationError{Field: "geometryComponent", Message: "no geometry in timeslice"})
		}
	}
	if !hasBaseline {
		errs = append(errs, ValidationError{Field: "interpretation", Message: "no BASELINE timeslice"})
	}
	return errs
}

func validateAirportHeliport(a *AirportHeliportType) []ValidationError {
	var errs []ValidationError
	if len(a.TimeSlice) == 0 {
		return append(errs, ValidationError{Field: "timeSlice", Message: "no timeslices"})
	}
	hasBaseline := false
	for _, tsp := range a.TimeSlice {
		ts := tsp.AirportHeliportTimeSlice
		if ts.Interpretation == string(InterpretationBaseline) {
			hasBaseline = true
		}
		if ts.Designator == nil || ts.Designator.Value == "" {
			errs = append(errs, ValidationError{Field: "designator", Message: "missing ICAO designator"})
		}
	}
	if !hasBaseline {
		errs = append(errs, ValidationError{Field: "interpretation", Message: "no BASELINE timeslice"})
	}
	return errs
}

func validateRunway(r *RunwayType) []ValidationError {
	var errs []ValidationError
	if len(r.TimeSlice) == 0 {
		return append(errs, ValidationError{Field: "timeSlice", Message: "no timeslices"})
	}
	for _, tsp := range r.TimeSlice {
		ts := tsp.RunwayTimeSlice
		if ts.Designator == nil || ts.Designator.Value == "" {
			errs = append(errs, ValidationError{Field: "designator", Message: "missing runway designator"})
		}
	}
	return errs
}

func validateNavaid(n *NavaidType) []ValidationError {
	var errs []ValidationError
	if len(n.TimeSlice) == 0 {
		return append(errs, ValidationError{Field: "timeSlice", Message: "no timeslices"})
	}
	for _, tsp := range n.TimeSlice {
		ts := tsp.NavaidTimeSlice
		if ts.Designator == nil || ts.Designator.Value == "" {
			errs = append(errs, ValidationError{Field: "designator", Message: "missing navaid designator"})
		}
		if ts.Type == nil || ts.Type.Value == "" {
			errs = append(errs, ValidationError{Field: "type", Message: "missing navaid type"})
		}
	}
	return errs
}

func validateDesignatedPoint(dp *DesignatedPointType) []ValidationError {
	var errs []ValidationError
	if len(dp.TimeSlice) == 0 {
		return append(errs, ValidationError{Field: "timeSlice", Message: "no timeslices"})
	}
	for _, tsp := range dp.TimeSlice {
		ts := tsp.DesignatedPointTimeSlice
		if ts.Designator == nil || ts.Designator.Value == "" {
			errs = append(errs, ValidationError{Field: "designator", Message: "missing designator"})
		}
	}
	return errs
}

func validateRouteSegment(rs *RouteSegmentType) []ValidationError {
	var errs []ValidationError
	if len(rs.TimeSlice) == 0 {
		return append(errs, ValidationError{Field: "timeSlice", Message: "no timeslices"})
	}
	for _, tsp := range rs.TimeSlice {
		ts := tsp.RouteSegmentTimeSlice
		if ts.Start == nil {
			errs = append(errs, ValidationError{Field: "start", Message: "missing start point"})
		}
		if ts.End == nil {
			errs = append(errs, ValidationError{Field: "end", Message: "missing end point"})
		}
	}
	return errs
}

// ValidateAll validates every feature in the slice and returns a flat list of
// all violations.  Features with no violations do not appear in the output.
func ValidateAll(features []Feature) []ValidationError {
	var all []ValidationError
	for _, f := range features {
		all = append(all, Validate(f)...)
	}
	return all
}
