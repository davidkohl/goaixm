// Package gml provides the subset of GML 3.2.1 types used by AIXM 5.1.1.
package gml

import "encoding/xml"

const Namespace = "http://www.opengis.net/gml/3.2"

// CodeType is a character string with an optional code space.
type CodeType struct {
	Value     string `xml:",chardata"`
	CodeSpace string `xml:"codeSpace,attr,omitempty"`
}

// MeasureType is a numeric value with a unit of measure.
type MeasureType struct {
	Value float64 `xml:",chardata"`
	Uom   string  `xml:"uom,attr"`
}

// NilReasonType records why a value is absent.
type NilReasonType = string

// AbstractGMLType is the base for all GML objects with identity.
type AbstractGMLType struct {
	ID          string     `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	Description string     `xml:"http://www.opengis.net/gml/3.2 description,omitempty"`
	Identifier  *CodeType  `xml:"http://www.opengis.net/gml/3.2 identifier"`
	Name        []CodeType `xml:"http://www.opengis.net/gml/3.2 name"`
}

// ---- Temporal types -------------------------------------------------------

// TimePositionType holds an ISO 8601 temporal position string.
type TimePositionType struct {
	Value                 string `xml:",chardata"`
	Frame                 string `xml:"frame,attr,omitempty"`
	CalendarEraName       string `xml:"calendarEraName,attr,omitempty"`
	IndeterminatePosition string `xml:"indeterminatePosition,attr,omitempty"`
}

// TimeInstantType is a zero-dimensional temporal geometric primitive.
type TimeInstantType struct {
	XMLName      xml.Name         `xml:"http://www.opengis.net/gml/3.2 TimeInstant"`
	ID           string           `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	TimePosition TimePositionType `xml:"http://www.opengis.net/gml/3.2 timePosition"`
}

// TimeInstantPropertyType wraps a TimeInstant or an XLink reference.
type TimeInstantPropertyType struct {
	TimeInstant *TimeInstantType `xml:"http://www.opengis.net/gml/3.2 TimeInstant"`
	HrefAttr    string           `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason   string           `xml:"nilReason,attr,omitempty"`
	Owns        bool             `xml:"owns,attr,omitempty"`
}

// TimePeriodType is a one-dimensional temporal geometric primitive.
type TimePeriodType struct {
	XMLName       xml.Name                 `xml:"http://www.opengis.net/gml/3.2 TimePeriod"`
	ID            string                   `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	BeginPosition *TimePositionType        `xml:"http://www.opengis.net/gml/3.2 beginPosition"`
	Begin         *TimeInstantPropertyType `xml:"http://www.opengis.net/gml/3.2 begin"`
	EndPosition   *TimePositionType        `xml:"http://www.opengis.net/gml/3.2 endPosition"`
	End           *TimeInstantPropertyType `xml:"http://www.opengis.net/gml/3.2 end"`
}

// TimePrimitivePropertyType wraps either a TimePeriod or a TimeInstant.
// AIXM validTime always uses TimePeriod; TimeInstant is included for completeness.
type TimePrimitivePropertyType struct {
	TimePeriod  *TimePeriodType  `xml:"http://www.opengis.net/gml/3.2 TimePeriod"`
	TimeInstant *TimeInstantType `xml:"http://www.opengis.net/gml/3.2 TimeInstant"`
	HrefAttr    string           `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason   string           `xml:"nilReason,attr,omitempty"`
	Owns        bool             `xml:"owns,attr,omitempty"`
}

// ---- Geometry types -------------------------------------------------------

// DirectPositionType holds space-separated coordinate values.
type DirectPositionType struct {
	Value        string `xml:",chardata"` // space-separated doubles
	SrsName      string `xml:"srsName,attr,omitempty"`
	SrsDimension uint   `xml:"srsDimension,attr,omitempty"`
	AxisLabels   string `xml:"axisLabels,attr,omitempty"`
	UomLabels    string `xml:"uomLabels,attr,omitempty"`
}

// DirectPositionListType holds a flat list of coordinate values for a sequence of positions.
type DirectPositionListType struct {
	Value        string `xml:",chardata"`
	SrsName      string `xml:"srsName,attr,omitempty"`
	SrsDimension uint   `xml:"srsDimension,attr,omitempty"`
	Count        uint   `xml:"count,attr,omitempty"`
}

// PointType is a GML Point geometry.
type PointType struct {
	XMLName      xml.Name            `xml:"http://www.opengis.net/gml/3.2 Point"`
	ID           string              `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName      string              `xml:"srsName,attr,omitempty"`
	SrsDimension uint                `xml:"srsDimension,attr,omitempty"`
	Pos          *DirectPositionType `xml:"http://www.opengis.net/gml/3.2 pos"`
}

