package geo_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/geo"
)

// aggregateXML is an airspace made of contributor airspaces: parts are
// (operation, contributor uuid) in sequence.
func aggregateXML(uuid, designator string, parts [][2]string) string {
	var b strings.Builder
	for i, p := range parts {
		fmt.Fprintf(&b, `<aixm:geometryComponent><aixm:AirspaceGeometryComponent gml:id="gc.%[1]s.%[2]d">
  <aixm:operation>%[3]s</aixm:operation><aixm:operationSequence>%[2]d</aixm:operationSequence>
  <aixm:theAirspaceVolume><aixm:AirspaceVolume gml:id="vol.%[1]s.%[2]d">
    <aixm:contributorAirspace><aixm:AirspaceVolumeDependency gml:id="dep.%[1]s.%[2]d">
      <aixm:dependency>FULL_GEOMETRY</aixm:dependency><aixm:theAirspace xlink:href="urn:uuid:%[4]s"/>
    </aixm:AirspaceVolumeDependency></aixm:contributorAirspace>
  </aixm:AirspaceVolume></aixm:theAirspaceVolume>
</aixm:AirspaceGeometryComponent></aixm:geometryComponent>`, uuid, i+1, p[0], p[1])
	}
	return fmt.Sprintf(`<message:hasMember><aixm:Airspace gml:id="uuid.%[1]s">
  <gml:identifier codeSpace="urn:uuid:">%[1]s</gml:identifier>
  <aixm:timeSlice><aixm:AirspaceTimeSlice gml:id="ts.%[1]s">
    <aixm:interpretation>BASELINE</aixm:interpretation>
    <aixm:type>CTA</aixm:type><aixm:designator>%[2]s</aixm:designator>%[3]s
  </aixm:AirspaceTimeSlice></aixm:timeSlice>
</aixm:Airspace></message:hasMember>`, uuid, designator, b.String())
}

// NestedContributors gives a contributor that is itself an aggregate as
// its own components (with their operations), recursively; a plain
// contributor stays a geometry.  Without it, the aggregate's polygons are
// packed together, its SUBTR component included.
func TestNestedContributors(t *testing.T) {
	const (
		p1, p2, p3 = "00000000-0000-0000-0000-0000000c0001", "00000000-0000-0000-0000-0000000c0002", "00000000-0000-0000-0000-0000000c0003"
		agg, top   = "00000000-0000-0000-0000-0000000c0010", "00000000-0000-0000-0000-0000000c0020"
	)
	sq := func(x0, y0, x1, y1 float64) []ringMember {
		return []ringMember{{coords: [][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}, {x0, y0}}}}
	}
	msg := borderMessage(t,
		airspaceXML(p1, "P1", sq(0, 50, 1, 51)),
		airspaceXML(p2, "P2", sq(0.5, 50, 1.5, 51)), // overlaps P1
		airspaceXML(p3, "P3", sq(0.2, 50.2, 0.4, 50.4)),
		aggregateXML(agg, "AGG", [][2]string{{"BASE", p1}, {"UNION", p2}, {"SUBTR", p3}}),
		aggregateXML(top, "TOP", [][2]string{{"BASE", agg}, {"UNION", p3}}))
	var a *aixm.Airspace
	for _, x := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
		if x.TimeSlice[0].Designator.String() == "TOP" {
			a = x
		}
	}
	flat, err := geo.AirspaceComponents(a, msg)
	if err != nil {
		t.Fatal(err)
	}
	if mp, ok := flat[0].Geometry.(geo.MultiPolygon); !ok || len(mp.Coordinates) != 3 || flat[0].Contributor != nil {
		t.Fatalf("without NestedContributors: %T (%v)", flat[0].Geometry, flat[0].Contributor)
	}
	comps, err := geo.Options{NestedContributors: true}.AirspaceComponents(a, msg)
	if err != nil {
		t.Fatal(err)
	}
	if len(comps) != 2 || comps[0].Geometry != nil || len(comps[0].Contributor) != 3 {
		t.Fatalf("nested: %d components, first %T with %d contributor components", len(comps), comps[0].Geometry, len(comps[0].Contributor))
	}
	for i, op := range []string{"BASE", "UNION", "SUBTR"} {
		c := comps[0].Contributor[i]
		if c.Operation != op || c.Contributor != nil {
			t.Errorf("AGG component %d: %s, nested %v", i, c.Operation, c.Contributor != nil)
		}
		if _, ok := c.Geometry.(geo.Polygon); !ok {
			t.Errorf("AGG component %d: %T", i, c.Geometry)
		}
	}
	if _, ok := comps[1].Geometry.(geo.Polygon); !ok || comps[1].Operation != "UNION" {
		t.Errorf("plain contributor P3: %s %T", comps[1].Operation, comps[1].Geometry)
	}
}

// DONLON's aggregated airspaces convert with NestedContributors.
func TestNestedContributorsDonlon(t *testing.T) {
	msg, err := goaixm.ParseFile(context.Background(), donlonPath, goaixm.Options{})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
		flat, err1 := geo.AirspaceComponents(a, msg)
		nested, err2 := geo.Options{NestedContributors: true}.AirspaceComponents(a, msg)
		if (err1 == nil) != (err2 == nil) || len(flat) != len(nested) {
			t.Errorf("%s: %v / %v, %d / %d components", a.UUID(), err1, err2, len(flat), len(nested))
		}
		for _, c := range nested {
			if c.Contributor != nil {
				n++
			}
		}
	}
	t.Logf("DONLON: %d components with a nested aggregate", n)
}
