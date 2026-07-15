// Package adr provides EUROCONTROL ADR 23.5.0 extension types for AIXM 5.1.1.
//
// Import this package with a blank import to register ADR features with the parse registry:
//
//	import _ "gitlab.com/davidkohl/goaixm/extensions/adr"
package adr

import (
	"encoding/xml"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/gml"
	"gitlab.com/davidkohl/goaixm/parse"
	"gitlab.com/davidkohl/goaixm/types"
)

const Namespace = "http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR"

// ---- ADR Data Types -------------------------------------------------------

// AircraftAddressesType holds a list of aircraft Mode S addresses.
type AircraftAddressesType = types.NilableString

// AircraftRegistrationsType holds a list of aircraft registration strings.
type AircraftRegistrationsType = types.NilableString

// CodeAirportHeliportNMDesignatorType is the NM designator for an aerodrome.
type CodeAirportHeliportNMDesignatorType = types.NilableString

// CodeAirspaceReservationImpactType codes the operational impact of an airspace reservation.
type CodeAirspaceReservationImpactType = types.NilableString

// ---- Extension types (injected into AIXM timeslice extension slots) --------

// AircraftCharacteristicExtensionType extends AircraftCharacteristic timeslices.
type AircraftCharacteristicExtensionType struct {
	XMLName              xml.Name                  `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AircraftCharacteristicExtension"`
	AircraftAddresses    *AircraftAddressesType    `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR aircraftAddresses,omitempty"`
	AircraftRegistrations *AircraftRegistrationsType `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR aircraftRegistrations,omitempty"`
}

// AirportHeliportExtensionType extends AirportHeliport timeslices.
type AirportHeliportExtensionType struct {
	XMLName        xml.Name                          `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AirportHeliportExtension"`
	DefaultTaxiTime *types.ValDurationType           `xml:"http://www.aixm.aero/schema/5.1.1 defaultTaxiTime,omitempty"`
	NMDesignator   *CodeAirportHeliportNMDesignatorType `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR nmDesignator,omitempty"`
}

// AirspaceExtensionType extends Airspace timeslices.
type AirspaceExtensionType struct {
	XMLName          xml.Name             `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AirspaceExtension"`
	FlexibleUse      *types.NilableString `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR flexibleUse,omitempty"`
	LevelChargeFlag  *types.NilableString `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR levelChargeFlag,omitempty"`
}

// ---- ADR feature types (new features, not extensions to existing ones) ----

// AirportHeliportSetType is an ADR feature grouping airports/heliports.
type AirportHeliportSetType struct {
	XMLName    xml.Name                                    `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AirportHeliportSet"`
	GMLID      string                                      `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	Identifier *gml.CodeType                               `xml:"http://www.opengis.net/gml/3.2 identifier"`
	Name       []gml.CodeType                              `xml:"http://www.opengis.net/gml/3.2 name"`
	TimeSlice  []AirportHeliportSetTimeSlicePropertyType   `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR timeSlice"`
}

func (f *AirportHeliportSetType) GetGMLID() string      { return f.GMLID }
func (f *AirportHeliportSetType) GetIdentifier() string {
	if f.Identifier == nil {
		return ""
	}
	return f.Identifier.Value
}
func (f *AirportHeliportSetType) GetNames() []string {
	out := make([]string, len(f.Name))
	for i, n := range f.Name {
		out[i] = n.Value
	}
	return out
}

// AirportHeliportSetTimeSlicePropertyType wraps a timeslice.
type AirportHeliportSetTimeSlicePropertyType struct {
	AirportHeliportSetTimeSlice AirportHeliportSetTimeSliceType `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AirportHeliportSetTimeSlice"`
	Owns                        bool                            `xml:"owns,attr,omitempty"`
}

// AirportHeliportSetTimeSliceType carries temporal properties.
type AirportHeliportSetTimeSliceType struct {
	XMLName          xml.Name                        `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AirportHeliportSetTimeSlice"`
	GMLID            string                          `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	ValidTime         gml.TimePrimitivePropertyType  `xml:"http://www.opengis.net/gml/3.2 validTime"`
	Interpretation   string                          `xml:"http://www.aixm.aero/schema/5.1.1 interpretation"`
	SequenceNumber   *uint32                         `xml:"http://www.aixm.aero/schema/5.1.1 sequenceNumber,omitempty"`
	CorrectionNumber *uint32                         `xml:"http://www.aixm.aero/schema/5.1.1 correctionNumber,omitempty"`
	Name             *types.NilableString            `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR name,omitempty"`
	Extension        []features.ExtensionContainer   `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR extension"`
}

// AirportHeliportSetPatternType is an ADR feature defining a pattern for airport sets.
type AirportHeliportSetPatternType struct {
	XMLName    xml.Name                                          `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AirportHeliportSetPattern"`
	GMLID      string                                            `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	Identifier *gml.CodeType                                     `xml:"http://www.opengis.net/gml/3.2 identifier"`
	Name       []gml.CodeType                                    `xml:"http://www.opengis.net/gml/3.2 name"`
	TimeSlice  []AirportHeliportSetPatternTimeSlicePropertyType  `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR timeSlice"`
}

func (f *AirportHeliportSetPatternType) GetGMLID() string      { return f.GMLID }
func (f *AirportHeliportSetPatternType) GetIdentifier() string {
	if f.Identifier == nil {
		return ""
	}
	return f.Identifier.Value
}
func (f *AirportHeliportSetPatternType) GetNames() []string {
	out := make([]string, len(f.Name))
	for i, n := range f.Name {
		out[i] = n.Value
	}
	return out
}

// AirportHeliportSetPatternTimeSlicePropertyType wraps a timeslice.
type AirportHeliportSetPatternTimeSlicePropertyType struct {
	AirportHeliportSetPatternTimeSlice AirportHeliportSetPatternTimeSliceType `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AirportHeliportSetPatternTimeSlice"`
	Owns                               bool                                   `xml:"owns,attr,omitempty"`
}

// AirportHeliportSetPatternTimeSliceType carries temporal properties.
type AirportHeliportSetPatternTimeSliceType struct {
	XMLName          xml.Name                        `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AirportHeliportSetPatternTimeSlice"`
	GMLID            string                          `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	ValidTime         gml.TimePrimitivePropertyType  `xml:"http://www.opengis.net/gml/3.2 validTime"`
	Interpretation   string                          `xml:"http://www.aixm.aero/schema/5.1.1 interpretation"`
	SequenceNumber   *uint32                         `xml:"http://www.aixm.aero/schema/5.1.1 sequenceNumber,omitempty"`
	CorrectionNumber *uint32                         `xml:"http://www.aixm.aero/schema/5.1.1 correctionNumber,omitempty"`
	Extension        []features.ExtensionContainer   `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR extension"`
}

// ---- Registration ----------------------------------------------------------

func init() {
	parse.DefaultRegistry.Register("AirportHeliportSet", func() any { return new(AirportHeliportSetType) })
	parse.DefaultRegistry.Register("AirportHeliportSetPattern", func() any { return new(AirportHeliportSetPatternType) })
}
