package goaixm

import (
	"regexp"
	"strings"

	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/gml"
	"github.com/davidkohl/goaixm/version"
	"github.com/davidkohl/goaixm/xmlnode"
)

// Message is a parsed AIXM message with its features indexed by identifier
// and kind.
type Message struct {
	// Kind is the local name of the root element: AIXMBasicMessage or
	// ADRMessage.
	Kind string
	// GMLID is the gml:id of the message.
	GMLID string
	// Source tells the version and root of the input document; for a
	// converted document Kind and GMLID describe the converted message.
	Source version.Detected
	// Conversion is the conversion report, nil for native 5.1.1 input.
	Conversion *convert.Report

	members  []Member
	byID     map[string]int
	byKind   map[string][]int
	byValue  map[aixm.Feature]int
	warnings []Warning
}

func newMessage() *Message {
	return &Message{byID: map[string]int{}, byKind: map[string][]int{}, byValue: map[aixm.Feature]int{}}
}

func (m *Message) add(mem Member) {
	i := len(m.members)
	m.members = append(m.members, mem)
	m.byKind[mem.Kind] = append(m.byKind[mem.Kind], i)
	m.byValue[mem.Feature] = i
	if id := mem.Feature.Common().UUID(); id != "" {
		m.byID[strings.ToLower(id)] = i
	}
}

// Members returns every decoded member in document order.
func (m *Message) Members() []Member { return m.members }

// Features returns every decoded feature in document order.
func (m *Message) Features() []aixm.Feature {
	out := make([]aixm.Feature, len(m.members))
	for i, mem := range m.members {
		out[i] = mem.Feature
	}
	return out
}

// ByKind returns the features of one kind (XML local name, e.g. "Airspace").
func (m *Message) ByKind(kind string) []aixm.Feature {
	idx := m.byKind[kind]
	out := make([]aixm.Feature, len(idx))
	for i, j := range idx {
		out[i] = m.members[j].Feature
	}
	return out
}

// Feature returns the feature with the given gml:identifier (UUID).
func (m *Message) Feature(uuid string) (aixm.Feature, bool) {
	i, ok := m.byID[strings.ToLower(strings.TrimSpace(uuid))]
	if !ok {
		return nil, false
	}
	return m.members[i].Feature, true
}

// Resolve returns the feature an xlink:href points to.  It accepts
// "urn:uuid:<id>", "#<id>", xpointer expressions containing the UUID, and
// bare UUIDs.
func (m *Message) Resolve(href string) (aixm.Feature, bool) {
	id := HrefID(href)
	if id == "" {
		return nil, false
	}
	return m.Feature(id)
}

// Member returns the member record of a feature returned by this message.
func (m *Message) Member(f aixm.Feature) (Member, bool) {
	i, ok := m.byValue[f]
	if !ok {
		return Member{}, false
	}
	return m.members[i], true
}

// Extensions returns the overlay values decoded for f (see Registry.Extend).
func (m *Message) Extensions(f aixm.Feature) []any {
	mem, _ := m.Member(f)
	return mem.Extensions
}

// Warnings returns the non-fatal problems met while parsing.
func (m *Message) Warnings() []Warning { return m.warnings }

// FeaturesOf returns the features of type T in document order, e.g.
// FeaturesOf[*aixm.Airspace](msg).
func FeaturesOf[T aixm.Feature](m *Message) []T {
	var out []T
	for _, mem := range m.members {
		if t, ok := mem.Feature.(T); ok {
			out = append(out, t)
		}
	}
	return out
}

var uuidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// HrefID extracts the identifier an xlink:href points to: the part after
// "urn:uuid:" or "#", the UUID inside an xpointer expression, or a bare
// UUID.  It returns "" when href names no identifier.
func HrefID(href string) string {
	href = strings.TrimSpace(href)
	switch {
	case href == "":
		return ""
	case strings.Contains(href, "xpointer("):
		return uuidRe.FindString(href)
	case strings.HasPrefix(href, "urn:uuid:"):
		return strings.TrimPrefix(href, "urn:uuid:")
	case strings.HasPrefix(href, "#"):
		return strings.TrimPrefix(href, "#")
	case len(href) == 36 && strings.Count(href, "-") == 4:
		return href
	}
	return ""
}

// Member is one decoded message:hasMember entry.
type Member struct {
	// Index is the position of the member in the message.
	Index int
	// Kind is the XML local name of the feature, e.g. "Airspace".
	Kind string
	// Feature is the decoded feature: the registered type, or a
	// *GenericFeature under FallbackGeneric.
	Feature aixm.Feature
	// Extensions holds the overlays registered with Registry.Extend, in
	// registration order.
	Extensions []any
	// Raw is the member's feature element as standalone, namespace-complete
	// XML.  Set with Options.KeepRaw, for generic features and for
	// converted members.
	Raw []byte
	// Provenance ties a converted member to its source element(s): source
	// version, kind, key and position.  Nil for native 5.1.1 input.
	Provenance *convert.Provenance
}

// Node returns the member's feature element as a lossless tree.  It needs
// Raw (see Options.KeepRaw), except for generic features.
func (m Member) Node() (*xmlnode.Node, error) {
	if g, ok := m.Feature.(*GenericFeature); ok && g.Root != nil {
		return g.Root, nil
	}
	if len(m.Raw) == 0 {
		return nil, errNoRaw
	}
	return xmlnode.Parse(m.Raw)
}

// Extension returns the first overlay of type T decoded for m, e.g.
// Extension[*MyAirspaceSchema](member).
func Extension[T any](m Member) (T, bool) {
	for _, e := range m.Extensions {
		if t, ok := e.(T); ok {
			return t, true
		}
	}
	var zero T
	return zero, false
}

// GenericFeature is a feature decoded without a registered type, kept as a
// lossless element tree.
type GenericFeature struct {
	aixm.FeatureCommon
	// Name is the XML local name of the feature.
	Name string
	// Root is the feature element.
	Root *xmlnode.Node
}

// Kind returns the XML local name of the feature.
func (g *GenericFeature) Kind() string { return g.Name }

// TimeSlices returns nil: a generic feature's time slices are only
// available in Root.
func (g *GenericFeature) TimeSlices() []aixm.TimeSlice { return nil }

func newGenericFeature(root *xmlnode.Node) *GenericFeature {
	g := &GenericFeature{Name: root.Local(), Root: root}
	g.GMLID, _ = root.AttrNS(xmlnode.NSGML, "id")
	if id := root.Child("identifier"); id != nil {
		cs, _ := id.Attr("codeSpace")
		g.Identifier = &gml.Code{Value: id.Text, CodeSpace: cs}
	}
	for _, n := range root.ChildrenNamed("name") {
		g.Names = append(g.Names, gml.Code{Value: n.Text})
	}
	return g
}
