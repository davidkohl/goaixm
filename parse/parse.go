package parse

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"

	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/message"
)

// Message wraps a decoded AIXMBasicMessage and provides typed access to features.
type Message struct {
	Raw      *message.AIXMBasicMessage
	decoded  []any            // decoded feature values in member order
	byID     map[string]any   // identifier -> feature
	byType   map[string][]any // local name -> features
	warnings []ParseWarning   // non-fatal decode problems
}

// ParseWarning records a non-fatal problem encountered during message parsing.
type ParseWarning struct {
	MemberIndex int
	LocalName   string
	Err         error
}

// Warnings returns any non-fatal problems encountered during parsing.
func (m *Message) Warnings() []ParseWarning { return m.warnings }

// ParseMessage decodes an AIXMBasicMessage from r.
func ParseMessage(r io.Reader) (*Message, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("parse: read: %w", err)
	}
	var raw message.AIXMBasicMessage
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse: unmarshal message: %w", err)
	}
	return resolveMessage(&raw)
}

// ADRMessage wraps a decoded ADRMessage.
type ADRMessage struct {
	Raw      *message.ADRMessage
	decoded  []any
	byID     map[string]any
	byType   map[string][]any
	warnings []ParseWarning
}

// Warnings returns any non-fatal problems encountered during parsing.
func (m *ADRMessage) Warnings() []ParseWarning { return m.warnings }

// ParseADRMessage decodes an ADRMessage from r.
func ParseADRMessage(r io.Reader) (*ADRMessage, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("parse: read: %w", err)
	}
	var raw message.ADRMessage
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse: unmarshal ADR message: %w", err)
	}
	decoded, byID, byType, warnings, err := resolveMembers(raw.HasMember)
	if err != nil {
		return nil, err
	}
	resolveXLinks(decoded, byID)
	return &ADRMessage{Raw: &raw, decoded: decoded, byID: byID, byType: byType, warnings: warnings}, nil
}

// resolveMessage decodes the raw members of an AIXMBasicMessage.
func resolveMessage(raw *message.AIXMBasicMessage) (*Message, error) {
	members := make([]message.BasicMessageMember, len(raw.HasMember))
	copy(members, raw.HasMember)
	// Convert to ADRMessageMember slice for reuse
	adrMembers := make([]message.ADRMessageMember, len(members))
	for i, m := range members {
		adrMembers[i] = message.ADRMessageMember{Inner: m.Inner, HrefAttr: m.HrefAttr, Owns: m.Owns}
	}
	decoded, byID, byType, warnings, err := resolveMembers(adrMembers)
	if err != nil {
		return nil, err
	}
	resolveXLinks(decoded, byID)
	return &Message{Raw: raw, decoded: decoded, byID: byID, byType: byType, warnings: warnings}, nil
}

// resolveMembers performs second-pass decode of raw member inner XML.
func resolveMembers(members []message.ADRMessageMember) ([]any, map[string]any, map[string][]any, []ParseWarning, error) {
	decoded := make([]any, 0, len(members))
	byID := make(map[string]any)
	byType := make(map[string][]any)
	var warnings []ParseWarning

	for idx, m := range members {
		if len(bytes.TrimSpace(m.Inner)) == 0 {
			continue
		}
		localName, err := peekLocalName(m.Inner)
		if err != nil {
			warnings = append(warnings, ParseWarning{MemberIndex: idx, Err: fmt.Errorf("peek local name: %w", err)})
			continue
		}
		v, err := DefaultRegistry.Decode(localName, wrapElement(m.Inner))
		if err != nil {
			warnings = append(warnings, ParseWarning{MemberIndex: idx, LocalName: localName, Err: err})
			continue
		}
		decoded = append(decoded, v)
		byType[localName] = append(byType[localName], v)
		if id := featureID(v); id != "" {
			byID[id] = v
		}
	}
	return decoded, byID, byType, warnings, nil
}

// ---- Message query methods ------------------------------------------------

// Features returns all decoded features in document order.
func (m *Message) Features() []any { return m.decoded }

// Feature looks up a feature by its GML identifier (UUID).
func (m *Message) Feature(identifier string) (any, bool) {
	v, ok := m.byID[identifier]
	return v, ok
}

// Resolve resolves an xlink:href value to a decoded feature.
// It accepts URNs ("urn:uuid:<id>"), fragment refs ("#<id>"), or bare UUIDs.
func (m *Message) Resolve(href string) (any, bool) {
	id := hrefToID(href)
	if id == "" {
		return nil, false
	}
	v, ok := m.byID[id]
	return v, ok
}

// ByType returns all features with the given XML local name.
func (m *Message) ByType(localName string) []any { return m.byType[localName] }

