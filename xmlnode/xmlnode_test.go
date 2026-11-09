package xmlnode_test

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"testing"

	"gitlab.com/davidkohl/goaixm/xmlnode"
)

// Tests run against the EUROCONTROL DONLON reference data set.
const donlonFile = "../testdata/donlon/EA_AIP_DS_FULL_20170701.xml"

// donlonSegment returns the DONLON route segment UA4 BARIM–WOB.
func donlonSegment(t *testing.T) *xmlnode.Node {
	t.Helper()
	f, err := os.Open(donlonFile)
	if err != nil {
		t.Fatalf("open DONLON fixture: %v", err)
	}
	defer f.Close()
	root, err := xmlnode.ParseReader(f)
	if err != nil {
		t.Fatal(err)
	}
	if root.Local() != "AIXMBasicMessage" {
		t.Fatalf("root = %s", root.Local())
	}
	for _, seg := range root.FindAll("hasMember/RouteSegment") {
		if seg.TextAt("identifier") == "cc9c7cc4-e000-4741-854d-b7d93973e099" {
			return seg
		}
	}
	t.Fatal("route segment cc9c7cc4-… not found")
	return nil
}

func TestParseAndQuery(t *testing.T) {
	seg := donlonSegment(t)
	if id, _ := seg.AttrNS(xmlnode.NSGML, "id"); id != "uuid.cc9c7cc4-e000-4741-854d-b7d93973e099" {
		t.Errorf("gml:id %q", id)
	}
	ts := seg.Find("timeSlice/RouteSegmentTimeSlice")
	if ts == nil {
		t.Fatal("no timeslice")
	}
	if got := ts.TextAt("interpretation"); got != "BASELINE" {
		t.Errorf("interpretation %q", got)
	}
	ul := ts.Child("upperLimit")
	if uom, _ := ul.Attr("uom"); uom != "FL" || ul.Text != "450" {
		t.Errorf("upperLimit %q %q", ul.Text, uom)
	}
	if v, _ := ts.Find("validTime/TimePeriod/endPosition").Attr("indeterminatePosition"); v != "unknown" {
		t.Errorf("endPosition indeterminatePosition %q", v)
	}
	p := ts.Find("start/EnRouteSegmentPoint/pointChoice_fixDesignatedPoint")
	if p == nil || p.Href() != "urn:uuid:bb9dcfb3-f45a-40f3-8852-fca1349801fc" {
		t.Fatalf("href lookup failed: %+v", p)
	}
	if title, _ := p.AttrNS(xmlnode.NSXLink, "title"); title != "BARIM" {
		t.Errorf("xlink:title %q", title)
	}
	var walked int
	seg.Walk(func(*xmlnode.Node) bool { walked++; return true })
	if walked < 20 {
		t.Errorf("walk visited only %d nodes", walked)
	}
}

func TestDecodeSubtree(t *testing.T) {
	ts := donlonSegment(t).Find("timeSlice/RouteSegmentTimeSlice")
	var v struct {
		Upper struct {
			Value string `xml:",chardata"`
			UOM   string `xml:"uom,attr"`
		} `xml:"http://www.aixm.aero/schema/5.1.1 upperLimit"`
		Start struct {
			Href string `xml:"http://www.w3.org/1999/xlink href,attr"`
		} `xml:"http://www.aixm.aero/schema/5.1.1 start>EnRouteSegmentPoint>pointChoice_fixDesignatedPoint"`
		PosList string `xml:"http://www.aixm.aero/schema/5.1.1 curveExtent>Curve>segments>GeodesicString>posList"`
	}
	if err := ts.Decode(&v); err != nil {
		t.Fatal(err)
	}
	if v.Upper.Value != "450" || v.Upper.UOM != "FL" {
		t.Errorf("decoded upper %+v", v.Upper)
	}
	if v.Start.Href == "" {
		t.Error("decoded href empty")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	seg := donlonSegment(t)
	b, err := json.Marshal(seg)
	if err != nil {
		t.Fatal(err)
	}
	var back xmlnode.Node
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Name != (xml.Name{Space: xmlnode.NSAIXM, Local: "RouteSegment"}) {
		t.Errorf("name %+v", back.Name)
	}
	if back.Find("timeSlice/RouteSegmentTimeSlice/start/EnRouteSegmentPoint/pointChoice_fixDesignatedPoint").Href() == "" {
		t.Error("href lost in JSON round trip")
	}
}
