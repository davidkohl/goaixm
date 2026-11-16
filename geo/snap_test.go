package geo_test

import (
	"math"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/geo"
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
