package parse

import (
	"fmt"

	"gitlab.com/davidkohl/goaixm/xmlnode"
)

// Fallback controls what happens to members whose local name has no primary
// type in the registry.
type Fallback int

const (
	// FallbackSkip drops unknown members and records a warning (the default).
	FallbackSkip Fallback = iota
	// FallbackGeneric decodes unknown members into *GenericFeature, a
	// lossless element tree, so nothing in the message is lost.
	FallbackGeneric
)

// Member is one decoded message:hasMember entry.
type Member struct {
	// Index is the position of the member in the message.
	Index int
	// LocalName is the XML local name of the feature element.
	LocalName string
	// Feature is the primary decoded value: the registered type, or a
	// *GenericFeature under FallbackGeneric.
	Feature any
	// Extensions holds the overlay values registered with Registry.Extend,
	// in registration order.
	Extensions []any
	// Raw is the feature element as namespace-complete XML (wrapped in a
	// _root_ element carrying the namespace declarations).  It is only set
	// when ParseOptions.KeepRaw / StreamOptions.KeepRaw is enabled, or for
	// generic features.
	Raw []byte
}

// Node parses the member's raw XML into a lossless element tree rooted at the
// feature element.  It requires Raw (see KeepRaw).
func (m Member) Node() (*xmlnode.Node, error) {
	if g, ok := m.Feature.(*GenericFeature); ok && g.Root != nil {
		return g.Root, nil
	}
	if len(m.Raw) == 0 {
		return nil, fmt.Errorf("parse: member %d (%s) has no raw XML; enable KeepRaw", m.Index, m.LocalName)
	}
	root, err := xmlnode.Parse(m.Raw)
	if err != nil {
		return nil, err
	}
	if root.Name.Local == "_root_" && len(root.Children) > 0 {
		return root.Children[0], nil
	}
	return root, nil
}

// Extension returns the first overlay of type T decoded for the member.
func Extension[T any](m Member) (T, bool) {
	for _, e := range m.Extensions {
		if t, ok := e.(T); ok {
			return t, true
		}
	}
	var zero T
	return zero, false
}

// GenericFeature is a feature decoded without a registered Go type.  It
// implements features.Feature so it is indexed and resolvable like any other
// feature.
type GenericFeature struct {
	LocalName string
	Root      *xmlnode.Node
}

// GetGMLID returns the gml:id attribute.
func (g *GenericFeature) GetGMLID() string {
	if g.Root == nil {
		return ""
	}
	v, _ := g.Root.AttrNS(xmlnode.NSGML, "id")
	return v
}

// GetIdentifier returns the gml:identifier value.
func (g *GenericFeature) GetIdentifier() string {
	if g.Root == nil {
		return ""
	}
	return g.Root.TextAt("identifier")
}

// GetNames returns the gml:name values.
func (g *GenericFeature) GetNames() []string {
	if g.Root == nil {
		return nil
	}
	var out []string
	for _, n := range g.Root.ChildrenNamed("name") {
		out = append(out, n.Text)
	}
	return out
}

// decodeConfig is the resolved set of decode options shared by the whole
// message and streaming decoders.
type decodeConfig struct {
	reg      *Registry
	fallback Fallback
	keepRaw  bool
}

func defaultConfig() decodeConfig { return decodeConfig{reg: DefaultRegistry} }

// decodeMember decodes one wrapped member.  On an extension decode error the
// returned Member still carries the primary feature alongside the error.
func decodeMember(cfg decodeConfig, idx int, localName string, wrapped []byte) (Member, error) {
	reg := cfg.reg
	if reg == nil {
		reg = DefaultRegistry
	}
	mem := Member{Index: idx, LocalName: localName}
	if cfg.keepRaw {
		mem.Raw = wrapped
	}

	if reg.Has(localName) {
		v, err := reg.Decode(localName, wrapped)
		if err != nil {
			return Member{}, err
		}
		mem.Feature = v
	} else {
		if cfg.fallback != FallbackGeneric {
			return Member{}, fmt.Errorf("parse: unknown feature type %q", localName)
		}
		root, err := xmlnode.Parse(wrapped)
		if err != nil {
			return Member{}, fmt.Errorf("parse: generic decode %s: %w", localName, err)
		}
		if root.Name.Local == "_root_" && len(root.Children) > 0 {
			root = root.Children[0]
		}
		mem.Feature = &GenericFeature{LocalName: localName, Root: root}
		mem.Raw = wrapped
	}

	ext, err := reg.DecodeExtensions(localName, wrapped)
	mem.Extensions = ext
	if err != nil {
		return mem, err
	}
	return mem, nil
}
