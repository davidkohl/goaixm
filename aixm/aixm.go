// Package aixm is the Go model of AIXM 5.1.1 (Aeronautical Information
// Exchange Model): features, their time slices, objects and data types.
//
// Most of the package is generated from the AIXM 5.1.1 XML schemas by
// tools/aixmgen (see generate.go); the files ending in _gen.go must not be
// edited by hand.  This file and geometry.go hold the hand-written parts.
//
// # Model
//
//   - A feature (Airspace, Navaid, …) has an identity (FeatureCommon) and a
//     list of time slices.  All features implement Feature.
//   - A time slice (AirspaceTimeSlice, …) holds the feature's properties for
//     a period of validity.  All time slices implement TimeSlice.
//   - Objects (AirspaceLayerClass, Timesheet, …) are complex values nested in
//     time slices.  Where the schema allows several kinds of object, a choice
//     type (SegmentPoint, …) has one field per kind.
//   - Data types (CodeAirspace, ValDistanceVertical, …) are aliases of Value,
//     which keeps the text, the unit of measure and the nil reason.
//   - References to other features are Reference values (xlink:href); resolve
//     them with goaixm.Message.Resolve.
package aixm

import (
	"sort"
	"strconv"
	"strings"

	"github.com/davidkohl/goaixm/gml"
)

//go:generate go run ../tools/aixmgen -xsd ../tools/xsd/aixm-5.1.1 -out .

// Namespace is the AIXM 5.1.1 XML namespace.
const Namespace = "http://www.aixm.aero/schema/5.1.1"

// Value is the representation of every AIXM data type: codes, texts,
// numbers and measures.  Measures carry their unit in UOM; a property that
// was encoded as xsi:nil carries the reason in NilReason.
type Value struct {
	Value     string `xml:",chardata"`
	NilReason string `xml:"nilReason,attr,omitempty"`
	// UOM is the unit of measure of value types (uom attribute), e.g. FT,
	// FL, M, NM or MHZ.  Empty for code and text types.
	UOM string `xml:"uom,attr,omitempty"`
}

// String returns the text of v; a nil v yields "".
func (v *Value) String() string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(v.Value)
}

// Float parses the text of v as a number.
func (v *Value) Float() (float64, bool) {
	if v == nil {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v.Value), 64)
	return f, err == nil
}

// Reference is a property that points to another feature by xlink:href,
// as AIXM requires for associations between features.
type Reference struct {
	Href      string `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	Title     string `xml:"http://www.w3.org/1999/xlink title,attr,omitempty"`
	NilReason string `xml:"nilReason,attr,omitempty"`
}

// Extension is a time slice or object extension element, kept as raw XML.
// Decode it into an extension schema (e.g. package extensions/adr) with
// xml.Unmarshal.
type Extension struct {
	Inner []byte `xml:",innerxml"`
	Owns  bool   `xml:"owns,attr,omitempty"`
}

// XHTML holds XHTML content as raw XML.
type XHTML struct {
	Inner     []byte `xml:",innerxml"`
	NilReason string `xml:"nilReason,attr,omitempty"`
}

// Interpretation is the kind of a time slice.
type Interpretation string

// Time slice interpretations.
const (
	Baseline  Interpretation = "BASELINE"
	Snapshot  Interpretation = "SNAPSHOT"
	TempDelta Interpretation = "TEMPDELTA"
	PermDelta Interpretation = "PERMDELTA"
)

// Feature is implemented by every AIXM feature.
type Feature interface {
	// Common returns the identity shared by all features.
	Common() *FeatureCommon
	// Kind returns the XML local name of the feature, e.g. "Airspace".
	Kind() string
	// TimeSlices returns the feature's time slices.
	TimeSlices() []TimeSlice
}

// TimeSlice is implemented by every AIXM time slice.
type TimeSlice interface {
	// Common returns the temporal properties shared by all time slices.
	Common() *TimeSliceCommon
}

// FeatureCommon holds the properties every feature has.  It is embedded in
// each feature type.
type FeatureCommon struct {
	GMLID      string     `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	Identifier *gml.Code  `xml:"http://www.opengis.net/gml/3.2 identifier"`
	Names      []gml.Code `xml:"http://www.opengis.net/gml/3.2 name"`
}

// Common returns c.  Through embedding it gives every feature access to its
// identity via the Feature interface.
func (c *FeatureCommon) Common() *FeatureCommon { return c }

// UUID returns the feature's gml:identifier, the identity shared by all its
// time slices.
func (c *FeatureCommon) UUID() string {
	if c == nil || c.Identifier == nil {
		return ""
	}
	return strings.TrimSpace(c.Identifier.Value)
}

// TimeSliceCommon holds the properties every time slice has.  It is embedded
// in each time slice type.
type TimeSliceCommon struct {
	GMLID            string                     `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	ValidTime        gml.TimePrimitiveProperty  `xml:"http://www.opengis.net/gml/3.2 validTime"`
	Interpretation   Interpretation             `xml:"http://www.aixm.aero/schema/5.1.1 interpretation"`
	SequenceNumber   *uint32                    `xml:"http://www.aixm.aero/schema/5.1.1 sequenceNumber"`
	CorrectionNumber *uint32                    `xml:"http://www.aixm.aero/schema/5.1.1 correctionNumber"`
	FeatureLifetime  *gml.TimePrimitiveProperty `xml:"http://www.aixm.aero/schema/5.1.1 featureLifetime"`
}

// Common returns c.  Through embedding it gives every time slice access to
// its temporal properties via the TimeSlice interface.
func (c *TimeSliceCommon) Common() *TimeSliceCommon { return c }

// Sequence returns the sequence and correction numbers (0 when absent).
func (c *TimeSliceCommon) Sequence() (sequence, correction uint32) {
	if c.SequenceNumber != nil {
		sequence = *c.SequenceNumber
	}
	if c.CorrectionNumber != nil {
		correction = *c.CorrectionNumber
	}
	return sequence, correction
}

// ObjectCommon holds the gml:id every object has.  It is embedded in each
// object type.
type ObjectCommon struct {
	GMLID string `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
}

// NewFeature returns an empty feature for the XML local name, e.g.
// NewFeature("Airspace") returns a *Airspace.
func NewFeature(kind string) (Feature, bool) {
	f, ok := featureFactories[kind]
	if !ok {
		return nil, false
	}
	return f(), true
}

// FeatureKinds returns the XML local names of all AIXM features, sorted.
func FeatureKinds() []string {
	out := make([]string, 0, len(featureFactories))
	for k := range featureFactories {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
