// Package testconv holds a converter used only by goaixm's tests to
// exercise the conversion framework end to end.
//
// Relabel "converts" an AIXM 5.x message into another 5.x version by
// rewriting the AIXM namespaces of every element and attribute; nothing else
// changes.  That is not a real version conversion (versions differ in more
// than their namespace), only a deterministic, fully accounted stand-in.
package testconv

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/version"
	"github.com/davidkohl/goaixm/xmlnode"
)

// Relabel is the namespace-rewriting test converter.
type Relabel struct {
	from, to version.Version
	id       convert.ConverterID
	// LoseLeaf, when true, leaves one leaf per member without an outcome,
	// breaking the accounting on purpose.
	LoseLeaf bool
	// Passes is the number of times Convert reads the source (default 1);
	// 2 imitates a converter with an index pass.
	Passes int
}

// New returns a Relabel converter from one 5.x version to another.
func New(from, to version.Version) *Relabel {
	return &Relabel{from: from, to: to, id: convert.NewID("goaixm/testconv", "0.1.0", []byte("relabel "+from.String()+">"+to.String()))}
}

func (c *Relabel) From() version.Version   { return c.from }
func (c *Relabel) To() version.Version     { return c.to }
func (c *Relabel) ID() convert.ConverterID { return c.id }

func (c *Relabel) relabel(ns string) string {
	from := version.NamespaceURI(c.from)
	if ns == from || strings.HasPrefix(ns, from+"/") {
		return version.NamespaceURI(c.to) + strings.TrimPrefix(ns, from)
	}
	return ns
}

func (c *Relabel) rewrite(n *xmlnode.Node) {
	n.Name.Space = c.relabel(n.Name.Space)
	for i := range n.Attrs {
		n.Attrs[i].Name.Space = c.relabel(n.Attrs[i].Name.Space)
	}
	for _, ch := range n.Children {
		c.rewrite(ch)
	}
}

// Convert implements convert.Converter.
func (c *Relabel) Convert(ctx context.Context, src convert.Source, dst convert.Sink) (*convert.Report, error) {
	passes := c.Passes
	if passes < 1 {
		passes = 1
	}
	for p := 1; p < passes; p++ { // index passes: read and discard
		if err := c.walk(ctx, src, func(xml.StartElement) error { return nil }, func(int, *xmlnode.Node) error { return nil }); err != nil {
			return nil, err
		}
	}
	rep := convert.NewReport()
	begin := func(root xml.StartElement) error {
		h := convert.Header{Version: c.to, Root: root.Name.Local}
		for _, a := range root.Attr {
			if a.Name.Local == "id" {
				h.GMLID = a.Value
			}
		}
		return dst.Begin(h)
	}
	member := func(index int, n *xmlnode.Node) error {
		kind := n.Local()
		k := rep.Kind(kind)
		k.In++
		k.Leaves += convert.CountLeaves(n)
		lost := false
		convert.WalkLeaves(n, func(l convert.Leaf) {
			if c.LoseLeaf && !lost {
				lost = true
				return
			}
			k.Record(l.Path, convert.Mapped, "relabel", l.Path, &convert.Sample{Value: l.Value})
		})
		key := n.TextAt("identifier")
		if key == "" {
			rep.AddIssue(convert.Issue{Code: "no_identifier", Severity: convert.Warning, Kind: kind, Message: "member without gml:identifier"})
		}
		c.rewrite(n)
		var buf bytes.Buffer
		if err := n.Encode(&buf); err != nil {
			return err
		}
		k.Out++
		return dst.Put(convert.Record{Kind: kind, XML: buf.Bytes(), Provenance: convert.Provenance{
			SourceVersion: c.from, SourceKind: kind, SourceKey: key, SourceIndex: []int{index},
		}})
	}
	if err := c.walk(ctx, src, begin, member); err != nil {
		return nil, err
	}
	if err := dst.End(); err != nil {
		return nil, err
	}
	return rep, nil
}

// walk reads the message and calls root for the root element and member for
// each hasMember child.
func (c *Relabel) walk(ctx context.Context, src convert.Source, root func(xml.StartElement) error, member func(int, *xmlnode.Node) error) error {
	rc, err := src.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	d := xml.NewDecoder(rc)
	depth, index, inMember := 0, -1, false
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("testconv: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case depth == 0:
				if err := root(t); err != nil {
					return err
				}
				depth++
			case inMember:
				n, err := xmlnode.FromDecoder(d, t)
				if err != nil {
					return err
				}
				if err := member(index, n); err != nil {
					return err
				}
			case depth == 1 && t.Name.Local == "hasMember":
				inMember = true
				index++
				depth++
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			depth--
			inMember = false
		}
	}
}
