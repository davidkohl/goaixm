package parse

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

// ParseOptions controls how ParseMessage decodes an AIXMBasicMessage.
type ParseOptions struct {
	// MaxBytes is the maximum number of bytes read from the reader before
	// returning an error.  Zero means no limit.
	MaxBytes int64

	// Registry is the feature type registry to use.  Nil uses DefaultRegistry.
	Registry *Registry

	// StrictMode causes unknown feature types and other non-fatal problems to
	// be returned as errors instead of warnings.
	StrictMode bool
}

// ParseMessageWithOptions decodes an AIXMBasicMessage using the given options.
func ParseMessageWithOptions(ctx context.Context, r io.Reader, opts ParseOptions) (*Message, error) {
	reg := opts.Registry
	if reg == nil {
		reg = DefaultRegistry
	}

	var src io.Reader = r
	if opts.MaxBytes > 0 {
		src = io.LimitReader(r, opts.MaxBytes+1)
	}

	data, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("parse: read: %w", err)
	}
	if opts.MaxBytes > 0 && int64(len(data)) > opts.MaxBytes {
		return nil, fmt.Errorf("parse: message exceeds maximum size of %d bytes", opts.MaxBytes)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Swap DefaultRegistry temporarily when a custom one is provided.
	if reg != DefaultRegistry {
		orig := DefaultRegistry
		DefaultRegistry = reg
		defer func() { DefaultRegistry = orig }()
	}

	msg, err := ParseMessage(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	if opts.StrictMode && len(msg.Warnings()) > 0 {
		w := msg.Warnings()[0]
		return nil, fmt.Errorf("parse: strict mode: member %d (%s): %w", w.MemberIndex, w.LocalName, w.Err)
	}
	return msg, nil
}

