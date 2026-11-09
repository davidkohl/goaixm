package aixm

import "github.com/davidkohl/goaixm/gml"

// The AIXM geometry types extend GML types and are written by hand; the
// generated model refers to them through the *Property types below.
// Convert them to coordinates with package geo.

// Point is an aixm:Point (a gml:Point with horizontal accuracy).
type Point struct {
	GMLID              string              `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string              `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                `xml:"srsDimension,attr,omitempty"`
	Pos                *gml.DirectPosition `xml:"http://www.opengis.net/gml/3.2 pos"`
	HorizontalAccuracy *ValDistance        `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy"`
}

// ElevatedPoint is an aixm:ElevatedPoint: a point with elevation.
type ElevatedPoint struct {
	GMLID              string               `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string               `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                 `xml:"srsDimension,attr,omitempty"`
	Pos                *gml.DirectPosition  `xml:"http://www.opengis.net/gml/3.2 pos"`
	HorizontalAccuracy *ValDistance         `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy"`
	Elevation          *ValDistanceVertical `xml:"http://www.aixm.aero/schema/5.1.1 elevation"`
	GeoidUndulation    *ValDistanceSigned   `xml:"http://www.aixm.aero/schema/5.1.1 geoidUndulation"`
	VerticalDatum      *CodeVerticalDatum   `xml:"http://www.aixm.aero/schema/5.1.1 verticalDatum"`
	VerticalAccuracy   *ValDistance         `xml:"http://www.aixm.aero/schema/5.1.1 verticalAccuracy"`
}

// Curve is an aixm:Curve (a gml:Curve with horizontal accuracy).
type Curve struct {
	GMLID              string             `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string             `xml:"srsName,attr,omitempty"`
	SrsDimension       uint               `xml:"srsDimension,attr,omitempty"`
	Segments           *gml.CurveSegments `xml:"http://www.opengis.net/gml/3.2 segments"`
	HorizontalAccuracy *ValDistance       `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy"`
}

// ElevatedCurve is an aixm:ElevatedCurve: a curve with elevation.
type ElevatedCurve struct {
	GMLID              string               `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string               `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                 `xml:"srsDimension,attr,omitempty"`
	Segments           *gml.CurveSegments   `xml:"http://www.opengis.net/gml/3.2 segments"`
	HorizontalAccuracy *ValDistance         `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy"`
	Elevation          *ValDistanceVertical `xml:"http://www.aixm.aero/schema/5.1.1 elevation"`
	GeoidUndulation    *ValDistanceSigned   `xml:"http://www.aixm.aero/schema/5.1.1 geoidUndulation"`
	VerticalDatum      *CodeVerticalDatum   `xml:"http://www.aixm.aero/schema/5.1.1 verticalDatum"`
	VerticalAccuracy   *ValDistance         `xml:"http://www.aixm.aero/schema/5.1.1 verticalAccuracy"`
}

// Surface is an aixm:Surface (a gml:Surface with horizontal accuracy).
type Surface struct {
	GMLID              string              `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string              `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                `xml:"srsDimension,attr,omitempty"`
	Patches            *gml.SurfacePatches `xml:"http://www.opengis.net/gml/3.2 patches"`
	HorizontalAccuracy *ValDistance        `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy"`
}

// ElevatedSurface is an aixm:ElevatedSurface: a surface with elevation.
type ElevatedSurface struct {
	GMLID              string               `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string               `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                 `xml:"srsDimension,attr,omitempty"`
	Patches            *gml.SurfacePatches  `xml:"http://www.opengis.net/gml/3.2 patches"`
	HorizontalAccuracy *ValDistance         `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy"`
	Elevation          *ValDistanceVertical `xml:"http://www.aixm.aero/schema/5.1.1 elevation"`
	GeoidUndulation    *ValDistanceSigned   `xml:"http://www.aixm.aero/schema/5.1.1 geoidUndulation"`
	VerticalDatum      *CodeVerticalDatum   `xml:"http://www.aixm.aero/schema/5.1.1 verticalDatum"`
	VerticalAccuracy   *ValDistance         `xml:"http://www.aixm.aero/schema/5.1.1 verticalAccuracy"`
}

