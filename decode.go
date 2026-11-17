package goaixm

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"

	"github.com/davidkohl/goaixm/version"
	"github.com/davidkohl/goaixm/xmlnode"
)

var errNoRaw = errors.New("goaixm: member has no raw XML; set Options.KeepRaw")

type rootInfo struct {
	kind   string
	gmlID  string
	source version.Detected
}

// warner returns the warning handler of opts: Strict turns a warning into
// an error, otherwise OnWarning decides.
func warner(opts Options) func(Warning) error {
	return func(w Warning) error {
		if opts.Strict {
			return w
		}
		if opts.OnWarning != nil {
			return opts.OnWarning(w)
		}
		return nil
	}
}

func registryOf(opts Options) *Registry {
	if opts.Registry == nil {
		return defaultRegistry
	}
	return opts.Registry
}

// stream is the single decoder of native 5.1.1 messages behind Parse and
// Stream.  It walks the document token by token, captures each hasMember
// child as a standalone, namespace-complete element and decodes it with the
// registry.  A root of another AIXM version fails with
// ErrUnsupportedVersion.  The caller applies Options.MaxBytes.
func stream(ctx context.Context, r io.Reader, opts Options, fn func(Member) error) (rootInfo, error) {
	reg := registryOf(opts)
	warn := warner(opts)

	var root rootInfo
	d := xml.NewDecoder(r)
	inMember := false
	index := -1
	for {
		if err := ctx.Err(); err != nil {
			return root, err
		}
		tok, err := d.Token()
		if err == io.EOF {
			if root.kind == "" {
				return root, errors.New("goaixm: no message element")
			}
			return root, nil
		}
		if err != nil {
			if errors.Is(err, ErrTooLarge) {
				return root, err
			}
			return root, fmt.Errorf("goaixm: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch {
		case root.kind == "":
			root.kind = se.Name.Local
			for _, a := range se.Attr {
				if a.Name.Space == xmlnode.NSGML && a.Name.Local == "id" {
					root.gmlID = a.Value
				}
			}
			det, derr := version.FromStart(se)
			root.source = det
			if err := checkNative(det, derr); err != nil {
				return root, err
			}
			continue
		case se.Name.Local == "hasMember":
			inMember = true
			index++
			continue
		case !inMember:
			// Message metadata, bounding boxes and the like.
			if err := d.Skip(); err != nil {
				return root, fmt.Errorf("goaixm: %w", err)
			}
			continue
		}
		inMember = false

		raw, err := captureElement(d, se)
		if err != nil {
			return root, err
		}
		mem, err := decodeMember(reg, opts, index, se.Name.Local, raw)
		if err != nil {
			if werr := warn(Warning{Index: index, Kind: se.Name.Local, Err: err}); werr != nil {
				return root, werr
			}
			if mem.Feature == nil {
				continue
			}
		}
		if err := fn(mem); err != nil {
			return root, err
		}
	}
}

// decodeMember decodes one member.  When only an overlay fails, the member
// is returned together with the error.
func decodeMember(reg *Registry, opts Options, index int, kind string, raw []byte) (Member, error) {
	mem := Member{Index: index, Kind: kind}
	if opts.KeepRaw {
		mem.Raw = raw
	}
	factory, overlays := reg.factory(kind)
	if factory == nil {
		if opts.Fallback != FallbackGeneric {
			return Member{}, fmt.Errorf("%w %q", ErrUnknownFeature, kind)
		}
		root, err := xmlnode.Parse(raw)
		if err != nil {
			return Member{}, fmt.Errorf("goaixm: generic decode %s: %w", kind, err)
		}
		mem.Feature = newGenericFeature(root)
		mem.Raw = raw
	} else {
		f := factory()
		if err := xml.Unmarshal(raw, f); err != nil {
			return Member{}, fmt.Errorf("goaixm: decode %s: %w", kind, err)
		}
		mem.Feature = f
	}
	for _, overlay := range overlays {
		v := overlay()
		if err := xml.Unmarshal(raw, v); err != nil {
			return mem, fmt.Errorf("goaixm: overlay %T for %s: %w", v, kind, err)
		}
		mem.Extensions = append(mem.Extensions, v)
	}
	return mem, nil
}

// captureElement re-encodes the element whose start token was just read,
// with full namespace declarations, so it can be decoded on its own.
func captureElement(d *xml.Decoder, start xml.StartElement) ([]byte, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := enc.EncodeToken(start); err != nil {
		return nil, err
	}
	for depth := 1; depth > 0; {
		tok, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("goaixm: capture %s: %w", start.Name.Local, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			err = enc.EncodeToken(t)
		case xml.EndElement:
			depth--
			err = enc.EncodeToken(t)
		case xml.CharData:
			err = enc.EncodeToken(t)
		}
		if err != nil {
			return nil, err
		}
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// limitReader fails with ErrTooLarge once more than n bytes were read.
type limitReader struct {
	r io.Reader
	n int64
}

func (l *limitReader) Read(p []byte) (int, error) {
	if l.n < 0 {
		return 0, ErrTooLarge
	}
	if int64(len(p)) > l.n+1 {
		p = p[:l.n+1]
	}
	n, err := l.r.Read(p)
	l.n -= int64(n)
	if l.n < 0 {
		return n, ErrTooLarge
	}
	return n, err
}
