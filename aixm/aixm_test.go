package aixm_test

import (
	"encoding/xml"
	"testing"

	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/gml"
)

// Every generated feature satisfies the Feature contract.
func TestEveryFeatureKind(t *testing.T) {
	kinds := aixm.FeatureKinds()
	if len(kinds) < 100 {
		t.Fatalf("only %d feature kinds", len(kinds))
	}
	for _, kind := range kinds {
		f, ok := aixm.NewFeature(kind)
		if !ok || f == nil {
			t.Fatalf("NewFeature(%q) failed", kind)
		}
		if f.Kind() != kind {
			t.Errorf("%s: Kind() = %q", kind, f.Kind())
		}
		if f.Common() == nil {
			t.Errorf("%s: Common() is nil", kind)
		}
		if len(f.TimeSlices()) != 0 {
			t.Errorf("%s: empty feature has time slices", kind)
		}
	}
	if _, ok := aixm.NewFeature("NoSuchFeature"); ok {
		t.Error("NewFeature accepted an unknown kind")
	}
}

func TestFeatureCommon(t *testing.T) {
	a := &aixm.Airspace{}
	if a.UUID() != "" {
		t.Errorf("UUID of empty feature = %q", a.UUID())
	}
	a.Identifier = &gml.Code{Value: " 1b54b2d6-a5ff-4e57-94c2-f4047a381c64 "}
	a.GMLID = "AS-1"
	var f aixm.Feature = a
	if got := f.Common().UUID(); got != "1b54b2d6-a5ff-4e57-94c2-f4047a381c64" {
		t.Errorf("UUID = %q", got)
	}
	if f.Common().GMLID != "AS-1" {
		t.Errorf("GMLID = %q", f.Common().GMLID)
	}
}

func TestTimeSlicesShareStorage(t *testing.T) {
	seq := uint32(3)
	a := &aixm.Airspace{TimeSlice: []aixm.AirspaceTimeSlice{{
		TimeSliceCommon: aixm.TimeSliceCommon{Interpretation: aixm.Baseline, SequenceNumber: &seq},
	}}}
	slices := a.TimeSlices()
	if len(slices) != 1 {
		t.Fatalf("got %d slices", len(slices))
	}
	c := slices[0].Common()
	if c.Interpretation != aixm.Baseline {
		t.Errorf("interpretation = %q", c.Interpretation)
	}
	if s, corr := c.Sequence(); s != 3 || corr != 0 {
		t.Errorf("Sequence() = %d, %d", s, corr)
	}
	c.Interpretation = aixm.Snapshot
	if a.TimeSlice[0].Interpretation != aixm.Snapshot {
		t.Error("TimeSlices does not point into the feature")
	}
}

// Objects nested in a time slice are decoded through their property element,
// abstract objects through a choice, and units of measure are kept.
func TestDecodeNestedObjects(t *testing.T) {
	const doc = `<aixm:RouteSegment xmlns:aixm="http://www.aixm.aero/schema/5.1.1"
  xmlns:gml="http://www.opengis.net/gml/3.2" xmlns:xlink="http://www.w3.org/1999/xlink" gml:id="rs1">
  <gml:identifier codeSpace="urn:uuid:">cc9c7cc4-e000-4741-854d-b7d93973e099</gml:identifier>
  <aixm:timeSlice><aixm:RouteSegmentTimeSlice gml:id="ts1">
    <aixm:interpretation>BASELINE</aixm:interpretation>
    <aixm:upperLimit uom="FL">450</aixm:upperLimit>
    <aixm:start><aixm:EnRouteSegmentPoint gml:id="p1">
      <aixm:reportingATC>COMPULSORY</aixm:reportingATC>
      <aixm:pointChoice_fixDesignatedPoint xlink:href="urn:uuid:bb9dcfb3-f45a-40f3-8852-fca1349801fc" xlink:title="BARIM"/>
    </aixm:EnRouteSegmentPoint></aixm:start>
    <aixm:routeFormed xlink:href="urn:uuid:a14a8751-5428-46bc-a2d1-32ef84d37b5c"/>
    <aixm:annotation><aixm:Note gml:id="n1"><aixm:purpose>REMARK</aixm:purpose></aixm:Note></aixm:annotation>
  </aixm:RouteSegmentTimeSlice></aixm:timeSlice>
</aixm:RouteSegment>`
	var rs aixm.RouteSegment
	if err := xml.Unmarshal([]byte(doc), &rs); err != nil {
		t.Fatal(err)
	}
	if rs.UUID() != "cc9c7cc4-e000-4741-854d-b7d93973e099" {
		t.Errorf("UUID = %q", rs.UUID())
	}
	ts := rs.TimeSlice[0]
	if ts.GMLID != "ts1" || ts.Interpretation != aixm.Baseline {
		t.Errorf("common = %+v", ts.TimeSliceCommon)
	}
	if v, ok := ts.UpperLimit.Float(); !ok || v != 450 || ts.UpperLimit.UOM != "FL" {
		t.Errorf("upperLimit = %+v", ts.UpperLimit)
	}
	if ts.Start == nil || ts.Start.GMLID != "p1" || ts.Start.ReportingATC.String() != "COMPULSORY" {
		t.Fatalf("start = %+v", ts.Start)
	}
	if ref := ts.Start.PointChoiceFixDesignatedPoint; ref == nil || ref.Title != "BARIM" {
		t.Errorf("fix reference = %+v", ref)
	}
	if ts.RouteFormed == nil || ts.RouteFormed.Href == "" {
		t.Errorf("routeFormed = %+v", ts.RouteFormed)
	}
	if len(ts.Annotation) != 1 || ts.Annotation[0].Purpose.String() != "REMARK" {
		t.Errorf("annotation = %+v", ts.Annotation)
	}
}

func TestChoiceObject(t *testing.T) {
	var c *aixm.SegmentPoint
	if c.Object() != nil {
		t.Error("nil choice returned an object")
	}
	c = &aixm.SegmentPoint{TerminalSegmentPoint: &aixm.TerminalSegmentPoint{}}
	if _, ok := c.Object().(*aixm.TerminalSegmentPoint); !ok {
		t.Errorf("Object() = %T", c.Object())
	}
}
