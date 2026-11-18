package aixm

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

// Validate returns the data-quality violations found in f.
// An empty slice means the feature is valid.
func Validate(f Feature) []ValidationError {
	var errs []ValidationError

	if f.Common().UUID() == "" {
		errs = append(errs, ValidationError{
			Field:   "identifier",
			Message: "missing GML identifier (UUID)",
		})
	}

	// Dispatch to per-type validators.
	switch v := f.(type) {
	case *Airspace:
		errs = append(errs, validateAirspace(v)...)
	case *AirportHeliport:
		errs = append(errs, validateAirportHeliport(v)...)
	case *Runway:
		errs = append(errs, validateRunway(v)...)
	case *Navaid:
		errs = append(errs, validateNavaid(v)...)
	case *DesignatedPoint:
		errs = append(errs, validateDesignatedPoint(v)...)
	case *RouteSegment:
		errs = append(errs, validateRouteSegment(v)...)
	}

	// Stamp each error with the feature ID.
	id := f.Common().UUID()
	for i := range errs {
		if errs[i].FeatureID == "" {
			errs[i].FeatureID = id
		}
	}
	return errs
}

// ---- Per-type validators ---------------------------------------------------

func validateAirspace(a *Airspace) []ValidationError {
	var errs []ValidationError
	if len(a.TimeSlice) == 0 {
		errs = append(errs, ValidationError{Field: "timeSlice", Message: "no timeslices"})
		return errs
	}
	hasBaseline := false
	for _, ts := range a.TimeSlice {
		if ts.Interpretation == Baseline {
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

func validateAirportHeliport(a *AirportHeliport) []ValidationError {
	var errs []ValidationError
	if len(a.TimeSlice) == 0 {
		return append(errs, ValidationError{Field: "timeSlice", Message: "no timeslices"})
	}
	hasBaseline := false
	for _, ts := range a.TimeSlice {
		if ts.Interpretation == Baseline {
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

func validateRunway(r *Runway) []ValidationError {
	var errs []ValidationError
	if len(r.TimeSlice) == 0 {
		return append(errs, ValidationError{Field: "timeSlice", Message: "no timeslices"})
	}
	for _, ts := range r.TimeSlice {
		if ts.Designator == nil || ts.Designator.Value == "" {
			errs = append(errs, ValidationError{Field: "designator", Message: "missing runway designator"})
		}
	}
	return errs
}

func validateNavaid(n *Navaid) []ValidationError {
	var errs []ValidationError
	if len(n.TimeSlice) == 0 {
		return append(errs, ValidationError{Field: "timeSlice", Message: "no timeslices"})
	}
	for _, ts := range n.TimeSlice {
		if ts.Designator == nil || ts.Designator.Value == "" {
			errs = append(errs, ValidationError{Field: "designator", Message: "missing navaid designator"})
		}
		if ts.Type == nil || ts.Type.Value == "" {
			errs = append(errs, ValidationError{Field: "type", Message: "missing navaid type"})
		}
	}
	return errs
}

func validateDesignatedPoint(dp *DesignatedPoint) []ValidationError {
	var errs []ValidationError
	if len(dp.TimeSlice) == 0 {
		return append(errs, ValidationError{Field: "timeSlice", Message: "no timeslices"})
	}
	for _, ts := range dp.TimeSlice {
		if ts.Designator == nil || ts.Designator.Value == "" {
			errs = append(errs, ValidationError{Field: "designator", Message: "missing designator"})
		}
	}
	return errs
}

func validateRouteSegment(rs *RouteSegment) []ValidationError {
	var errs []ValidationError
	if len(rs.TimeSlice) == 0 {
		return append(errs, ValidationError{Field: "timeSlice", Message: "no timeslices"})
	}
	for _, ts := range rs.TimeSlice {
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
