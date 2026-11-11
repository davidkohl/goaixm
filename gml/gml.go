// Package gml provides the subset of GML 3.2.1 types used by AIXM 5.1.1.
package gml

import "encoding/xml"

// Namespace is the GML 3.2 XML namespace.
const Namespace = "http://www.opengis.net/gml/3.2"

// Code is a character string with an optional code space.
type Code struct {
	Value     string `xml:",chardata"`
	CodeSpace string `xml:"codeSpace,attr,omitempty"`
}

// Measure is a numeric value with a unit of measure.
type Measure struct {
	Value float64 `xml:",chardata"`
	UOM   string  `xml:"uom,attr"`
}

// NilReason records why a value is absent.
type NilReason = string

// AbstractGML is the base for all GML objects with identity.
type AbstractGML struct {
	ID          string `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	Description string `xml:"http://www.opengis.net/gml/3.2 description,omitempty"`
	Identifier  *Code  `xml:"http://www.opengis.net/gml/3.2 identifier"`
	Name        []Code `xml:"http://www.opengis.net/gml/3.2 name"`
}

// ---- Temporal types -------------------------------------------------------

// TimePosition holds an ISO 8601 temporal position string.
type TimePosition struct {
	Value                 string `xml:",chardata"`
	Frame                 string `xml:"frame,attr,omitempty"`
	CalendarEraName       string `xml:"calendarEraName,attr,omitempty"`
	IndeterminatePosition string `xml:"indeterminatePosition,attr,omitempty"`
}

// TimeInstant is a zero-dimensional temporal geometric primitive.
type TimeInstant struct {
	XMLName      xml.Name     `xml:"http://www.opengis.net/gml/3.2 TimeInstant"`
	ID           string       `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	TimePosition TimePosition `xml:"http://www.opengis.net/gml/3.2 timePosition"`
}

// TimeInstantProperty wraps a TimeInstant or an XLink reference.
type TimeInstantProperty struct {
	TimeInstant *TimeInstant `xml:"http://www.opengis.net/gml/3.2 TimeInstant"`
	Href        string       `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason   string       `xml:"nilReason,attr,omitempty"`
	Owns        bool         `xml:"owns,attr,omitempty"`
}

// TimePeriod is a one-dimensional temporal geometric primitive.
type TimePeriod struct {
	XMLName       xml.Name             `xml:"http://www.opengis.net/gml/3.2 TimePeriod"`
	ID            string               `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	BeginPosition *TimePosition        `xml:"http://www.opengis.net/gml/3.2 beginPosition"`
	Begin         *TimeInstantProperty `xml:"http://www.opengis.net/gml/3.2 begin"`
	EndPosition   *TimePosition        `xml:"http://www.opengis.net/gml/3.2 endPosition"`
	End           *TimeInstantProperty `xml:"http://www.opengis.net/gml/3.2 end"`
}

// TimePrimitiveProperty wraps either a TimePeriod or a TimeInstant.
// AIXM validTime always uses TimePeriod; TimeInstant is included for completeness.
type TimePrimitiveProperty struct {
	TimePeriod  *TimePeriod  `xml:"http://www.opengis.net/gml/3.2 TimePeriod"`
	TimeInstant *TimeInstant `xml:"http://www.opengis.net/gml/3.2 TimeInstant"`
	Href        string       `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason   string       `xml:"nilReason,attr,omitempty"`
	Owns        bool         `xml:"owns,attr,omitempty"`
}

// ---- Geometry types -------------------------------------------------------

// DirectPosition holds space-separated coordinate values.
type DirectPosition struct {
	Value        string `xml:",chardata"` // space-separated doubles
	SrsName      string `xml:"srsName,attr,omitempty"`
	SrsDimension uint   `xml:"srsDimension,attr,omitempty"`
	AxisLabels   string `xml:"axisLabels,attr,omitempty"`
	UomLabels    string `xml:"uomLabels,attr,omitempty"`
}

// DirectPositionList holds a flat list of coordinate values for a sequence of positions.
type DirectPositionList struct {
	Value        string `xml:",chardata"`
	SrsName      string `xml:"srsName,attr,omitempty"`
	SrsDimension uint   `xml:"srsDimension,attr,omitempty"`
	Count        uint   `xml:"count,attr,omitempty"`
}

// Point is a GML Point geometry.
// The element name is not pinned: aixm:Point substitutes for gml:Point.
type Point struct {
	ID           string          `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName      string          `xml:"srsName,attr,omitempty"`
	SrsDimension uint            `xml:"srsDimension,attr,omitempty"`
	Pos          *DirectPosition `xml:"http://www.opengis.net/gml/3.2 pos"`
}

