// Package goaixm parses AIXM 5.1.1 (Aeronautical Information Exchange Model)
// messages into the Go model of package aixm.
//
// # Parsing
//
//	msg, err := goaixm.ParseFile(ctx, "data.xml", goaixm.Options{})
//	for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
//		fmt.Println(a.UUID(), a.TimeSlice[0].Designator.String())
//	}
//
// Parse keeps the whole message and indexes it, so references between
// features can be followed with Message.Resolve.  Stream visits one member
// at a time with memory bounded by the largest member:
//
//	err := goaixm.StreamFile(ctx, "big.xml", goaixm.Options{}, func(m goaixm.Member) error {
//		fmt.Println(m.Kind, m.Feature.Common().UUID())
//		return nil
//	})
//
// Both accept AIXMBasicMessage and EUROCONTROL ADRMessage documents.
//
// # Extending the model
//
// A Registry decides how each member is decoded.  NewDefaultRegistry knows
// every AIXM 5.1.1 feature.  Register replaces the type of a feature, Extend
// adds overlay schemas (partial structs decoded alongside the primary type,
// see Extension), and Options.Fallback = FallbackGeneric keeps members
// without a registered type as lossless element trees (GenericFeature).
//
// Related packages: aixm (the model), geo (geometry and GeoJSON), filter
// (time slice selection), xmlnode (lossless XML trees) and extensions/adr
// (EUROCONTROL ADR extensions).
package goaixm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrUnknownFeature is reported (wrapped in a Warning) for members whose
// kind has no registered type when Options.Fallback is FallbackSkip.
var ErrUnknownFeature = errors.New("goaixm: unknown feature kind")

// ErrTooLarge is returned when a message exceeds Options.MaxBytes.
var ErrTooLarge = errors.New("goaixm: message exceeds maximum size")

// Fallback controls what happens to members whose kind has no primary type
// in the registry.
type Fallback int

const (
	// FallbackSkip drops such members and records a Warning (the default).
	FallbackSkip Fallback = iota
	// FallbackGeneric decodes them into *GenericFeature, a lossless element
	// tree, so nothing in the message is lost.
	FallbackGeneric
)

// Options configures Parse and Stream.  The zero value parses with the
// default AIXM 5.1.1 registry.
type Options struct {
	// Registry decides how members are decoded.  Nil means the AIXM 5.1.1
	// types (see NewDefaultRegistry).
	Registry *Registry
	// Fallback selects what happens to members without a registered type.
	Fallback Fallback
	// KeepRaw retains each member's XML in Member.Raw, so it can be
	// re-examined with Member.Node.
	KeepRaw bool
	// Strict turns the first non-fatal problem into an error instead of a
	// Warning.
	Strict bool
	// MaxBytes limits the size of the message; 0 means no limit.
	MaxBytes int64
	// OnWarning, when set, is called for every member that could not be
	// decoded.  Returning an error aborts parsing.  Parse also records
	// warnings in Message.Warnings.
	OnWarning func(Warning) error
}

// Warning is a non-fatal problem with one member of a message.
type Warning struct {
	// Index is the position of the member in the message.
	Index int
	// Kind is the XML local name of the member's feature, if known.
	Kind string
	Err  error
}

func (w Warning) Error() string {
	return fmt.Sprintf("member %d (%s): %v", w.Index, w.Kind, w.Err)
}

func (w Warning) Unwrap() error { return w.Err }

// Parse decodes a complete message from r and indexes its features.
func Parse(ctx context.Context, r io.Reader, opts Options) (*Message, error) {
	msg := newMessage()
	userWarn := opts.OnWarning
	opts.OnWarning = func(w Warning) error {
		msg.warnings = append(msg.warnings, w)
		if userWarn != nil {
			return userWarn(w)
		}
		return nil
	}
	root, err := stream(ctx, r, opts, func(m Member) error {
		msg.add(m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	msg.Kind, msg.GMLID = root.kind, root.gmlID
	return msg, nil
}

// ParseFile decodes the message in the file at path.
func ParseFile(ctx context.Context, path string, opts Options) (*Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(ctx, f, opts)
}

// Stream decodes the members of a message from r one at a time and calls fn
// for each, in document order.  Memory use is proportional to the largest
// member.  Features are not indexed, so references cannot be resolved; use
// Parse for that.  Returning an error from fn aborts streaming with that
// error.
func Stream(ctx context.Context, r io.Reader, opts Options, fn func(Member) error) error {
	_, err := stream(ctx, r, opts, fn)
	return err
}

// StreamFile streams the message in the file at path.
func StreamFile(ctx context.Context, path string, opts Options, fn func(Member) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return Stream(ctx, f, opts, fn)
}
