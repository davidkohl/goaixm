package goaixm

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"

	"github.com/davidkohl/goaixm/convert"
	"github.com/davidkohl/goaixm/version"
)

// checkNative refuses a root that version.Detect recognises as another AIXM
// version than 5.1.1 (or as AIXM 4.x without a usable version).
func checkNative(det version.Detected, err error) error {
	switch {
	case err == nil && det.Version != version.AIXM511:
		return fmt.Errorf("%w: AIXM %v %s (no converter in Options.Converters)", ErrUnsupportedVersion, det.Version, det.Root)
	case err != nil && det.Namespace == "" && version.IsAIXM4Root(det.Root):
		return fmt.Errorf("%w: %v", ErrUnsupportedVersion, err)
	}
	return nil
}

func needsConversion(det version.Detected, err error) bool {
	return err == nil && det.Version != version.AIXM511
}

// peek detects the version of the document in r and returns a reader that
// yields the whole document again.
func peek(r io.Reader) (io.Reader, version.Detected, error) {
	var head bytes.Buffer
	det, err := version.Detect(io.TeeReader(r, &head))
	return io.MultiReader(&head, r), det, err
}

func (root rootInfo) setOn(msg *Message) {
	if msg != nil {
		msg.Kind, msg.GMLID, msg.Source = root.kind, root.gmlID, root.source
	}
}

// readReader reads a one-shot reader: natively, or (with Converters, for
// another version) spooled to a temporary file and converted.
func readReader(ctx context.Context, r io.Reader, opts Options, msg *Message, fn func(Member) error) error {
	if opts.MaxBytes > 0 {
		r = &limitReader{r: r, n: opts.MaxBytes}
	}
	if opts.Converters != nil {
		full, det, derr := peek(r)
		r = full
		if needsConversion(det, derr) {
			src, cleanup, err := convert.Spool(r, opts.SpoolDir, 0)
			if err != nil {
				return err
			}
			defer cleanup()
			return convertSource(ctx, src, det, opts, msg, fn)
		}
	}
	root, err := stream(ctx, r, opts, fn)
	if err != nil {
		return err
	}
	root.setOn(msg)
	return nil
}

// readSource reads a re-openable source: natively, or converted directly
// from src without a temporary copy.
func readSource(ctx context.Context, src convert.Source, opts Options, msg *Message, fn func(Member) error) error {
	rc, err := src.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	var r io.Reader = rc
	if opts.MaxBytes > 0 {
		r = &limitReader{r: r, n: opts.MaxBytes}
	}
	if opts.Converters != nil {
		full, det, derr := peek(r)
		r = full
		if needsConversion(det, derr) {
			rc.Close()
			if opts.MaxBytes > 0 {
				src = limitedSource{src, opts.MaxBytes}
			}
			return convertSource(ctx, src, det, opts, msg, fn)
		}
	}
	root, err := stream(ctx, r, opts, fn)
	if err != nil {
		return err
	}
	root.setOn(msg)
	return nil
}

// convertSource converts src to 5.1.1 with the registered converters and
// decodes the records.
func convertSource(ctx context.Context, src convert.Source, det version.Detected, opts Options, msg *Message, fn func(Member) error) error {
	c, err := opts.Converters.Converter(det.Version, version.AIXM511)
	if err != nil {
		return fmt.Errorf("%w: AIXM %v %s: %v", ErrUnsupportedVersion, det.Version, det.Root, err)
	}
	sink := newMemberSink(opts, fn)
	rep, err := convert.Run(ctx, c, src, sink)
	if err != nil {
		return err
	}
	if msg != nil {
		msg.Kind, msg.GMLID, msg.Source = sink.root(), sink.header.GMLID, det
	}
	if opts.OnConverted != nil {
		return opts.OnConverted(rep)
	}
	return nil
}

// limitedSource applies Options.MaxBytes to every read of a source.
type limitedSource struct {
	convert.Source
	n int64
}

func (l limitedSource) Open() (io.ReadCloser, error) {
	rc, err := l.Source.Open()
	if err != nil {
		return nil, err
	}
	return struct {
		io.Reader
		io.Closer
	}{&limitReader{r: rc, n: l.n}, rc}, nil
}

// MemberSink returns a convert.Sink that decodes converted AIXM 5.1.1
// records into Members with the registry, fallback and warning handling of
// opts and calls fn for each.  Raw is always set and Provenance carries the
// record's provenance; Index counts the records.  Begin fails for another
// version than 5.1.1.  Use it with convert.Run or convert.Tee, e.g. to
// archive the converted document while decoding it.
func MemberSink(opts Options, fn func(Member) error) convert.Sink { return newMemberSink(opts, fn) }

type memberSink struct {
	opts   Options
	reg    *Registry
	warn   func(Warning) error
	fn     func(Member) error
	index  int
	header convert.Header
}

func newMemberSink(opts Options, fn func(Member) error) *memberSink {
	opts.KeepRaw = true
	return &memberSink{opts: opts, reg: registryOf(opts), warn: warner(opts), fn: fn}
}

func (s *memberSink) root() string {
	if s.header.Root == "" {
		return "AIXMBasicMessage"
	}
	return s.header.Root
}

func (s *memberSink) Begin(h convert.Header) error {
	if h.Version != version.AIXM511 {
		return fmt.Errorf("%w: MemberSink decodes AIXM 5.1.1, not %v", ErrUnsupportedVersion, h.Version)
	}
	s.header = h
	return nil
}

func (s *memberSink) Put(r convert.Record) error {
	index := s.index
	s.index++
	kind, err := elementName(r.XML)
	if err != nil {
		return s.warn(Warning{Index: index, Kind: r.Kind, Err: err})
	}
	if r.Kind != "" && r.Kind != kind {
		return fmt.Errorf("goaixm: record %d: kind %q but element %q", index, r.Kind, kind)
	}
	mem, err := decodeMember(s.reg, s.opts, index, kind, r.XML)
	if err != nil {
		if werr := s.warn(Warning{Index: index, Kind: kind, Err: err}); werr != nil {
			return werr
		}
		if mem.Feature == nil {
			return nil
		}
	}
	p := r.Provenance
	mem.Provenance = &p
	return s.fn(mem)
}

func (s *memberSink) End() error { return nil }

// elementName returns the local name of the first element in b.
func elementName(b []byte) (string, error) {
	d := xml.NewDecoder(bytes.NewReader(b))
	for {
		tok, err := d.Token()
		if err != nil {
			return "", fmt.Errorf("goaixm: record: %w", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Local, nil
		}
	}
}