// LinearRing is a closed ring defined by a list of positions.
type LinearRing struct {
	XMLName xml.Name            `xml:"http://www.opengis.net/gml/3.2 LinearRing"`
	PosList *DirectPositionList `xml:"http://www.opengis.net/gml/3.2 posList"`
	Pos     []DirectPosition    `xml:"http://www.opengis.net/gml/3.2 pos"`
}

// GeodesicString is a curve segment defined by individual pos elements.
type GeodesicString struct {
	XMLName xml.Name            `xml:"http://www.opengis.net/gml/3.2 GeodesicString"`
	Pos     []DirectPosition    `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList *DirectPositionList `xml:"http://www.opengis.net/gml/3.2 posList"`
}

// CurveMember holds a single Curve within a Ring.
//
// A curveMember may hold a gml:Curve, an aixm:Curve (which substitutes for
// gml:Curve per AIXM_Features.xsd), or no inline geometry at all — only an
// xlink:href referencing a GeoBorder elsewhere in the message.  All three
// occur in real data; the namespace-free tag on Curve accepts both curve
// forms.
type CurveMember struct {
	Curve *Curve `xml:"Curve"`
	Href  string `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
}

// Ring is a closed ring composed of curve members.
type Ring struct {
	XMLName     xml.Name      `xml:"http://www.opengis.net/gml/3.2 Ring"`
	CurveMember []CurveMember `xml:"http://www.opengis.net/gml/3.2 curveMember"`
}

// AbstractRingProperty wraps a LinearRing or Ring.
type AbstractRingProperty struct {
	LinearRing *LinearRing `xml:"http://www.opengis.net/gml/3.2 LinearRing"`
	Ring       *Ring       `xml:"http://www.opengis.net/gml/3.2 Ring"`
}

// PolygonPatch is a single planar surface patch with optional interior rings.
type PolygonPatch struct {
	XMLName  xml.Name               `xml:"http://www.opengis.net/gml/3.2 PolygonPatch"`
	Exterior *AbstractRingProperty  `xml:"http://www.opengis.net/gml/3.2 exterior"`
	Interior []AbstractRingProperty `xml:"http://www.opengis.net/gml/3.2 interior"`
}

// SurfacePatches wraps one or more surface patches.
type SurfacePatches struct {
	PolygonPatch []PolygonPatch `xml:"http://www.opengis.net/gml/3.2 PolygonPatch"`
}

// Surface is a GML Surface geometry built from patches.
type Surface struct {
	XMLName      xml.Name        `xml:"http://www.opengis.net/gml/3.2 Surface"`
	ID           string          `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName      string          `xml:"srsName,attr,omitempty"`
	SrsDimension uint            `xml:"srsDimension,attr,omitempty"`
	Patches      *SurfacePatches `xml:"http://www.opengis.net/gml/3.2 patches"`
}

// LineStringSegment is a straight-line curve segment.
type LineStringSegment struct {
	XMLName       xml.Name            `xml:"http://www.opengis.net/gml/3.2 LineStringSegment"`
	Interpolation string              `xml:"interpolation,attr,omitempty"`
	Pos           []DirectPosition    `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList       *DirectPositionList `xml:"http://www.opengis.net/gml/3.2 posList"`
}

// Arc is a circular arc defined by three points (start, mid, end).
type Arc struct {
	XMLName xml.Name            `xml:"http://www.opengis.net/gml/3.2 Arc"`
	Pos     []DirectPosition    `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList *DirectPositionList `xml:"http://www.opengis.net/gml/3.2 posList"`
}

// Circle is a full circle defined by three points on its circumference.
type Circle struct {
	XMLName xml.Name            `xml:"http://www.opengis.net/gml/3.2 Circle"`
	Pos     []DirectPosition    `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList *DirectPositionList `xml:"http://www.opengis.net/gml/3.2 posList"`
}

// Angle is an angular measure with a unit of measure (gml:Angle).
type Angle struct {
	Value float64 `xml:",chardata"`
	UOM   string  `xml:"uom,attr"`
}

