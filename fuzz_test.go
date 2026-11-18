package goaixm_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/davidkohl/goaixm"
)

// FuzzParse verifies that Parse never panics on arbitrary input.
// Run with: go test -fuzz=FuzzParse .
func FuzzParse(f *testing.F) {
	f.Add([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<message:AIXMBasicMessage
  xmlns:message="http://www.aixm.aero/schema/5.1.1/message"
  xmlns:aixm="http://www.aixm.aero/schema/5.1.1"
  xmlns:gml="http://www.opengis.net/gml/3.2"
  gml:id="M-fuzz">
  <message:hasMember><aixm:Airspace gml:id="a"><aixm:timeSlice/></aixm:Airspace></message:hasMember>
</message:AIXMBasicMessage>`))
	f.Add([]byte(``))
	f.Add([]byte(`<`))
	f.Add([]byte(`<?xml`))

	f.Fuzz(func(t *testing.T, data []byte) {
		for _, fb := range []goaixm.Fallback{goaixm.FallbackSkip, goaixm.FallbackGeneric} {
			_, _ = goaixm.Parse(context.Background(), bytes.NewReader(data), goaixm.Options{Fallback: fb, KeepRaw: true})
		}
	})
}
