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
	return StreamMessageWithOptions(ctx, r, StreamOptions{}, func(m Member) error {
		return handler(m.LocalName, m.Feature)
	})
}

// StreamOptions configures StreamMessageWithOptions.
type StreamOptions struct {
	// Registry is the feature type registry to use.  Nil uses DefaultRegistry.
	Registry *Registry
	// Fallback selects what happens to members with no registered type.
	Fallback Fallback
	// KeepRaw retains each member's XML in Member.Raw.
	KeepRaw bool
	// OnWarning, when set, is called for every member that could not be
	// decoded.  Returning an error aborts streaming.  When nil such members
	// are skipped silently.
	OnWarning func(ParseWarning) error
}

// MemberHandler is called for each decoded member in document order.
type MemberHandler func(Member) error

// StreamMessageWithOptions is StreamMessage with an explicit registry,
// fallback, raw retention and warning reporting.  Each member is delivered
// with its overlay extensions (see Registry.Extend).
func StreamMessageWithOptions(ctx context.Context, r io.Reader, opts StreamOptions, handler MemberHandler) error {
	cfg := decodeConfig{reg: opts.Registry, fallback: opts.Fallback, keepRaw: opts.KeepRaw}
	if cfg.reg == nil {
		cfg.reg = DefaultRegistry
	}
	warn := func(w ParseWarning) error {
		if opts.OnWarning != nil {
			return opts.OnWarning(w)
		}
		return nil
	}

	d := xml.NewDecoder(r)
	inMember := false
	idx := -1

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
			idx++
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
			if werr := warn(ParseWarning{MemberIndex: idx, LocalName: localName, Err: err}); werr != nil {
				return werr
			}
			continue
		}

		mem, err := decodeMember(cfg, idx, localName, wrapElement(raw))
		if err != nil {
			if werr := warn(ParseWarning{MemberIndex: idx, LocalName: localName, Err: err}); werr != nil {
				return werr
			}
			if mem.Feature == nil {
				continue
			}
		}

		if err := handler(mem); err != nil {
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
