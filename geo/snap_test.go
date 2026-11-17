package geo_test

import (
	"math"
	"slices"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/geo"
	"github.com/davidkohl/goaixm/gml"
)

// The border junction snap moves an explicit vertex lying just beyond a
// referenced border (its own segment crosses the border) onto the border,
// reports it, and leaves vertices on the near side alone.
func TestBorderJunctionSnap(t *testing.T) {
	// The south border at lon −31.5 (position 5) is at 52 + 0.01·sin 5.
	yb := 52 + 0.01*math.Sin(5)
	beyond := [2]float64{-31.5, yb - 0.0001} // ≈ 11 m south: wrong side
	near := [2]float64{-31.5, yb + 0.0001}   // ≈ 11 m north: inside
	ring := func(end [2]float64) []ringMember {
		return []ringMember{{href: uuidSouth}, {coords: [][2]float64{{-30, 52}, {-31, 52.6}, end}}}
	}
	msg := borderMessage(t,
		airspaceXML("00000000-0000-0000-0000-0000000a0101", "BEYOND", ring(beyond)),
		airspaceXML("00000000-0000-0000-0000-0000000a0102", "NEAR", ring(near)))
	conv := func(designator string, o geo.Options) [][2]float64 {
		for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
			if a.TimeSlice[0].Designator.String() == designator {
				comps, err := o.AirspaceComponents(a, msg)
				if err != nil {
					t.Fatal(err)
				}
				return comps[0].Geometry.(geo.Polygon).Coordinates[0]
			}
		}
		t.Fatal(designator)
		return nil
	}

	if r := conv("BEYOND", geo.Options{}); !contains(r, beyond) {
		t.Fatal("without the snap the source vertex is served")
	}
	var reps []geo.Repair
	o := geo.Options{BorderSnapTolerance: 25, OnRepair: func(r geo.Repair) { reps = append(reps, r) }}
	r := conv("BEYOND", o)
	if contains(r, beyond) || len(reps) != 1 {
		t.Fatalf("snap: vertex still served (%v) or %d repairs", contains(r, beyond), len(reps))
	}
	rep := reps[0]
	if rep.Kind != geo.RepairBorderJunctionSnap || rep.Member != 1 || !contains([][2]float64{rep.From}, beyond) || !contains(r, rep.To) ||
		rep.DistanceM < 10 || rep.DistanceM > 12 || rep.Border != "urn:uuid:"+uuidSouth {
		t.Errorf("repair %+v", rep)
	}
	// The served vertex lies on the border segment between positions 5 and 6 (or 4 and 5).
	if math.Abs(rep.To[1]-yb) > 0.0001 {
		t.Errorf("snapped to %v, border at %.6f", rep.To, yb)
	}

	reps = nil
	if r := conv("NEAR", o); !contains(r, near) || len(reps) != 0 {
		t.Errorf("a vertex on the near side was snapped: %+v", reps)
	}
	reps = nil
	o.BorderSnapTolerance = 5
	if r := conv("BEYOND", o); !contains(r, beyond) || len(reps) != 0 {
		t.Errorf("snapped beyond the tolerance: %+v", reps)
	}
}

// With densification the snapped end piece is densified again, from the
// explicit vertex before it to the cut point: every served position of it
// lies on that geodesic.  Rings are numbered over the feature.
func TestBorderJunctionSnapDensified(t *testing.T) {
	yb := 52 + 0.01*math.Sin(5)
	beyond := [2]float64{-31.5, yb - 0.0001}
	src := [2]float64{-31, 52.6}
	msg := borderMessage(t, airspaceXML("00000000-0000-0000-0000-0000000a0103", "DENSE",
		[]ringMember{{href: uuidSouth}, {coords: [][2]float64{{-30, 52}, src, beyond}}}))
	var reps []geo.Repair
	o := geo.Options{ArcTolerance: 1, GeodesicTolerance: 1, BorderSnapTolerance: 25, OnRepair: func(r geo.Repair) { reps = append(reps, r) }}
	a := goaixm.FeaturesOf[*aixm.Airspace](msg)[0]
	comps, err := o.AirspaceComponents(a, msg)
	if err != nil {
		t.Fatal(err)
	}
	ring := comps[0].Geometry.(geo.Polygon).Coordinates[0]
	if len(reps) != 1 {
		t.Fatalf("%d repairs", len(reps))
	}
	to := reps[0].To
	// The served end piece is exactly the densified geodesic src → cut point.
	want, err := geo.Options{GeodesicTolerance: 1}.SegmentCoords(&gml.CurveSegments{GeodesicString: []gml.GeodesicString{{
		PosList: &gml.DirectPositionList{Value: posList([][2]float64{src, to})}}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("piece %d positions, ends %v %v, cut %v", len(want), want[0], want[len(want)-1], to)
	if len(want) < 5 {
		t.Fatalf("only %d positions on the piece", len(want))
	}
	has := func(r [][2]float64) bool {
		for k := 0; k+len(want) <= len(r); k++ {
			if slicesEqual(r[k:k+len(want)], want) {
				return true
			}
			rev := slices.Clone(r[k : k+len(want)])
			slices.Reverse(rev)
			if slicesEqual(rev, want) {
				return true
			}
		}
		return false
	}
	if !has(ring) {
		t.Errorf("the end piece is not the geodesic densified to the cut point")
	}
	if reps[0].Ring != 0 {
		t.Errorf("ring %d", reps[0].Ring)
	}

	// The legacy behaviour keeps the old densified positions.
	reps = nil
	o.LegacySnapAfterDensify = true
	comps, _ = o.AirspaceComponents(a, msg)
	if legacy := comps[0].Geometry.(geo.Polygon).Coordinates[0]; len(reps) != 1 || has(legacy) {
		t.Errorf("legacy snap: %d repairs, piece densified to the cut point: %v", len(reps), has(legacy))
	}
}

func slicesEqual(a, b [][2]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