// LinearRingType is a closed ring defined by a list of positions.
type LinearRingType struct {
	XMLName xml.Name                `xml:"http://www.opengis.net/gml/3.2 LinearRing"`
	PosList *DirectPositionListType `xml:"http://www.opengis.net/gml/3.2 posList"`
	Pos     []DirectPositionType    `xml:"http://www.opengis.net/gml/3.2 pos"`
}

// GeodesicStringType is a curve segment defined by individual pos elements.
type GeodesicStringType struct {
	XMLName xml.Name             `xml:"http://www.opengis.net/gml/3.2 GeodesicString"`
	Pos     []DirectPositionType `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList *DirectPositionListType `xml:"http://www.opengis.net/gml/3.2 posList"`
}

// CurveMemberType holds a single Curve within a Ring.
type CurveMemberType struct {
	Curve *CurveType `xml:"http://www.opengis.net/gml/3.2 Curve"`
}

// RingType is a closed ring composed of curve members.
type RingType struct {
	XMLName     xml.Name          `xml:"http://www.opengis.net/gml/3.2 Ring"`
	CurveMember []CurveMemberType `xml:"http://www.opengis.net/gml/3.2 curveMember"`
}

// AbstractRingPropertyType wraps a LinearRing or Ring.
type AbstractRingPropertyType struct {
	LinearRing *LinearRingType `xml:"http://www.opengis.net/gml/3.2 LinearRing"`
	Ring       *RingType       `xml:"http://www.opengis.net/gml/3.2 Ring"`
}

// PolygonPatchType is a single planar surface patch with optional interior rings.
type PolygonPatchType struct {
	XMLName  xml.Name                   `xml:"http://www.opengis.net/gml/3.2 PolygonPatch"`
	Exterior *AbstractRingPropertyType  `xml:"http://www.opengis.net/gml/3.2 exterior"`
	Interior []AbstractRingPropertyType `xml:"http://www.opengis.net/gml/3.2 interior"`
}

// SurfacePatchesType wraps one or more surface patches.
type SurfacePatchesType struct {
	PolygonPatch []PolygonPatchType `xml:"http://www.opengis.net/gml/3.2 PolygonPatch"`
}

// SurfaceType is a GML Surface geometry built from patches.
type SurfaceType struct {
	XMLName      xml.Name            `xml:"http://www.opengis.net/gml/3.2 Surface"`
	ID           string              `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName      string              `xml:"srsName,attr,omitempty"`
	SrsDimension uint                `xml:"srsDimension,attr,omitempty"`
	Patches      *SurfacePatchesType `xml:"http://www.opengis.net/gml/3.2 patches"`
}

// LineStringSegmentType is a straight-line curve segment.
type LineStringSegmentType struct {
	XMLName      xml.Name                `xml:"http://www.opengis.net/gml/3.2 LineStringSegment"`
	Interpolation string                 `xml:"interpolation,attr,omitempty"`
	PosList       *DirectPositionListType `xml:"http://www.opengis.net/gml/3.2 posList"`
}

// ArcType is a circular arc defined by three points (start, mid, end).
type ArcType struct {
	XMLName xml.Name             `xml:"http://www.opengis.net/gml/3.2 Arc"`
	Pos     []DirectPositionType `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList *DirectPositionListType `xml:"http://www.opengis.net/gml/3.2 posList"`
}

// CircleType is a full circle defined by three points on its circumference.
type CircleType struct {
	XMLName xml.Name             `xml:"http://www.opengis.net/gml/3.2 Circle"`
	Pos     []DirectPositionType `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList *DirectPositionListType `xml:"http://www.opengis.net/gml/3.2 posList"`
}

// CurveSegmentsType wraps one or more curve segments.
type CurveSegmentsType struct {
	LineStringSegment []LineStringSegmentType `xml:"http://www.opengis.net/gml/3.2 LineStringSegment"`
	GeodesicString    []GeodesicStringType    `xml:"http://www.opengis.net/gml/3.2 GeodesicString"`
	Arc               []ArcType               `xml:"http://www.opengis.net/gml/3.2 Arc"`
	Circle            []CircleType            `xml:"http://www.opengis.net/gml/3.2 Circle"`
}

// CurveType is a GML Curve geometry built from segments.
type CurveType struct {
	XMLName      xml.Name           `xml:"http://www.opengis.net/gml/3.2 Curve"`
	ID           string             `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName      string             `xml:"srsName,attr,omitempty"`
	SrsDimension uint               `xml:"srsDimension,attr,omitempty"`
	Segments     *CurveSegmentsType `xml:"http://www.opengis.net/gml/3.2 segments"`
}