// PointProperty wraps a Point inline or references one by xlink:href.
//
// The inline point may be a gml:Point or an aixm:Point, which substitutes for
// it; the namespace-free tag accepts both.
type PointProperty struct {
	Point     *Point `xml:"Point"`
	Href      string `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason string `xml:"nilReason,attr,omitempty"`
}

// ArcByCenterPoint is a circular arc defined by a centre point, a radius
// and start/end bearings, rather than by three points on its circumference.
//
// The centre may be given as pos, posList, pointProperty or pointRep
// (geometryPrimitives.xsd); all four are captured.  Start and end angles are
// true bearings in degrees clockwise from north.
type ArcByCenterPoint struct {
	XMLName       xml.Name            `xml:"http://www.opengis.net/gml/3.2 ArcByCenterPoint"`
	Pos           *DirectPosition     `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList       *DirectPositionList `xml:"http://www.opengis.net/gml/3.2 posList"`
	PointProperty *PointProperty      `xml:"http://www.opengis.net/gml/3.2 pointProperty"`
	PointRep      *PointProperty      `xml:"http://www.opengis.net/gml/3.2 pointRep"`
	Radius        Measure             `xml:"http://www.opengis.net/gml/3.2 radius"`
	StartAngle    *Angle              `xml:"http://www.opengis.net/gml/3.2 startAngle"`
	EndAngle      *Angle              `xml:"http://www.opengis.net/gml/3.2 endAngle"`
	NumArc        int                 `xml:"numArc,attr,omitempty"`
	Interpolation string              `xml:"interpolation,attr,omitempty"`
}

