// Package goaixm provides a Go library for parsing and working with AIXM 5.1.1 data.
//
// # Quick start
//
//	msg, err := goaixm.ParseFile("data.xml")
//	if err != nil { log.Fatal(err) }
//	for _, as := range msg.Airspaces() {
//	    fmt.Println(as.GetIdentifier())
//	}
//
// # Streaming large messages
//
//	err := goaixm.StreamFile("big.xml", ctx, func(name string, feat any) error {
//	    fmt.Println(name)
//	    return nil
//	})
//
// # Filtering timeslices
//
//	slices := filter.Chain(
//	    filter.ByInterpretation(features.InterpretationBaseline),
//	    filter.ByValidTime(time.Now()),
//	).Apply(as.TimeSlices())
//
// # Effective state (baseline + deltas)
//
//	state, err := filter.EffectiveAt(as.TimeSlices(), time.Now())
//
// # GeoJSON export
//
//	fc, err := geo.AirspaceToGeoJSON(as)
//	json, _ := geo.MarshalGeoJSON(fc)
package goaixm

import (
	"context"
	"io"
	"os"

	"gitlab.com/davidkohl/goaixm/parse"
)

// ParseMessage decodes an AIXMBasicMessage from r.
func ParseMessage(r io.Reader) (*parse.Message, error) {
	return parse.ParseMessage(r)
}

// ParseFile decodes an AIXMBasicMessage from the file at path.
func ParseFile(path string) (*parse.Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parse.ParseMessage(f)
}

// ParseFileWithOptions decodes an AIXMBasicMessage from the file at path using
// the given options (size limit, strict mode, custom registry).
func ParseFileWithOptions(ctx context.Context, path string, opts parse.ParseOptions) (*parse.Message, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parse.ParseMessageWithOptions(ctx, f, opts)
}

// StreamFile streams an AIXMBasicMessage from path, calling handler once for
// each decoded feature.  Peak memory usage is proportional to the largest
// single hasMember element rather than the full file size.
func StreamFile(ctx context.Context, path string, handler parse.StreamHandler) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return parse.StreamMessage(ctx, f, handler)
}

// ParseADRMessage decodes a EUROCONTROL ADRMessage from r.
func ParseADRMessage(r io.Reader) (*parse.ADRMessage, error) {
	return parse.ParseADRMessage(r)
}
