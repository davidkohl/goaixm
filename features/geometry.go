package features

import (
	"encoding/xml"

	"gitlab.com/davidkohl/goaixm/gml"
	"gitlab.com/davidkohl/goaixm/types"
)

const (
	aixmNS = "http://www.aixm.aero/schema/5.1.1"
	gmlNS  = "http://www.opengis.net/gml/3.2"
)

// PointType extends gml:Point with AIXM horizontal accuracy and annotations.
type PointType struct {
	XMLName            xml.Name                `xml:"http://www.aixm.aero/schema/5.1.1 Point"`
	GMLID              string                  `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string                  `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                    `xml:"srsDimension,attr,omitempty"`
	Pos                *gml.DirectPositionType `xml:"http://www.opengis.net/gml/3.2 pos"`
	HorizontalAccuracy *types.ValDistanceType  `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy,omitempty"`
}

// ElevatedPointType extends PointType with elevation information.
type ElevatedPointType struct {
	XMLName            xml.Name                       `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedPoint"`
	GMLID              string                         `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string                         `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                           `xml:"srsDimension,attr,omitempty"`
	Pos                *gml.DirectPositionType        `xml:"http://www.opengis.net/gml/3.2 pos"`
	HorizontalAccuracy *types.ValDistanceType         `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy,omitempty"`
	Elevation          *types.ValDistanceVerticalType `xml:"http://www.aixm.aero/schema/5.1.1 elevation,omitempty"`
	GeoidUndulation    *types.ValDistanceSignedType   `xml:"http://www.aixm.aero/schema/5.1.1 geoidUndulation,omitempty"`
	VerticalDatum      *types.CodeVerticalDatumType   `xml:"http://www.aixm.aero/schema/5.1.1 verticalDatum,omitempty"`
	VerticalAccuracy   *types.ValDistanceType         `xml:"http://www.aixm.aero/schema/5.1.1 verticalAccuracy,omitempty"`
}

// ElevatedPointPropertyType wraps an ElevatedPoint (inline or XLink).
type ElevatedPointPropertyType struct {
	ElevatedPoint *ElevatedPointType `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedPoint,omitempty"`
	HrefAttr      string             `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason     string             `xml:"nilReason,attr,omitempty"`
	Owns          bool               `xml:"owns,attr,omitempty"`
}

// SurfaceType extends gml:Surface with AIXM accuracy.
type SurfaceType struct {
	XMLName            xml.Name                `xml:"http://www.aixm.aero/schema/5.1.1 Surface"`
	GMLID              string                  `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string                  `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                    `xml:"srsDimension,attr,omitempty"`
	Patches            *gml.SurfacePatchesType `xml:"http://www.opengis.net/gml/3.2 patches"`
	HorizontalAccuracy *types.ValDistanceType  `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy,omitempty"`
}

// ElevatedSurfaceType extends SurfaceType with elevation information.
type ElevatedSurfaceType struct {
	XMLName            xml.Name                       `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedSurface"`
	GMLID              string                         `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string                         `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                           `xml:"srsDimension,attr,omitempty"`
	Patches            *gml.SurfacePatchesType        `xml:"http://www.opengis.net/gml/3.2 patches"`
	HorizontalAccuracy *types.ValDistanceType         `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy,omitempty"`
	Elevation          *types.ValDistanceVerticalType `xml:"http://www.aixm.aero/schema/5.1.1 elevation,omitempty"`
	GeoidUndulation    *types.ValDistanceSignedType   `xml:"http://www.aixm.aero/schema/5.1.1 geoidUndulation,omitempty"`
	VerticalDatum      *types.CodeVerticalDatumType   `xml:"http://www.aixm.aero/schema/5.1.1 verticalDatum,omitempty"`
	VerticalAccuracy   *types.ValDistanceType         `xml:"http://www.aixm.aero/schema/5.1.1 verticalAccuracy,omitempty"`
}

// ElevatedSurfacePropertyType wraps an ElevatedSurface (inline or XLink).
type ElevatedSurfacePropertyType struct {
	ElevatedSurface *ElevatedSurfaceType `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedSurface,omitempty"`
	HrefAttr        string               `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason       string               `xml:"nilReason,attr,omitempty"`
	Owns            bool                 `xml:"owns,attr,omitempty"`
}

// CurveType extends gml:Curve with AIXM accuracy.
type CurveType struct {
	XMLName            xml.Name               `xml:"http://www.aixm.aero/schema/5.1.1 Curve"`
	GMLID              string                 `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string                 `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                   `xml:"srsDimension,attr,omitempty"`
	Segments           *gml.CurveSegmentsType `xml:"http://www.opengis.net/gml/3.2 segments"`
	HorizontalAccuracy *types.ValDistanceType `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy,omitempty"`
}

