package goaixm_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"gitlab.com/davidkohl/goaixm"
	"gitlab.com/davidkohl/goaixm/features"
	"gitlab.com/davidkohl/goaixm/filter"
	"gitlab.com/davidkohl/goaixm/geo"
	"gitlab.com/davidkohl/goaixm/parse"
)

func Example_parseAndListAirspaces() {
	msg, err := goaixm.ParseFile("testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		log.Fatal(err)
	}
	for _, a := range msg.Airspaces() {
		if len(a.TimeSlice) == 0 {
			continue
		}
		ts := a.TimeSlice[0].AirspaceTimeSlice
		if ts.Designator != nil {
			fmt.Println(ts.Designator.Value)
		}
	}
}

func Example_filterByValidTime() {
	msg, err := goaixm.ParseFile("testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		log.Fatal(err)
	}
	now := time.Date(2017, 7, 1, 12, 0, 0, 0, time.UTC)
	for _, a := range msg.Airspaces() {
		slices := a.TimeSlices()
		active := filter.Chain(
			filter.ByInterpretation(features.InterpretationBaseline),
			filter.ByValidTime(now),
		).Apply(slices)
		if len(active) > 0 {
			ts := active[0].(*features.AirspaceTimeSliceType)
			if ts.Designator != nil {
				_ = ts.Designator.Value
			}
		}
	}
}

func Example_effectiveAt() {
	msg, err := goaixm.ParseFile("testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		log.Fatal(err)
	}
	at := time.Date(2017, 7, 1, 12, 0, 0, 0, time.UTC)
	for _, a := range msg.Airspaces() {
		state, err := filter.EffectiveAt(a.TimeSlices(), at)
		if err != nil {
			continue // no BASELINE valid at this time
		}
		ts := state.Slice.(*features.AirspaceTimeSliceType)
		if ts.Designator != nil {
			_ = ts.Designator.Value
		}
	}
}

func Example_streamLargeFile() {
	ctx := context.Background()
	err := goaixm.StreamFile(ctx, "testdata/donlon/EA_AIP_DS_FULL_20170701.xml",
		func(localName string, feat any) error {
			if localName == "Airspace" {
				a := feat.(*features.AirspaceType)
				_ = a.GetIdentifier()
			}
			return nil
		})
	if err != nil {
		log.Fatal(err)
	}
}

func Example_geoJSON() {
	msg, err := goaixm.ParseFile("testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		log.Fatal(err)
	}
	for _, a := range msg.Airspaces() {
		fc, err := geo.AirspaceToGeoJSON(a)
		if err != nil {
			continue
		}
		data, err := geo.MarshalGeoJSON(fc)
		if err != nil {
			log.Fatal(err)
		}
		_ = data
		break
	}
}

func Example_resolveXLink() {
	msg, err := goaixm.ParseFile("testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		log.Fatal(err)
	}
	// Resolve a feature by its GML identifier (UUID).
	feat, ok := msg.Resolve("urn:uuid:1234-...")
	if ok {
		_ = feat
	}
}

func Example_validateFeatures() {
	msg, err := goaixm.ParseFile("testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		log.Fatal(err)
	}
	var feats []features.Feature
	for _, v := range msg.Features() {
		if f, ok := v.(features.Feature); ok {
			feats = append(feats, f)
		}
	}
	errs := features.ValidateAll(feats)
	for _, e := range errs {
		fmt.Println(e)
	}
}

func Example_parseWithOptions() {
	ctx := context.Background()
	opts := parse.ParseOptions{
		MaxBytes:   50 * 1024 * 1024, // 50 MB limit
		StrictMode: false,
	}
	f, err := os.Open("testdata/donlon/EA_AIP_DS_FULL_20170701.xml")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	msg, err := parse.ParseMessageWithOptions(ctx, f, opts)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("features: %d, warnings: %d\n", len(msg.Features()), len(msg.Warnings()))
}
