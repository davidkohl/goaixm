// Package adr provides the EUROCONTROL ADR 23.5.0 extensions of AIXM 5.1.1:
// the ADR-only features (AirportHeliportSet, AirportHeliportSetPattern) and
// the extension objects ADR adds to AIXM time slices.
//
// Register the ADR features with a registry before parsing:
//
//	reg := goaixm.NewDefaultRegistry()
//	adr.Register(reg)
//	msg, err := goaixm.ParseFile(ctx, path, goaixm.Options{Registry: reg})
//
// ADR extension objects arrive in the Extension slots of AIXM time slices;
// decode them with DecodeExtension:
//
//	for _, e := range airport.TimeSlice[0].Extension {
//		if ext, ok := adr.DecodeExtension[adr.AirportHeliportExtension](e); ok {
//			fmt.Println(ext.NMDesignator.String())
//		}
//	}
package adr

import (
	"encoding/xml"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
)

// Namespace is the ADR extension XML namespace.
const Namespace = "http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR"

// ---- data types ---------------------------------------------------------------

// AircraftAddresses holds a list of aircraft Mode S addresses.
type AircraftAddresses = aixm.Value

// AircraftRegistrations holds a list of aircraft registrations.
type AircraftRegistrations = aixm.Value

// CodeAirportHeliportNMDesignator is the Network Manager designator of an
// aerodrome.
type CodeAirportHeliportNMDesignator = aixm.Value

// CodeAirspaceReservationImpact codes the operational impact of an airspace
// reservation.
type CodeAirspaceReservationImpact = aixm.Value

// ---- extension objects (carried in AIXM time slice extension slots) ------------

// AircraftCharacteristicExtension extends aixm.AircraftCharacteristic.
type AircraftCharacteristicExtension struct {
	XMLName               xml.Name               `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AircraftCharacteristicExtension"`
	AircraftAddresses     *AircraftAddresses     `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR aircraftAddresses"`
	AircraftRegistrations *AircraftRegistrations `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR aircraftRegistrations"`
}

// AirportHeliportExtension extends aixm.AirportHeliportTimeSlice.
type AirportHeliportExtension struct {
	XMLName         xml.Name                         `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AirportHeliportExtension"`
	DefaultTaxiTime *aixm.ValDuration                `xml:"http://www.aixm.aero/schema/5.1.1 defaultTaxiTime"`
	NMDesignator    *CodeAirportHeliportNMDesignator `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR nmDesignator"`
}

// AirspaceExtension extends aixm.AirspaceTimeSlice.
type AirspaceExtension struct {
	XMLName         xml.Name    `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AirspaceExtension"`
	FlexibleUse     *aixm.Value `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR flexibleUse"`
	LevelChargeFlag *aixm.Value `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR levelChargeFlag"`
}

// DecodeExtension decodes an AIXM extension slot into the ADR extension
// object T (e.g. AirportHeliportExtension).  It reports false when the slot
// holds a different extension.
func DecodeExtension[T AircraftCharacteristicExtension | AirportHeliportExtension | AirspaceExtension](e aixm.Extension) (*T, bool) {
	v := new(T)
	if err := xml.Unmarshal(e.Inner, v); err != nil {
		return nil, false
	}
	return v, true
}

// ---- ADR features -------------------------------------------------------------

// AirportHeliportSet is an ADR feature grouping aerodromes.
type AirportHeliportSet struct {
	XMLName xml.Name `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AirportHeliportSet"`
	aixm.FeatureCommon
	TimeSlice []AirportHeliportSetTimeSlice `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR timeSlice>AirportHeliportSetTimeSlice"`
}

// AirportHeliportSetTimeSlice holds the properties of an AirportHeliportSet.
type AirportHeliportSetTimeSlice struct {
	aixm.TimeSliceCommon
	Name      *aixm.Value      `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR name"`
	Extension []aixm.Extension `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR extension"`
}

// Kind returns "AirportHeliportSet".
func (*AirportHeliportSet) Kind() string { return "AirportHeliportSet" }

// TimeSlices returns the feature's time slices.
func (f *AirportHeliportSet) TimeSlices() []aixm.TimeSlice {
	out := make([]aixm.TimeSlice, len(f.TimeSlice))
	for i := range f.TimeSlice {
		out[i] = &f.TimeSlice[i]
	}
	return out
}

// AirportHeliportSetPattern is an ADR feature defining a pattern for
// aerodrome sets.
type AirportHeliportSetPattern struct {
	XMLName xml.Name `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR AirportHeliportSetPattern"`
	aixm.FeatureCommon
	TimeSlice []AirportHeliportSetPatternTimeSlice `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR timeSlice>AirportHeliportSetPatternTimeSlice"`
}

// AirportHeliportSetPatternTimeSlice holds the properties of an
// AirportHeliportSetPattern.
type AirportHeliportSetPatternTimeSlice struct {
	aixm.TimeSliceCommon
	Extension []aixm.Extension `xml:"http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR extension"`
}

// Kind returns "AirportHeliportSetPattern".
func (*AirportHeliportSetPattern) Kind() string { return "AirportHeliportSetPattern" }

// TimeSlices returns the feature's time slices.
func (f *AirportHeliportSetPattern) TimeSlices() []aixm.TimeSlice {
	out := make([]aixm.TimeSlice, len(f.TimeSlice))
	for i := range f.TimeSlice {
		out[i] = &f.TimeSlice[i]
	}
	return out
}

// Register adds the ADR features to reg.
func Register(reg *goaixm.Registry) {
	reg.Register("AirportHeliportSet", func() aixm.Feature { return new(AirportHeliportSet) })
	reg.Register("AirportHeliportSetPattern", func() aixm.Feature { return new(AirportHeliportSetPattern) })
}
