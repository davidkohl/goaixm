package convert

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/davidkohl/goaixm/version"
)

// Source is a document that can be read more than once (converters may need
// several passes).
type Source interface {
	// Name identifies the document in reports (a file's base name).
	Name() string
	// Open returns a fresh reader positioned at the start of the document.
	Open() (io.ReadCloser, error)
}

// FileSource returns a Source reading the file at path.  Its Name is the
// file's base name, so reports do not depend on where the file lives.
func FileSource(path string) Source { return fileSource(path) }

type fileSource string

func (f fileSource) Name() string                 { return filepath.Base(string(f)) }
func (f fileSource) Open() (io.ReadCloser, error) { return os.Open(string(f)) }

// BytesSource returns a Source over b, named name.
func BytesSource(name string, b []byte) Source { return &bytesSource{name, b} }

type bytesSource struct {
	name string
	b    []byte
}

func (s *bytesSource) Name() string { return s.name }
func (s *bytesSource) Open() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.b)), nil
}

// ErrTooLarge is returned by Spool when the input exceeds its limit.
var ErrTooLarge = errors.New("convert: input exceeds the size limit")

// Spool copies r to a temporary file in dir (os.TempDir when empty), so a
// one-shot reader can serve as a Source.  limit bounds the bytes copied
// (0 means no limit; beyond it Spool fails with ErrTooLarge).  cleanup
// removes the file; it is safe to call more than once.  The Source is named
// "spool".
func Spool(r io.Reader, dir string, limit int64) (src Source, cleanup func(), err error) {
	f, err := os.CreateTemp(dir, "goaixm-spool-*.xml")
	if err != nil {
		return nil, nil, fmt.Errorf("convert: spool: %w", err)
	}
	ts := &tempSource{path: f.Name(), name: "spool"}
	in := r
	if limit > 0 {
		in = io.LimitReader(r, limit+1)
	}
	n, err := io.Copy(f, in)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && limit > 0 && n > limit {
		err = ErrTooLarge
	}
	if err != nil {
		ts.remove()
		if errors.Is(err, ErrTooLarge) {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("convert: spool: %w", err)
	}
	return ts, ts.remove, nil
}

// tempSource is a temporary file owned by this package.
type tempSource struct {
	path, name string
}

func (t *tempSource) Name() string                 { return t.name }
func (t *tempSource) Open() (io.ReadCloser, error) { return os.Open(t.path) }
func (t *tempSource) remove()                      { os.Remove(t.path) }

// runToTemp runs one converter into a temporary XMLWriter document and
// returns it as a Source for the next step.
func runToTemp(ctx context.Context, c Converter, src Source) (*tempSource, *Report, error) {
	f, err := os.CreateTemp("", "goaixm-chain-*.xml")
	if err != nil {
		return nil, nil, fmt.Errorf("convert: chain: %w", err)
	}
	ts := &tempSource{path: f.Name(), name: src.Name()}
	rep, err := Run(ctx, c, src, XMLWriter(f))
	if cerr := f.Close(); err == nil && cerr != nil {
		err = fmt.Errorf("convert: chain: %w", cerr)
	}
	if err != nil {
		ts.remove()
		return nil, nil, err
	}
	return ts, rep, nil
}

// describe reads src once: size, sha256 and the root element.
func describe(src Source) (SourceInfo, error) {
	rc, err := src.Open()
	if err != nil {
		return SourceInfo{}, err
	}
	defer rc.Close()
	h := sha256.New()
	cr := &countingReader{r: io.TeeReader(rc, h)}
	det, err := version.Detect(cr)
	if err != nil {
		return SourceInfo{}, fmt.Errorf("convert: %s: %w", src.Name(), err)
	}
	if _, err := io.Copy(io.Discard, cr); err != nil {
		return SourceInfo{}, fmt.Errorf("convert: %s: %w", src.Name(), err)
	}
	return SourceInfo{
		Name: src.Name(), Bytes: cr.n, SHA256: hex.EncodeToString(h.Sum(nil)),
		Version: det.Version, Root: det.Root, Attrs: det.Attrs,
	}, nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}
