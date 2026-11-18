package geo_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"os"
	"runtime"
	"testing"

	"github.com/davidkohl/goaixm"
	"github.com/davidkohl/goaixm/aixm"
	"github.com/davidkohl/goaixm/geo"
	"github.com/davidkohl/goaixm/gml"
)

// goldenDigest is the digest of determinismDigest, computed on amd64
// (GOAMD64=v1, where the Go compiler never fuses a multiply and an add
// into one FMA instruction) before the FMA guard existed.  Every
// architecture must give it: a difference means some formula on the path
// is evaluated with a fused multiply-add (or another architecture-dependent
// rounding) and the converted positions differ in their last bits between
// platforms.  Update it only for a deliberate change of the algorithm (and
// then bump DensifyVersion).
const goldenDigest = "cf3b802401c264d23df552394faa1fef31dcc02a67d245bbb64a0a31a24ff156"

// determinismDigest hashes the bits of every position the conversion
// computes for a fixed workload: the DONLON airspaces in the zero and the
// densifying options, plus circles, arcs, geodesics and the Vincenty
// solutions over a grid of latitudes, radii and bearings.
func determinismDigest(t *testing.T) string {
	t.Helper()
	h := sha256.New()
	put := func(vs ...float64) {
		var b [8]byte
		for _, v := range vs {
			binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
			h.Write(b[:])
		}
	}
	line := func(c [][2]float64) {
		for _, p := range c {
			put(p[0], p[1])
		}
		put(math.NaN()) // separator
	}

	msg, err := goaixm.ParseFile(context.Background(), donlonPath, goaixm.Options{})
	if err != nil {
		t.Fatal(err)
	}
	dense := geo.Options{ArcTolerance: 1, GeodesicTolerance: 1, PreferSourceVertices: true,
		BorderSnapTolerance: 25, BorderCutPoints: true, NestedContributors: true}
	for _, o := range []geo.Options{{}, dense} {
		for _, a := range goaixm.FeaturesOf[*aixm.Airspace](msg) {
			comps, err := o.AirspaceComponents(a, msg)
			if err != nil {
				continue
			}
			digestComponents(comps, line)
		}
	}

	for lat := -80.0; lat <= 80; lat += 7.3 {
		for _, nm := range []float64{0.3, 1, 2.5, 5, 10, 25, 60} {
			segs := &gml.CurveSegments{CircleByCenterPoint: []gml.CircleByCenterPoint{{
				Pos: &gml.DirectPosition{Value: pos(lat, 8.6+lat/10)}, Radius: gml.Measure{Value: nm, UOM: "NM"}}}}
			for _, o := range []geo.Options{{}, {ArcTolerance: 1}, {ArcTolerance: 0.1}} {
				c, err := o.SegmentCoords(segs, nil)
				if err != nil {
					t.Fatal(err)
				}
				line(c)
			}
			arc := &gml.CurveSegments{ArcByCenterPoint: []gml.ArcByCenterPoint{{
				Pos: &gml.DirectPosition{Value: pos(lat, -3.2)}, Radius: gml.Measure{Value: nm, UOM: "NM"},
				StartAngle: &gml.Angle{Value: 17.5, UOM: "deg"}, EndAngle: &gml.Angle{Value: 243.25, UOM: "deg"}}}}
			c, err := geo.Options{ArcTolerance: 1, PreferSourceVertices: true}.SegmentCoords(arc, nil)
			if err != nil {
				t.Fatal(err)
			}
			line(c)
			three := &gml.CurveSegments{Arc: []gml.Arc{{PosList: &gml.DirectPositionList{
				Value: pos(lat, 1) + " " + pos(lat+0.1, 1.15) + " " + pos(lat, 1.3)}}}}
			if c, err = (geo.Options{ArcTolerance: 1}).SegmentCoords(three, nil); err != nil {
				t.Fatal(err)
			}
			line(c)
		}
		geod := &gml.CurveSegments{GeodesicString: []gml.GeodesicString{{PosList: &gml.DirectPositionList{
			Value: pos(lat, -20) + " " + pos(lat+5, 25) + " " + pos(lat-3, 60)}}}}
		for _, tol := range []float64{1, 0.05} {
			c, err := geo.Options{GeodesicTolerance: tol}.SegmentCoords(geod, nil)
			if err != nil {
				t.Fatal(err)
			}
			line(c)
		}
		for b := 0.0; b < 360; b += 13.7 {
			for _, d := range []float64{1, 1852, 92600, 1e6, 9e6} {
				la, lo := geo.DestinationPoint(lat, 11.1, b, d)
				put(la, lo, geo.GeodesicDistance(lat, 11.1, la, lo))
				az, _ := geo.GeodesicAzimuth(lat, 11.1, la, lo)
				put(az)
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func digestComponents(comps []geo.AirspaceComponent, line func([][2]float64)) {
	for _, c := range comps {
		switch g := c.Geometry.(type) {
		case geo.Polygon:
			for _, r := range g.Coordinates {
				line(r)
			}
		case geo.MultiPolygon:
			for _, p := range g.Coordinates {
				for _, r := range p {
					line(r)
				}
			}
		case geo.LineString:
			line(g.Coordinates)
		}
		digestComponents(c.Contributor, line)
	}
}

// TestDeterminismDigest: the conversion gives bit-identical positions on
// every architecture (no fused multiply-add on the computation path).
// Run it with GOARCH=amd64 and GOARCH=arm64 (an emulated architecture
// executes the test binary fine).
func TestDeterminismDigest(t *testing.T) {
	got := determinismDigest(t)
	if os.Getenv("GOAIXM_PRINT_DIGEST") != "" {
		t.Logf("%s/%s digest %s", runtime.GOOS, runtime.GOARCH, got)
	}
	if got != goldenDigest {
		t.Errorf("%s: determinism digest %s, want %s", runtime.GOARCH, got, goldenDigest)
	}
}
