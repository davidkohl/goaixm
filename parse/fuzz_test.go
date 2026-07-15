package parse_test

import (
	"bytes"
	"testing"

	"gitlab.com/davidkohl/goaixm/parse"
)

// FuzzParseMessage verifies that ParseMessage never panics on arbitrary input.
// Run with: go test -fuzz=FuzzParseMessage ./parse/...
func FuzzParseMessage(f *testing.F) {
	// Seed corpus with a valid minimal message.
	f.Add([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<message:AIXMBasicMessage
  xmlns:message="http://www.aixm.aero/schema/5.1.1/message"
  xmlns:aixm="http://www.aixm.aero/schema/5.1.1"
  xmlns:gml="http://www.opengis.net/gml/3.2"
  gml:id="M-fuzz">
</message:AIXMBasicMessage>`))

	// Seed with empty input and truncated XML.
	f.Add([]byte(``))
	f.Add([]byte(`<`))
	f.Add([]byte(`<?xml`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic — errors are fine.
		_, _ = parse.ParseMessage(bytes.NewReader(data))
	})
}
