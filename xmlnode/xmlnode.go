// Package xmlnode provides a small, lossless, schema-free XML element tree.
//
// goaixm decodes features into generated Go structs, which only carry the
// elements those structs declare.  A Node keeps everything — every element,
// attribute and text value — so callers can read data the typed model does
// not (yet) cover, store a feature losslessly, or decode a subtree into their
// own struct with Decode.
package xmlnode

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Well-known namespaces, handy for Attr lookups.
const (
	NSGML   = "http://www.opengis.net/gml/3.2"
	NSAIXM  = "http://www.aixm.aero/schema/5.1.1"
	NSXLink = "http://www.w3.org/1999/xlink"
	NSXSI   = "http://www.w3.org/2001/XMLSchema-instance"
)

// Node is one XML element.
type Node struct {
	Name     xml.Name
	Attrs    []xml.Attr
	Text     string // trimmed character data directly inside this element
	Children []*Node
}

// Parse builds a tree from the first element found in data.
func Parse(data []byte) (*Node, error) {
	return ParseReader(bytes.NewReader(data))
}

// ParseReader builds a tree from the first element read from r.
func ParseReader(r io.Reader) (*Node, error) {
	d := xml.NewDecoder(r)
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("xmlnode: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			return FromDecoder(d, se)
		}
	}
}

// FromDecoder builds a tree for the element whose start token was just read.
func FromDecoder(d *xml.Decoder, start xml.StartElement) (*Node, error) {
	root := newNode(start)
	stack := []*Node{root}
	text := make([][]byte, 1)
	for len(stack) > 0 {
		tok, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("xmlnode: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := newNode(t)
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, n)
			stack = append(stack, n)
			text = append(text, nil)
		case xml.CharData:
			text[len(text)-1] = append(text[len(text)-1], t...)
		case xml.EndElement:
			n := stack[len(stack)-1]
			n.Text = strings.TrimSpace(string(text[len(text)-1]))
			stack = stack[:len(stack)-1]
			text = text[:len(text)-1]
		}
	}
	return root, nil
}

func newNode(se xml.StartElement) *Node {
	n := &Node{Name: se.Name}
	for _, a := range se.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue // namespace declarations are resolved already
		}
		n.Attrs = append(n.Attrs, a)
	}
	return n
}

// Local returns the element's local name.
func (n *Node) Local() string { return n.Name.Local }

// Attr returns the value of the attribute with the given local name.  When
// several attributes share a local name, the first wins; use AttrNS to be
// specific.
func (n *Node) Attr(local string) (string, bool) {
	for _, a := range n.Attrs {
		if a.Name.Local == local {
			return a.Value, true
		}
	}
	return "", false
}

// AttrNS returns the value of the attribute in namespace space.
func (n *Node) AttrNS(space, local string) (string, bool) {
	for _, a := range n.Attrs {
		if a.Name.Space == space && a.Name.Local == local {
			return a.Value, true
		}
	}
	return "", false
}

// Href returns the xlink:href attribute, if present.
func (n *Node) Href() string {
	v, _ := n.AttrNS(NSXLink, "href")
	return v
}

// IsNil reports whether the element is xsi:nil="true".
func (n *Node) IsNil() bool {
	v, _ := n.AttrNS(NSXSI, "nil")
	return v == "true"
}

// Child returns the first direct child with the given local name.
func (n *Node) Child(local string) *Node {
	for _, c := range n.Children {
		if c.Name.Local == local {
			return c
		}
	}
	return nil
}

// ChildrenNamed returns all direct children with the given local name.
func (n *Node) ChildrenNamed(local string) []*Node {
	var out []*Node
	for _, c := range n.Children {
		if c.Name.Local == local {
			out = append(out, c)
		}
	}
	return out
}

// Find follows a slash-separated path of local names from n, taking the first
// match at each step.  "*" matches any element.  It returns nil when the path
// does not exist.
func (n *Node) Find(path string) *Node {
	all := n.FindAll(path)
	if len(all) == 0 {
		return nil
	}
	return all[0]
}