// ElevatedCurveType extends CurveType with elevation information.
type ElevatedCurveType struct {
	XMLName            xml.Name                       `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedCurve"`
	GMLID              string                         `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string                         `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                           `xml:"srsDimension,attr,omitempty"`
	Segments           *gml.CurveSegmentsType         `xml:"http://www.opengis.net/gml/3.2 segments"`
	HorizontalAccuracy *types.ValDistanceType         `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy,omitempty"`
	Elevation          *types.ValDistanceVerticalType `xml:"http://www.aixm.aero/schema/5.1.1 elevation,omitempty"`
	GeoidUndulation    *types.ValDistanceSignedType   `xml:"http://www.aixm.aero/schema/5.1.1 geoidUndulation,omitempty"`
	VerticalDatum      *types.CodeVerticalDatumType   `xml:"http://www.aixm.aero/schema/5.1.1 verticalDatum,omitempty"`
	VerticalAccuracy   *types.ValDistanceType         `xml:"http://www.aixm.aero/schema/5.1.1 verticalAccuracy,omitempty"`
}

// ElevatedCurvePropertyType wraps an ElevatedCurve (inline or XLink).
type ElevatedCurvePropertyType struct {
	ElevatedCurve *ElevatedCurveType `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedCurve,omitempty"`
	HrefAttr      string             `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason     string             `xml:"nilReason,attr,omitempty"`
	Owns          bool               `xml:"owns,attr,omitempty"`
}

// PointPropertyType wraps a Point (inline or XLink).
type PointPropertyType struct {
	Point     *PointType `xml:"http://www.aixm.aero/schema/5.1.1 Point,omitempty"`
	HrefAttr  string     `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason string     `xml:"nilReason,attr,omitempty"`
	Owns      bool       `xml:"owns,attr,omitempty"`
}

// CurvePropertyType wraps a Curve (inline or XLink).
type CurvePropertyType struct {
	Curve     *CurveType `xml:"http://www.aixm.aero/schema/5.1.1 Curve,omitempty"`
	HrefAttr  string     `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason string     `xml:"nilReason,attr,omitempty"`
	Owns      bool       `xml:"owns,attr,omitempty"`
}

// SurfacePropertyType wraps a Surface (inline or XLink).
type SurfacePropertyType struct {
	Surface   *SurfaceType `xml:"http://www.aixm.aero/schema/5.1.1 Surface,omitempty"`
	HrefAttr  string       `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason string       `xml:"nilReason,attr,omitempty"`
	Owns      bool         `xml:"owns,attr,omitempty"`
}

// HorizontalProjectionType holds the 2D surface projection of an airspace
// volume.
//
// The XSD types this property as aixm:SurfacePropertyType, so the surface may
// be given either as an aixm:Surface or as an aixm:ElevatedSurface, which
// substitutes for it.  Both forms occur in published AIP data.
type HorizontalProjectionType struct {
	Surface         *SurfaceType         `xml:"http://www.aixm.aero/schema/5.1.1 Surface,omitempty"`
	ElevatedSurface *ElevatedSurfaceType `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedSurface,omitempty"`
	HrefAttr        string               `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason       string               `xml:"nilReason,attr,omitempty"`
}

// Patches returns the surface patches of whichever surface variant is present,
// or nil when the projection holds no inline surface.
func (h HorizontalProjectionType) Patches() *gml.SurfacePatchesType {
	switch {
	case h.Surface != nil:
		return h.Surface.Patches
	case h.ElevatedSurface != nil:
		return h.ElevatedSurface.Patches
	default:
		return nil
	}
}

// SrsName returns the CRS of whichever surface variant is present.
func (h HorizontalProjectionType) SrsName() string {
	switch {
	case h.Surface != nil:
		return h.Surface.SrsName
	case h.ElevatedSurface != nil:
		return h.ElevatedSurface.SrsName
	default:
		return ""
	}
}

// Airspace dependency codes (AIXM CodeAirspaceDependencyType).
const (
	DependencyFullGeometry   = "FULL_GEOMETRY"
	DependencyHorzProjection = "HORZ_PROJECTION"
)

// Airspace aggregation operations (AIXM CodeAirspaceAggregationType).
const (
	AggregationBase         = "BASE"
	AggregationUnion        = "UNION"
	AggregationIntersection = "INTERS"
	AggregationSubtraction  = "SUBTR"
)

// AirspaceVolumeType defines the 3D volume of an airspace.
type AirspaceVolumeType struct {
	XMLName               xml.Name                         `xml:"http://www.aixm.aero/schema/5.1.1 AirspaceVolume"`
	GMLID                 string                           `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	UpperLimit            *types.ValDistanceVerticalType   `xml:"http://www.aixm.aero/schema/5.1.1 upperLimit,omitempty"`
	UpperLimitReference   *types.CodeVerticalReferenceType `xml:"http://www.aixm.aero/schema/5.1.1 upperLimitReference,omitempty"`
	MaximumLimit          *types.ValDistanceVerticalType   `xml:"http://www.aixm.aero/schema/5.1.1 maximumLimit,omitempty"`
	MaximumLimitReference *types.CodeVerticalReferenceType `xml:"http://www.aixm.aero/schema/5.1.1 maximumLimitReference,omitempty"`
	LowerLimit            *types.ValDistanceVerticalType   `xml:"http://www.aixm.aero/schema/5.1.1 lowerLimit,omitempty"`
	LowerLimitReference   *types.CodeVerticalReferenceType `xml:"http://www.aixm.aero/schema/5.1.1 lowerLimitReference,omitempty"`
	MinimumLimit          *types.ValDistanceVerticalType   `xml:"http://www.aixm.aero/schema/5.1.1 minimumLimit,omitempty"`
	MinimumLimitReference *types.CodeVerticalReferenceType `xml:"http://www.aixm.aero/schema/5.1.1 minimumLimitReference,omitempty"`
	Width                 *types.ValDistanceType           `xml:"http://www.aixm.aero/schema/5.1.1 width,omitempty"`
	HorizontalProjection  HorizontalProjectionType         `xml:"http://www.aixm.aero/schema/5.1.1 horizontalProjection"`

	// Centreline, with Width, describes a corridor airspace rather than one
	// bounded by a horizontal projection.
	Centreline *CurvePropertyType `xml:"http://www.aixm.aero/schema/5.1.1 centreline,omitempty"`

	// ContributorAirspace aggregates the geometry of another airspace instead
	// of defining it directly.
	ContributorAirspace *AirspaceVolumeDependencyRefType `xml:"http://www.aixm.aero/schema/5.1.1 contributorAirspace,omitempty"`
}