// CircleByCenterPoint is a full circle defined by a centre point and a
// radius.  It is an ArcByCenterPoint with no start or end angle.
type CircleByCenterPoint struct {
	XMLName       xml.Name            `xml:"http://www.opengis.net/gml/3.2 CircleByCenterPoint"`
	Pos           *DirectPosition     `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList       *DirectPositionList `xml:"http://www.opengis.net/gml/3.2 posList"`
	PointProperty *PointProperty      `xml:"http://www.opengis.net/gml/3.2 pointProperty"`
	PointRep      *PointProperty      `xml:"http://www.opengis.net/gml/3.2 pointRep"`
	Radius        Measure             `xml:"http://www.opengis.net/gml/3.2 radius"`
	NumArc        int                 `xml:"numArc,attr,omitempty"`
	Interpolation string              `xml:"interpolation,attr,omitempty"`
}

// CurveSegment is a single segment of a curve, retaining its position in
// document order.  Exactly one of the pointer fields is non-nil.
type CurveSegment struct {
	LineStringSegment   *LineStringSegment
	GeodesicString      *GeodesicString
	Arc                 *Arc
	Circle              *Circle
	ArcByCenterPoint    *ArcByCenterPoint
	CircleByCenterPoint *CircleByCenterPoint
}

// CurveSegments wraps one or more curve segments.
//
// The typed slices record the segments by kind; segments holds them in
// document order.  Order matters: a curve of GeodesicString, ArcByCenterPoint,
// GeodesicString describes a boundary that runs out along a radial, around an
// arc, and back, and re-assembling it by kind would produce a self-intersecting
// polygon.  Use Ordered to traverse the segments as they were written.
type CurveSegments struct {
	LineStringSegment   []LineStringSegment   `xml:"http://www.opengis.net/gml/3.2 LineStringSegment"`
	GeodesicString      []GeodesicString      `xml:"http://www.opengis.net/gml/3.2 GeodesicString"`
	Arc                 []Arc                 `xml:"http://www.opengis.net/gml/3.2 Arc"`
	Circle              []Circle              `xml:"http://www.opengis.net/gml/3.2 Circle"`
	ArcByCenterPoint    []ArcByCenterPoint    `xml:"http://www.opengis.net/gml/3.2 ArcByCenterPoint"`
	CircleByCenterPoint []CircleByCenterPoint `xml:"http://www.opengis.net/gml/3.2 CircleByCenterPoint"`

	// order records the kind and per-kind index of each segment in document
	// order.  It is populated by UnmarshalXML and is therefore empty for
	// values built in Go rather than decoded from XML.
	//
	// Indices are stored rather than pointers because the typed slices are
	// grown by append during decoding, which would leave earlier pointers
	// dangling at a stale backing array.
	order []segmentRef
}

// segmentRef identifies one segment by its kind and its index within the
// corresponding typed slice of CurveSegments.
type segmentRef struct {
	kind  string
	index int
}

// Ordered returns the curve's segments in document order.
//
// For values decoded from XML this is the order they appeared in.  For values
// constructed in Go, where no ordering was ever recorded, the segments are
// synthesised from the typed slices by kind.
func (s *CurveSegments) Ordered() []CurveSegment {
	if s == nil {
		return nil
	}
	if len(s.order) > 0 {
		out := make([]CurveSegment, 0, len(s.order))
		for _, ref := range s.order {
			var seg CurveSegment
			switch ref.kind {
			case "GeodesicString":
				seg.GeodesicString = &s.GeodesicString[ref.index]
			case "LineStringSegment":
				seg.LineStringSegment = &s.LineStringSegment[ref.index]
			case "Arc":
				seg.Arc = &s.Arc[ref.index]
			case "Circle":
				seg.Circle = &s.Circle[ref.index]
			case "ArcByCenterPoint":
				seg.ArcByCenterPoint = &s.ArcByCenterPoint[ref.index]
			case "CircleByCenterPoint":
				seg.CircleByCenterPoint = &s.CircleByCenterPoint[ref.index]
			}
			out = append(out, seg)
		}
		return out
	}
	var out []CurveSegment
	for i := range s.GeodesicString {
		out = append(out, CurveSegment{GeodesicString: &s.GeodesicString[i]})
	}
	for i := range s.LineStringSegment {
		out = append(out, CurveSegment{LineStringSegment: &s.LineStringSegment[i]})
	}
	for i := range s.Arc {
		out = append(out, CurveSegment{Arc: &s.Arc[i]})
	}
	for i := range s.Circle {
		out = append(out, CurveSegment{Circle: &s.Circle[i]})
	}
	for i := range s.ArcByCenterPoint {
		out = append(out, CurveSegment{ArcByCenterPoint: &s.ArcByCenterPoint[i]})
	}
	for i := range s.CircleByCenterPoint {
		out = append(out, CurveSegment{CircleByCenterPoint: &s.CircleByCenterPoint[i]})
	}
	return out
}

// UnmarshalXML decodes the segments of a curve, recording both their kind and
// their document order.
func (s *CurveSegments) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := s.decodeSegment(d, t); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// decodeSegment decodes one curve segment element, appending it to both the
// typed slice for its kind and the ordered list.
func (s *CurveSegments) decodeSegment(d *xml.Decoder, start xml.StartElement) error {
	kind := start.Name.Local
	switch kind {
	case "GeodesicString":
		var v GeodesicString
		if err := d.DecodeElement(&v, &start); err != nil {
			return err
		}
		s.GeodesicString = append(s.GeodesicString, v)
		s.order = append(s.order, segmentRef{kind, len(s.GeodesicString) - 1})
	case "LineStringSegment":
		var v LineStringSegment
		if err := d.DecodeElement(&v, &start); err != nil {
			return err
		}
		s.LineStringSegment = append(s.LineStringSegment, v)
		s.order = append(s.order, segmentRef{kind, len(s.LineStringSegment) - 1})
	case "Arc":
		var v Arc
		if err := d.DecodeElement(&v, &start); err != nil {
			return err
		}
		s.Arc = append(s.Arc, v)
		s.order = append(s.order, segmentRef{kind, len(s.Arc) - 1})
	case "Circle":
		var v Circle
		if err := d.DecodeElement(&v, &start); err != nil {
			return err
		}
		s.Circle = append(s.Circle, v)
		s.order = append(s.order, segmentRef{kind, len(s.Circle) - 1})
	case "ArcByCenterPoint":
		var v ArcByCenterPoint
		if err := d.DecodeElement(&v, &start); err != nil {
			return err
		}
		s.ArcByCenterPoint = append(s.ArcByCenterPoint, v)
		s.order = append(s.order, segmentRef{kind, len(s.ArcByCenterPoint) - 1})
	case "CircleByCenterPoint":
		var v CircleByCenterPoint
		if err := d.DecodeElement(&v, &start); err != nil {
			return err
		}
		s.CircleByCenterPoint = append(s.CircleByCenterPoint, v)
		s.order = append(s.order, segmentRef{kind, len(s.CircleByCenterPoint) - 1})
	default:
		return d.Skip()
	}
	return nil
}

// Curve is a GML Curve geometry built from segments.
// The element name is not pinned: aixm:Curve substitutes for gml:Curve.
type Curve struct {
	ID           string         `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName      string         `xml:"srsName,attr,omitempty"`
	SrsDimension uint           `xml:"srsDimension,attr,omitempty"`
	Segments     *CurveSegments `xml:"http://www.opengis.net/gml/3.2 segments"`
}