// Airspaces returns all Airspace features.
func (m *Message) Airspaces() []*features.AirspaceType {
	return typed[*features.AirspaceType](m.byType["Airspace"])
}

// Airports returns all AirportHeliport features.
func (m *Message) Airports() []*features.AirportHeliportType {
	return typed[*features.AirportHeliportType](m.byType["AirportHeliport"])
}

// Runways returns all Runway features.
func (m *Message) Runways() []*features.RunwayType {
	return typed[*features.RunwayType](m.byType["Runway"])
}

// Navaids returns all Navaid features.
func (m *Message) Navaids() []*features.NavaidType {
	return typed[*features.NavaidType](m.byType["Navaid"])
}

// Routes returns all Route features.
func (m *Message) Routes() []*features.RouteType {
	return typed[*features.RouteType](m.byType["Route"])
}

// DesignatedPoints returns all DesignatedPoint features.
func (m *Message) DesignatedPoints() []*features.DesignatedPointType {
	return typed[*features.DesignatedPointType](m.byType["DesignatedPoint"])
}

// RunwayDirections returns all RunwayDirection features.
func (m *Message) RunwayDirections() []*features.RunwayDirectionType {
	return typed[*features.RunwayDirectionType](m.byType["RunwayDirection"])
}

// Procedures returns all InstrumentApproachProcedure features.
func (m *Message) Procedures() []*features.InstrumentApproachProcedureType {
	return typed[*features.InstrumentApproachProcedureType](m.byType["InstrumentApproachProcedure"])
}

// Departures returns all StandardInstrumentDeparture features.
func (m *Message) Departures() []*features.StandardInstrumentDepartureType {
	return typed[*features.StandardInstrumentDepartureType](m.byType["StandardInstrumentDeparture"])
}

// Arrivals returns all StandardInstrumentArrival features.
func (m *Message) Arrivals() []*features.StandardInstrumentArrivalType {
	return typed[*features.StandardInstrumentArrivalType](m.byType["StandardInstrumentArrival"])
}

// Taxiways returns all Taxiway features.
func (m *Message) Taxiways() []*features.TaxiwayType {
	return typed[*features.TaxiwayType](m.byType["Taxiway"])
}

// Units returns all Unit (ATC unit) features.
func (m *Message) Units() []*features.UnitType {
	return typed[*features.UnitType](m.byType["Unit"])
}

// Aprons returns all Apron features.
func (m *Message) Aprons() []*features.ApronType {
	return typed[*features.ApronType](m.byType["Apron"])
}

// ---- ADRMessage query methods ---------------------------------------------

func (m *ADRMessage) Features() []any                 { return m.decoded }
func (m *ADRMessage) Feature(id string) (any, bool)   { v, ok := m.byID[id]; return v, ok }
func (m *ADRMessage) ByType(localName string) []any   { return m.byType[localName] }
func (m *ADRMessage) Airspaces() []*features.AirspaceType {
	return typed[*features.AirspaceType](m.byType["Airspace"])
}
func (m *ADRMessage) Airports() []*features.AirportHeliportType {
	return typed[*features.AirportHeliportType](m.byType["AirportHeliport"])
}

// ---- Helpers ---------------------------------------------------------------

func typed[T any](raw []any) []T {
	out := make([]T, 0, len(raw))
	for _, v := range raw {
		if t, ok := v.(T); ok {
			out = append(out, t)
		}
	}
	return out
}

// peekLocalName reads the first start element's local name from inner XML.
func peekLocalName(inner []byte) (string, error) {
	d := xml.NewDecoder(bytes.NewReader(inner))
	for {
		tok, err := d.Token()
		if err != nil {
			return "", err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Local, nil
		}
	}
}

// wrapElement wraps inner XML with namespace declarations so that xml.Unmarshal
// can resolve prefix-qualified names (e.g. "aixm:Airspace") from inner XML
// captured without the parent document's namespace bindings.
func wrapElement(inner []byte) []byte {
	const ns = `<_root_` +
		` xmlns:gml="http://www.opengis.net/gml/3.2"` +
		` xmlns:aixm="http://www.aixm.aero/schema/5.1.1"` +
		` xmlns:message="http://www.aixm.aero/schema/5.1.1/message"` +
		` xmlns:adr="http://www.aixm.aero/schema/5.1.1/extensions/EUR/ADR"` +
		` xmlns:xlink="http://www.w3.org/1999/xlink"` +
		` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"` +
		`>`
	var buf bytes.Buffer
	buf.WriteString(ns)
	buf.Write(inner)
	buf.WriteString("</_root_>")
	return buf.Bytes()
}

// featureID extracts the GML identifier from a feature via the Feature interface.
func featureID(v any) string {
	if f, ok := v.(features.Feature); ok {
		return f.GetIdentifier()
	}
	return ""
}
