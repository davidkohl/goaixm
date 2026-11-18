// Package aixm45 reads AIXM 4.5 documents.
//
// AIXM 4.5 (the AICM-based exchange format) has no namespace and no GML: a
// document is an AIXM-Snapshot or AIXM-Update root with one flat list of
// top-level elements named by three-letter entity codes (Org, Ahp, Rwy,
// Dpn, Ase, …).  Each element identifies itself with a natural key, the
// "…Uid" child (e.g. AhpUid with codeId), and refers to others through
// further …Uid elements, nested for dependent entities (RdnUid contains
// RwyUid contains AhpUid).
//
// The package offers:
//
//   - a streaming Reader that yields one top-level element at a time as a
//     lossless xmlnode tree, so documents of any size read in bounded memory;
//   - Key, the canonical natural key of a …Uid element, the basis of stable
//     identities when converting to AIXM 5.x;
//   - ParseLat/ParseLong, exact parsing of the 4.5 coordinate formats;
//   - typed views (Org, Uni, Dpn, Vor, …) over the element trees of the
//     entities a converter commonly needs.  They are deliberately thin: every
//     field keeps its source node, so a converter can account for each value
//     it uses, and anything a view does not name stays reachable in the tree.
//
// There is no generated 4.5 model: the views cover what is read, and the
// trees keep everything else.
package aixm45

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"

	"github.com/davidkohl/goaixm/version"
	"github.com/davidkohl/goaixm/xmlnode"
)

// Root element names of AIXM 4.5 documents.
const (
	RootSnapshot = "AIXM-Snapshot"
	RootUpdate   = "AIXM-Update"
)

// ErrNotAIXM45 is returned when a document is not an AIXM 4.5 snapshot or
// update.
var ErrNotAIXM45 = errors.New("aixm45: not an AIXM 4.5 document")

// Header is the root element of a 4.5 document.
type Header struct {
	// Root is RootSnapshot or RootUpdate.
	Root string
	// Version is the version attribute ("4.5").
	Version string
	// Origin, Created and Effective are the attributes of the same name
	// (empty when absent): the producer, the creation time and the time the
	// data takes effect.
	Origin, Created, Effective string
	// Attrs holds every root attribute by local name (namespace
	// declarations excluded).
	Attrs map[string]string
	// Node is the root element with its attributes and without children,
	// for accounting (convert.WalkLeaves).
	Node *xmlnode.Node
}

// Element is one top-level element of a 4.5 document.
type Element struct {
	// Index is the element's position among the top-level elements,
	// counted from 0.
	Index int
	// Node is the element with everything it contains.
	Node *xmlnode.Node
}

// Kind returns the element's local name, e.g. "Ahp".
func (e Element) Kind() string { return e.Node.Local() }

// Reader reads the top-level elements of a 4.5 document one at a time.
type Reader struct {
	d     *xml.Decoder
	h     Header
	index int
	done  bool
}

// NewReader reads r up to the root element and checks that it is an AIXM
// 4.5 document (ErrNotAIXM45 otherwise).
func NewReader(r io.Reader) (*Reader, error) {
	d := xml.NewDecoder(r)
	for {
		tok, err := d.Token()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("%w: no root element", ErrNotAIXM45)
			}
			return nil, fmt.Errorf("aixm45: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		h, err := header(se)
		if err != nil {
			return nil, err
		}
		return &Reader{d: d, h: h}, nil
	}
}

func header(se xml.StartElement) (Header, error) {
	if se.Name.Space != "" || (se.Name.Local != RootSnapshot && se.Name.Local != RootUpdate) {
		return Header{}, fmt.Errorf("%w: root %q", ErrNotAIXM45, se.Name.Local)
	}
	h := Header{Root: se.Name.Local, Attrs: map[string]string{}, Node: &xmlnode.Node{Name: se.Name}}
	for _, a := range se.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		h.Node.Attrs = append(h.Node.Attrs, a)
		if _, dup := h.Attrs[a.Name.Local]; !dup {
			h.Attrs[a.Name.Local] = a.Value
		}
	}
	h.Version, h.Origin, h.Created, h.Effective = h.Attrs["version"], h.Attrs["origin"], h.Attrs["created"], h.Attrs["effective"]
	v, err := version.Parse(h.Version)
	if err != nil || v.Major != 4 || v.Minor != 5 {
		return Header{}, fmt.Errorf("%w: version %q", ErrNotAIXM45, h.Version)
	}
	return h, nil
}

// Header returns the document's root element.
func (r *Reader) Header() Header { return r.h }

// Next returns the next top-level element, or io.EOF after the last one.
// Character data and comments between elements are skipped.
func (r *Reader) Next() (Element, error) {
	if r.done {
		return Element{}, io.EOF
	}
	for {
		tok, err := r.d.Token()
		if err != nil {
			if err == io.EOF {
				err = fmt.Errorf("aixm45: %w", io.ErrUnexpectedEOF)
			} else {
				err = fmt.Errorf("aixm45: %w", err)
			}
			r.done = true
			return Element{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n, err := xmlnode.FromDecoder(r.d, t)
			if err != nil {
				r.done = true
				return Element{}, fmt.Errorf("aixm45: element %d: %w", r.index, err)
			}
			e := Element{Index: r.index, Node: n}
			r.index++
			return e, nil
		case xml.EndElement:
			r.done = true
			return Element{}, io.EOF
		}
	}
}

// Stream reads the document from r and calls fn for every top-level
// element in document order.  It returns the header; an error from fn or a
// cancelled ctx stops the reading and is returned.
func Stream(ctx context.Context, r io.Reader, fn func(Element) error) (Header, error) {
	rd, err := NewReader(r)
	if err != nil {
		return Header{}, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return rd.h, err
		}
		e, err := rd.Next()
		if err == io.EOF {
			return rd.h, nil
		}
		if err != nil {
			return rd.h, err
		}
		if err := fn(e); err != nil {
			return rd.h, err
		}
	}
}
