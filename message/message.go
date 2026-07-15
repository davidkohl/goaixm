// Package message provides AIXM message wrapper types for AIXMBasicMessage and ADRMessage.
package message

import "encoding/xml"

const (
	BasicMessageNamespace = "http://www.aixm.aero/schema/5.1.1/message"
	ADRMessageNamespace   = "http://www.eurocontrol.int/cfmu/b2b/ADRMessage"
	GMLNamespace          = "http://www.opengis.net/gml/3.2"
)

// BasicMessageMember holds the raw inner XML of a single hasMember element.
// The parse package resolves this to a concrete feature type using the feature registry.
type BasicMessageMember struct {
	Inner    []byte `xml:",innerxml"`
	HrefAttr string `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	Owns     bool   `xml:"owns,attr,omitempty"`
}

// AIXMBasicMessage is the primary AIXM exchange format: a collection of AIXM features.
type AIXMBasicMessage struct {
	XMLName   xml.Name             `xml:"http://www.aixm.aero/schema/5.1.1/message AIXMBasicMessage"`
	GMLID     string               `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	HasMember []BasicMessageMember `xml:"http://www.aixm.aero/schema/5.1.1/message hasMember"`
}

// ADRMessageMember holds the raw inner XML of a single ADRMessage hasMember element.
type ADRMessageMember struct {
	Inner    []byte `xml:",innerxml"`
	HrefAttr string `xml:"http://www.w3.org/1999/xlink href,attr,omitempty"`
	Owns     bool   `xml:"owns,attr,omitempty"`
}

// ADRMessage is the EUROCONTROL ADR exchange format.
type ADRMessage struct {
	XMLName   xml.Name           `xml:"http://www.eurocontrol.int/cfmu/b2b/ADRMessage ADRMessage"`
	GMLID     string             `xml:"http://www.opengis.net/gml/3.2 id,attr,omitempty"`
	HasMember []ADRMessageMember `xml:"http://www.eurocontrol.int/cfmu/b2b/ADRMessage hasMember"`
}