// AirspaceVolumeDependencyType records that a volume takes its geometry from
// another airspace.
type AirspaceVolumeDependencyType struct {
	XMLName     xml.Name                          `xml:"http://www.aixm.aero/schema/5.1.1 AirspaceVolumeDependency"`
	GMLID       string                            `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	Dependency  *types.CodeAirspaceDependencyType `xml:"http://www.aixm.aero/schema/5.1.1 dependency,omitempty"`
	TheAirspace *AirspaceRefType                  `xml:"http://www.aixm.aero/schema/5.1.1 theAirspace,omitempty"`
}

// AirspaceVolumeDependencyRefType wraps an AirspaceVolumeDependency (inline or
// XLink).
//
// This stands in for the generated AirspaceVolumeDependencyPropertyType, which
// is an XLink-only stub and cannot hold the inline dependency element that
// real data uses.
type AirspaceVolumeDependencyRefType struct {
	AirspaceVolumeDependency *AirspaceVolumeDependencyType `xml:"http://www.aixm.aero/schema/5.1.1 AirspaceVolumeDependency,omitempty"`
	HrefAttr                 string                        `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason                string                        `xml:"nilReason,attr,omitempty"`
	Owns                     bool                          `xml:"owns,attr,omitempty"`
}

// AirspaceRefType references an Airspace, either inline or by XLink.
//
// This overrides the generated XLink-only AirspacePropertyType alias so that
// parse.resolveXLinks can populate the Airspace pointer from the message
// index, letting aggregated geometry be assembled without a second lookup.
type AirspaceRefType struct {
	Airspace  *AirspaceType `xml:"http://www.aixm.aero/schema/5.1.1 Airspace,omitempty"`
	HrefAttr  string        `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason string        `xml:"nilReason,attr,omitempty"`
	Owns      bool          `xml:"owns,attr,omitempty"`
}

// AirspaceVolumePropertyType wraps an AirspaceVolume (inline or XLink).
type AirspaceVolumePropertyType struct {
	AirspaceVolume *AirspaceVolumeType `xml:"http://www.aixm.aero/schema/5.1.1 AirspaceVolume,omitempty"`
	HrefAttr       string              `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason      string              `xml:"nilReason,attr,omitempty"`
	Owns           bool                `xml:"owns,attr,omitempty"`
}

// AirspaceGeometryComponentType holds a single geometry component of an airspace.
//
// Operation says how this component combines with the others (BASE, UNION,
// INTERS or SUBTR) and OperationSequence gives the order in which the
// combination is applied.
type AirspaceGeometryComponentType struct {
	XMLName           xml.Name                           `xml:"http://www.aixm.aero/schema/5.1.1 AirspaceGeometryComponent"`
	GMLID             string                             `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	Operation         *types.CodeAirspaceAggregationType `xml:"http://www.aixm.aero/schema/5.1.1 operation,omitempty"`
	OperationSequence *types.NoSequenceType              `xml:"http://www.aixm.aero/schema/5.1.1 operationSequence,omitempty"`
	TheAirspaceVolume AirspaceVolumePropertyType         `xml:"http://www.aixm.aero/schema/5.1.1 theAirspaceVolume"`
}

// AirspaceGeometryComponentPropertyType wraps an AirspaceGeometryComponent (inline or XLink).
// This overrides the generated XLink-only stub in features.go.
type AirspaceGeometryComponentPropertyType struct {
	AirspaceGeometryComponent *AirspaceGeometryComponentType `xml:"http://www.aixm.aero/schema/5.1.1 AirspaceGeometryComponent,omitempty"`
	HrefAttr                  string                         `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason                 string                         `xml:"nilReason,attr,omitempty"`
	Owns                      bool                           `xml:"owns,attr,omitempty"`
}
