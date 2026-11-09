package goaixm_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/filter"
	"github.com/davidkohl/goaixm/geo"
)

// The examples use the EUROCONTROL DONLON reference data set
// (git submodule testdata/donlon).

func ExampleParseFile() {
	ctx := context.Background()
	msg, err := goaixm.ParseFile(ctx, "testdata/donlon/EA_AIP_DS_FULL_20170701.xml", goaixm.Options{})
	if err != nil {
		log.Fatal(err)
	}
	for _, ad := range goaixm.FeaturesOf[*aixm.AirportHeliport](msg) {
		ts := ad.TimeSlice[0]
		fmt.Println(ts.Designator.String(), ts.Name.String())
	}
	// Output:
	// EADH DONLON/DOWNTOWN HELIPORT
	// EADD DONLON/INTL.
}

func ExampleMessage_Resolve() {
	ctx := context.Background()
	msg, err := goaixm.ParseFile(ctx, "testdata/donlon/EA_AIP_DS_FULL_20170701.xml", goaixm.Options{})
	if err != nil {
		log.Fatal(err)
	}
	seg := goaixm.FeaturesOf[*aixm.RouteSegment](msg)[0]
	start := seg.TimeSlice[0].Start.PointChoiceFixDesignatedPoint
	if f, ok := msg.Resolve(start.Href); ok {
		dp := f.(*aixm.DesignatedPoint)
		fmt.Println(dp.TimeSlice[0].Designator.String())
	}
	// Output: BARIM
}

func ExampleStreamFile() {
	ctx := context.Background()
	counts := map[string]int{}
	err := goaixm.StreamFile(ctx, "testdata/donlon/EA_AIP_DS_FULL_20170701.xml", goaixm.Options{},
		func(m goaixm.Member) error {
			counts[m.Kind]++
			return nil
		})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(counts["Airspace"], counts["Navaid"])
	// Output: 20 14
}

func ExampleRegistry_Extend() {
	// An overlay declares only the elements it needs.
	type airspaceClass struct {
		Classes []string `xml:"http://www.aixm.aero/schema/5.1.1 timeSlice>AirspaceTimeSlice>class>AirspaceLayerClass>classification"`
	}
	reg := goaixm.NewDefaultRegistry()
	reg.Extend("Airspace", func() any { return new(airspaceClass) })

	ctx := context.Background()
	msg, err := goaixm.ParseFile(ctx, "testdata/donlon/EA_AIP_DS_FULL_20170701.xml", goaixm.Options{Registry: reg})
	if err != nil {
		log.Fatal(err)
	}
	for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
		m, _ := msg.Member(a)
		if c, ok := goaixm.Extension[*airspaceClass](m); ok && len(c.Classes) > 0 {
			fmt.Println(a.TimeSlice[0].Designator.String(), c.Classes[0])
			break
		}
	}
}

func ExampleFeaturesOf() {
	ctx := context.Background()
	msg, err := goaixm.ParseFile(ctx, "testdata/donlon/EA_AIP_DS_FULL_20170701.xml", goaixm.Options{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(goaixm.FeaturesOf[*aixm.Runway](msg)))
	// Output: 3
}

func Example_currentState() {
	ctx := context.Background()
	msg, err := goaixm.ParseFile(ctx, "testdata/donlon/EA_AIP_DS_FULL_20170701.xml", goaixm.Options{})
	if err != nil {
		log.Fatal(err)
	}
	at := time.Date(2017, 7, 1, 12, 0, 0, 0, time.UTC)
	for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg)[:1] {
		ts, err := filter.Current(a.TimeSlices(), at)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(ts.Common().Interpretation)
	}
	// Output: BASELINE
}

func Example_geoJSON() {
	ctx := context.Background()
	msg, err := goaixm.ParseFile(ctx, "testdata/donlon/EA_AIP_DS_FULL_20170701.xml", goaixm.Options{})
	if err != nil {
		log.Fatal(err)
	}
	a := goaixm.FeaturesOf[*aixm.Airspace](msg)[0]
	fc, err := geo.AirspaceToGeoJSONWithRefs(a, msg)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(fc.Type, len(fc.Features) > 0)
	// Output: FeatureCollection true
}