// PointProperty holds a point, given as an aixm:Point or as an
// aixm:ElevatedPoint (which substitutes for it), or a reference.
type PointProperty struct {
	Point         *Point         `xml:"http://www.aixm.aero/schema/5.1.1 Point"`
	ElevatedPoint *ElevatedPoint `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedPoint"`
	Href          string         `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason     string         `xml:"nilReason,attr,omitempty"`
}

// Pos returns the position of whichever point form is present, or nil.
func (p *PointProperty) Pos() *gml.DirectPosition {
	switch {
	case p == nil:
		return nil
	case p.Point != nil:
		return p.Point.Pos
	case p.ElevatedPoint != nil:
		return p.ElevatedPoint.Pos
	}
	return nil
}

// ElevatedPointProperty holds an aixm:ElevatedPoint or a reference.
type ElevatedPointProperty struct {
	ElevatedPoint *ElevatedPoint `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedPoint"`
	Href          string         `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason     string         `xml:"nilReason,attr,omitempty"`
}

// Pos returns the position of the point, or nil.
func (p *ElevatedPointProperty) Pos() *gml.DirectPosition {
	if p == nil || p.ElevatedPoint == nil {
		return nil
	}
	return p.ElevatedPoint.Pos
}

// CurveProperty holds a curve, given as an aixm:Curve or as an
// aixm:ElevatedCurve (which substitutes for it), or a reference.
type CurveProperty struct {
	Curve         *Curve         `xml:"http://www.aixm.aero/schema/5.1.1 Curve"`
	ElevatedCurve *ElevatedCurve `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedCurve"`
	Href          string         `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason     string         `xml:"nilReason,attr,omitempty"`
}

// Segments returns the segments of whichever curve form is present, or nil.
func (c *CurveProperty) Segments() *gml.CurveSegments {
	switch {
	case c == nil:
		return nil
	case c.Curve != nil:
		return c.Curve.Segments
	case c.ElevatedCurve != nil:
		return c.ElevatedCurve.Segments
	}
	return nil
}

// ElevatedCurveProperty holds an aixm:ElevatedCurve or a reference.
type ElevatedCurveProperty struct {
	ElevatedCurve *ElevatedCurve `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedCurve"`
	Href          string         `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason     string         `xml:"nilReason,attr,omitempty"`
}

// Segments returns the segments of the curve, or nil.
func (c *ElevatedCurveProperty) Segments() *gml.CurveSegments {
	if c == nil || c.ElevatedCurve == nil {
		return nil
	}
	return c.ElevatedCurve.Segments
}

// SurfaceProperty holds a surface, given as an aixm:Surface or as an
// aixm:ElevatedSurface (which substitutes for it), or a reference.  Both
// forms occur in published data.
type SurfaceProperty struct {
	Surface         *Surface         `xml:"http://www.aixm.aero/schema/5.1.1 Surface"`
	ElevatedSurface *ElevatedSurface `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedSurface"`
	Href            string           `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason       string           `xml:"nilReason,attr,omitempty"`
}

// Patches returns the patches of whichever surface form is present, or nil.
func (s *SurfaceProperty) Patches() *gml.SurfacePatches {
	switch {
	case s == nil:
		return nil
	case s.Surface != nil:
		return s.Surface.Patches
	case s.ElevatedSurface != nil:
		return s.ElevatedSurface.Patches
	}
	return nil
}

// ElevatedSurfaceProperty holds an aixm:ElevatedSurface or a reference.
type ElevatedSurfaceProperty struct {
	ElevatedSurface *ElevatedSurface `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedSurface"`
	Href            string           `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason       string           `xml:"nilReason,attr,omitempty"`
}

// Patches returns the patches of the surface, or nil.
func (s *ElevatedSurfaceProperty) Patches() *gml.SurfacePatches {
	if s == nil || s.ElevatedSurface == nil {
		return nil
	}
	return s.ElevatedSurface.Patches
}

// Airspace aggregation operations (CodeAirspaceAggregation).
const (
	AggregationBase         = "BASE"
	AggregationUnion        = "UNION"
	AggregationIntersection = "INTERS"
	AggregationSubtraction  = "SUBTR"
)

// Airspace dependency codes (CodeAirspaceDependency).
const (
	DependencyFullGeometry   = "FULL_GEOMETRY"
	DependencyHorzProjection = "HORZ_PROJECTION"
)
