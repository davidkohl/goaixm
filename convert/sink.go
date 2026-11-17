package convert

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"hash"
	"io"
	"sort"
	"strings"

	"github.com/davidkohl/goaixm/version"
)

// Header describes the target document.
type Header struct {
	// Version of the target document.
	Version version.Version
	// Root is the local name of the target root element; "" means
	// AIXMBasicMessage.
	Root string
	// GMLID is the gml:id of the message, deterministic for equal input.
	GMLID string
	// Attrs holds carried-over source root attributes (e.g. origin,
	// created, effective), for information.
	Attrs map[string]string
}

// Record is one member element in the target version's schema: standalone,
// namespace-complete XML of the feature (the child of message:hasMember),
// without an XML declaration.
type Record struct {
	// Kind is the target local name of the feature, e.g. "Airspace".
	Kind string
	// XML is the feature element.
	XML []byte
	// Provenance ties the record to its source.
	Provenance Provenance
}

// Provenance ties a target record to its source.
type Provenance struct {
	// SourceVersion is the AIXM version of the source document.
	SourceVersion version.Version `json:"sourceVersion"`
	// SourceKind names the source element(s), several joined by "+" for a
	// merged record ("Ase+Abd").
	SourceKind string `json:"sourceKind"`
	// SourceKey is the canonical identity of the source element in the
	// source version (a natural key, or the identifier).
	SourceKey string `json:"sourceKey,omitempty"`
	// SourceIndex holds the position(s) of the source element(s) among the
	// top-level elements of the source document.
	SourceIndex []int `json:"sourceIndex,omitempty"`
	// SourceIDs holds producer ids carried for traceability.
	SourceIDs []string `json:"sourceIds,omitempty"`
}

// Sink receives a converted document: Begin once, Put for every record,
// End once.
type Sink interface {
	Begin(Header) error
	Put(Record) error
	End() error
}

// XMLWriter returns a Sink writing the records as one AIXMBasicMessage
// document of the header's 5.x version to w.  The output is deterministic
// byte for byte: equal headers and records give equal documents.  Header
// attributes are written, sorted, in a comment before the root element.
// Begin fails for versions before 5 and other roots than AIXMBasicMessage.
func XMLWriter(w io.Writer) Sink { return &xmlWriter{w: bufio.NewWriter(w)} }

type xmlWriter struct {
	w *bufio.Writer
}

func (x *xmlWriter) Begin(h Header) error {
	root := h.Root
	if root == "" {
		root = "AIXMBasicMessage"
	}
	if h.Version.Major < 5 || root != "AIXMBasicMessage" {
		return fmt.Errorf("convert: XMLWriter cannot write %s of AIXM %v", root, h.Version)
	}
	id := h.GMLID
	if id == "" {
		id = "message"
	}
	w := x.w
	w.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	if len(h.Attrs) > 0 {
		keys := make([]string, 0, len(h.Attrs))
		for k := range h.Attrs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		w.WriteString("<!-- source")
		for _, k := range keys {
			fmt.Fprintf(w, " %s=%q", commentSafe(k), commentSafe(h.Attrs[k]))
		}
		w.WriteString(" -->\n")
	}
	fmt.Fprintf(w, `<message:AIXMBasicMessage xmlns:message="%s" xmlns:aixm="%s" xmlns:gml="http://www.opengis.net/gml/3.2" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" gml:id="%s">`+"\n",
		version.MessageNamespaceURI(h.Version), version.NamespaceURI(h.Version), attrEscape(id))
	return nil
}

func (x *xmlWriter) Put(r Record) error {
	b := bytes.TrimSpace(r.XML)
	if len(b) == 0 || b[0] != '<' || bytes.HasPrefix(b, []byte("<?")) {
		return fmt.Errorf("convert: record %s is not a bare element", r.Kind)
	}
	x.w.WriteString("<message:hasMember>")
	x.w.Write(b)
	x.w.WriteString("</message:hasMember>\n")
	return nil
}

func (x *xmlWriter) End() error {
	x.w.WriteString("</message:AIXMBasicMessage>\n")
	return x.w.Flush()
}

func attrEscape(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

// commentSafe keeps s from ending the comment and from breaking %q.
func commentSafe(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 {
			return ' '
		}
		return r
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "- -")
	}
	return s
}

// Tee returns a Sink that forwards every call to each sink in order (e.g. a
// file writer and a decoder).  The first error aborts.
func Tee(sinks ...Sink) Sink { return tee(sinks) }

type tee []Sink

func (t tee) Begin(h Header) error {
	for _, s := range t {
		if err := s.Begin(h); err != nil {
			return err
		}
	}
	return nil
}

func (t tee) Put(r Record) error {
	for _, s := range t {
		if err := s.Put(r); err != nil {
			return err
		}
	}
	return nil
}

func (t tee) End() error {
	for _, s := range t {
		if err := s.End(); err != nil {
			return err
		}
	}
	return nil
}

// runSink wraps the caller's sink in Run: it enforces the call order,
// checks the context, counts records and hashes the XMLWriter form.
type runSink struct {
	ctx     context.Context
	to      version.Version
	dst     Sink
	hash    hash.Hash
	count   *countingWriter
	xw      Sink
	state   int // 0 new, 1 begun, 2 ended
	records int
	targets map[string]int
}

func newRunSink(ctx context.Context, to version.Version, dst Sink) *runSink {
	h := sha256.New()
	cw := &countingWriter{w: h}
	return &runSink{ctx: ctx, to: to, dst: dst, hash: h, count: cw, xw: XMLWriter(cw), targets: map[string]int{}}
}

func (s *runSink) Begin(h Header) error {
	if s.state != 0 {
		return fmt.Errorf("%w: Begin called twice", ErrSinkOrder)
	}
	if h.Version != s.to {
		return fmt.Errorf("%w: converter writes AIXM %v, header says %v", ErrVersionMismatch, s.to, h.Version)
	}
	s.state = 1
	if s.to.Major >= 5 {
		if err := s.xw.Begin(h); err != nil {
			return err
		}
	} else {
		s.xw = nil // no canonical XML form: hash the record bytes only
	}
	return s.dst.Begin(h)
}

func (s *runSink) Put(r Record) error {
	if s.state != 1 {
		return fmt.Errorf("%w: Put outside Begin/End", ErrSinkOrder)
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if r.Kind == "" || len(bytes.TrimSpace(r.XML)) == 0 {
		return errors.New("convert: record without kind or XML")
	}
	if s.xw != nil {
		if err := s.xw.Put(r); err != nil {
			return err
		}
	} else {
		s.count.Write(r.XML)
	}
	s.records++
	s.targets[r.Kind]++
	return s.dst.Put(r)
}

func (s *runSink) End() error {
	if s.state != 1 {
		return fmt.Errorf("%w: End without Begin or twice", ErrSinkOrder)
	}
	s.state = 2
	if s.xw != nil {
		if err := s.xw.End(); err != nil {
			return err
		}
	}
	return s.dst.End()
}

func (s *runSink) finish() error {
	if s.state != 2 {
		return fmt.Errorf("%w: converter returned without End", ErrSinkOrder)
	}
	return nil
}

func (s *runSink) output() OutputInfo {
	return OutputInfo{Records: s.records, Bytes: s.count.n, SHA256: hex.EncodeToString(s.hash.Sum(nil))}
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
