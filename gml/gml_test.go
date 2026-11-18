package gml_test

import (
	"encoding/xml"
	"testing"

	"github.com/davidkohl/goaixm/gml"
)

const ns = `xmlns:gml="http://www.opengis.net/gml/3.2" xmlns:aixm="http://www.aixm.aero/schema/5.1.1" xmlns:xlink="http://www.w3.org/1999/xlink"`

// Segments keep document order across kinds.
func TestCurveSegmentsOrder(t *testing.T) {
	doc := `<gml:segments ` + ns + `>
	  <gml:GeodesicString><gml:posList>1 1 2 2</gml:posList></gml:GeodesicString>
	  <gml:ArcByCenterPoint numArc="1"><gml:pos>3 3</gml:pos><gml:radius uom="[nmi_i]">5</gml:radius>
	    <gml:startAngle uom="deg">0</gml:startAngle><gml:endAngle uom="deg">90</gml:endAngle></gml:ArcByCenterPoint>
	  <gml:GeodesicString><gml:posList>4 4 5 5</gml:posList></gml:GeodesicString>
	  <gml:LineStringSegment><gml:posList>6 6 7 7</gml:posList></gml:LineStringSegment>
	</gml:segments>`
	var s gml.CurveSegments
	if err := xml.Unmarshal([]byte(doc), &s); err != nil {
		t.Fatal(err)
	}
	segs := s.Ordered()
	if len(segs) != 4 {
		t.Fatalf("got %d segments", len(segs))
	}
	if segs[0].GeodesicString == nil || segs[1].ArcByCenterPoint == nil ||
		segs[2].GeodesicString == nil || segs[3].LineStringSegment == nil {
		t.Errorf("order lost: %+v", segs)
	}
	if segs[2].GeodesicString.PosList.Value != "4 4 5 5" {
		t.Errorf("third segment = %q", segs[2].GeodesicString.PosList.Value)
	}
	arc := segs[1].ArcByCenterPoint
	if arc.Radius.UOM != "[nmi_i]" || arc.Radius.Value != 5 {
		t.Errorf("radius = %+v", arc.Radius)
	}
	var nilSegs *gml.CurveSegments
	if nilSegs.Ordered() != nil {
		t.Error("nil CurveSegments returned segments")
	}
}

// A ring's curve members accept gml:Curve, aixm:Curve (substitution) and a
// bare reference.
func TestRingCurveMembers(t *testing.T) {
	doc := `<gml:Ring ` + ns + `>
	  <gml:curveMember><gml:Curve gml:id="A"><gml:segments><gml:GeodesicString><gml:posList>1 1 2 2</gml:posList></gml:GeodesicString></gml:segments></gml:Curve></gml:curveMember>
	  <gml:curveMember><aixm:Curve gml:id="B"><gml:segments><gml:GeodesicString><gml:posList>2 2 3 3</gml:posList></gml:GeodesicString></gml:segments></aixm:Curve></gml:curveMember>
	  <gml:curveMember xlink:href="urn:uuid:border"/>
	</gml:Ring>`
	var r gml.Ring
	if err := xml.Unmarshal([]byte(doc), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.CurveMember) != 3 {
		t.Fatalf("got %d members", len(r.CurveMember))
	}
	if c := r.CurveMember[0].Curve; c == nil || c.ID != "A" {
		t.Errorf("gml:Curve member = %+v", c)
	}
	if c := r.CurveMember[1].Curve; c == nil || c.ID != "B" || c.Segments == nil {
		t.Errorf("aixm:Curve member = %+v", c)
	}
	if m := r.CurveMember[2]; m.Curve != nil || m.Href != "urn:uuid:border" {
		t.Errorf("reference member = %+v", m)
	}
}

func TestPointPropertySubstitution(t *testing.T) {
	for _, el := range []string{"gml:Point", "aixm:Point"} {
		doc := `<gml:pointProperty ` + ns + `><` + el + ` gml:id="p"><gml:pos>52 13</gml:pos></` + el + `></gml:pointProperty>`
		var p gml.PointProperty
		if err := xml.Unmarshal([]byte(doc), &p); err != nil {
			t.Fatal(err)
		}
		if p.Point == nil || p.Point.Pos == nil || p.Point.Pos.Value != "52 13" {
			t.Errorf("%s: point = %+v", el, p.Point)
		}
	}
}

func TestTimePrimitive(t *testing.T) {
	doc := `<gml:validTime ` + ns + `><gml:TimePeriod gml:id="t"><gml:beginPosition>2017-07-01T00:00:00Z</gml:beginPosition>
	  <gml:endPosition indeterminatePosition="unknown"/></gml:TimePeriod></gml:validTime>`
	var tp gml.TimePrimitiveProperty
	if err := xml.Unmarshal([]byte(doc), &tp); err != nil {
		t.Fatal(err)
	}
	if tp.TimePeriod == nil || tp.TimePeriod.BeginPosition.Value != "2017-07-01T00:00:00Z" {
		t.Errorf("period = %+v", tp.TimePeriod)
	}
	if ep := tp.TimePeriod.EndPosition; ep == nil || ep.IndeterminatePosition != "unknown" {
		t.Errorf("end = %+v", ep)
	}
}
