// Package gml provides the subset of GML 3.2.1 types used by AIXM 5.1.1.
package gml

import "encoding/xml"

const Namespace = "http://www.opengis.net/gml/3.2"

// xlinkNamespace is the XLink namespace used for href references.
const xlinkNamespace = "http://www.w3.org/1999/xlink"

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
	XMLName xml.Name                `xml:"http://www.opengis.net/gml/3.2 GeodesicString"`
	Pos     []DirectPositionType    `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList *DirectPositionListType `xml:"http://www.opengis.net/gml/3.2 posList"`
}

// CurveMemberType holds a single Curve within a Ring.
//
// A curveMember may hold a gml:Curve, an aixm:Curve (which substitutes for
// gml:Curve per AIXM_Features.xsd), or no inline geometry at all — only an
// xlink:href referencing a GeoBorder elsewhere in the message.  All three
// occur in real data, so all three are captured here.
type CurveMemberType struct {
	Curve    *CurveType `xml:"http://www.opengis.net/gml/3.2 Curve"`
	AnyCurve *CurveType `xml:"-"` // an aixm:Curve, filled in by UnmarshalXML
	HrefAttr string     `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
}

// TheCurve returns the inline curve regardless of the namespace it was
// encoded in, or nil when the member is a bare reference.
func (cm CurveMemberType) TheCurve() *CurveType {
	if cm.Curve != nil {
		return cm.Curve
	}
	return cm.AnyCurve
}

// UnmarshalXML decodes a curveMember, accepting a Curve child in any
// namespace.  CurveType pins its XMLName to the GML namespace, so a plain
// struct field cannot bind an aixm:Curve; the element is decoded here instead
// and stored in AnyCurve.
func (cm *CurveMemberType) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		if a.Name.Space == xlinkNamespace && a.Name.Local == "href" {
			cm.HrefAttr = a.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local != "Curve" {
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			// Decode into a synthetic GML-namespaced element so that
			// CurveType's XMLName constraint is satisfied whatever the
			// source namespace was.
			var c CurveType
			el := t
			el.Name = xml.Name{Space: Namespace, Local: "Curve"}
			if err := d.DecodeElement(&c, &el); err != nil {
				return err
			}
			if t.Name.Space == Namespace {
				cm.Curve = &c
			} else {
				cm.AnyCurve = &c
			}
		case xml.EndElement:
			return nil
		}
	}
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
	XMLName       xml.Name                `xml:"http://www.opengis.net/gml/3.2 LineStringSegment"`
	Interpolation string                  `xml:"interpolation,attr,omitempty"`
	PosList       *DirectPositionListType `xml:"http://www.opengis.net/gml/3.2 posList"`
}

// ArcType is a circular arc defined by three points (start, mid, end).
type ArcType struct {
	XMLName xml.Name                `xml:"http://www.opengis.net/gml/3.2 Arc"`
	Pos     []DirectPositionType    `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList *DirectPositionListType `xml:"http://www.opengis.net/gml/3.2 posList"`
}

// CircleType is a full circle defined by three points on its circumference.
type CircleType struct {
	XMLName xml.Name                `xml:"http://www.opengis.net/gml/3.2 Circle"`
	Pos     []DirectPositionType    `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList *DirectPositionListType `xml:"http://www.opengis.net/gml/3.2 posList"`
}

// AngleType is an angular measure with a unit of measure (gml:AngleType).
type AngleType struct {
	Value float64 `xml:",chardata"`
	Uom   string  `xml:"uom,attr"`
}

// PointPropertyType wraps a Point inline or references one by xlink:href.
//
// The inline point may be a gml:Point or an aixm:Point, which substitutes for
// it; both are accepted.
type PointPropertyType struct {
	Point     *PointType `xml:"-"` // decoded by UnmarshalXML, in either namespace
	HrefAttr  string     `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason string     `xml:"nilReason,attr,omitempty"`
}

// UnmarshalXML decodes a point property, accepting a Point child in any
// namespace.  PointType pins its XMLName to the GML namespace, so a plain
// struct field cannot bind an aixm:Point.
func (p *PointPropertyType) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		switch {
		case a.Name.Space == xlinkNamespace && a.Name.Local == "href":
			p.HrefAttr = a.Value
		case a.Name.Local == "nilReason":
			p.NilReason = a.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local != "Point" {
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			var pt PointType
			el := t
			el.Name = xml.Name{Space: Namespace, Local: "Point"}
			if err := d.DecodeElement(&pt, &el); err != nil {
				return err
			}
			p.Point = &pt
		case xml.EndElement:
			return nil
		}
	}
}

// ArcByCenterPointType is a circular arc defined by a centre point, a radius
// and start/end bearings, rather than by three points on its circumference.
//
// The centre may be given as pos, posList, pointProperty or pointRep
// (geometryPrimitives.xsd); all four are captured.  Start and end angles are
// true bearings in degrees clockwise from north.
type ArcByCenterPointType struct {
	XMLName       xml.Name                `xml:"http://www.opengis.net/gml/3.2 ArcByCenterPoint"`
	Pos           *DirectPositionType     `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList       *DirectPositionListType `xml:"http://www.opengis.net/gml/3.2 posList"`
	PointProperty *PointPropertyType      `xml:"http://www.opengis.net/gml/3.2 pointProperty"`
	PointRep      *PointPropertyType      `xml:"http://www.opengis.net/gml/3.2 pointRep"`
	Radius        MeasureType             `xml:"http://www.opengis.net/gml/3.2 radius"`
	StartAngle    *AngleType              `xml:"http://www.opengis.net/gml/3.2 startAngle"`
	EndAngle      *AngleType              `xml:"http://www.opengis.net/gml/3.2 endAngle"`
	NumArc        int                     `xml:"numArc,attr,omitempty"`
	Interpolation string                  `xml:"interpolation,attr,omitempty"`
}

