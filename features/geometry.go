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
	XMLName            xml.Name           `xml:"http://www.aixm.aero/schema/5.1.1 Point"`
	GMLID              string             `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string             `xml:"srsName,attr,omitempty"`
	SrsDimension       uint               `xml:"srsDimension,attr,omitempty"`
	Pos                *gml.DirectPositionType `xml:"http://www.opengis.net/gml/3.2 pos"`
	HorizontalAccuracy *types.ValDistanceType  `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy,omitempty"`
}

// ElevatedPointType extends PointType with elevation information.
type ElevatedPointType struct {
	XMLName            xml.Name                `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedPoint"`
	GMLID              string                  `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string                  `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                    `xml:"srsDimension,attr,omitempty"`
	Pos                *gml.DirectPositionType `xml:"http://www.opengis.net/gml/3.2 pos"`
	HorizontalAccuracy *types.ValDistanceType  `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy,omitempty"`
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
	XMLName            xml.Name            `xml:"http://www.aixm.aero/schema/5.1.1 Surface"`
	GMLID              string              `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string              `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                `xml:"srsDimension,attr,omitempty"`
	Patches            *gml.SurfacePatchesType `xml:"http://www.opengis.net/gml/3.2 patches"`
	HorizontalAccuracy *types.ValDistanceType  `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy,omitempty"`
}

// ElevatedSurfaceType extends SurfaceType with elevation information.
type ElevatedSurfaceType struct {
	XMLName            xml.Name                `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedSurface"`
	GMLID              string                  `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string                  `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                    `xml:"srsDimension,attr,omitempty"`
	Patches            *gml.SurfacePatchesType `xml:"http://www.opengis.net/gml/3.2 patches"`
	HorizontalAccuracy *types.ValDistanceType  `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy,omitempty"`
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
	XMLName            xml.Name            `xml:"http://www.aixm.aero/schema/5.1.1 Curve"`
	GMLID              string              `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string              `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                `xml:"srsDimension,attr,omitempty"`
	Segments           *gml.CurveSegmentsType `xml:"http://www.opengis.net/gml/3.2 segments"`
	HorizontalAccuracy *types.ValDistanceType `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy,omitempty"`
}

// ElevatedCurveType extends CurveType with elevation information.
type ElevatedCurveType struct {
	XMLName            xml.Name               `xml:"http://www.aixm.aero/schema/5.1.1 ElevatedCurve"`
	GMLID              string                 `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	SrsName            string                 `xml:"srsName,attr,omitempty"`
	SrsDimension       uint                   `xml:"srsDimension,attr,omitempty"`
	Segments           *gml.CurveSegmentsType `xml:"http://www.opengis.net/gml/3.2 segments"`
	HorizontalAccuracy *types.ValDistanceType `xml:"http://www.aixm.aero/schema/5.1.1 horizontalAccuracy,omitempty"`
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

// HorizontalProjectionType holds the 2D surface projection of an airspace volume.
type HorizontalProjectionType struct {
	Surface *SurfaceType `xml:"http://www.aixm.aero/schema/5.1.1 Surface,omitempty"`
}

// AirspaceVolumeType defines the 3D volume of an airspace.
type AirspaceVolumeType struct {
	XMLName              xml.Name                  `xml:"http://www.aixm.aero/schema/5.1.1 AirspaceVolume"`
	GMLID                string                    `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	UpperLimit           *types.ValDistanceVerticalType `xml:"http://www.aixm.aero/schema/5.1.1 upperLimit,omitempty"`
	UpperLimitReference  *types.CodeVerticalReferenceType `xml:"http://www.aixm.aero/schema/5.1.1 upperLimitReference,omitempty"`
	LowerLimit           *types.ValDistanceVerticalType `xml:"http://www.aixm.aero/schema/5.1.1 lowerLimit,omitempty"`
	LowerLimitReference  *types.CodeVerticalReferenceType `xml:"http://www.aixm.aero/schema/5.1.1 lowerLimitReference,omitempty"`
	HorizontalProjection HorizontalProjectionType  `xml:"http://www.aixm.aero/schema/5.1.1 horizontalProjection"`
}

// AirspaceVolumePropertyType wraps an AirspaceVolume (inline or XLink).
type AirspaceVolumePropertyType struct {
	AirspaceVolume *AirspaceVolumeType `xml:"http://www.aixm.aero/schema/5.1.1 AirspaceVolume,omitempty"`
	HrefAttr       string              `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason      string              `xml:"nilReason,attr,omitempty"`
	Owns           bool                `xml:"owns,attr,omitempty"`
}

// AirspaceGeometryComponentType holds a single geometry component of an airspace.
type AirspaceGeometryComponentType struct {
	XMLName           xml.Name                   `xml:"http://www.aixm.aero/schema/5.1.1 AirspaceGeometryComponent"`
	GMLID             string                     `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	TheAirspaceVolume AirspaceVolumePropertyType `xml:"http://www.aixm.aero/schema/5.1.1 theAirspaceVolume"`
}

// AirspaceGeometryComponentPropertyType wraps an AirspaceGeometryComponent (inline or XLink).
// This overrides the generated XLink-only stub in features.go.
type AirspaceGeometryComponentPropertyType struct {
	AirspaceGeometryComponent *AirspaceGeometryComponentType `xml:"http://www.aixm.aero/schema/5.1.1 AirspaceGeometryComponent,omitempty"`
	HrefAttr                  string                         `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	NilReason                 string                         `xml:"nilReason,attr,omitempty"`
	Owns                      bool                           `xml:"owns,attr,omitempty"`
}
