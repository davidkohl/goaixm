package aixm45to51

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
)

const (
	nsAIXM  = "http://www.aixm.aero/schema/5.1.1"
	nsGML   = "http://www.opengis.net/gml/3.2"
	nsXLink = "http://www.w3.org/1999/xlink"
	srs4326 = "urn:ogc:def:crs:EPSG::4326"
)

func esc(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// el renders <aixm:name attrs…>text</aixm:name>; attrs are name, value
// pairs, written in the given order.
func el(name, text string, attrs ...string) string {
	var b strings.Builder
	b.WriteString("<aixm:" + name)
	for i := 0; i+1 < len(attrs); i += 2 {
		fmt.Fprintf(&b, ` %s="%s"`, attrs[i], esc(attrs[i+1]))
	}
	if text == "" {
		b.WriteString("/>")
		return b.String()
	}
	b.WriteString(">" + esc(text) + "</aixm:" + name + ">")
	return b.String()
}

func href(name, uuid string) string {
	return `<aixm:` + name + ` xlink:href="urn:uuid:` + uuid + `"/>`
}

// wrap renders <aixm:name>inner</aixm:name> with inner already XML.
func wrap(name, inner string) string {
	return "<aixm:" + name + ">" + inner + "</aixm:" + name + ">"
}

// feature is a 5.1.1 feature with one BASELINE time slice under
// construction.
type feature struct {
	name  string // e.g. "DesignatedPoint"
	uuid  string
	n     int // gml:id counter
	props *props
}

func newFeature(name, uuid string) *feature {
	f := &feature{name: name, uuid: uuid}
	f.props = f.newProps()
	return f
}

// id returns a new gml:id unique within the document: prefix, counter and
// the feature's UUID.
func (f *feature) id(prefix string) string {
	f.n++
	return fmt.Sprintf("%s%d-%s", prefix, f.n, f.uuid)
}

type prop struct{ name, xml string }

// props collects the properties of a feature time slice or an object, to
// be rendered in schema order.
type props struct {
	f    *feature
	list []prop
}

func (f *feature) newProps() *props { return &props{f: f} }

func (p *props) add(name, xml string) { p.list = append(p.list, prop{name, xml}) }

// note adds an annotation Note.  propertyName may be "".
func (p *props) note(propertyName, purpose, text string) {
	var b strings.Builder
	b.WriteString(`<aixm:Note gml:id="` + p.f.id("n") + `">`)
	if propertyName != "" {
		b.WriteString(el("propertyName", propertyName))
	}
	b.WriteString(el("purpose", purpose))
	b.WriteString(`<aixm:translatedNote><aixm:LinguisticNote gml:id="` + p.f.id("ln") + `">`)
	b.WriteString(el("note", text))
	b.WriteString(`</aixm:LinguisticNote></aixm:translatedNote></aixm:Note>`)
	p.add("annotation", wrap("annotation", b.String()))
}

// render writes the properties in the order of names (a property's
// position in the schema sequence); properties of the same name keep their
// order.  A property not in names is a converter bug.
func (p *props) render(order []string) string {
	pos := make(map[string]int, len(order))
	for i, n := range order {
		pos[n] = i
	}
	list := append([]prop(nil), p.list...)
	for _, q := range list {
		if _, ok := pos[q.name]; !ok {
			panic("aixm45to51: property " + q.name + " not in the schema order")
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return pos[list[i].name] < pos[list[j].name] })
	var b strings.Builder
	for _, q := range list {
		b.WriteString(q.xml)
	}
	return b.String()
}

// object renders <aixm:tag gml:id=…>properties</aixm:tag>.
func (p *props) object(tag, idPrefix string, order []string, attrs ...string) string {
	var b strings.Builder
	b.WriteString(`<aixm:` + tag + ` gml:id="` + p.f.id(idPrefix) + `"`)
	for i := 0; i+1 < len(attrs); i += 2 {
		fmt.Fprintf(&b, ` %s="%s"`, attrs[i], esc(attrs[i+1]))
	}
	b.WriteString(">" + p.render(order) + "</aixm:" + tag + ">")
	return b.String()
}

// timeSlice describes the time slice every feature gets.
type timeSlice struct {
	begin    string // gml:beginPosition of validTime and featureLifetime
	sequence uint32
}

// bytes renders the feature as a standalone, namespace-complete element.
func (f *feature) bytes(ts timeSlice, order []string) []byte {
	var b strings.Builder
	period := func(prefix string) string {
		return `<gml:TimePeriod gml:id="` + prefix + `-` + f.uuid + `"><gml:beginPosition>` + esc(ts.begin) +
			`</gml:beginPosition><gml:endPosition indeterminatePosition="unknown"/></gml:TimePeriod>`
	}
	fmt.Fprintf(&b, `<aixm:%s xmlns:aixm="%s" xmlns:gml="%s" xmlns:xlink="%s" gml:id="%s-%s">`, f.name, nsAIXM, nsGML, nsXLink, f.name, f.uuid)
	b.WriteString(`<gml:identifier codeSpace="urn:uuid:">` + f.uuid + `</gml:identifier>`)
	fmt.Fprintf(&b, `<aixm:timeSlice><aixm:%sTimeSlice gml:id="ts-%s">`, f.name, f.uuid)
	b.WriteString(`<gml:validTime>` + period("vt") + `</gml:validTime>`)
	b.WriteString(`<aixm:interpretation>BASELINE</aixm:interpretation>`)
	fmt.Fprintf(&b, `<aixm:sequenceNumber>%d</aixm:sequenceNumber><aixm:correctionNumber>0</aixm:correctionNumber>`, ts.sequence)
	b.WriteString(`<aixm:featureLifetime>` + period("lt") + `</aixm:featureLifetime>`)
	b.WriteString(f.props.render(order))
	fmt.Fprintf(&b, `</aixm:%sTimeSlice></aixm:timeSlice></aixm:%s>`, f.name, f.name)
	return []byte(b.String())
}

// Schema order of the properties written, from AIXM_Features.xsd (5.1.1).
var (
	orderOrg  = []string{"name", "designator", "type", "military", "contact", "relatedOrganisationAuthority", "annotation"}
	orderUnit = []string{"name", "type", "compliantICAO", "designator", "military", "position", "airportLocation", "ownerOrganisation", "contact", "relatedUnit", "availability", "annotation"}
	orderDpn  = []string{"designator", "type", "name", "location", "aimingPoint", "airportHeliport", "runwayPoint", "annotation"}
	// NavaidEquipmentPropertyGroup followed by the equipment's own group.
	orderEquipment  = []string{"designator", "name", "emissionClass", "mobile", "magneticVariation", "magneticVariationAccuracy", "dateMagneticVariation", "flightChecked", "location", "authority", "monitoring", "availability", "annotation"}
	orderVOR        = append(append([]string(nil), orderEquipment...), "type", "frequency", "zeroBearingDirection", "declination")
	orderDME        = append(append([]string(nil), orderEquipment...), "type", "channel", "ghostFrequency", "displace")
	orderNDB        = append(append([]string(nil), orderEquipment...), "frequency", "class", "emissionBand")
	orderTACAN      = append(append([]string(nil), orderEquipment...), "channel", "declination")
	orderNavaid     = []string{"type", "designator", "name", "flightChecked", "purpose", "signalPerformance", "courseQuality", "integrityLevel", "touchDownLiftOff", "navaidEquipment", "location", "runwayDirection", "servedAirport", "availability", "annotation"}
	orderVS         = []string{"name", "type", "lighted", "markingICAOStandard", "group", "length", "width", "radius", "lightingICAOStandard", "synchronisedLighting", "marker", "part", "hostedPassengerService", "supportedGroundLight", "hostedNavaidEquipment", "hostedSpecialNavStation", "hostedUnit", "hostedOrganisation", "supportedService", "annotation", "lightingAvailability"}
	orderVSPart     = []string{"verticalExtent", "verticalExtentAccuracy", "type", "constructionStatus", "markingPattern", "markingFirstColour", "markingSecondColour", "mobile", "frangible", "visibleMaterial", "designator", "horizontalProjection_surfaceExtent", "horizontalProjection_linearExtent", "horizontalProjection_location", "lighting"}
	orderAhp        = []string{"designator", "name", "locationIndicatorICAO", "designatorIATA", "type", "certifiedICAO", "privateUse", "controlType", "fieldElevation", "fieldElevationAccuracy", "verticalDatum", "magneticVariation", "magneticVariationAccuracy", "dateMagneticVariation", "magneticVariationChange", "referenceTemperature", "altimeterCheckLocation", "secondaryPowerSupply", "windDirectionIndicator", "landingDirectionIndicator", "transitionAltitude", "transitionLevel", "lowestTemperature", "abandoned", "certificationDate", "certificationExpirationDate", "contaminant", "servedCity", "responsibleOrganisation", "ARP", "aviationBoundary", "altimeterSource", "contact", "availability", "annotation"}
	orderCity       = []string{"name", "annotation"}
	orderAhpResp    = []string{"role", "theOrganisationAuthority"}
	orderAuthority  = []string{"type", "annotation", "theOrganisationAuthority"}
	orderComponent  = []string{"collocationGroup", "markerPosition", "providesNavigableLocation", "annotation", "theNavaidEquipment"}
	orderRunway     = []string{"designator", "type", "nominalLength", "lengthAccuracy", "nominalWidth", "widthAccuracy", "widthShoulder", "lengthStrip", "widthStrip", "lengthOffset", "widthOffset", "abandoned", "surfaceProperties", "associatedAirportHeliport", "overallContaminant", "annotation", "areaContaminant"}
	orderSurface    = []string{"composition", "preparation", "surfaceCondition", "classPCN", "pavementTypePCN", "pavementSubgradePCN", "maxTyrePressurePCN", "evaluationMethodPCN", "classLCN", "weightSIWL", "tyrePressureSIWL", "weightAUW", "annotation"}
	orderRdn        = []string{"designator", "trueBearing", "trueBearingAccuracy", "magneticBearing", "patternVFR", "slopeTDZ", "elevationTDZ", "elevationTDZAccuracy", "approachMarkingType", "approachMarkingCondition", "classLightingJAR", "precisionApproachGuidance", "usedRunway", "startingElement", "annotation", "availability"}
	orderAirspace   = []string{"type", "designator", "localType", "name", "designatorICAO", "controlType", "upperLowerSeparation", "class", "protectedRoute", "geometryComponent", "activation", "annotation"}
	orderGeomComp   = []string{"operation", "operationSequence", "annotation", "theAirspaceVolume"}
	orderVolume     = []string{"upperLimit", "upperLimitReference", "maximumLimit", "maximumLimitReference", "lowerLimit", "lowerLimitReference", "minimumLimit", "minimumLimitReference", "width", "horizontalProjection", "centreline", "contributorAirspace", "annotation"}
	orderDependency = []string{"dependency", "annotation", "theAirspace"}
	orderLayerClass = []string{"classification", "associatedLevels"}
	// PropertiesWithSchedulePropertyGroup followed by the activation's own
	// group.
	orderActivation = []string{"timeInterval", "annotation", "specialDateAuthority", "activity", "status", "levels", "user", "aircraft"}
	orderTimesheet  = []string{"timeReference", "startDate", "endDate", "day", "dayTil", "startTime", "startEvent", "startTimeRelativeEvent", "startEventInterpretation", "endTime", "endEvent", "endTimeRelativeEvent", "endEventInterpretation", "daylightSavingAdjust", "excluded", "annotation"}
	orderGeoBorder  = []string{"name", "type", "border", "annotation"}
	// gml:pos first, then Point and ElevatedPoint properties.
	orderPoint = []string{"pos", "horizontalAccuracy", "annotation", "elevation", "geoidUndulation", "verticalDatum", "verticalAccuracy"}
)
