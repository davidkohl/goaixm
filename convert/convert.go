// Package convert is the framework for converting AIXM documents between
// schema versions.
//
// A Converter reads a Source (a document that can be read more than once)
// of one AIXM version and writes the equivalent document of another version
// to a Sink, one standalone member element (Record) at a time, in a
// deterministic order.  It returns a Report that accounts for every source
// value: each leaf (element text and attribute) of every source element gets
// exactly one Outcome, so nothing is dropped silently.
//
// Converters are values the caller constructs and registers; this package
// ships none.  A Registry finds the shortest chain of registered converters
// between two versions, and Chain composes converters:
//
//	reg := convert.NewRegistry(myConverter)
//	c, err := reg.Converter(version.AIXM45, version.AIXM511)
//	rep, err := convert.Run(ctx, c, convert.FileSource("in.xml"), convert.XMLWriter(out))
//
// Run is the entry point: it calls the converter, checks the order of Sink
// calls, completes the report (source and output digests, record counts)
// and verifies its accounting (Report.Check).  goaixm.Options.Converters
// uses it to decode documents of other versions directly into members.
//
// Records are target-version XML rather than Go structs, so every consumer
// of the target version's XML works unchanged, and the converted document
// can be written out (XMLWriter), archived, hashed and validated.
package convert

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/davidkohl/goaixm/version"
)

// Converter converts documents of one AIXM version into another.
//
// Implementations must be deterministic: the same input gives the same
// records in the same order and the same report.  Callers normally use Run
// rather than calling Convert directly.
type Converter interface {
	// From is the version of the documents the converter reads.
	From() version.Version
	// To is the version of the records it writes.
	To() version.Version
	// ID identifies the exact conversion rules.
	ID() ConverterID
	// Convert reads src (as many passes as it needs), calls dst.Begin once,
	// dst.Put for every record in deterministic order and dst.End once, and
	// returns the report with Kinds and Issues filled in (Run completes the
	// rest).  A Sink error or ctx cancellation aborts the conversion and is
	// returned.  Problems with single source elements go into the report,
	// not the error.
	Convert(ctx context.Context, src Source, dst Sink) (*Report, error)
}

// ConverterID identifies the exact conversion rules, for traceability: a
// stored report says which rules produced a converted document.
type ConverterID struct {
	// Name of the converter, e.g. "goaixm/aixm45to51".
	Name string `json:"name"`
	// Version of the rules (semver), bumped on every rule change.
	Version string `json:"version"`
	// MappingDigest is the sha256 (hex) over the converter's mapping tables.
	MappingDigest string `json:"mappingDigest"`
	// Module is the version of the module that contains the converter, as
	// recorded in the build (see ModuleVersion).
	Module string `json:"module"`
}

func (id ConverterID) String() string {
	return fmt.Sprintf("%s %s (mapping %.12s, module %s)", id.Name, id.Version, id.MappingDigest, id.Module)
}

// NewID returns a ConverterID with MappingDigest computed over tables (the
// converter's embedded mapping data, in a fixed order) and Module set to
// ModuleVersion().
func NewID(name, ver string, tables ...[]byte) ConverterID {
	h := sha256.New()
	var n [8]byte
	for _, t := range tables {
		binary.BigEndian.PutUint64(n[:], uint64(len(t)))
		h.Write(n[:])
		h.Write(t)
	}
	return ConverterID{Name: name, Version: ver, MappingDigest: hex.EncodeToString(h.Sum(nil)), Module: ModuleVersion()}
}

const modulePath = "github.com/davidkohl/goaixm"

// ModuleVersion returns the version of the goaixm module in the running
// binary: the module version from the build info ("v1.2.3", "(devel)"),
// followed by the VCS revision when the build recorded one (goaixm built as
// the main module), "unknown" without build info.
func ModuleVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	mod := &bi.Main
	if mod.Path != modulePath {
		mod = nil
		for _, d := range bi.Deps {
			if d.Path == modulePath {
				mod = d
				break
			}
		}
	}
	if mod == nil {
		return "unknown"
	}
	if mod.Replace != nil {
		mod = mod.Replace
	}
	v := mod.Version
	if v == "" {
		v = "(devel)"
	}
	if mod == &bi.Main {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				v += " " + s.Value
			}
		}
	}
	return v
}

// ErrNoPath is returned when the registry has no chain of converters
// between two versions.
var ErrNoPath = errors.New("convert: no conversion path")