// FindAll follows a slash-separated path of local names from n and returns
// every match.  "*" matches any element.
func (n *Node) FindAll(path string) []*Node {
	cur := []*Node{n}
	for _, step := range strings.Split(strings.Trim(path, "/"), "/") {
		if step == "" {
			continue
		}
		var next []*Node
		for _, c := range cur {
			for _, ch := range c.Children {
				if step == "*" || ch.Name.Local == step {
					next = append(next, ch)
				}
			}
		}
		cur = next
		if len(cur) == 0 {
			return nil
		}
	}
	return cur
}

// TextAt returns the trimmed text of the element at path, or "".
func (n *Node) TextAt(path string) string {
	if c := n.Find(path); c != nil {
		return c.Text
	}
	return ""
}

// Walk calls fn for n and every descendant in document order.  Returning
// false from fn skips that node's children.
func (n *Node) Walk(fn func(*Node) bool) {
	if !fn(n) {
		return
	}
	for _, c := range n.Children {
		c.Walk(fn)
	}
}

// Encode writes the subtree back to XML.  Namespaces are emitted as full
// xmlns declarations by encoding/xml, so the output is self-contained and can
// be fed to xml.Unmarshal.
func (n *Node) Encode(w io.Writer) error {
	enc := xml.NewEncoder(w)
	if err := n.encode(enc); err != nil {
		return err
	}
	return enc.Flush()
}

func (n *Node) encode(enc *xml.Encoder) error {
	start := xml.StartElement{Name: n.Name, Attr: n.Attrs}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if n.Text != "" {
		if err := enc.EncodeToken(xml.CharData(n.Text)); err != nil {
			return err
		}
	}
	for _, c := range n.Children {
		if err := c.encode(enc); err != nil {
			return err
		}
	}
	return enc.EncodeToken(start.End())
}

// Decode unmarshals the subtree rooted at n into v, which may be any struct
// with encoding/xml tags — typically a goaixm type such as
// *features.CurveType, or a caller-defined schema.
func (n *Node) Decode(v any) error {
	var buf bytes.Buffer
	if err := n.Encode(&buf); err != nil {
		return err
	}
	return xml.Unmarshal(buf.Bytes(), v)
}

// jsonNode is the JSON shape of a Node.
type jsonNode struct {
	Name     string            `json:"name"`
	NS       string            `json:"ns,omitempty"`
	Attrs    map[string]string `json:"attrs,omitempty"`
	Text     string            `json:"text,omitempty"`
	Children []*Node           `json:"children,omitempty"`
}

// MarshalJSON renders the node as {"name","ns","attrs","text","children"}.
// Attribute keys are "local" for unqualified attributes and "space local"
// otherwise, so the encoding is lossless.
func (n *Node) MarshalJSON() ([]byte, error) {
	j := jsonNode{Name: n.Name.Local, NS: n.Name.Space, Text: n.Text, Children: n.Children}
	if len(n.Attrs) > 0 {
		j.Attrs = make(map[string]string, len(n.Attrs))
		for _, a := range n.Attrs {
			k := a.Name.Local
			if a.Name.Space != "" {
				k = a.Name.Space + " " + a.Name.Local
			}
			j.Attrs[k] = a.Value
		}
	}
	return json.Marshal(j)
}

// UnmarshalJSON is the inverse of MarshalJSON.
func (n *Node) UnmarshalJSON(b []byte) error {
	var j jsonNode
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	n.Name = xml.Name{Space: j.NS, Local: j.Name}
	n.Text = j.Text
	n.Children = j.Children
	n.Attrs = nil
	for k, v := range j.Attrs {
		name := xml.Name{Local: k}
		if i := strings.LastIndexByte(k, ' '); i >= 0 {
			name = xml.Name{Space: k[:i], Local: k[i+1:]}
		}
		n.Attrs = append(n.Attrs, xml.Attr{Name: name, Value: v})
	}
	return nil
}
