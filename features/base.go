package features

import "gitlab.com/davidkohl/goaixm/gml"

// Interpretation is the AIXM timeslice interpretation code.
type Interpretation string

const (
	InterpretationBaseline  Interpretation = "BASELINE"
	InterpretationSnapshot  Interpretation = "SNAPSHOT"
	InterpretationTempDelta Interpretation = "TEMPDELTA"
	InterpretationPermDelta Interpretation = "PERMDELTA"
)

// Feature is implemented by every AIXM feature type.
type Feature interface {
	GetGMLID() string
	GetIdentifier() string
	GetNames() []string
}

// TimeSlice is implemented by every AIXM timeslice type.
type TimeSlice interface {
	GetValidTime() gml.TimePrimitivePropertyType
	GetInterpretation() Interpretation
	GetSequenceNumber() uint32
	GetCorrectionNumber() uint32
}

// ---- Interface implementations on generated types -------------------------

func (f *AirspaceType) GetGMLID() string      { return f.GMLID }
func (f *AirspaceType) GetIdentifier() string { return identifierValue(f.Identifier) }
func (f *AirspaceType) GetNames() []string    { return codeTypeValues(f.Name) }
func (f *AirspaceType) TimeSlices() []TimeSlice {
	out := make([]TimeSlice, len(f.TimeSlice))
	for i := range f.TimeSlice {
		out[i] = &f.TimeSlice[i].AirspaceTimeSlice
	}
	return out
}

func (f *AirportHeliportType) GetGMLID() string      { return f.GMLID }
func (f *AirportHeliportType) GetIdentifier() string { return identifierValue(f.Identifier) }
func (f *AirportHeliportType) GetNames() []string    { return codeTypeValues(f.Name) }
func (f *AirportHeliportType) TimeSlices() []TimeSlice {
	out := make([]TimeSlice, len(f.TimeSlice))
	for i := range f.TimeSlice {
		out[i] = &f.TimeSlice[i].AirportHeliportTimeSlice
	}
	return out
}

func (f *RunwayType) GetGMLID() string      { return f.GMLID }
func (f *RunwayType) GetIdentifier() string { return identifierValue(f.Identifier) }
func (f *RunwayType) GetNames() []string    { return codeTypeValues(f.Name) }
func (f *RunwayType) TimeSlices() []TimeSlice {
	out := make([]TimeSlice, len(f.TimeSlice))
	for i := range f.TimeSlice {
		out[i] = &f.TimeSlice[i].RunwayTimeSlice
	}
	return out
}

func (f *NavaidType) GetGMLID() string      { return f.GMLID }
func (f *NavaidType) GetIdentifier() string { return identifierValue(f.Identifier) }
func (f *NavaidType) GetNames() []string    { return codeTypeValues(f.Name) }
func (f *NavaidType) TimeSlices() []TimeSlice {
	out := make([]TimeSlice, len(f.TimeSlice))
	for i := range f.TimeSlice {
		out[i] = &f.TimeSlice[i].NavaidTimeSlice
	}
	return out
}

func (f *RouteType) GetGMLID() string      { return f.GMLID }
func (f *RouteType) GetIdentifier() string { return identifierValue(f.Identifier) }
func (f *RouteType) GetNames() []string    { return codeTypeValues(f.Name) }

func (f *RouteSegmentType) GetGMLID() string      { return f.GMLID }
func (f *RouteSegmentType) GetIdentifier() string { return identifierValue(f.Identifier) }
func (f *RouteSegmentType) GetNames() []string    { return codeTypeValues(f.Name) }

func (f *DesignatedPointType) GetGMLID() string      { return f.GMLID }
func (f *DesignatedPointType) GetIdentifier() string { return identifierValue(f.Identifier) }
func (f *DesignatedPointType) GetNames() []string    { return codeTypeValues(f.Name) }
func (f *DesignatedPointType) TimeSlices() []TimeSlice {
	out := make([]TimeSlice, len(f.TimeSlice))
	for i := range f.TimeSlice {
		out[i] = &f.TimeSlice[i].DesignatedPointTimeSlice
	}
	return out
}

func (f *ObstacleAreaType) GetGMLID() string      { return f.GMLID }
func (f *ObstacleAreaType) GetIdentifier() string { return identifierValue(f.Identifier) }
func (f *ObstacleAreaType) GetNames() []string    { return codeTypeValues(f.Name) }

func (f *VerticalStructureType) GetGMLID() string      { return f.GMLID }
func (f *VerticalStructureType) GetIdentifier() string { return identifierValue(f.Identifier) }
func (f *VerticalStructureType) GetNames() []string    { return codeTypeValues(f.Name) }