// CircleByCenterPointType is a full circle defined by a centre point and a
// radius.  It is an ArcByCenterPoint with no start or end angle.
type CircleByCenterPointType struct {
	XMLName       xml.Name                `xml:"http://www.opengis.net/gml/3.2 CircleByCenterPoint"`
	Pos           *DirectPositionType     `xml:"http://www.opengis.net/gml/3.2 pos"`
	PosList       *DirectPositionListType `xml:"http://www.opengis.net/gml/3.2 posList"`
	PointProperty *PointPropertyType      `xml:"http://www.opengis.net/gml/3.2 pointProperty"`
	PointRep      *PointPropertyType      `xml:"http://www.opengis.net/gml/3.2 pointRep"`
	Radius        MeasureType             `xml:"http://www.opengis.net/gml/3.2 radius"`
	NumArc        int                     `xml:"numArc,attr,omitempty"`
	Interpolation string                  `xml:"interpolation,attr,omitempty"`
}

// CurveSegment is a single segment of a curve, retaining its position in
// document order.  Exactly one of the pointer fields is non-nil.
type CurveSegment struct {
	LineStringSegment   *LineStringSegmentType
	GeodesicString      *GeodesicStringType
	Arc                 *ArcType
	Circle              *CircleType
	ArcByCenterPoint    *ArcByCenterPointType
	CircleByCenterPoint *CircleByCenterPointType
}

// CurveSegmentsType wraps one or more curve segments.
//
// The typed slices record the segments by kind; segments holds them in
// document order.  Order matters: a curve of GeodesicString, ArcByCenterPoint,
// GeodesicString describes a boundary that runs out along a radial, around an
// arc, and back, and re-assembling it by kind would produce a self-intersecting
// polygon.  Use Ordered to traverse the segments as they were written.
type CurveSegmentsType struct {
	LineStringSegment   []LineStringSegmentType   `xml:"http://www.opengis.net/gml/3.2 LineStringSegment"`
	GeodesicString      []GeodesicStringType      `xml:"http://www.opengis.net/gml/3.2 GeodesicString"`
	Arc                 []ArcType                 `xml:"http://www.opengis.net/gml/3.2 Arc"`
	Circle              []CircleType              `xml:"http://www.opengis.net/gml/3.2 Circle"`
	ArcByCenterPoint    []ArcByCenterPointType    `xml:"http://www.opengis.net/gml/3.2 ArcByCenterPoint"`
	CircleByCenterPoint []CircleByCenterPointType `xml:"http://www.opengis.net/gml/3.2 CircleByCenterPoint"`

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
// corresponding typed slice of CurveSegmentsType.
type segmentRef struct {
	kind  string
	index int
}

// Ordered returns the curve's segments in document order.
//
// For values decoded from XML this is the order they appeared in.  For values
// constructed in Go, where no ordering was ever recorded, the segments are
// synthesised from the typed slices by kind.
func (s *CurveSegmentsType) Ordered() []CurveSegment {
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
func (s *CurveSegmentsType) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
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
func (s *CurveSegmentsType) decodeSegment(d *xml.Decoder, start xml.StartElement) error {
	kind := start.Name.Local
	switch kind {
	case "GeodesicString":
		var v GeodesicStringType
		if err := d.DecodeElement(&v, &start); err != nil {
			return err
		}
		s.GeodesicString = append(s.GeodesicString, v)
		s.order = append(s.order, segmentRef{kind, len(s.GeodesicString) - 1})
	case "LineStringSegment":
		var v LineStringSegmentType
		if err := d.DecodeElement(&v, &start); err != nil {
			return err
		}
		s.LineStringSegment = append(s.LineStringSegment, v)
		s.order = append(s.order, segmentRef{kind, len(s.LineStringSegment) - 1})
	case "Arc":
		var v ArcType
		if err := d.DecodeElement(&v, &start); err != nil {
			return err
		}
		s.Arc = append(s.Arc, v)
		s.order = append(s.order, segmentRef{kind, len(s.Arc) - 1})
	case "Circle":
		var v CircleType
		if err := d.DecodeElement(&v, &start); err != nil {
			return err
		}
		s.Circle = append(s.Circle, v)
		s.order = append(s.order, segmentRef{kind, len(s.Circle) - 1})
	case "ArcByCenterPoint":
		var v ArcByCenterPointType
		if err := d.DecodeElement(&v, &start); err != nil {
			return err
		}
		s.ArcByCenterPoint = append(s.ArcByCenterPoint, v)
		s.order = append(s.order, segmentRef{kind, len(s.ArcByCenterPoint) - 1})
	case "CircleByCenterPoint":
		var v CircleByCenterPointType
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

// CurveType is a GML Curve geometry built from segments.
type CurveType struct {
	XMLName      xml.Name           `xml:"http://www.opengis.net/gml/3.2 Curve"`
	ID           string             `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName      string             `xml:"srsName,attr,omitempty"`
	SrsDimension uint               `xml:"srsDimension,attr,omitempty"`
	Segments     *CurveSegmentsType `xml:"http://www.opengis.net/gml/3.2 segments"`
}
