package aixm45_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/davidkohl/goaixm/aixm45"
)

// FuzzStream verifies that the reader never panics, and that element
// indexes count up from 0.  Run with: go test -fuzz=FuzzStream ./aixm45
func FuzzStream(f *testing.F) {
	f.Add([]byte(doc))
	f.Add([]byte(`<AIXM-Snapshot version="4.5"><Org/><Ahp><AhpUid><codeId>X</codeId></AhpUid></Ahp></AIXM-Snapshot>`))
	f.Add([]byte(`<AIXM-Update version="4.5">`))
	f.Add([]byte(``))
	f.Fuzz(func(t *testing.T, data []byte) {
		next := 0
		_, err := aixm45.Stream(context.Background(), bytes.NewReader(data), func(e aixm45.Element) error {
			if e.Index != next || e.Node == nil {
				t.Fatalf("element %d at position %d", e.Index, next)
			}
			next++
			_ = aixm45.Key(e.Node.Child(e.Kind() + "Uid"))
			_ = aixm45.NewAhp(e.Node)
			_ = aixm45.NewObs(e.Node)
			return nil
		})
		_ = err
	})
}

// FuzzCoordinate verifies that coordinate parsing never panics and that
// every accepted coordinate renders back in its own layout.
func FuzzCoordinate(f *testing.F) {
	for _, s := range []string{"484207.80N", "0021434.70E", "4842.13S", "48.5N", "179.99999999W", "900000N", "", "N"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, parse := range []func(string) (aixm45.Coord, error){aixm45.ParseLat, aixm45.ParseLong} {
			c, err := parse(s)
			if err != nil {
				if !errors.Is(err, aixm45.ErrCoordinate) {
					t.Fatalf("%q: unexpected error %v", s, err)
				}
				continue
			}
			if back := c.Layout(c.Format, c.Fraction); back != c.Text {
				t.Fatalf("%q: Layout gives %q", c.Text, back)
			}
			_ = c.Canonical()
			_ = c.Decimal(c.Decimals())
		}
	})
}