func (f *OrganisationAuthorityType) GetGMLID() string      { return f.GMLID }
func (f *OrganisationAuthorityType) GetIdentifier() string { return identifierValue(f.Identifier) }
func (f *OrganisationAuthorityType) GetNames() []string    { return codeTypeValues(f.Name) }

// GeoBorder implements Feature so that borders shared between airspaces —
// national boundaries and the like — can be resolved from a curveMember
// reference during geometry assembly.
func (f *GeoBorderType) GetGMLID() string      { return f.GMLID }
func (f *GeoBorderType) GetIdentifier() string { return identifierValue(f.Identifier) }
func (f *GeoBorderType) GetNames() []string    { return codeTypeValues(f.Name) }

// ---- TimeSlice interface implementations ----------------------------------

func (ts *NavaidTimeSliceType) GetValidTime() gml.TimePrimitivePropertyType { return ts.ValidTime }
func (ts *NavaidTimeSliceType) GetInterpretation() Interpretation {
	return Interpretation(ts.Interpretation)
}
func (ts *NavaidTimeSliceType) GetSequenceNumber() uint32 {
	if ts.SequenceNumber != nil {
		return *ts.SequenceNumber
	}
	return 0
}
func (ts *NavaidTimeSliceType) GetCorrectionNumber() uint32 {
	if ts.CorrectionNumber != nil {
		return *ts.CorrectionNumber
	}
	return 0
}

func (ts *DesignatedPointTimeSliceType) GetValidTime() gml.TimePrimitivePropertyType {
	return ts.ValidTime
}
func (ts *DesignatedPointTimeSliceType) GetInterpretation() Interpretation {
	return Interpretation(ts.Interpretation)
}
func (ts *DesignatedPointTimeSliceType) GetSequenceNumber() uint32 {
	if ts.SequenceNumber != nil {
		return *ts.SequenceNumber
	}
	return 0
}
func (ts *DesignatedPointTimeSliceType) GetCorrectionNumber() uint32 {
	if ts.CorrectionNumber != nil {
		return *ts.CorrectionNumber
	}
	return 0
}

func (ts *AirspaceTimeSliceType) GetValidTime() gml.TimePrimitivePropertyType {
	return ts.ValidTime
}
func (ts *AirspaceTimeSliceType) GetInterpretation() Interpretation {
	return Interpretation(ts.Interpretation)
}
func (ts *AirspaceTimeSliceType) GetSequenceNumber() uint32 {
	if ts.SequenceNumber != nil {
		return *ts.SequenceNumber
	}
	return 0
}
func (ts *AirspaceTimeSliceType) GetCorrectionNumber() uint32 {
	if ts.CorrectionNumber != nil {
		return *ts.CorrectionNumber
	}
	return 0
}

func (ts *AirportHeliportTimeSliceType) GetValidTime() gml.TimePrimitivePropertyType {
	return ts.ValidTime
}
func (ts *AirportHeliportTimeSliceType) GetInterpretation() Interpretation {
	return Interpretation(ts.Interpretation)
}
func (ts *AirportHeliportTimeSliceType) GetSequenceNumber() uint32 {
	if ts.SequenceNumber != nil {
		return *ts.SequenceNumber
	}
	return 0
}
func (ts *AirportHeliportTimeSliceType) GetCorrectionNumber() uint32 {
	if ts.CorrectionNumber != nil {
		return *ts.CorrectionNumber
	}
	return 0
}

func (ts *RunwayTimeSliceType) GetValidTime() gml.TimePrimitivePropertyType { return ts.ValidTime }
func (ts *RunwayTimeSliceType) GetInterpretation() Interpretation {
	return Interpretation(ts.Interpretation)
}
func (ts *RunwayTimeSliceType) GetSequenceNumber() uint32 {
	if ts.SequenceNumber != nil {
		return *ts.SequenceNumber
	}
	return 0
}
func (ts *RunwayTimeSliceType) GetCorrectionNumber() uint32 {
	if ts.CorrectionNumber != nil {
		return *ts.CorrectionNumber
	}
	return 0
}

// ---- Helpers ---------------------------------------------------------------

func identifierValue(c *gml.CodeType) string {
	if c == nil {
		return ""
	}
	return c.Value
}

func codeTypeValues(codes []gml.CodeType) []string {
	out := make([]string, len(codes))
	for i, c := range codes {
		out[i] = c.Value
	}
	return out
}
