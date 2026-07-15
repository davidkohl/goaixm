package parse

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
)

// StreamHandler is called for each successfully decoded feature in document order.
// Return a non-nil error to abort streaming.
type StreamHandler func(localName string, feature any) error

// StreamMessage decodes an AIXMBasicMessage from r, calling handler for each
// hasMember element as it is decoded.  The entire document is never held in
// memory at once — peak memory is proportional to the size of the largest
// single hasMember element.
//
// XLink resolution across features is NOT available in streaming mode because
// the full byID index is never built.  Use ParseMessage for that.
//
// The context ctx is checked before each member is decoded; cancellation
// aborts streaming and returns ctx.Err().
func StreamMessage(ctx context.Context, r io.Reader, handler StreamHandler) error {
	d := xml.NewDecoder(r)
	inMember := false

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("parse: stream token: %w", err)
		}

		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}

		// Detect hasMember elements (namespace-qualified or bare).
		if se.Name.Local == "hasMember" {
			inMember = true
			continue
		}

		if !inMember {
			continue
		}

		// We are inside a hasMember — this start element is the feature root.
		localName := se.Name.Local
		inMember = false

		// Capture the raw XML of this element (including its own start tag).
		raw, err := captureElement(d, se)
		if err != nil {
			// Non-fatal: record and skip.
			continue
		}

		wrapped := wrapElement(raw)
		v, err := DefaultRegistry.Decode(localName, wrapped)
		if err != nil {
			// Unknown type — skip silently (caller can use a registry with
			// only the types they care about to filter).
			continue
		}

		if err := handler(localName, v); err != nil {
			return err
		}
	}
}

// captureElement re-serialises the element (which has already had its start
// token consumed by the caller) back to raw XML bytes.
func captureElement(d *xml.Decoder, start xml.StartElement) ([]byte, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)

	if err := enc.EncodeToken(start); err != nil {
		return nil, err
	}

	depth := 1
	for depth > 0 {
		tok, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("parse: capture element: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}
		case xml.EndElement:
			depth--
			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}
		case xml.CharData:
			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}
		case xml.Attr:
			// Attributes are part of the start element; already handled.
		}
	}

	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