// ErrSinkOrder is returned by Run when a converter calls its Sink out of
// order (Put before Begin, anything after End, End missing).
var ErrSinkOrder = errors.New("convert: sink called out of order")

// ErrVersionMismatch is returned by Run when the source document is not of
// the converter's From version, or the converter begins a document of
// another version than its To version.
var ErrVersionMismatch = errors.New("convert: version mismatch")

// Run converts src with c into dst and returns the completed report.
//
// It detects the source version (which must be c.From()), checks that c
// calls dst in order (Begin, Put…, End) with records of c.To(), checks ctx
// before every record, and completes the report: Format, Converter, From,
// To, Source (name, bytes, sha256, root), Output (records, bytes and sha256
// of the XMLWriter form of the output) and Targets.  Issues are sorted.
// Finally the accounting is verified with Report.Check; a violation is a
// converter bug and returned as an error together with the report.
func Run(ctx context.Context, c Converter, src Source, dst Sink) (*Report, error) {
	info, err := describe(src)
	if err != nil {
		return nil, err
	}
	if info.Version != c.From() {
		return nil, fmt.Errorf("%w: %s is AIXM %v, converter %s reads %v", ErrVersionMismatch, info.Name, info.Version, c.ID().Name, c.From())
	}
	rs := newRunSink(ctx, c.To(), dst)
	rep, err := c.Convert(ctx, src, rs)
	if err != nil {
		return nil, err
	}
	if err := rs.finish(); err != nil {
		return nil, err
	}
	if rep == nil {
		rep = NewReport()
	}
	rep.Format = ReportFormat
	rep.Converter = c.ID()
	rep.From, rep.To = c.From(), c.To()
	rep.Source = info
	rep.Output = rs.output()
	rep.Targets = rs.targets
	rep.sortIssues()
	if err := rep.Check(); err != nil {
		return rep, err
	}
	return rep, nil
}

// Chain composes converters into one: cs[i].To() must equal cs[i+1].From().
// Intermediate documents are written with XMLWriter to temporary files,
// because the next converter may need several passes, and removed
// afterwards; intermediate versions must therefore be AIXM 5.x.  The
// chain's report has one entry in Steps per converter (each completed by
// Run) and no Kinds of its own.
func Chain(cs ...Converter) (Converter, error) {
	if len(cs) == 0 {
		return nil, errors.New("convert: empty chain")
	}
	for i := 1; i < len(cs); i++ {
		if cs[i-1].To() != cs[i].From() {
			return nil, fmt.Errorf("convert: chain step %d writes %v, step %d reads %v", i-1, cs[i-1].To(), i, cs[i].From())
		}
		if cs[i-1].To().Major < 5 {
			return nil, fmt.Errorf("convert: intermediate version %v cannot be written", cs[i-1].To())
		}
	}
	if len(cs) == 1 {
		return cs[0], nil
	}
	names := make([]string, len(cs))
	vers := make([]string, len(cs))
	var digests [][]byte
	for i, c := range cs {
		id := c.ID()
		names[i], vers[i] = id.Name, id.Version
		digests = append(digests, []byte(id.MappingDigest))
	}
	id := NewID("chain("+strings.Join(names, ",")+")", strings.Join(vers, ","), digests...)
	return &chain{cs: append([]Converter(nil), cs...), id: id}, nil
}

type chain struct {
	cs []Converter
	id ConverterID
}

func (c *chain) From() version.Version { return c.cs[0].From() }
func (c *chain) To() version.Version   { return c.cs[len(c.cs)-1].To() }
func (c *chain) ID() ConverterID       { return c.id }

func (c *chain) Convert(ctx context.Context, src Source, dst Sink) (*Report, error) {
	rep := NewReport()
	cur := src
	var temps []*tempSource
	defer func() {
		for _, t := range temps {
			t.remove()
		}
	}()
	for i, step := range c.cs {
		if i == len(c.cs)-1 {
			r, err := Run(ctx, step, cur, dst)
			if err != nil {
				return nil, err
			}
			rep.Steps = append(rep.Steps, r)
			break
		}
		next, r, err := runToTemp(ctx, step, cur)
		if err != nil {
			return nil, err
		}
		temps = append(temps, next)
		rep.Steps = append(rep.Steps, r)
		cur = next
	}
	return rep, nil
}
